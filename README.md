# lord-helmchen

```sh
# Converts a local Helm chart into a CRD schema and applies it to the cluster.
go run ./schemagen ../vshnjuiceshop | ka apply -f-

# Converts a Helm chart from an OCI registry into a CRD schema and applies it to the cluster.
go run ./schemagen oci://ghcr.io/stefanprodan/charts/podinfo:6.14.0 | k apply -f-
```

## Source controller

```sh
k apply -k https://github.com/fluxcd/source-controller//config/default
k apply -k https://github.com/fluxcd/image-reflector-controller//config/default

k apply -f- <<EOF
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: podinfo
  namespace: default
spec:
  interval: 5m0s
  url: https://stefanprodan.github.io/podinfo
EOF

k apply -f- <<EOF
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: podinfo
  namespace: default
spec:
  interval: 5m0s
  type: oci
  url: oci://ghcr.io/stefanprodan/charts/podinfo
EOF

k apply -f- <<EOF
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: podinfo
  namespace: default
spec:
  interval: 5m0s
  url: oci://ghcr.io/stefanprodan/charts/podinfo
  ref:
    tag: 6.14.0
EOF

k apply -f- <<EOF
apiVersion: image.toolkit.fluxcd.io/v1
kind: ImageRepository
metadata:
  name: podinfo-oci
  namespace: default
spec:
  image: ghcr.io/stefanprodan/charts/podinfo
  interval: 5m
  exclusionList:
  - ^.*\.sig$
  - ^sha256-.+$
---
apiVersion: image.toolkit.fluxcd.io/v1
kind: ImagePolicy
metadata:
  name: podinfo-oci-v6
  namespace: default
spec:
  imageRepositoryRef:
    name: podinfo-oci
  digestReflectionPolicy: IfNotPresent
  policy:
    semver:
      range: "6.*"
---
apiVersion: image.toolkit.fluxcd.io/v1
kind: ImagePolicy
metadata:
  name: podinfo-oci-v5
  namespace: default
spec:
  imageRepositoryRef:
    name: podinfo-oci
  digestReflectionPolicy: IfNotPresent
  policy:
    semver:
      range: "5.x"
EOF
```
