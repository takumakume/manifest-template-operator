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

package v1alpha1

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Unstructured values for rendering Helm Charts
// +k8s:deepcopy-gen=false
type Spec struct {
	// Object is a JSON compatible map with string, float, int, bool, []interface{}, or
	// map[string]interface{} children.
	Object map[string]interface{} `json:"-"`
}

// MarshalJSON ensures that the unstructured object produces proper
// JSON when passed to Go's standard JSON library.
func (u *Spec) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.Object)
}

// UnmarshalJSON ensures that the unstructured object properly decodes
// JSON when passed to Go's standard JSON library.
func (u *Spec) UnmarshalJSON(data []byte) error {
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	u.Object = m

	return nil
}

// Declaring this here prevents it from being generated.
func (u *Spec) DeepCopyInto(out *Spec) {
	out.Object = runtime.DeepCopyJSON(u.Object)
}

// ManifestTemplateSpec defines the desired state of ManifestTemplate
type ManifestTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Kind
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// APIVersion
	// +kubebuilder:validation:Required
	APIVersion string `json:"apiVersion"`

	// Metadata
	// +kubebuilder:validation:Required
	Metadata ManifestTemplateSpecMeta `json:"Metadata"`

	// Spec
	// +kubebuilder:validation:XPreserveUnknownFields
	// +kubebuilder:validation:Required
	Spec Spec `json:"Spec"`
}

type ManifestTemplateSpecMeta struct {
	// Name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ManifestTemplateStatus defines the observed state of ManifestTemplate
type ManifestTemplateStatus struct {
	// Ready is the manifest application state
	Ready corev1.ConditionStatus `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ManifestTemplate is the Schema for the manifesttemplates API
type ManifestTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManifestTemplateSpec   `json:"spec,omitempty"`
	Status ManifestTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManifestTemplateList contains a list of ManifestTemplate
type ManifestTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManifestTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManifestTemplate{}, &ManifestTemplateList{})
}
