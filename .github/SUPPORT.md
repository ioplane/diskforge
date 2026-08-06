---
title: Diskforge Support
status: active
date: 2026-08-06
owners:
  - ioplane/diskforge-maintainers
tags:
  - support
---

# Diskforge Support

Diskforge is community maintained and has no commercial support commitment.

Use [GitHub Discussions](https://github.com/ioplane/diskforge/discussions) for
usage and design questions. Use the issue forms for reproducible bugs,
documentation defects, and feature proposals. Vulnerabilities must follow the
private process in [SECURITY.md](SECURITY.md).

## Include with a support request

- `diskforge version --json` output;
- Linux distribution and kernel version;
- rescue or live mode;
- the complete structured error with secrets and device serials redacted;
- whether the source is raw or Zstandard;
- the command shape with the confirmation token, digest, private URLs, and host
  identifiers redacted;
- minimal reproduction steps on a disposable loop-backed target.

Do not attach proprietary disk images, SSH keys, access tokens, private URLs,
or production storage inventories. Maintainers may close requests that cannot
be reproduced safely or that seek approval for a specific destructive
production operation.

## Scope

The project supports the documented Linux AMD64 release, public Go API, CLI,
safety policy, and release artifacts. Cloud provisioning, image authoring,
hypervisor configuration, operating-system support after boot, and recovery of
already overwritten data are outside project support.
