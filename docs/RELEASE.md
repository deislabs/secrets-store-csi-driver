# Release Process

## Overview

The release process consists of three phases: versioning, building and publishing

Versioning involves maintaining the following files:

- Makefile - the Makefile contains a `IMAGE_VERSION` variable that defines the version of the project.
- secrets-store-csi-driver.yaml - the Linux driver deployment yaml that contains the latest release tag image of the project.
- secrets-store-csi-driver-windows.yaml - the Windows driver deployment yaml that contains the latest release tag image of the project.
- `deploy/` dir that contains all the secrets-store-csi-driver resources to be deployed to the cluster including the latest release tag image of the project.

The steps below explain how to update these files. In addition, the repository should be tagged with the semantic version identifying the release.

Building involves obtaining a copy of the repository and triggering an automatic build as part of the [prow job](https://testgrid.k8s.io/sig-auth-secrets-store-csi-driver#secrets-store-csi-driver-push-image).

Publishing involves creating a release tag and creating a new Release on GitHub.

## Versioning

1. Make sure that the `docs` include all necessary information included in the release (example [tag compare](https://github.com/kubernetes-sigs/secrets-store-csi-driver/compare/v0.0.21...master)).
1. Create a new release branch `release-X.X`

    ```bash
    # TODO
    ```
    
1. Update the version to the semantic version of the new release similar to [this](https://github.com/kubernetes-sigs/secrets-store-csi-driver/pull/251)
1. Commit the changes and push to remote repository to create a pull request to the `master` branch

    ```bash
    git checkout -b bump-version-<NEW VERSION>
    git commit -m "Bump versions for <NEW VERSION>"
    git push <YOUR FORK>
    ```

1. Cherry-pick the bump to the `release-X.X` branch

    ```bash
    # TODO
    ```

1. Once the PR is merged to `release-X.X`, the [prow job](https://testgrid.k8s.io/sig-auth-secrets-store-csi-driver#secrets-store-csi-driver-push-image) is triggered to build and push the new version to staging repo (`gcr.io/k8s-staging-csi-secrets-store/driver`)
1. Once the prow job completes, follow the [instructions](https://github.com/kubernetes/k8s.io/tree/main/k8s.gcr.io#image-promoter) to promote the image to production repo
    - Run generate script to append the new image to promoter manifest

    ```bash
    k8s.gcr.io/images/k8s-staging-csi-secrets-store/generate.sh > k8s.gcr.io/images/k8s-staging-csi-secrets-store/images.yaml
    ```

    - Preview the changes

    ```bash
    git diff
    ```

    - Commit the changes and push to remote repository to create a pull request
1. Once the image promoter PR is merged, the image will be promoted to prod repo (`k8s.gcr.io/csi-secrets-store/driver`)
  
## Building and releasing

1. Execute the promote-staging-manifest target to generate patch and promote staging manifest to release

    ```bash
   make promote-staging-manifest NEWVERSION=0.0.12 CURRENTVERSION=0.0.11
    ```

1. Preview the changes

    ```bash
   git diff
    ```

1. Commit the changes and push to remote repository to create a pull request

    ```bash
    git checkout -b release-<NEW VERSION>
    git commit -a -s -m "release: update manifests and helm chart for <NEW VERSION>"
    git push <YOUR FORK>
    ```

1. Once the PR is merged to `master`, cherry pick the commit to the `release-X.X` branch:

    ```bash
    # TODO
    ```

1. Once the PR is merged to `release-X.X`, tag `release-X.X` with release version and push tags to remote repository.
    - An [OWNER](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/OWNERS) runs git tag and pushes the tag with git push

   ```bash
   git checkout master
   git pull origin master
   git tag -a <NEW VERSION> -m '<NEW VERSION>'
   git push origin <NEW VERSION>
   ```

## Publishing

1. Create a draft release in GitHub and associate it with the tag that was just created
1. Collect release notes (example [tag compare](https://github.com/kubernetes-sigs/secrets-store-csi-driver/compare/v0.0.21...master))
1. Write the release notes similar to [this](https://github.com/kubernetes-sigs/secrets-store-csi-driver/releases/tag/v0.0.12) and upload all the artifacts from the `deploy/` dir
1. Publish release
