# Upgrades

This page includes instructions for upgrading the driver to the latest version.

```bash
helm upgrade csi-secrets-store secrets-store-csi-driver/secrets-store-csi-driver --namespace=NAMESPACE
```

Set `NAMESPACE` to the same namespace where the driver was originally installed,
(i.e. `kube-system`)

If you are upgrading from one of the following versions there may be additional
steps that you should take.

## pre `v0.1.0`

>**NOTE**: [CustomResourceDefinitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) (CRDs) have been moved from `templates` to `crds` directory in the helm charts. To manage the lifecycle of the CRDs during install/upgrade, helm `pre-install` and `pre-upgrade` hook has been added. This hook will create a pod that runs only on **linux** nodes and deploys the CRDs in the Kubernetes cluster.

In case there is an issue with these hooks we recommend backing up your
`SecretProviderClass`es in case of any issues with the hooks:

```bash
kubectl get secretproviderclass -A -o yaml > spc-all-backup.yaml
```

The filtered watch feature is enabled by default in `v0.1.0` (see
[#550](https://github.com/kubernetes-sigs/secrets-store-csi-driver/issues/550)).
All existing `nodePublishSecretRef` Kubernetes Secrets used in volume mounts
must have the `secrets-store.csi.k8s.io/used=true` label otherwise secret
rotations will fail with `failed to get node publish secret` errors.

Label these Kubernetes Secrets by running:

```bash
kubectl label secret <node publish secret ref name> secrets-store.csi.k8s.io/used=true
```

## pre `v0.0.23`

`v0.0.23` sets `syncSecret.enabled=false` by default. This means the RBAC clusterrole and clusterrolebinding required for [sync mounted content as Kubernetes secret](https://secrets-store-csi-driver.sigs.k8s.io/topics/sync-as-kubernetes-secret.html) will no longer be created by default as part of `helm install/upgrade`. If you're using the driver to sync mounted content as Kubernetes secret, you'll need to set `syncSecret.enabled=true` as part of `helm install/upgrade`.

## pre `v0.0.20`

`v0.0.20` removed support for non-gRPC based providers. Follow your provider
documentation to upgrade providers to use gRPC before upgrading the driver to
`v0.0.20` or greater.

## pre `v0.0.18`

`v0.0.17` and earlier installed the driver to the `default` namespace when using
the YAML based install. Newer versions of the driver YAML files install the
driver to the `kube-system` namespace. After applying the new YAML files to your
cluster run the following to clean up old resources:

```bash
kubectl delete daemonset csi-secrets-store --namespace=default
kubectl delete daemonset csi-secrets-store-windows --namespace=default
kubectl delete serviceaccount secrets-store-csi-driver --namespace=default
```

## pre `v0.0.12`

The `SecretProviderClass` needs to be in the same namespace as the pod
referencing it as of `v0.0.12`.

Defining driver configuration and provider-specific parameters to the CSI driver
in `pod.Spec[].Volumes` has been deprecated in `v0.0.12`. It is now mandatory to
use `SecretProviderClass` custom resource.
