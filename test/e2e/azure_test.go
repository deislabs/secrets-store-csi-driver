// +build azure

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

package e2e

import (
	"os"

	. "github.com/onsi/ginkgo"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/azure"
)

var _ = Describe("Testing CSI Driver with Azure provider", func() {
	It("Install Azure provider and setup Azure", func() {
		cli := clusterProxy.GetClient()
		azure.InstallAndWaitProvider(ctx, azure.InstallAndWaitProviderInput{
			Creator:        cli,
			GetLister:      cli,
			Namespace:      csiNamespace,
			KubeconfigPath: clusterProxy.GetKubeconfigPath(),
		})
	})

	AzureSpec(ctx, func() AzureSpecInput {
		return AzureSpecInput{
			clusterProxy: clusterProxy,
			skipCleanup:  skipCleanup,
			chartPath:    chartPath,
			clientID:     os.Getenv("AZURE_CLIENT_ID"),
			clientSecret: os.Getenv("AZURE_CLIENT_SECRET"),
		}
	})
})
