---
title: Diskforge CLI Reference
status: active
date: 2026-08-06
owners:
  - ioplane/diskforge-maintainers
tags:
  - cli
  - automation
---

# Diskforge CLI Reference

## Streams and exit codes

Successful commands emit one JSON object to stdout, except plain `version`.
`write` emits newline-delimited progress objects to stderr. Failures emit one
structured error object to stderr.

| Exit code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Operational I/O or environment failure |
| `2` | Invalid command, flag, or request syntax |
| `3` | Safety gate refusal |
| `4` | Context cancellation or deadline |

Error envelope:

```json
{
  "error": {
    "kind": "gate",
    "code": "target_mounted",
    "message": "target_mounted: target descendant vdb1 is mounted"
  }
}
```

Automation MUST branch on the exit code, `kind`, and optional `code`; it MUST
NOT parse `message`.

## `inspect`

Observes the target, source filesystem, mounts, swap, and mode-specific host
state without opening the target writable.

```text
diskforge inspect \
  --mode <rescue|live> \
  --target </dev/name> \
  --image </absolute/source> \
  --sha256 <64-lowercase-hex> \
  --expected-bytes <positive-integer>
```

The result contains the immutable inspection and confirmation token. Inspection
stats the image but does not establish a trusted source descriptor; `write`
repeats inspection and complete verification.

## `stage`

Downloads and atomically publishes a bounded source image.

```text
diskforge stage \
  --url <absolute-http-or-https-url> \
  --destination </canonical/absolute/path> \
  --sha256 <64-lowercase-hex> \
  --maximum-bytes <positive-bounded-integer>
```

URLs with user information are rejected. The maximum applies to compressed
download bytes. The destination becomes visible only after complete download,
SHA-256 verification, file synchronization, atomic rename, and directory
synchronization.

## `write`

Repeats inspection, verifies the complete source, validates confirmation, and
executes the guarded write.

```text
diskforge write \
  --mode <rescue|live> \
  --target </dev/name> \
  --image </absolute/source> \
  --sha256 <64-lowercase-hex> \
  --expected-bytes <positive-integer> \
  --confirmation <exact-inspection-token> \
  [--dry-run] \
  [--reboot]
```

`--dry-run` performs full inspection and source verification without requiring
confirmation and without opening the target. `--reboot` is mandatory for a
non-dry-run live operation and has no effect on rescue policy.

Progress objects have stable fields:

```json
{"written_bytes":4194304,"expected_bytes":34359738368}
```

## `version`

```text
diskforge version [--json]
```

Plain output is human readable. JSON output contains `version`, `commit`,
`build_date`, `go_version`, `os`, and `arch`. Release builds inject version,
full commit, and commit date through linker flags.
