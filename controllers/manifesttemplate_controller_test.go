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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	manifesttemplatev1alpha1 "github.com/takumakume/manifest-template-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		err = k8sClient.DeleteAllOf(ctx, &corev1.Event{}, client.InNamespace("test"))
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

		events := &corev1.EventList{}
		Eventually(func() error {
			k8sClient.List(ctx, events, &client.ListOptions{
				Namespace: manifestTemplate.GetNamespace(),
			})
			if len(events.Items) != 1 {
				return fmt.Errorf("no events have been added")
			}
			return nil
		}, 5, 1).Should(Succeed())

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

		new1ManifestTemplate := manifestTemplate.DeepCopy()
		new1ManifestTemplate.Spec.Spec = manifesttemplatev1alpha1.Spec{
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
		Expect(k8sClient.Patch(ctx, new1ManifestTemplate, client.MergeFrom(manifestTemplate))).Should(Succeed())

		updated1 := &corev1.Service{}
		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "test1"}, updated1)
			if err != nil {
				return err
			}

			if !(len(updated1.Spec.Ports) == 2 && updated1.Spec.Ports[1].Port == int32(443)) {
				return fmt.Errorf("object is not updated = %+v", updated1)
			}

			return nil
		}, 5, 1).Should(Succeed())

		events = &corev1.EventList{}
		Eventually(func() error {
			k8sClient.List(ctx, events, &client.ListOptions{
				Namespace: manifestTemplate.GetNamespace(),
			})
			if len(events.Items) != 2 {
				return fmt.Errorf("no events have been added")
			}
			return nil
		}, 5, 1).Should(Succeed())

		new2ManifestTemplate := new1ManifestTemplate.DeepCopy()
		new2ManifestTemplate.Spec.Spec = manifesttemplatev1alpha1.Spec{
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
					{
						"name": "healthz",
						"port": 8081,
					},
				},
				"selector": map[string]interface{}{
					"app": "test1",
					"ns":  "{{ .Self.ObjectMeta.Namespace }}",
				},
			},
		}
		Expect(k8sClient.Patch(ctx, new2ManifestTemplate, client.MergeFrom(new1ManifestTemplate))).Should(Succeed())

		updated2 := &corev1.Service{}
		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "test1"}, updated2)
			if err != nil {
				return err
			}

			if !(len(updated2.Spec.Ports) == 3 && updated2.Spec.Ports[2].Port == int32(8081)) {
				return fmt.Errorf("object is not updated = %+v", updated2)
			}

			return nil
		}, 5, 1).Should(Succeed())

		events = &corev1.EventList{}
		Eventually(func() error {
			k8sClient.List(ctx, events, &client.ListOptions{
				Namespace: manifestTemplate.GetNamespace(),
			})
			if len(events.Items) != 3 {
				return fmt.Errorf("no events have been added")
			}
			return nil
		}, 5, 1).Should(Succeed())
	})

	It("resource already exists", func() {
		currentSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exists-svc",
				Namespace: "test",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "http",
						Port: 80,
					},
				},
				Selector: map[string]string{
					"app": "test",
				},
			},
		}
		Expect(k8sClient.Create(ctx, currentSvc)).Should(Succeed())
		manifestTemplate := &manifesttemplatev1alpha1.ManifestTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exists",
				Namespace: "test",
			},
			Spec: manifesttemplatev1alpha1.ManifestTemplateSpec{
				Kind:       "Service",
				APIVersion: "v1",
				Metadata: manifesttemplatev1alpha1.ManifestTemplateSpecMeta{
					Name:      "exists-svc",
					Namespace: "test",
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
							"app":        "test",
							"additional": "value",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, manifestTemplate)).Should(Succeed())

		Eventually(func() error {
			o := &manifesttemplatev1alpha1.ManifestTemplate{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "exists"}, o); err != nil {
				return err
			}
			if o.Status.Ready != corev1.ConditionFalse {
				return fmt.Errorf("invalid .Status.Ready = %v", o.Status.Ready)
			}
			return nil
		}, 5, 1).Should(Succeed())

		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "exists-svc"}, svc)).Should(Succeed())
		Expect(svc.Spec.Selector["additional"]).Should(Equal(""))
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

func Test_desiredYAML(t *testing.T) {
	type args struct {
		manifestTemplate *manifesttemplatev1alpha1.ManifestTemplate
	}
	tests := []struct {
		name    string
		args    args
		want    string
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
			want: `apiVersion: v1
kind: Service
metadata:
  annotations:
    annotation1: annotation1value
    ns: 'test'
  labels:
    label1: label1value
    ns: 'test'
  name: 'test1'
  namespace: 'test'
  ownerReferences:
  - apiVersion: manifest-template.takumakume.github.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: ManifestTemplate
    name: sample
    uid: ""
spec:
  ports:
  - name: http
    port: 80
  selector:
    app: test1
    ns: 'test'
`,
		},
		{
			name: "default namespace",
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
							Name: "test1",
							// add default namespace
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
								},
							},
						},
					},
				},
			},
			want: `apiVersion: v1
kind: Service
metadata:
  name: test1
  namespace: test
  ownerReferences:
  - apiVersion: manifest-template.takumakume.github.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: ManifestTemplate
    name: sample
    uid: ""
spec:
  ports:
  - name: http
    port: 80
  selector:
    app: test1
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := desiredYAML(tt.args.manifestTemplate)
			if (err != nil) != tt.wantErr {
				t.Errorf("desiredYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("desiredYAML() = %s, want %s", got, tt.want)
			}
		})
	}
}
