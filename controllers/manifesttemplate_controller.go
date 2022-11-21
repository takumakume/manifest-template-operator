/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	serializeryaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	manifesttemplatev1alpha1 "github.com/takumakume/manifest-template-operator/api/v1alpha1"
)

// ManifestTemplateReconciler reconciles a ManifestTemplate object
type ManifestTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=manifest-template.takumakume.github.io,resources=manifesttemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=manifest-template.takumakume.github.io,resources=manifesttemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=manifest-template.takumakume.github.io,resources=manifesttemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ManifestTemplate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *ManifestTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log := log.FromContext(ctx).WithValues("ManifestTemplate", req.NamespacedName.String())

	manifestTemplate := &manifesttemplatev1alpha1.ManifestTemplate{}
	if err := r.Get(ctx, req.NamespacedName, manifestTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "unable to fetch ManifestTemplate")
		return ctrl.Result{}, err
	}

	log.Info("starting reconcile loop")
	defer log.Info("finish reconcile loop")

	if !manifestTemplate.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return ctrl.Result{}, err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	discoveryClient := clientSet.Discovery()

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO
	manifest := `---
kind: Service
metadata:
  name: test1
  namespace: test
spec:
  ports:
  - port: 80
  selector:
    app: test1
---
kind: Service
metadata:
  name: test2
  namespace: test
spec:
  ports:
  - port: 80
  selector:
    app: test2
`

	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 100)
	for {
		//var rawObj runtime.RawExtension
		var rawObj []byte
		if err := decoder.Decode(&rawObj); err != nil {
			fmt.Println("end of contents")
			break
		}

		obj := &unstructured.Unstructured{}

		// Create client (client, err := dynamic.NewClient(rawObj.Raw, obj))
		dec := serializeryaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, gvk, err := dec.Decode(rawObj, nil, obj)
		if err != nil {
			return ctrl.Result{}, err
		}

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return ctrl.Result{}, err
		}

		var client dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if obj.GetNamespace() == "" {
				// TODO: set namespace
				obj.SetNamespace(metav1.NamespaceDefault)
			}
			client = dynamicClient.Resource(mapping.Resource).Namespace(obj.GetNamespace())
		} else {
			client = dynamicClient.Resource(mapping.Resource)
		}

		// Apply (res, err := dynamic.Apply(client, obj))
		data, err := json.Marshal(obj)
		if err != nil {
			return ctrl.Result{}, err
		}

		opt := metav1.PatchOptions{FieldManager: "example"}
		o, err := client.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, opt)
		if err != nil {
			return ctrl.Result{}, err
		}
		fmt.Printf("applied: %+v\n", o)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&manifesttemplatev1alpha1.ManifestTemplate{}).
		Complete(r)
}
