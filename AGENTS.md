# AGENTS.md

GitReleaser (`releaser`): Go CLI for independent per-service SemVer releases in a monorepo, using only Git tags (`<service>/v<semver>`) as source of truth. Config: `releaser.yml` at the repo root.

## Layout

- `cmd/releaser/` — entrypoint; maps errors to exit codes via `cli.ExitCode`
- `internal/cli/` — cobra commands (`run.go`, `template.go`, ...)
- `internal/config/` — `releaser.yml` parsing and validation
- `internal/service/` — affected detection, latest release, next version
- `internal/version/` — SemVer handling (Masterminds/semver)
- `internal/git/` — thin wrapper over the `git` binary (`exec.Command`)

## Commands

```sh
go test ./...
go vet ./...
gofmt -l .
CGO_ENABLED=0 go build -o releaser ./cmd/releaser
```

## Rules

- `SPECS.md` is the authoritative behaviour spec: read the relevant section before changing behaviour and keep it in sync.
- User-visible changes go in `CHANGELOG.md` under `[Unreleased]` (Keep a Changelog); update `README.md` when usage changes.
- Exit codes (SPECS §37): `0` ok, `1` validation/state, `2` config, `3` Git, `4` reserved. On error write nothing to stdout.
- Stdout is for machine-consumable output only; diagnostics go to stderr.
- Git is invoked via the CLI only, no Git libraries. No new dependencies without a good reason; keep the binary static (`CGO_ENABLED=0`).
- Tests use real temporary Git repos (see helpers in `internal/cli/regression_test.go`); add a regression test for every bug fix.
- Code, comments, docs and commit messages in English.
- Don't commit `releaser` (built binary) or `git-test/`.
