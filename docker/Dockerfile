ARG BASEIMAGE=us.gcr.io/k8s-artifacts-prod/build-image/debian-base:buster-v1.8.0

FROM golang:1.16 as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
ARG TARGETARCH
ARG TARGETOS
ARG TARGETPLATFORM
ARG IMAGE_VERSION

RUN export GOOS=$TARGETOS && \
    export GOARCH=$TARGETARCH && \
    make build

FROM $BASEIMAGE
# upgrading libsystemd0 and libudev1 due to CVE-2021-33910
COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/_output/secrets-store-csi /secrets-store-csi
RUN clean-install ca-certificates mount libsystemd0 libudev1

LABEL maintainers="ritazh"
LABEL description="Secrets Store CSI Driver"

ENTRYPOINT ["/secrets-store-csi"]
