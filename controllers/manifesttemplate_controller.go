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
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"

	"gopkg.in/yaml.v3"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0kubun/pp"
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
	log.Info(pp.Sprint(manifestTemplate))

	desired, err := desireUnstructured(manifestTemplate)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info(pp.Sprint(desired))

	group, version := getGroupVersion(manifestTemplate.Spec.APIVersion)

	exists := &unstructured.Unstructured{}
	exists.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    manifestTemplate.Spec.Kind,
		Version: version,
	})
	objKey := client.ObjectKey{
		Namespace: desired.GetNamespace(),
		Name:      desired.GetName(),
	}
	if err := r.Get(ctx, objKey, exists); err != nil {
		if apierrors.IsNotFound(err) {
			// create
			log.Info(fmt.Sprintf("======== try create: %+v", pp.Sprint(desired)))
			if err := r.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("======== created")
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// update
		log.Info(fmt.Sprintf("======== exists: %+v", pp.Sprint(exists)))
		log.Info(fmt.Sprintf("======== try update: %+v", pp.Sprint(desired)))
		if err := r.Update(ctx, desired); err != nil {
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

func desireUnstructured(manifestTemplate *manifesttemplatev1alpha1.ManifestTemplate) (*unstructured.Unstructured, error) {
	ownerRef := metav1.NewControllerRef(
		&manifestTemplate.ObjectMeta,
		schema.GroupVersionKind{
			Group:   manifesttemplatev1alpha1.GroupVersion.Group,
			Version: manifesttemplatev1alpha1.GroupVersion.Version,
			Kind:    "ManifestTemplate",
		})
	ownerRef.Name = manifestTemplate.Name
	ownerRef.UID = manifestTemplate.GetUID()

	u := &unstructured.Unstructured{}

	render := newRender(manifestTemplate)

	name, err := render.render(manifestTemplate.Spec.Metadata.Name)
	if err != nil {
		return nil, err
	}

	namespace, err := render.render(manifestTemplate.Spec.Metadata.Namespace)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{}
	if len(manifestTemplate.Spec.Metadata.Labels) > 0 {
		for k, v := range manifestTemplate.Spec.Metadata.Labels {
			if o, err := render.render(v); err == nil {
				labels[k] = o
			} else {
				return nil, err
			}
		}
	}

	annotations := map[string]string{}
	if len(manifestTemplate.Spec.Metadata.Annotations) > 0 {
		for k, v := range manifestTemplate.Spec.Metadata.Annotations {
			if o, err := render.render(v); err == nil {
				annotations[k] = o
			} else {
				return nil, err
			}
		}
	}

	raw, err := yaml.Marshal(manifestTemplate.Spec.Spec.Object)
	if err != nil {
		return nil, err
	}
	renderd, err := render.render(string(raw))
	if err != nil {
		return nil, err
	}
	spec := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(renderd), &spec); err != nil {
		return nil, err
	}

	u.Object = map[string]interface{}{
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
	}

	group, version := getGroupVersion(manifestTemplate.Spec.APIVersion)

	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    manifestTemplate.Spec.Kind,
		Group:   group,
		Version: version,
	})

	return u, nil
}

func getGroupVersion(apiVersion string) (string, string) {
	group := ""
	version := ""
	s := strings.Split(apiVersion, "/")
	if len(s) != 1 {
		group = s[0]
	}
	version = s[len(s)-1]

	return group, version
}

type Render struct {
	data map[string]interface{}
}

func newRender(manifestTemplate *manifesttemplatev1alpha1.ManifestTemplate) *Render {
	return &Render{
		data: map[string]interface{}{
			"Self": manifestTemplate,
		},
	}
}

func (r *Render) render(tmpl string) (string, error) {
	tpl, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, r.data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
