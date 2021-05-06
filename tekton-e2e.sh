#!/bin/sh

set -euxo pipefail

# install tekton
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

# clone repo, run e2e tests
tmp=$(mktemp -d)
git clone https://github.com/tektoncd/pipeline $tmp && cd $tmp
export SYSTEM_NAMESPACE=tekton-pipelines
go test -tags=e2e -timeout=20m ./test/
