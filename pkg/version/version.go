/*
Copyright 2019 The Kubernetes Authors.
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

package version

import (
	"fmt"
	"runtime"
)

var (
	// Vcs is the commit hash for the binary build
	Vcs string
	// BuildTime is the date for the binary build
	BuildTime string
	// BuildVersion is the secrets-store-csi-driver version. Will be overwritten from build.
	BuildVersion = "local-dev"
)

// GetUserAgent returns a user agent of the format: csi-secrets-store/<controller name>/<version> (<goos>/<goarch>) <vcs>/<timestamp>
func GetUserAgent(controllerName string) string {
	return fmt.Sprintf("csi-secrets-store/%s/%s (%s/%s) %s/%s", controllerName, BuildVersion, runtime.GOOS, runtime.GOARCH, Vcs, BuildTime)
}
