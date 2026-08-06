---
title: Go-Native Repository Layout
status: accepted
date: 2026-08-06
owners:
  - ioplane/diskforge-maintainers
tags:
  - architecture
  - repository
  - tooling
---

# Go-Native Repository Layout

<!-- cspell:words containerignore -->

## Decision

Diskforge keeps its stable importable package at the module root and its
installable command under `cmd/diskforge`. Private implementation packages
remain under `internal`. Repository infrastructure is grouped by purpose so
that the root exposes the module, its primary documentation, and the canonical
Compose entry point instead of every tool-specific asset.

The layout follows the official Go guidance for a repository that contains an
importable package and a command. It deliberately does not introduce `pkg`,
`src`, `lib`, a second Go module, shell scripts, a Makefile, or another task
runner configuration.

## Goals

- Preserve `github.com/ioplane/diskforge` as the public package import path.
- Keep the root readable without hiding stable Go API files behind another
  directory.
- Make container, test, documentation, governance, and tooling ownership clear.
- Preserve automatic GitHub discovery of community health files.
- Keep every build, test, lint, and release operation inside OCI containers.
- Prevent accidental top-level files from gradually recreating root clutter.
- Preserve file history through Git-aware moves.

## Target layout

```text
.
├── .config/
│   ├── cspell.json
│   ├── golangci.yml
│   ├── markdownlint-cli2.yaml
│   ├── yamllint.yml
│   ├── goreleaser.yaml
│   └── release-please/
│       ├── config.json
│       └── manifest.json
├── .github/
│   ├── CODE_OF_CONDUCT.md
│   ├── CONTRIBUTING.md
│   ├── GOVERNANCE.md
│   ├── MAINTAINERS.md
│   ├── SECURITY.md
│   ├── SUPPORT.md
│   ├── ISSUE_TEMPLATE/
│   └── workflows/
├── cmd/diskforge/
├── deployments/containers/
│   ├── README.md
│   ├── development.Containerfile
│   └── release.Containerfile
├── docs/
│   ├── README.md
│   ├── architecture/
│   │   ├── README.md
│   │   └── 0001-*.md
│   └── contracts/
├── internal/
│   ├── image/
│   ├── linux/
│   ├── naming/
│   ├── policy/
│   └── version/
├── test/
│   ├── integration/
│   │   ├── diskforge_test.go
│   │   └── testdata/proc-swaps
│   └── repository/layout_test.go
├── diskforge.go
├── diskforge_internal_test.go
├── diskforge_test.go
├── errors.go
├── types.go
├── types_test.go
├── go.mod
├── go.sum
├── compose.yaml
├── README.md
├── CHANGELOG.md
├── LICENSE
└── NOTICE
```

The root also retains `.containerignore`, `.editorconfig`, `.gitattributes`,
and `.gitignore` because their consumers discover them at the build context or
repository root.

## Package boundaries

The module root remains package `diskforge`. Moving it would change the public
import path and add a directory that conveys no additional ownership boundary.
The root package owns only exported requests, results, errors, options, and the
`Engine` facade.

`cmd/diskforge` owns CLI parsing and presentation. Packages below `internal`
remain inaccessible to external modules and can be refactored without creating
a public compatibility commitment. No implementation package moves merely to
make the tree look more elaborate.

The privileged public-API acceptance test moves from the module root to
`test/integration`. Its package imports `github.com/ioplane/diskforge` exactly
as an external consumer does. Its controlled procfs fixture resides in the
adjacent `testdata` directory, which the Go tool ignores as a package.

## Infrastructure ownership

OCI build definitions live in `deployments/containers`. The filenames describe
their roles instead of encoding a role as a suffix at the repository root.
`compose.yaml` remains at the root because it is the canonical developer and CI
entry point and benefits from standard Compose discovery.

Tool configuration lives under `.config`. Every tool that does not natively
discover its new path is invoked with an explicit configuration argument.
GoReleaser accepts `.config/goreleaser.yaml` natively, but Diskforge still uses
an explicit path in Compose so that the contract is visible. Release Please is
given explicit `config-file` and `manifest-file` inputs.

