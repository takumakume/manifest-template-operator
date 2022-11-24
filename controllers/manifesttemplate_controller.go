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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
//+kubebuilder:rbac:groups="*",resources="*",verbs=get;list;watch;create;update;patch;delete

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

	apiVersion := "v1"
	var group string
	var version string
	s := strings.Split(apiVersion, "/")
	if len(s) != 1 {
		group = s[0]
	}
	version = s[len(s)-1]

	kind := "Service"
	name := "test1"
	namespace := manifestTemplate.ObjectMeta.Namespace
	labels := map[string]string{
		"label1": "label1value",
	}
	annotations := map[string]string{
		"anno1": "anno1value",
	}
	spec := map[string]interface{}{
		"ports": []map[string]interface{}{
			{
				"port": 80,
			},
		},
		"selector": map[string]interface{}{
			"app": "test1",
		},
	}
	ownerRef := metav1.NewControllerRef(
		&manifestTemplate.ObjectMeta,
		schema.GroupVersionKind{
			Group:   manifesttemplatev1alpha1.GroupVersion.Group,
			Version: manifesttemplatev1alpha1.GroupVersion.Version,
			Kind:    "ManifestTemplate",
		})
	ownerRef.Name = manifestTemplate.Name
	ownerRef.UID = manifestTemplate.GetUID()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":        name,
				"namespace":   namespace,
				"labels":      labels,
				"annotations": annotations,
				"ownerReferences": []metav1.OwnerReference{
					*ownerRef,
				},
			},
			"spec": spec,
		},
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    kind,
		Version: version,
	})

	exists := &unstructured.Unstructured{}
	exists.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    kind,
		Version: version,
	})
	err := r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, exists)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create
			log.Info(fmt.Sprintf("======== try create: %+v", u))
			if err := r.Create(ctx, u); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("======== created")
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// update
		log.Info(fmt.Sprintf("======== exists: %+v", exists))
		log.Info(fmt.Sprintf("======== try update: %+v", u))
		if err := r.Update(ctx, u); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("======== updated")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManifestTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&manifesttemplatev1alpha1.ManifestTemplate{}).
		Complete(r)
}
