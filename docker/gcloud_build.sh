#!/bin/bash

set -ex

gcloud docker --authorize-only

if [ -z $DOCKER_TAG ]
then
    export DOCKER_TAG=experiment
fi

if [ -z $BAZEL_OUTBASE ]
then
    bazel run //docker:mixer gcr.io/$PROJECT/mixer:$DOCKER_TAG
else
    bazel --output_base=$BAZEL_OUTBASE run //docker:mixer gcr.io/$PROJECT/mixer:$DOCKER_TAG
fi

gcloud docker -- push gcr.io/$PROJECT/mixer:$DOCKER_TAG
