## Changes

<!-- What changed and why. -->

## Checklist

- [ ] `make check` passes locally (build, lint, `go test -race`).
- [ ] Subprocesses use `exec.Command(name, args...)`; no input from flags, AWS responses or env reaches a shell.
- [ ] Dependency direction stays `main` → `cmd` → `internal`; `internal/` has no CLI/UI imports.
- [ ] New or changed logic has tests (fuzz tests for parsing of external data).
