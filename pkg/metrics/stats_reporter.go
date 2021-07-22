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
	"runtime"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

const (
	secretProviderClassKey    = "secretproviderclass"
	namespaceKey              = "namespace"
	osTypeKey                 = "os_type"
	secretObjectName          = "secret_name"
	secretObjectTypeKeyPrefix = "secret_type_"
	providerKey               = "provider"
	labelKeyPrefix            = "label_"

	metricInfoName    = "kube_secretproviderclass_info"
	metricInfoDesc    = "Information about SecretProviderClass"
	metricTypeName    = "kube_secretproviderclass_type"
	metricTypeDesc    = "Type about SecretProviderClass"
	metricLabelsName  = "kube_secretproviderclass_labels"
	metricLabelsDesc  = "Kubernetes labels converted to OpenTelemetry labels"
	metricCreatedName = "kube_secretproviderclass_created"
	metricCreatedDesc = "Unix creation timestamp"
)

var runtimeOS = runtime.GOOS

type secretProviderClassReporter struct {
	client.Client
	metricInfo    metric.Int64ValueObserver
	metricType    metric.Int64ValueObserver
	metricLabel   metric.Int64ValueObserver
	metricCreated metric.Float64ValueObserver
	// lock prevents a race between batch observer and instrument registration.
	lock sync.Mutex
}

// NewSecretProviderClassReporter a reporter with given kubernetes client
func NewSecretProviderClassReporter(client client.Client) manager.Runnable {
	return &secretProviderClassReporter{
		Client: client,
	}
}

func (r *secretProviderClassReporter) Start(ctx context.Context) error {
	meter := global.Meter("sigs.k8s.io/secrets-store-csi-driver")
	r.batchObserverInit(meter)
	return nil
}
func (r *secretProviderClassReporter) batchObserverInit(meter metric.Meter) {
	obs := metric.Must(meter).NewBatchObserver(r.batchFunc())
	r.metricInfo = obs.NewInt64ValueObserver(metricInfoName, metric.WithDescription(metricInfoDesc))
	r.metricType = obs.NewInt64ValueObserver(metricTypeName, metric.WithDescription(metricTypeDesc))
	r.metricLabel = obs.NewInt64ValueObserver(metricLabelsName, metric.WithDescription(metricLabelsDesc))
	r.metricCreated = obs.NewFloat64ValueObserver(metricCreatedName, metric.WithDescription(metricCreatedDesc))
}

func (r *secretProviderClassReporter) batchFunc() metric.BatchObserverFunc {
	return func(ctx context.Context, result metric.BatchObserverResult) {
		r.lock.Lock()
		defer r.lock.Unlock()

		spcList := &v1alpha1.SecretProviderClassList{}
		err := r.List(ctx, spcList, &client.ListOptions{})
		if err != nil {
			klog.ErrorS(err, "failed to list secret provider class")
			return
		}

		for _, spc := range spcList.Items {
			commonAttr := []attribute.KeyValue{
				attribute.String(secretProviderClassKey, spc.Name),
				attribute.String(namespaceKey, spc.Namespace),
				attribute.String(osTypeKey, runtimeOS),
			}
			infoAttr := commonAttr
			result.Observe(infoAttr, r.metricInfo.Observation(1))

			typeAttr := append(commonAttr, attribute.String(providerKey, string(spc.Spec.Provider)))
			for _, secretObj := range spc.Spec.SecretObjects {
				typeAttr = append(typeAttr, attribute.String(secretObjectName, secretObj.SecretName))
				typeAttr = append(typeAttr, attribute.String(secretObjectTypeKeyPrefix+secretObj.SecretName, secretObj.Type))
			}
			result.Observe(typeAttr, r.metricType.Observation(1))

			labelAttr := commonAttr
			for k, v := range spc.Labels {
				labelAttr = append(labelAttr, attribute.String(labelKeyPrefix+k, v))
			}
			result.Observe(labelAttr, r.metricLabel.Observation(1))

			createdAttr := commonAttr
			created := float64(0)
			if !spc.CreationTimestamp.IsZero() {
				created = float64(spc.CreationTimestamp.Unix())
			}
			result.Observe(createdAttr, r.metricCreated.Observation(created))
		}

	}
}
