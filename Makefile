# Copyright 2018 The Kubernetes Authors.
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

GOPATH  := $(shell go env GOPATH)
GOARCH  := $(shell go env GOARCH)
GOOS    := $(shell go env GOOS)

ORG_PATH=sigs.k8s.io
PROJECT_NAME := secrets-store-csi-driver
BUILD_COMMIT := $(shell git rev-parse --short HEAD)
REPO_PATH="$(ORG_PATH)/$(PROJECT_NAME)"

REGISTRY ?= gcr.io/k8s-staging-csi-secrets-store
IMAGE_NAME ?= driver
# Release version is the current supported release for the driver
# Update this version when the helm chart is being updated for release
RELEASE_VERSION := v0.0.22
IMAGE_VERSION ?= v0.0.22
# Use a custom version for E2E tests if we are testing in CI
ifdef CI
override IMAGE_VERSION := v0.1.0-e2e-$(BUILD_COMMIT)
endif
IMAGE_TAG=$(REGISTRY)/$(IMAGE_NAME):$(IMAGE_VERSION)

# build variables
BUILD_TIMESTAMP := $$(date +%Y-%m-%d-%H:%M)
BUILD_TIME_VAR := $(REPO_PATH)/pkg/version.BuildTime
BUILD_VERSION_VAR := $(REPO_PATH)/pkg/version.BuildVersion
VCS_VAR := $(REPO_PATH)/pkg/version.Vcs
LDFLAGS ?= "-X $(BUILD_TIME_VAR)=$(BUILD_TIMESTAMP) -X $(BUILD_VERSION_VAR)=$(IMAGE_VERSION) -X $(VCS_VAR)=$(BUILD_COMMIT)"

GO_FILES=$(shell go list ./... | grep -v /test/sanity)
TOOLS_MOD_DIR := ./hack/tools
TOOLS_DIR := $(abspath ./hack/tools)
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin

# we use go modules to manage dependencies
GO111MODULE = on
# for using docker buildx and docker manifest command
DOCKER_CLI_EXPERIMENTAL = enabled
export GOPATH GOBIN GO111MODULE DOCKER_CLI_EXPERIMENTAL

# Generate all combination of all OS, ARCH, and OSVERSIONS for iteration
ALL_OS = linux windows
ALL_ARCH.linux = amd64
ALL_OS_ARCH.linux = $(foreach arch, ${ALL_ARCH.linux}, linux-$(arch))
ALL_ARCH.windows = amd64
ALL_OSVERSIONS.windows := 1809 1903 1909 2004
ALL_OS_ARCH.windows = $(foreach arch, $(ALL_ARCH.windows), $(foreach osversion, ${ALL_OSVERSIONS.windows}, windows-${osversion}-${arch}))
ALL_OS_ARCH = $(foreach os, $(ALL_OS), ${ALL_OS_ARCH.${os}})

# The current context of image building
# The architecture of the image
ARCH ?= amd64
# OS Version for the Windows images: 1809, 1903, 1909, 2004
OSVERSION ?= 1809
# Output type of docker buildx build
OUTPUT_TYPE ?= registry
BUILDKIT_VERSION ?= v0.8.1

# Binaries
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize
PROTOC_GEN_GO := $(TOOLS_BIN_DIR)/protoc-gen-go
PROTOC := $(TOOLS_BIN_DIR)/bin/protoc
TRIVY := trivy
HELM := helm
BATS := bats
AZURE_CLI := az
KIND := kind
KUBECTL := kubectl
ENVSUBST := envsubst

# Test variables
KIND_VERSION ?= 0.10.0
KUBERNETES_VERSION ?= 1.20.2
BATS_VERSION ?= 1.2.1
TRIVY_VERSION ?= 0.14.0
PROTOC_VERSION ?= 3.15.2

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: lint go-test

.PHONY: go-test # Run unit tests
go-test:
	go test $(GO_FILES) -v

.PHONY: sanity-test # Run CSI sanity tests for the driver
sanity-test:
	go test -v ./test/sanity

