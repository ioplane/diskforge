---
title: Diskforge Governance
status: active
date: 2026-08-06
owners:
  - ioplane/diskforge-maintainers
tags:
  - governance
---

# Diskforge Governance

Diskforge uses a maintainer-led governance model. Maintainers are listed in
[MAINTAINERS.md](MAINTAINERS.md) and own releases, security response, repository
settings, and final merge decisions.

## Decision making

Routine changes are decided through pull-request review. Maintainers seek
technical consensus and record material architecture, safety-policy, public
API, or release-process decisions under `docs/architecture/`.

When consensus is not reached, a maintainer documents the alternatives, safety
impact, compatibility impact, and final decision. Destructive safety takes
priority over convenience and backward compatibility: Diskforge may introduce
a stricter refusal in a compatible release when evidence is ambiguous.

## Maintainer responsibilities

Maintainers:

- review destructive behavior and negative tests;
- protect release credentials and branch rules;
- coordinate vulnerability response;
- ensure releases follow SemVer and Conventional Commits;
- keep governance, support, and compatibility statements current;
- disclose conflicts of interest affecting a decision.

## Becoming or leaving as a maintainer

Existing maintainers may nominate contributors with sustained, technically
sound work and demonstrated safety judgment. A nomination requires unanimous
approval from active maintainers. Inactive maintainers may move to emeritus
status after six months without project activity. A maintainer may resign at
any time or be removed for a serious Code of Conduct or security breach.

## Changes to governance

Governance changes require a public pull request, approval from every active
maintainer, and a minimum seven-day review period unless an urgent security
incident requires temporary protective action.
