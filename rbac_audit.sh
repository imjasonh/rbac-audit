#!/bin/sh

set -euxo pipefail

function cleanup() {
  kind delete cluster --name=audit
}

cleanup || true
rm /tmp/audit/*.log || true

mkdir -p /tmp/audit
cp policy.yaml /tmp/audit/policy.yaml

kind create cluster --config=config.yaml --name=audit

trap "cleanup" SIGINT SIGTERM

echo "### RUNNING $@ ###"
"$@"

echo audit log at /tmp/audit/*.log

go run ./

cleanup
