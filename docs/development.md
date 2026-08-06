---
title: Diskforge Development
status: active
date: 2026-08-06
owners:
  - ioplane/diskforge-maintainers
tags:
  - development
  - containers
  - testing
---

# Diskforge Development

## Supported environment

Every code-generating, formatting, dependency, analysis, test, and build tool
runs inside `localhost/ioplane/diskforge-dev:1.26.5`, built from the pinned
`deployments/containers/development.Containerfile`. Podman is the supported OCI
runtime. Host-installed Go, Python, Node.js, linters, and release tools are
outside the evidence boundary.

On macOS, configure a rootful Podman Desktop machine and invoke the Podman
Desktop binary explicitly when multiple installations exist:

```console
/opt/podman/bin/podman version
/opt/podman/bin/podman info
```

## Build the development image

Build the image once. Compose services reference it without a `build` section
so parallel service creation cannot trigger duplicate image builds.

```console
/opt/podman/bin/podman build --pull=never \
  --tag localhost/ioplane/diskforge-dev:1.26.5 \
  --file deployments/containers/development.Containerfile .
```

The image contains exact tool versions and pinned multi-stage base digests.
Dependency updates change the pin, documentation, and verification evidence in
the same pull request.

## Verification services

| Service | Evidence |
| --- | --- |
| `format` | gofumpt and goimports formatting |
| `lint` | golangci-lint v2 strict default-all policy |
| `lint-integration` | Same policy with the `integration` build tag |
| `gopls` | Whole-module language-server diagnostics |
| `test` | Deterministic unit tests without cache |
| `race` | Unit tests with the race detector |
| `fuzz` | Image verifier and decoder fuzz smoke test |
| `vuln` | govulncheck against the current advisory database |
| `docs` | Markdown, spelling, and strict YAML checks |
| `integration` | Privileged isolated loop-device write acceptance |

Run services sequentially on Podman Desktop:

```console
/opt/podman/bin/podman compose run --rm format
/opt/podman/bin/podman compose run --rm lint
/opt/podman/bin/podman compose run --rm lint-integration
/opt/podman/bin/podman compose run --rm gopls
/opt/podman/bin/podman compose run --rm test
/opt/podman/bin/podman compose run --rm race
/opt/podman/bin/podman compose run --rm fuzz
/opt/podman/bin/podman compose run --rm vuln
/opt/podman/bin/podman compose run --rm docs
/opt/podman/bin/podman compose run --rm integration
```

Independent analyzers can use the same read-only service image:

```console
/opt/podman/bin/podman compose run --rm test gosec -quiet ./...
/opt/podman/bin/podman compose run --rm test staticcheck ./...
/opt/podman/bin/podman compose run --rm test deadcode -test ./...
```

## Static release build

Release configuration builds with `CGO_ENABLED=0`, `-trimpath`, and deterministic
linker flags. A local verification build writes only to ignored `dist/`:

```console
/opt/podman/bin/podman run --rm --network=none \
  --env CGO_ENABLED=0 --env GOOS=linux --env GOARCH=amd64 \
  --volume "$PWD:/workspace:ro" --volume "$PWD/dist:/out:rw" \
  --workdir /workspace localhost/ioplane/diskforge-dev:1.26.5 \
  go build -trimpath -ldflags='-s -w -buildid=' \
  -o /out/diskforge ./cmd/diskforge
```

Use the image's `file` utility to prove the output is a static Linux AMD64 ELF.

## API validation

Library and syscall changes are checked against current Context7 documentation,
the pinned module's `go doc` output, and gopls diagnostics. Record material API
ownership or compatibility decisions under `docs/architecture/`.
