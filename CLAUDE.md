# CLAUDE.md

miniecs is a Go CLI that lists AWS ECS resources and opens a shell in a
container through ECS Exec (`session-manager-plugin`).

## Layout

- `main.go` calls `cmd.Execute()` only.
- `cmd/`: cobra commands, flags, fuzzy finder and table output.
- `internal/pkg/aws/ecs/`: AWS ECS data access. No CLI or UI code.

## Commands

- `make check`: build, lint (golangci-lint) and `go test -race`. Same gates as CI.
- `make hooks`: enable the pre-push hook that runs `make check`.
- Fuzz locally: `go test -run '^$' -fuzz <FuzzName> -fuzztime 30s ./internal/pkg/aws/ecs`

## Code Conventions

- Dependency direction is `main` → `cmd` → `internal`. `internal/` must not
  import `cmd/`, cobra, go-fuzzyfinder or tablewriter (enforced by depguard).
- Use AWS SDK for Go v2 only.
- Return errors wrapped with `%w`. Compare with `errors.Is` / `errors.As`
  (enforced by errorlint). Only `cmd/` may call `log.Fatal`.
- Access AWS through the `ECSClient` interface so tests can use mocks.
- New parsing of external data (ARNs, JSON, names) needs a unit test, and a
  fuzz test that checks invariants when the input space is open-ended.
- Do not lower the coverage gate or raise its tolerance to make CI pass.
  Add tests instead.

## Security

- Run subprocesses with `exec.Command(name, args...)`. Never pass a string
  built from flags, AWS responses or environment variables to `sh -c` or
  any other shell.
- Do not log or print credentials, session tokens or the ECS Exec session
  JSON.
- Do not add `//nolint` for gosec. If a finding is a false positive, exclude
  the rule in `.golangci.yml` with a comment that explains why.
- GitHub Actions: pin every `uses:` to a full commit SHA with a version
  comment, keep `permissions` minimal per job, and pass `${{ }}` values to
  `run:` steps through `env:`.
- Do not bypass hooks or checks (`--no-verify`, `|| true`, skipping tests).
