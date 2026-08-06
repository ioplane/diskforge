# Pull request

## Outcome

<!-- Describe the observable result, not the implementation sequence. -->

## Safety and compatibility

- [ ] I identified changes to destructive ordering, refusal behavior, trust
      boundaries, or confirmed identity fields.
- [ ] I identified changes to exported Go API, JSON, CLI, exit codes, gate
      codes, confirmation tokens, or artifact names.
- [ ] New behavior started with a failing test and includes negative coverage.
- [ ] Public behavior and contracts are documented in English.

## Container verification

<!-- List exact Podman Compose services and results. -->

- [ ] `format`
- [ ] `lint`
- [ ] `lint-integration`
- [ ] `gopls`
- [ ] `test`
- [ ] `race`
- [ ] `fuzz`
- [ ] `vuln`
- [ ] `docs`
- [ ] `integration` when destructive or Linux behavior changed

## Release note

<!-- Use a Conventional Commit title and explain user-visible impact. -->
