# `rbac-audit`

Have you ever wondered whether your controller actually needs all the permissions it has granted to it? Wonder no more!

This repo contains scripts to start a [KinD](https://kind.sigs.k8s.io) cluster configured to keep audit logs for API resource access.
You can run e2e tests against this cluster to exercise your system, then run `main.go` to generate a readable RBAC policy for your controller's service account.

The tools generate two RBAC policies:

1. A namespaced `Role` for the namespace the controller runs inside, consisting of requests for resources in that SA's namespace
1. A cluster-scoped `ClusterRole` for any resources accessed outside of the controller namespace.

## Caveats

- Only API requests made during e2e tests are considered; you _have_ written comprehensive e2e tests, right? ..._Right?!_
- It's an early demo, not yet generalized for any system.
- This is hacky, buggy software. Don't rely on it for anything mission-critical. Manually inspect the diff and use your human brain.

## Examples

### [Tekton Pipelines](https://github.com/tektoncd/pipeline)

```
./kind_audit_cluster.sh       # setup cluster
./tekton-e2e.sh               # run e2e tests
go run ./ > tekton-rbac.yaml  # generate RBAC policy
```

See [tekton-rbac.yaml](./tekton-rbac.yaml)

### [Shipwright Build](https://github.com/shipwright-io/build)

```
./kind_audit_cluster.sh shipwright-build shipwright-build-controller
./shipwright-e2e.sh
go run ./ --ns shipwright-build --s system:serviceaccount:shipwright-build:shipwright-build-controller > shipwright-rbac.yaml
```

See [shipwright-rbac.yaml](./shipwright-rbac.yaml)

## TODO

- concisely diff two policies to determine gaps (canonicalizing RBAC rule YAMLs)
- generate Markdown from RBAC policies to easily communicate permissions to users

## Acknowledgements

This work is _heavily_ inspired by https://github.com/liggitt/audit2rbac, though I didn't know about it when I started writing this. That repo has a _fantastic_ demo video.

This wouldn't be possible without KinD. ❤️
