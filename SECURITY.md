---
title: Diskforge Security Policy
status: active
date: 2026-08-06
owners:
  - ioplane/diskforge-maintainers
tags:
  - security
  - vulnerability-reporting
---

# Diskforge Security Policy

## Reporting a vulnerability

Report vulnerabilities through
[GitHub private vulnerability reporting](https://github.com/ioplane/diskforge/security/advisories/new).
Do not open a public issue, pull request, or discussion before coordinated
disclosure.

Include the affected version or commit, deployment mode, prerequisites,
observable impact, and a minimal reproducer when safe. Do not include real
credentials, private images, or production device identities.

Maintainers target acknowledgment within 3 business days, initial assessment
within 7 calendar days, and a remediation or coordinated-disclosure plan within
30 calendar days. Complex issues may require more time; the reporter will
receive status updates at least every 14 days.

## Supported versions

Before `v1.0.0`, only the newest released minor line receives security fixes.
After `v1.0.0`, the current major release and the immediately preceding major
release receive fixes when technically practical. Unreleased commits and
modified binaries are evaluated at maintainer discretion.

## Security properties

Security reports are especially valuable when Diskforge can:

- open a target writable before every applicable gate succeeds;
- write to a partition, mounted target, swap target, or source backing device;
- accept a confirmation token for a different target or source identity;
- follow a replaced target symlink or write through a changed device identity;
- write unverified, short, surplus, or changed source content;
- exceed documented Zstandard memory or window limits;
- continue a live-root overwrite without mandatory synchronization and reboot;
- expose release credentials or publish unverifiable artifacts;
- allow path, JSON, archive, or workflow injection through an identifier.

The safety policy is not a replacement for operator authorization, physical
security, verified source provenance, backups, or an isolated rescue system.
Diskforge does not make an untrusted image safe to boot.

## Release verification

Every release provides SHA-256 checksums, SPDX SBOMs, a keyless signature, and
GitHub build provenance. Verify artifacts as described in
[docs/release.md](docs/release.md) before granting root access.

## Disclosure and credit

Maintainers coordinate disclosure dates and request CVEs when appropriate.
Reporter credit is included with consent. Reporters who need anonymity may say
so in the private advisory.
