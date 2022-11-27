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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0kubun/pp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	manifesttemplatev1alpha1 "github.com/takumakume/manifest-template-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var True bool = true

var _ = Describe("ManifestTemplate controller", func() {
	BeforeEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &manifesttemplatev1alpha1.ManifestTemplate{}, client.InNamespace("test"))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Service{}, client.InNamespace("test"))
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(100 * time.Millisecond)
	})

	It("create and update", func() {
		manifestTemplate := &manifesttemplatev1alpha1.ManifestTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sample",
				Namespace: "test",
			},
			Spec: manifesttemplatev1alpha1.ManifestTemplateSpec{
				Kind:       "Service",
				APIVersion: "v1",
				Metadata: manifesttemplatev1alpha1.ManifestTemplateSpecMeta{
					Name:      "{{ .Self.ObjectMeta.Namespace }}1",
					Namespace: "{{ .Self.ObjectMeta.Namespace }}",
					Labels: map[string]string{
						"label1": "label1value",
					},
					Annotations: map[string]string{
						"annotation1": "annotation1value",
					},
				},
				Spec: manifesttemplatev1alpha1.Spec{
					Object: map[string]interface{}{
						"ports": []map[string]interface{}{
							{
								"name": "http",
								"port": 80,
							},
						},
						"selector": map[string]interface{}{
							"app": "test1",
							"ns":  "{{ .Self.ObjectMeta.Namespace }}",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, manifestTemplate)).Should(Succeed())

		generated := &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "test1"}, generated)
		}, 5, 1).Should(Succeed())
		Expect(generated.ObjectMeta.Namespace).Should(Equal("test"))
		Expect(generated.ObjectMeta.Labels).Should(Equal(map[string]string{"label1": "label1value"}))
		Expect(generated.ObjectMeta.Annotations).Should(Equal(map[string]string{"annotation1": "annotation1value"}))
		Expect(generated.Spec.Ports[0].Port).Should(Equal(int32(80)))
		Expect(generated.Spec.Selector["app"]).Should(Equal("test1"))
		Expect(generated.Spec.Selector["ns"]).Should(Equal("test"))

		manifestTemplate.Spec.Spec = manifesttemplatev1alpha1.Spec{
			Object: map[string]interface{}{
				"ports": []map[string]interface{}{
					{
						"name": "http",
						"port": 80,
					},
					{
						"name": "https",
						"port": 443,
					},
				},
				"selector": map[string]interface{}{
					"app": "test1",
					"ns":  "{{ .Self.ObjectMeta.Namespace }}",
				},
			},
		}
		Expect(k8sClient.Update(ctx, manifestTemplate)).Should(Succeed())

		Eventually(func() error {
			o := &corev1.Service{}
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "test1"}, o)
			if err != nil {
				return err
			}

			if !(len(o.Spec.Ports) == 2 && o.Spec.Ports[1].Port == int32(443)) {
				return fmt.Errorf("object is not updated = %+v", o)
			}

			return nil
		}, 5, 1).Should(Succeed())
	})

	It("manifest valid", func() {
		rawTestData, errReading := os.ReadFile(filepath.Join("testdata", "valid.yaml"))
		if errReading != nil {
			Fail("Failed to read valid test file")
		}
		manifestTemplate := &manifesttemplatev1alpha1.ManifestTemplate{}

		d := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(rawTestData)), len(rawTestData))
		errDecording := d.Decode(manifestTemplate)
		if errDecording != nil {
			Fail("Failed to decode test data")
		}
		Expect(k8sClient.Create(ctx, manifestTemplate)).Should(Succeed())

		generated := &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "valid-svc"}, generated)
		}, 5, 1).Should(Succeed())
		Expect(generated.ObjectMeta.Namespace).Should(Equal("test"))
		Expect(generated.ObjectMeta.Labels).Should(Equal(map[string]string{"label1": "label1value"}))
		Expect(generated.ObjectMeta.Annotations).Should(Equal(map[string]string{"annotation1": "annotation1value"}))
		Expect(generated.Spec.Ports[0].Port).Should(Equal(int32(80)))
		Expect(generated.Spec.Selector["app"]).Should(Equal("test1"))
		Expect(generated.Spec.Selector["ns"]).Should(Equal("test"))
	})
})

func Test_desireUnstructured(t *testing.T) {
	type args struct {
		manifestTemplate *manifesttemplatev1alpha1.ManifestTemplate
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "default",
			args: args{
				manifestTemplate: &manifesttemplatev1alpha1.ManifestTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample",
						Namespace: "test",
					},
					Spec: manifesttemplatev1alpha1.ManifestTemplateSpec{
						Kind:       "Service",
						APIVersion: "v1",
						Metadata: manifesttemplatev1alpha1.ManifestTemplateSpecMeta{
							Name:      "{{ .Self.ObjectMeta.Namespace }}1",
							Namespace: "{{ .Self.ObjectMeta.Namespace }}",
							Labels: map[string]string{
								"label1": "label1value",
								"ns":     "{{ .Self.ObjectMeta.Namespace }}",
							},
							Annotations: map[string]string{
								"annotation1": "annotation1value",
								"ns":          "{{ .Self.ObjectMeta.Namespace }}",
							},
						},
						Spec: manifesttemplatev1alpha1.Spec{
							Object: map[string]interface{}{
								"ports": []map[string]interface{}{
									{
										"name": "http",
										"port": 80,
									},
								},
								"selector": map[string]interface{}{
									"app": "test1",
									"ns":  "{{ .Self.ObjectMeta.Namespace }}",
								},
							},
						},
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"ownerReferences": []metav1.OwnerReference{
							{
								APIVersion:         "manifest-template.takumakume.github.io/v1alpha1",
								Kind:               "ManifestTemplate",
								Name:               "sample",
								UID:                "",
								Controller:         &True,
								BlockOwnerDeletion: &True,
							},
						},
						"name":      "test1",
						"namespace": "test",
						"labels": map[string]string{
							"label1": "label1value",
							"ns":     "test",
						},
						"annotations": map[string]string{
							"annotation1": "annotation1value",
							"ns":          "test",
						},
					},
					"spec": map[string]interface{}{
						"ports": []interface{}{
							map[string]interface{}{
								"name": "http",
								"port": 80,
							},
						},
						"selector": map[string]interface{}{
							"app": "test1",
							"ns":  "test",
						},
					},
					"apiVersion": "v1",
					"kind":       "Service",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := desireUnstructured(tt.args.manifestTemplate)
			if (err != nil) != tt.wantErr {
				t.Errorf("desireUnstructured() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("desireUnstructured() = %v, want %v", pp.Sprint(got), pp.Sprint(tt.want))
			}
		})
	}
}
