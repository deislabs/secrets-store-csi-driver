/*
Copyright 2020 The Kubernetes Authors.
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

package secretsstore

import (
	"context"
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

var (
	providerKey             = "provider"
	errorKey                = "error_type"
	osTypeKey               = "os_type"
	nodePublishTotal        metric.Int64Counter
	nodeUnPublishTotal      metric.Int64Counter
	nodePublishErrorTotal   metric.Int64Counter
	nodeUnPublishErrorTotal metric.Int64Counter
	syncK8sSecretTotal      metric.Int64Counter
	syncK8sSecretDuration   metric.Float64ValueRecorder
	runtimeOS               = runtime.GOOS
)

type reporter struct {
	meter metric.Meter
}

type StatsReporter interface {
	ReportNodePublishCtMetric(provider string)
	ReportNodeUnPublishCtMetric()
	ReportNodePublishErrorCtMetric(provider, errType string)
	ReportNodeUnPublishErrorCtMetric()
	ReportSyncK8SecretCtMetric(provider string, count int)
	ReportSyncK8SecretDuration(duration float64)
}

func NewStatsReporter() StatsReporter {
	meter := global.Meter("sigs.k8s.io/secrets-store-csi-driver")
	nodePublishTotal = metric.Must(meter).NewInt64Counter("total_node_publish", metric.WithDescription("Total number of node publish calls"))
	nodeUnPublishTotal = metric.Must(meter).NewInt64Counter("total_node_unpublish", metric.WithDescription("Total number of node unpublish calls"))
	nodePublishErrorTotal = metric.Must(meter).NewInt64Counter("total_node_publish_error", metric.WithDescription("Total number of node publish calls with error"))
	nodeUnPublishErrorTotal = metric.Must(meter).NewInt64Counter("total_node_unpublish_error", metric.WithDescription("Total number of node unpublish calls with error"))
	syncK8sSecretTotal = metric.Must(meter).NewInt64Counter("total_sync_k8s_secret", metric.WithDescription("Total number of k8s secrets synced"))
	syncK8sSecretDuration = metric.Must(meter).NewFloat64ValueRecorder("sync_k8s_secret_duration_sec", metric.WithDescription("Distribution of how long it took to sync k8s secret"))
	return &reporter{meter: meter}
}

func (r *reporter) ReportNodePublishCtMetric(provider string) {
	attributes := []attribute.KeyValue{attribute.String(providerKey, provider), attribute.String(osTypeKey, runtimeOS)}
	nodePublishTotal.Add(context.Background(), 1, attributes...)
}

func (r *reporter) ReportNodeUnPublishCtMetric() {
	nodeUnPublishTotal.Add(context.Background(), 1, []attribute.KeyValue{attribute.String(osTypeKey, runtimeOS)}...)
}

func (r *reporter) ReportNodePublishErrorCtMetric(provider, errType string) {
	attributes := []attribute.KeyValue{attribute.String(providerKey, provider), attribute.String(errorKey, errType), attribute.String(osTypeKey, runtimeOS)}
	nodePublishErrorTotal.Add(context.Background(), 1, attributes...)
}

func (r *reporter) ReportNodeUnPublishErrorCtMetric() {
	nodeUnPublishErrorTotal.Add(context.Background(), 1, []attribute.KeyValue{attribute.String(osTypeKey, runtimeOS)}...)
}

func (r *reporter) ReportSyncK8SecretCtMetric(provider string, count int) {
	attributes := []attribute.KeyValue{attribute.String(providerKey, provider), attribute.String(osTypeKey, runtimeOS)}
	syncK8sSecretTotal.Add(context.Background(), int64(count), attributes...)
}

func (r *reporter) ReportSyncK8SecretDuration(duration float64) {
	r.meter.RecordBatch(context.Background(), []attribute.KeyValue{attribute.String(osTypeKey, runtimeOS)}, syncK8sSecretDuration.Measurement(duration))
}