.PHONY: image-scan
image-scan: $(TRIVY)
	# show all vulnerabilities
	$(TRIVY) --severity MEDIUM,HIGH,CRITICAL $(IMAGE_TAG)
	# show vulnerabilities that have been fixed
	$(TRIVY) --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL $(IMAGE_TAG)

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(CONTROLLER_GEN): $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go ## Build controller-gen from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		go build -tags=tools -o $(TOOLS_BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOLANGCI_LINT): ## Build golangci-lint from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		go build -tags=tools -o $(TOOLS_BIN_DIR)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(KUSTOMIZE): ## Build kustomize from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		go build -tags=tools -o $(TOOLS_BIN_DIR)/kustomize sigs.k8s.io/kustomize/kustomize/v3

$(PROTOC_GEN_GO):
	cd $(TOOLS_MOD_DIR) && \
		go build -tags=tools -o $(TOOLS_BIN_DIR)/protoc-gen-go github.com/golang/protobuf/protoc-gen-go

## --------------------------------------
## Testing Binaries
## --------------------------------------

$(HELM): ## Install helm3 if not present
	helm version --short | grep -q v3 || (curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash)

$(KIND): ## Download and install kind
	kind --version | grep -q $(KIND_VERSION) || (curl -L https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-linux-amd64 --output kind && chmod +x kind && mv kind /usr/local/bin/)

$(AZURE_CLI): ## Download and install azure cli
	curl -sL https://aka.ms/InstallAzureCLIDeb | bash

$(KUBECTL): ## Install kubectl
	curl -LO https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv kubectl /usr/local/bin/

$(TRIVY): ## Install trivy for image vulnerability scan
	trivy -v | grep -q $(TRIVY_VERSION) || (curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v$(TRIVY_VERSION))

