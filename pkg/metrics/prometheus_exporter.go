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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	otProm "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func newPipeline(config otProm.Config, options ...controller.Option) (*controller.Controller, *otProm.Exporter, error) {
	c := controller.New(
		processor.New(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			export.CumulativeExportKindSelector(),
			// Enable memory to keep previously reported metrics
			// However, SecretProviderClass metrics "kube_secretproviderclass_*" should be
			// flush last reported metrics to update.corresponding SecretProvidierClass
			// objects with changealble fields such as labels, secretObjects.
			// If disable it, memory cache is flushed every collecting interval so that
			// the other metrics can't be always retrieved from memory cache.
			//
			// TODO: Either combine multiple controllers or provide different controllers
			// and metrics paths and reimplement them to the expected behavior.
			// https://github.com/open-telemetry/opentelemetry-go/issues/1716
			processor.WithMemory(true),
		),
		options...,
	)
	exp, err := otProm.New(config, c)
	if err != nil {
		return nil, nil, err
	}
	return c, exp, nil
}

func initPrometheusExporter() error {
	cfg := otProm.Config{
		Registry: metrics.Registry.(*prometheus.Registry), // using the controller-runtime prometheus metrics registry
		DefaultHistogramBoundaries: []float64{
			0.1, 0.2, 0.3, 0.4, 0.5, 1, 1.5, 2, 2.5, 3.0, 5.0, 10.0, 15.0, 30.0,
		},
	}
	// Drop default OpenTelemetry go.opentelemetry.io/otel/sdk@v1.0.0-RC1 labels
	// service_name, telemetry_sdk_language, telemetry_sdk_name, telemetry_sdk_version
	_, exp, err := newPipeline(cfg, controller.WithResource(resource.Empty()))
	if err != nil {
		return err
	}
	global.SetMeterProvider(exp.MeterProvider())

	return nil
}
