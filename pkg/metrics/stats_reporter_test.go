/*
Copyright 2021 The Kubernetes Authors.

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

package metrics

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/scheme"
)

func Test_secretProviderClassReporter(t *testing.T) {
	tests := []struct {
		name                  string
		secretProviderClasses []*v1alpha1.SecretProviderClass
		want                  string
	}{
		{
			name: "single secretproviderclass",
			secretProviderClasses: []*v1alpha1.SecretProviderClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							"app": "test-app",
						},
						CreationTimestamp: metav1.Date(2021, time.Month(7), 12, 10, 30, 40, 0, time.UTC),
					},
					Spec: v1alpha1.SecretProviderClassSpec{
						Provider: "provider1",
						SecretObjects: []*v1alpha1.SecretObject{
							{
								SecretName: "secret1",
								Type:       "Opaque",
							},
						},
					},
				},
			},
			want: `# HELP kube_secretproviderclass_created Unix creation timestamp
# TYPE kube_secretproviderclass_created gauge
kube_secretproviderclass_created{namespace="test",os_type="linux",secretproviderclass="test"} 1.62608584e+09
# HELP kube_secretproviderclass_info Information about SecretProviderClass
# TYPE kube_secretproviderclass_info gauge
kube_secretproviderclass_info{namespace="test",os_type="linux",secretproviderclass="test"} 1
# HELP kube_secretproviderclass_labels Kubernetes labels converted to OpenTelemetry labels
# TYPE kube_secretproviderclass_labels gauge
kube_secretproviderclass_labels{label_app="test-app",namespace="test",os_type="linux",secretproviderclass="test"} 1
# HELP kube_secretproviderclass_type Type about SecretProviderClass
# TYPE kube_secretproviderclass_type gauge
kube_secretproviderclass_type{namespace="test",os_type="linux",provider="provider1",secret_name="secret1",secret_type_secret1="Opaque",secretproviderclass="test"} 1
`,
		},
		{
			name: "multiple secretproviderclasses",
			secretProviderClasses: []*v1alpha1.SecretProviderClass{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test1",
						Namespace: "test",
					},
					Spec: v1alpha1.SecretProviderClassSpec{
						Provider: "vault",
						SecretObjects: []*v1alpha1.SecretObject{
							{
								SecretName: "secret1",
								Type:       "Opaque",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test2",
						Namespace: "test",
					},
					Spec: v1alpha1.SecretProviderClassSpec{
						Provider: "azure",
						SecretObjects: []*v1alpha1.SecretObject{
							{
								SecretName: "secret2",
								Type:       "Opaque",
							},
						},
					},
				},
			},
			want: `# HELP kube_secretproviderclass_created Unix creation timestamp
# TYPE kube_secretproviderclass_created gauge
kube_secretproviderclass_created{namespace="test",os_type="linux",secretproviderclass="test1"} 0
kube_secretproviderclass_created{namespace="test",os_type="linux",secretproviderclass="test2"} 0
# HELP kube_secretproviderclass_info Information about SecretProviderClass
# TYPE kube_secretproviderclass_info gauge
kube_secretproviderclass_info{namespace="test",os_type="linux",secretproviderclass="test1"} 1
kube_secretproviderclass_info{namespace="test",os_type="linux",secretproviderclass="test2"} 1
# HELP kube_secretproviderclass_labels Kubernetes labels converted to OpenTelemetry labels
# TYPE kube_secretproviderclass_labels gauge
kube_secretproviderclass_labels{namespace="test",os_type="linux",secretproviderclass="test1"} 1
kube_secretproviderclass_labels{namespace="test",os_type="linux",secretproviderclass="test2"} 1
# HELP kube_secretproviderclass_type Type about SecretProviderClass
# TYPE kube_secretproviderclass_type gauge
kube_secretproviderclass_type{namespace="test",os_type="linux",provider="azure",secret_name="secret2",secret_type_secret2="Opaque",secretproviderclass="test2"} 1
kube_secretproviderclass_type{namespace="test",os_type="linux",provider="vault",secret_name="secret1",secret_type_secret1="Opaque",secretproviderclass="test1"} 1
`,
		},
	}

	s := scheme.Scheme
	s.AddKnownTypes(schema.GroupVersion{Group: v1alpha1.GroupVersion.Group, Version: v1alpha1.GroupVersion.Version},
		&v1alpha1.SecretProviderClass{},
		&v1alpha1.SecretProviderClassList{},
	)
	ctx := context.Background()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := fake.NewFakeClientWithScheme(s)
			for _, spc := range test.secretProviderClasses {
				c.Create(ctx, spc, &client.CreateOptions{})
			}

			cont, exp, err := newPipeline(prometheus.Config{}, controller.WithResource(resource.Empty()))
			if err != nil {
				t.Fatal(err)
			}
			meter := exp.MeterProvider().Meter("test")
			r := &secretProviderClassReporter{Client: c}
			r.batchObserverInit(meter)
			cont.Collect(ctx)

			if diff := compareExport(exp, test.want); diff != "" {
				t.Errorf("secretProviderClassReporter() result mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func compareExport(exporter *prometheus.Exporter, want string) string {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	exporter.ServeHTTP(rec, req)
	return cmp.Diff(want, rec.Body.String())
}
