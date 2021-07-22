/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/metrics"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/rotation"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/version"

	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	json "k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/controllers"
	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"
	// +kubebuilder:scaffold:imports
)

var (
	endpoint           = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName         = flag.String("drivername", "secrets-store.csi.k8s.io", "name of the driver")
	nodeID             = flag.String("nodeid", "", "node id")
	logFormatJSON      = flag.Bool("log-format-json", false, "set log formatter to json")
	providerVolumePath = flag.String("provider-volume", "/etc/kubernetes/secrets-store-csi-providers", "Volume path for provider")
	// this will be removed in a future release
	metricsAddr          = flag.String("metrics-addr", ":8095", "The address the metric endpoint binds to")
	enableSecretRotation = flag.Bool("enable-secret-rotation", false, "Enable secret rotation feature [alpha]")
	rotationPollInterval = flag.Duration("rotation-poll-interval", 2*time.Minute, "Secret rotation poll interval duration")
	enableProfile        = flag.Bool("enable-pprof", false, "enable pprof profiling")
	profilePort          = flag.Int("pprof-port", 6065, "port for pprof profiling")
	maxCallRecvMsgSize   = flag.Int("max-call-recv-msg-size", 1024*1024*4, "maximum size in bytes of gRPC response from plugins")

	// enable filtered watch for NodePublishSecretRef secrets. The filtering is done on the csi driver label: secrets-store.csi.k8s.io/used=true
	// For Kubernetes secrets used to provide credentials for use with the CSI driver, set the label by running: kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true
	filteredWatchSecret = flag.Bool("filtered-watch-secret", true, "enable filtered watch for NodePublishSecretRef secrets with label secrets-store.csi.k8s.io/used=true")

	// Enable optional healthcheck for provider clients that exist in memory
	providerHealthCheck         = flag.Bool("provider-health-check", false, "Enable health check for configured providers")
	providerHealthCheckInterval = flag.Duration("provider-health-check-interval", 2*time.Minute, "Provider healthcheck interval duration")

	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()

	if *logFormatJSON {
		klog.SetLogger(json.JSONLogger)
	}
	if *enableProfile {
		klog.Infof("Starting profiling on port %d", *profilePort)
		go func() {
			addr := fmt.Sprintf("%s:%d", "localhost", *profilePort)
			klog.ErrorS(http.ListenAndServe(addr, nil), "unable to start profiling server")
		}()
	}
	if *filteredWatchSecret {
		klog.Infof("Filtered watch for nodePublishSecretRef secret based on secrets-store.csi.k8s.io/used=true label enabled")
	}

	// initialize metrics exporter before creating measurements
	err := metrics.InitMetricsExporter()
	if err != nil {
		klog.Fatalf("failed to initialize metrics exporter, error: %+v", err)
	}

	cfg := ctrl.GetConfigOrDie()
	cfg.UserAgent = version.GetUserAgent("controller")

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		LeaderElection:     false,
		MapperProvider: func(c *rest.Config) (meta.RESTMapper, error) {
			return apiutil.NewDynamicRESTMapper(c, apiutil.WithLazyDiscovery)
		},
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				// this enables filtered watch of pods based on the node name
				// only pods running on the same node as the csi driver will be cached
				&corev1.Pod{}: {
					Field: fields.OneTermEqualSelector("spec.nodeName", *nodeID),
				},
				// this enables filtered watch of secretproviderclasspodstatuses based on the internal node label
				// internal.secrets-store.csi.k8s.io/node-name=<node name> added by csi driver
				&v1alpha1.SecretProviderClassPodStatus{}: {
					Label: labels.SelectorFromSet(
						labels.Set{
							v1alpha1.InternalNodeLabel: *nodeID,
						},
					),
				},
				// this enables filtered watch of secrets based on the label (eg. secrets-store.csi.k8s.io/managed=true)
				// added to the secrets created by the CSI driver
				&corev1.Secret{}: {
					Label: labels.SelectorFromSet(
						labels.Set{
							controllers.SecretManagedLabel: "true",
						},
					),
				},
			},
		}),
	})
	if err != nil {
		klog.Fatalf("failed to start manager, error: %+v", err)
	}
	err = mgr.Add(metrics.NewSecretProviderClassReporter(mgr.GetClient()))
	if err != nil {
		klog.Fatalf("failed to start secretproviderclass reporter, error: %+v", err)
	}

	reconciler, err := controllers.New(mgr, *nodeID)
	if err != nil {
		klog.Fatalf("failed to create secret provider class pod status reconciler, error: %+v", err)
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		klog.Fatalf("failed to create controller, error: %+v", err)
	}
	// +kubebuilder:scaffold:builder

	ctx := withShutdownSignal(context.Background())

	// create provider clients
	providerClients := secretsstore.NewPluginClientBuilder(*providerVolumePath, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(*maxCallRecvMsgSize)))
	defer providerClients.Cleanup()

	// enable provider health check
	if *providerHealthCheck {
		klog.InfoS("provider health check enabled", "interval", *providerHealthCheckInterval)
		go providerClients.HealthCheck(ctx, *providerHealthCheckInterval)
	}

	go func() {
		klog.Infof("starting manager")
		if err := mgr.Start(ctx); err != nil {
			klog.Fatalf("failed to run manager, error: %+v", err)
		}
	}()

	go func() {
		reconciler.RunPatcher(ctx)
	}()

	// Secret rotation
	if *enableSecretRotation {
		rec, err := rotation.NewReconciler(mgr.GetCache(), scheme, *providerVolumePath, *nodeID, *rotationPollInterval, providerClients, *filteredWatchSecret)
		if err != nil {
			klog.Fatalf("failed to initialize rotation reconciler, error: %+v", err)
		}
		go rec.Run(ctx.Done())
	}

	driver := secretsstore.GetDriver()
	driver.Run(ctx, *driverName, *nodeID, *endpoint, *providerVolumePath, providerClients, mgr.GetClient())
}

// withShutdownSignal returns a copy of the parent context that will close if
// the process receives termination signals.
func withShutdownSignal(ctx context.Context) context.Context {
	nctx, cancel := context.WithCancel(ctx)
	stopCh := ctrl.SetupSignalHandler().Done()

	go func() {
		<-stopCh
		klog.Info("received shutdown signal")
		cancel()
	}()
	return nctx
}
