# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REGISTRY?=docker.io/deislabs
IMAGE_NAME=driver
CRD_IMAGE_NAME=driver-crds
IMAGE_VERSION?=v0.1.0
BUILD_TIMESTAMP := $(shell date +%Y-%m-%d-%H:%M)
BUILD_COMMIT := $(shell git rev-parse --short HEAD)
IMAGE_TAG=$(REGISTRY)/$(IMAGE_NAME):$(IMAGE_VERSION)
CRD_IMAGE_TAG=$(REGISTRY)/$(CRD_IMAGE_NAME):$(IMAGE_VERSION)
export

export DOCKER_CLI_EXPERIMENTAL=enabled

build-and-push:
	bash -x ./build.sh build_and_push

manifest: build-and-push
	bash -x ./build.sh manifest

.PHONY: build-and-push manifest
