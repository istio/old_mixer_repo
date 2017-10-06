# Overview

This document describes the general developement workflow for
developing and working with Mixer in the broader context of Istio.
This document includes information on building Mixer properly and
then building the associated containers in a local environment
prior to prow updating the istio/istio dependency repositories.

## Prepare tree

Prepare the Mixer repository and Go environment variables as
specified in the 
[contribution guidelines](https://github.com/istio/istio/blob/master/CONTRIBUTING.md).

## Prepare a registry account on gcr.io

Follow the
[Google Container Registry Quickstart](https://https://cloud.google.com/container-registry/docs/quickstart).

## Using the codebase

Follow the
[Using the code base docuementation](https://github.com/istio/istio/blob/master/devel/README.md#using-the-code-base://github.com/istio/istio/blob/master/devel/README.md).

## Publish Mixer containers to Google Container Registry

This script publishes Mixer container images to GCR.

```
bin/publish-docker-images.sh -h gcr.io/my-project -t my-tag
```

where

* The `-h` parameter `my-project` is the composition of the hostname
  and the project id. This should be customized.
* The `-t` parameter `my-tag` is the desired tag. This should be customized.

## Build new Istio manifests

Use [updateVersion.sh](https://github.com/istio/istio/blob/master/install/updateVersion.sh) to geneate new manifests with the specified Mixer containers.

```
cd $ISTIO
git clone https://github.com/$GITHUB_USER/istio.git
cd istio
install/updateVersion.sh -r gcr.io/my-project,tag
```

where

* $ISTIO and $GITHUB_USER are defined in 
[contribution guidelines](https://github.com/istio/istio/blob/master/CONTRIBUTING.md).
* `my-project` is equivalent to the `-h` parameter specified to
  `publish-docker-images.sh`.
* `my-tag` is equivalent to the `-t` parameter specified to
  `publish-docker-images.sh`.

## Deploy Istio manifests

Follow the
[Istio quickstart](https://istio.io/docs/setup/install-kubernetes.html)
to deploy the new Mixer containers.