$(BATS): ## Install bats for running the tests
	bats --version | grep -q $(BATS_VERSION) || (curl -sSLO https://github.com/bats-core/bats-core/archive/v${BATS_VERSION}.tar.gz && tar -zxvf v${BATS_VERSION}.tar.gz && bash bats-core-${BATS_VERSION}/install.sh /usr/local)

$(ENVSUBST): ## Install envsubst for running the tests
	envsubst -V || (apt-get -o Acquire::Retries=30 update && apt-get -o Acquire::Retries=30 install gettext-base -y)

$(PROTOC): ## Install protoc
	curl -sSLO https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip bin/protoc -d $(TOOLS_BIN_DIR) && rm protoc-${PROTOC_VERSION}-linux-x86_64.zip 

## --------------------------------------
## Linting
## --------------------------------------
.PHONY: test-style
test-style: lint lint-charts

.PHONY: lint
lint: $(GOLANGCI_LINT)
	# Setting timeout to 5m as default is 1m
	$(GOLANGCI_LINT) run --timeout=5m -v

lint-full: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run -v --fast=false

lint-charts: $(HELM) # Run helm lint tests
	helm lint --strict charts/secrets-store-csi-driver
	helm lint --strict manifest_staging/charts/secrets-store-csi-driver

## --------------------------------------
## Builds
## --------------------------------------
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver

.PHONY: build-windows
build-windows:
	CGO_ENABLED=0 GOOS=windows go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver

.PHONY: build-darwin
build-darwin:
	CGO_ENABLED=0 GOOS=darwin go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver

.PHONY: container
container:
	docker build --no-cache --build-arg LDFLAGS=$(LDFLAGS) -t $(IMAGE_TAG) -f docker/Dockerfile .

.PHONY: container-linux
container-linux: docker-buildx-builder
	docker buildx build --no-cache --output=type=$(OUTPUT_TYPE) --platform="linux/$(ARCH)" --build-arg LDFLAGS=$(LDFLAGS) \
 		-t $(IMAGE_TAG)-linux-$(ARCH) -f docker/Dockerfile .

.PHONY: container-windows
container-windows: docker-buildx-builder
	docker buildx build --no-cache --output=type=$(OUTPUT_TYPE) --platform="windows/$(ARCH)" --build-arg LDFLAGS=$(LDFLAGS) \
		--build-arg BASEIMAGE=mcr.microsoft.com/windows/nanoserver:$(OSVERSION) \
		--build-arg BASEIMAGE_CORE=gcr.io/k8s-staging-e2e-test-images/windows-servercore-cache:1.0-linux-amd64-$(OSVERSION) \
 		-t $(IMAGE_TAG)-windows-$(OSVERSION)-$(ARCH) -f docker/windows.Dockerfile .

.PHONY: docker-buildx-builder
docker-buildx-builder:
	@if ! docker buildx ls | grep -q container-builder; then\
		DOCKER_CLI_EXPERIMENTAL=enabled docker buildx create --name container-builder --use --driver-opt image=moby/buildkit:$(BUILDKIT_VERSION);\
	fi

.PHONY: container-all
container-all:
	$(MAKE) container-linux
	for osversion in $(ALL_OSVERSIONS.windows); do \
  		OSVERSION=$${osversion} $(MAKE) container-windows; \
  	done

.PHONY: push-manifest
push-manifest:
	docker manifest create --amend $(IMAGE_TAG) $(foreach osarch, $(ALL_OS_ARCH), $(IMAGE_TAG)-${osarch})
	# add "os.version" field to windows images (based on https://github.com/kubernetes/kubernetes/blob/master/build/pause/Makefile)
	set -x; \
	registry_prefix=$(shell (echo ${REGISTRY} | grep -Eq ".*[\/\.].*") && echo "" || echo "docker.io/"); \
	manifest_image_folder=`echo "$${registry_prefix}${IMAGE_TAG}" | sed "s|/|_|g" | sed "s/:/-/"`; \
	for arch in $(ALL_ARCH.windows); do \
		for osversion in $(ALL_OSVERSIONS.windows); do \
			BASEIMAGE=mcr.microsoft.com/windows/nanoserver:$${osversion}; \
			full_version=`docker manifest inspect $${BASEIMAGE} | jq -r '.manifests[0].platform["os.version"]'`; \
			sed -i -r "s/(\"os\"\:\"windows\")/\0,\"os.version\":\"$${full_version}\"/" "${HOME}/.docker/manifests/$${manifest_image_folder}/$${manifest_image_folder}-windows-$${osversion}-$${arch}"; \
		done; \
	done
	docker manifest push --purge $(IMAGE_TAG)
	docker manifest inspect $(IMAGE_TAG)

## --------------------------------------
## E2E Testing
## --------------------------------------
.PHONY: e2e-bootstrap
e2e-bootstrap: $(HELM) $(BATS) $(KIND) $(KUBECTL) $(ENVSUBST) #setup all required binaries and kind cluster for testing
ifndef TEST_WINDOWS
	$(MAKE) setup-kind
endif
	docker pull $(IMAGE_TAG) || $(MAKE) e2e-container

.PHONY: setup-kind
setup-kind: $(KIND)
	# Create kind cluster if it doesn't exist
	if [ $$(kind get clusters) ]; then kind delete cluster; fi
	kind create cluster --image kindest/node:v$(KUBERNETES_VERSION)

.PHONY: e2e-container
e2e-container:
ifdef TEST_WINDOWS
	$(MAKE) container-all push-manifest
else
	$(MAKE) container
	kind load docker-image --name kind $(IMAGE_TAG)
endif

.PHONY: e2e-test
e2e-test: e2e-bootstrap e2e-helm-deploy # run test for windows
	$(MAKE) e2e-azure

.PHONY: e2e-teardown
e2e-teardown: $(HELM)
	helm delete csi-secrets-store --namespace default

.PHONY: e2e-helm-deploy
e2e-helm-deploy:
	helm install csi-secrets-store manifest_staging/charts/secrets-store-csi-driver --namespace default --wait --timeout=15m -v=5 --debug \
		--set linux.image.pullPolicy="IfNotPresent" \
		--set windows.image.pullPolicy="IfNotPresent" \
		--set linux.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
		--set linux.image.tag=$(IMAGE_VERSION) \
		--set windows.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
		--set windows.image.tag=$(IMAGE_VERSION) \
		--set windows.enabled=true \
		--set linux.enabled=true \
		--set enableSecretRotation=true \
		--set rotationPollInterval=30s

.PHONY: e2e-helm-deploy-release # test helm package for the release
e2e-helm-deploy-release:
	set -x; \
	current_release=$(shell (echo ${RELEASE_VERSION} | sed s/"v"//)); \
	helm install csi-secrets-store charts/secrets-store-csi-driver-$${current_release}.tgz --namespace default --wait --timeout=15m -v=5 --debug \
		--set linux.image.pullPolicy="IfNotPresent" \
		--set windows.image.pullPolicy="IfNotPresent" \
		--set windows.enabled=true \
		--set linux.enabled=true \
		--set enableSecretRotation=true \
		--set rotationPollInterval=30s

.PHONY: e2e-kind-cleanup
e2e-kind-cleanup:
	kind delete cluster --name kind

.PHONY: e2e-azure
e2e-azure: $(AZURE_CLI)
	bats -t test/bats/azure.bats

.PHONY: e2e-vault
e2e-vault:
	bats -t test/bats/vault.bats

.PHONY: e2e-gcp
e2e-gcp:
	bats -t test/bats/gcp.bats

## --------------------------------------
## Generate
## --------------------------------------
# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: $(CONTROLLER_GEN) $(KUSTOMIZE)
	# Generate the base CRD/RBAC
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=secretproviderclasses-role paths="./apis/..." paths="./controllers" output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* manifest_staging/charts/secrets-store-csi-driver/templates
	cp config/crd/bases/* manifest_staging/deploy/

	# generate rbac-secretproviderclass
	$(KUSTOMIZE) build config/rbac -o manifest_staging/deploy/rbac-secretproviderclass.yaml
	cp config/rbac/role.yaml config/rbac/role_binding.yaml config/rbac/serviceaccount.yaml manifest_staging/charts/secrets-store-csi-driver/templates/
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role.yaml
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role_binding.yaml
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ include "sscd.labels" . | indent 2 }}\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/serviceaccount.yaml

	# Generate secret syncing specific RBAC
	$(CONTROLLER_GEN) rbac:roleName=secretprovidersyncing-role paths="./controllers/syncsecret" output:dir=config/rbac-syncsecret
	$(KUSTOMIZE) build config/rbac-syncsecret -o manifest_staging/deploy/rbac-secretprovidersyncing.yaml
	cp config/rbac-syncsecret/role.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret.yaml
	cp config/rbac-syncsecret/role_binding.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret_binding.yaml
	@sed -i '1s/^/{{ if .Values.syncSecret.enabled }}\n/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret.yaml
	@sed -i '1s/^/{{ if .Values.syncSecret.enabled }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret_binding.yaml

.PHONY: generate-protobuf
generate-protobuf: $(PROTOC) $(PROTOC_GEN_GO) # generates protobuf
	$(PROTOC) -I . provider/v1alpha1/service.proto --go_out=plugins=grpc:. --plugin=$(PROTOC_GEN_GO)

## --------------------------------------
## Release
## --------------------------------------
.PHONY: release-manifest
release-manifest:
	$(MAKE) manifests
	@sed -i "s/version: .*/version: ${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/Chart.yaml
	@sed -i "s/appVersion: .*/appVersion: ${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/Chart.yaml
	@sed -i "s/tag: v${CURRENTVERSION}/tag: v${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/values.yaml
	@sed -i "s/v${CURRENTVERSION}/v${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/README.md
	@sed -i "s/driver:v${CURRENTVERSION}/driver:v${NEWVERSION}/" manifest_staging/deploy/secrets-store-csi-driver.yaml manifest_staging/deploy/secrets-store-csi-driver-windows.yaml

.PHONY: promote-staging-manifest
promote-staging-manifest: #promote staging manifests to release dir
	$(MAKE) release-manifest
	@rm -rf deploy
	@cp -r manifest_staging/deploy .
	@rm -rf charts/secrets-store-csi-driver
	@cp -r manifest_staging/charts/secrets-store-csi-driver ./charts
	@mkdir -p ./charts/tmp
	@helm package ./charts/secrets-store-csi-driver -d ./charts/tmp/
	@helm repo index ./charts/tmp --url https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/master/charts --merge ./charts/index.yaml
	@mv ./charts/tmp/* ./charts
	@rm -rf ./charts/tmp
