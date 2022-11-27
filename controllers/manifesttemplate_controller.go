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
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

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

	if !manifestTemplate.GetDeletionTimestamp().IsZero() {
		log.Info("skip reconcile loop")
		return ctrl.Result{}, nil
	}

	log.Info("starting reconcile loop")
	defer log.Info("finish reconcile loop")

	desired, err := desireUnstructured(manifestTemplate)
	if err != nil {
		log.Error(err, "failed to render object")
		return ctrl.Result{}, err
	}

	group, version := getGroupVersion(manifestTemplate.Spec.APIVersion)
	exists := &unstructured.Unstructured{}
	exists.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    manifestTemplate.Spec.Kind,
		Version: version,
	})

	ownerRef := metav1.NewControllerRef(
		&manifestTemplate.ObjectMeta,
		schema.GroupVersionKind{
			Group:   manifesttemplatev1alpha1.GroupVersion.Group,
			Version: manifesttemplatev1alpha1.GroupVersion.Version,
			Kind:    "ManifestTemplate",
		})
	ownerRef.Name = manifestTemplate.Name
	ownerRef.UID = manifestTemplate.GetUID()
	exists.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})

	objKey := client.ObjectKey{
		Namespace: desired.GetNamespace(),
		Name:      desired.GetName(),
	}
	if err := r.Get(ctx, objKey, exists); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(fmt.Sprintf("create resource = %s", pp.Sprint(desired)))
			if err := r.Create(ctx, desired); err != nil {
				log.Error(err, "failed to create resource")
				return ctrl.Result{}, err
			}

		} else {
			return ctrl.Result{}, err
		}
	} else {
		if manifestTemplate.Status.LastAppliedConfigration != "" {
			lastAppliedConfigration := &unstructured.Unstructured{}
			if err := yaml.Unmarshal([]byte(manifestTemplate.Status.LastAppliedConfigration), lastAppliedConfigration); err != nil {
				return ctrl.Result{}, err
			}

			// TODO: Change metadata.Name or Namespace -> recreate

			if reflect.DeepEqual(lastAppliedConfigration, desired) {
				log.Info("resource up to date")
				return ctrl.Result{}, nil
			}
		}
		log.Info(fmt.Sprintf("update resource = desired %s", pp.Sprint(desired)))

		if err := r.Update(ctx, desired); err != nil {
			log.Error(err, "failed to update resource")
			return ctrl.Result{}, err
		}
	}

	mt := manifestTemplate.DeepCopy()
	mt.Status.Ready = v1.ConditionTrue
	lastDesired, err := desireUnstructured(manifestTemplate)
	if err != nil {
		log.Error(err, "failed to render object")
		return ctrl.Result{}, err
	}
	raw, err := yaml.Marshal(lastDesired)
	if err != nil {
		return ctrl.Result{}, err
	}
	mt.Status.LastAppliedConfigration = string(raw)
	if err := r.Status().Patch(ctx, mt, client.MergeFrom(manifestTemplate)); err != nil {
		log.Error(err, "failed to patch ManifestTemplate status")
		return ctrl.Result{}, err
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
		},
		"spec": spec,
	}

	group, version := getGroupVersion(manifestTemplate.Spec.APIVersion)

	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    manifestTemplate.Spec.Kind,
		Group:   group,
		Version: version,
	})

	// WORKAROUND: To make yaml once because the order of map[string]interface is changed when comparing with lastAppliedConfigration.
	rawNew, err := yaml.Marshal(u)
	if err != nil {
		return nil, err
	}
	new := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(rawNew), &new); err != nil {
		return nil, err
	}

	return new, nil
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
