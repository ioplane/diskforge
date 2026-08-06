# Repository Settings

This file documents settings that cannot be enforced from the source tree.

## General

- Visibility: public
- Default branch: `main`
- Issues, Discussions, and private vulnerability reporting: enabled
- Projects and Wiki: disabled
- Merge method: squash only
- Automatically delete head branches: enabled
- Web commit signoff: enabled

## Branch protection for `main`

- Require changes from non-administrators to satisfy protected-branch checks.
- Do not require approving reviews while the project has a single maintainer.
- Require conversation resolution.
- Require linear history.
- Require branches to be up to date.
- Require the `required` and `Dependency review` checks from GitHub Actions.
- Block force pushes and deletions.
- Permit administrator bypass for repository recovery and automated release
  maintenance.
- Restrict tag creation for `v*` to repository administrators and automation.

## Actions and security

- Allow GitHub-authored, verified-creator, and explicitly approved actions only.
- Require actions to be pinned to full commit SHAs.
- Default workflow token permission: read repository contents.
- Allow Actions to create and approve pull requests only for Release Please.
- Enable Dependabot alerts and security updates, secret scanning, push
  protection, dependency graph, and code scanning.
- Register `RELEASE_PLEASE_TOKEN` only if release event propagation cannot use
  the workflow-call output path; scope it to this repository.

Settings are reviewed after workflow names or required checks change and before
every first release on a new major line.
