#!/bin/sh

set -euxo pipefail

ns=${1:-tekton-pipelines}
sa=${2:-tekton-pipelines-controller}

rm /tmp/audit/*.log || true

mkdir -p /tmp/audit
cat << EOF > /tmp/audit/policy.yaml
apiVersion: audit.k8s.io/v1beta1
kind: Policy
rules:
- level: Metadata
  users: ["system:serviceaccount:${ns}:${sa}"]
  stages:
  - ResponseComplete
EOF

cat /tmp/audit/policy.yaml

kind delete cluster --name=audit || true
kind create cluster --config=config.yaml --name=audit
