# Container build definitions

This directory owns the reproducible OCI environments used to develop, verify,
and release Diskforge. Neither definition produces a runtime image for writing
disks.

`development.Containerfile` provides the pinned Go toolchain, language server,
linters, analyzers, documentation tools, and test utilities used by Compose and
continuous integration.

`release.Containerfile` provides the pinned GoReleaser, Syft, and Cosign tools
used to assemble, describe, sign, and publish release artifacts.

Build both local images with Podman Desktop from the repository root:

```console
/opt/podman/bin/podman build --pull=never \
  --tag localhost/ioplane/diskforge-dev:1.26.5 \
  --file deployments/containers/development.Containerfile .
/opt/podman/bin/podman build --pull=never \
  --tag localhost/ioplane/diskforge-release:2.17.1 \
  --file deployments/containers/release.Containerfile .
```

The repository root remains the build context because the development image
copies `go.mod` and `go.sum` to seed its immutable module cache.
