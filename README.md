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

### [Shipwright Build](https://github.com/shipwright-io/build) and other systems

By default, the tools track access by the `tekton-pipelines-controller` SA, in the `tekton-pipelines` namespace.

You can override these with args to `./kind_audit_cluster.sh` and `main.go`:

```
./kind_audit_cluster.sh shipwright-build shipwright-build-controller
./shipwright-e2e.sh  # run e2e tests
go run ./ \
    --namespace shipwright-build \
    --serviceaccount shipwright-build-controller > shipwright-rbac.yaml
```

See [shipwright-rbac.yaml](./shipwright-rbac.yaml)

## Known Issues

- The [`OwnerReferencesPermissionEnforcement`](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#ownerreferencespermissionenforcement) admission controller requires additional permissions to be able to set `OwnerReferences` on objects, which rbac-audit won't detect.
  If your controller sets `OwnerReferences`, especially with `blockOwnerDeletion`, and you expect to have this admission controller enabled, take this into consideration.

## TODO

- attempt to further limit policies to only `resourceNames` that are accessed
- concisely diff two policies to determine gaps (canonicalizing RBAC rule YAMLs)
- generate Markdown from RBAC policies to easily communicate permissions to users
- replicate audit2rbac's awesome demo
- have `kind_audit_cluster.sh` replace your previous kubeconfig when it's done with the cluster

## Acknowledgements

This work is _heavily_ inspired by https://github.com/liggitt/audit2rbac, though I didn't know about it when I started writing this. That repo has a _fantastic_ demo video.

This wouldn't be possible without KinD. ❤️
