#!/bin/sh

set -euxo pipefail

# install tekton
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

# install shipwright
kubectl apply -f https://github.com/shipwright-io/build/releases/download/v0.4.0/release.yaml

# clone repo, run e2e tests
tmp=$(mktemp -d)
git clone https://github.com/shipwright-io/build $tmp && cd $tmp
make test-e2e
