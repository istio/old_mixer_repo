#!/bin/bash

set -ex

gcloud config set project istio-testing

bazel run //docker:mixer gcr.io/istio-testing/mixer:experiment

gcloud docker -- push gcr.io/istio-testing/mixer:experiment