Generated release output moves from `dist` to `.artifacts/release`. The entire
`.artifacts` tree is ignored by Git. CI attestation paths and local artifact
verification use the same location.

## Community and documentation ownership

GitHub-supported community health files move to `.github`, which GitHub checks
before the repository root and `docs`. `MAINTAINERS.md` moves with the other
governance documents even though it is a Diskforge convention rather than a
GitHub-recognized health-file type. README, CODEOWNERS, issue templates, and
documentation links use the new paths.

`docs/architecture.md` becomes `docs/architecture/README.md`. This removes the
file-and-directory name collision while retaining the architecture overview as
the index for decision records. The accepted layout decision is this record;
no tool- or agent-specific design directory is added.

## Root layout contract

`test/repository/layout_test.go` enforces the approved top-level allowlist. It
permits the Git metadata directory and the ignored `.artifacts` output tree but
rejects unexpected top-level files or directories. This is a Go test executed
inside the development container; no shell validation script is introduced.

The contract also checks that required files and directories exist and that
obsolete locations such as root container definitions, root community health
files, `dist`, `pkg`, `src`, and `scripts` are absent.

## Migration map

| Current path | Target path |
| --- | --- |
| `Containerfile.dev` | `deployments/containers/development.Containerfile` |
| `Containerfile.release` | `deployments/containers/release.Containerfile` |
| `integration_test.go` | `test/integration/diskforge_test.go` |
| `testdata/integration/proc-swaps` | `test/integration/testdata/proc-swaps` |
| `docs/architecture.md` | `docs/architecture/README.md` |
| Root community health Markdown | `.github/<same-name>.md` |
| `.cspell.json` | `.config/cspell.json` |
| `.golangci.yml` | `.config/golangci.yml` |
| `.markdownlint-cli2.yaml` | `.config/markdownlint-cli2.yaml` |
| `.yamllint.yml` | `.config/yamllint.yml` |
| `.goreleaser.yaml` | `.config/goreleaser.yaml` |
| `release-please-config.json` | `.config/release-please/config.json` |
| `.release-please-manifest.json` | `.config/release-please/manifest.json` |
| `dist/` | `.artifacts/release/` |

Tracked files are moved with `git mv`. Generated `dist` content is removed only
after its reproducibility has been proven by a fresh release snapshot.

## Verification contract

The migration is complete only when all of the following evidence is current:

- the public package import path and exported API diff are unchanged;
- `go list`, unit, race, fuzz, and privileged integration gates pass;
- golangci-lint, integration lint, gopls, Staticcheck, gosec, dead-code, and
  govulncheck gates pass;
- Markdown, spelling, YAML, actionlint, Compose, and GoReleaser checks pass;
- the repository layout test passes from a clean checkout;
- a GoReleaser snapshot produces the expected static Linux AMD64 archive,
  checksums, source archive, and two SPDX documents below `.artifacts/release`;
- GitHub still recognizes the community profile and both required branch checks;
- Release Please updates its existing `v0.1.0` pull request from the relocated
  configuration without replacing or duplicating it.

The first release remains blocked until the migrated tree passes the complete
local OCI gate and Blacksmith CI.

## Alternatives rejected

### Move the public package below `pkg/diskforge`

Rejected because it would change the import path to
`github.com/ioplane/diskforge/pkg/diskforge` and create an unnecessary public
package layer. The official Go module layout keeps the primary importable
package at the module root.

### Keep all configuration and governance files at the root

Rejected because it preserves the current navigation problem and gives
tool-specific metadata the same prominence as the public module contract.

### Minimize the root at any cost

Rejected because moving `compose.yaml`, core module documentation, public Go
files, or root-discovered repository metadata would trade clarity for explicit
flags and surprising paths.

## References

- [Organizing a Go module](https://go.dev/doc/modules/layout)
- [Go internal package rules](https://pkg.go.dev/cmd/go#hdr-Internal_Directories)
- [GitHub community health files](https://docs.github.com/en/communities/setting-up-your-project-for-healthy-contributions/creating-a-default-community-health-file)
- [GoReleaser configuration discovery](https://goreleaser.com/customization/)
- [Release Please action inputs](https://github.com/googleapis/release-please-action#action-inputs)
