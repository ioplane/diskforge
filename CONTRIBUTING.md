---
title: Contributing to Diskforge
status: active
date: 2026-08-06
owners:
  - ioplane/diskforge-maintainers
tags:
  - contributing
  - development
---

# Contributing to Diskforge

Diskforge writes whole block devices. Changes are accepted only when their
safety effect is explicit, tested, and reviewable.

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md)
and license contributions under [Apache-2.0](LICENSE).

## Before opening a change

- Use an issue for new behavior, public API changes, safety-policy changes, or
  new release targets.
- Use a private security advisory for a vulnerability. Do not open a public
  issue; follow [SECURITY.md](SECURITY.md).
- Keep pull requests focused. Separate mechanical changes from behavior.
- Do not include real credentials, private image names, host identifiers,
  production disk inventories, or proprietary images in fixtures.

## Commit and compatibility contract

Commits MUST follow
[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```text
feat(policy): reject a newly ambiguous target relation
fix(image): preserve cancellation during source verification
docs(safety): explain the live reboot gate
```

Use `!` and a `BREAKING CHANGE:` footer for incompatible public API, JSON, CLI,
gate-code, or artifact-name changes. Release Please derives the SemVer change
and changelog from these commits.

Public names MUST follow the mandatory
[naming contract](docs/contracts/naming.md). Human-readable error messages may
improve; callers must use typed errors and stable gate codes.

## Development environment

All formatting, dependency operations, builds, analysis, and tests run inside
the pinned OCI image. Do not submit evidence from host-installed Go tools.

On macOS with Podman Desktop, use its binary explicitly:

```console
/opt/podman/bin/podman build --pull=never \
  --tag localhost/ioplane/diskforge-dev:1.26.5 \
  --file Containerfile.dev .
```

Run one-shot Compose services sequentially:

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

The integration service requires a rootful Podman machine and privileged
container. It allocates a free loop device, creates a bounded temporary backing
file, compares every written byte, detaches the loop device, and removes the
device node it created. It never accepts a host block device path.

## Testing expectations

Behavior changes start with a failing test. Safety-policy changes need table
tests for acceptance and refusal. I/O changes need short read/write, surplus
data, cancellation, synchronization failure, and descriptor-lifetime coverage
as applicable.

Changes touching the destructive path MUST preserve and test this order:

1. observe the complete host and target state;
2. verify the complete source through one retained descriptor;
3. validate target-bound confirmation;
4. prepare live mode, when selected;
5. open and revalidate the whole block device;
6. stream exactly the declared expanded bytes;
7. synchronize and flush;
8. reboot immediately in live mode.

No unit test may open a real block device. The integration test may use only
the loop device it created from its own temporary file.

## Pull requests

Complete the pull-request template and include:

- the user-visible outcome;
- the changed safety assumptions and trust boundaries;
- exact container commands and results;
- compatibility and release-note impact;
- documentation updates for public behavior.

Maintainers may request narrower commits, additional negative tests, or an
architecture decision record before accepting a change. A green CI run is
necessary but does not replace review of destructive behavior.
