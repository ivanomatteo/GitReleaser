# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.4]

### Added

- `run [service] <script> [-- args...]` command that runs a script for each service (or only for the given one, or with `--affected` only for affected services), exposing `RELEASER_NAME`, `RELEASER_PATHS`, `RELEASER_DEPS`, `RELEASER_VERSION`, `RELEASER_TAG` (empty if not available) and `RELEASER_VAR_<KEY>`. It stops at the first error, propagating the script's exit code.
- `template [service] <file> [-o <path>]` command that executes a Go template for each service (or only for the given one) with the fields `.Name`, `.Paths`, `.Deps`, `.Version`, `.Tag` and `.Vars` (with `-` replaced by `_` in the keys, for example `.Vars.docker_file`). Output goes to stdout or, with `-o`, to one file per service whose path is itself a template.

### Changed

- Go 1.27 or later is required; dependencies updated (`Masterminds/semver/v3` 3.5.0, `spf13/cobra` 1.10.2, `spf13/pflag` 1.0.10).
- The default `releaser.yml` is looked up in the directory given by `--repo` instead of the current directory. An explicitly passed relative `--config` stays relative to the current directory.
- Unknown keys in `releaser.yml` are a configuration error (exit code 2): a typo such as `dependecies` no longer silently disables dependencies.
- Service names must be valid as a Git tag component (no spaces, `~ ^ : ? * [ \`, `..`, `@{`, and no leading `.`/`-` or trailing `.`/`.lock`).
- `release --version` rejects a version that is not greater than the latest release; `--force` allows creating the tag anyway, but it does not become the latest release.
- Untracked files no longer block a release: the working tree is considered clean if there are no changes to tracked files.
- With `--push`, all created tags are published with a single `git push --atomic`. If creation or push fails, the local tags created by the command are deleted, so the command can be rerun.
- `release <service>` without a bump or `--version` now reports the missing argument immediately instead of the "not affected" error.

### Fixed

- Files with non-ASCII characters, spaces or quotes in their names were not recognized as service changes, because of `git diff` path quoting. Detection now uses `git diff-tree -z`, independent of the user's Git configuration.
- In a shallow clone, missing history was reported as "release tag is not an ancestor of HEAD" (exit code 1); it is now reported as incomplete history (exit code 3), also with `--root`.
- `release --root` inferred wrong prefixes from tags such as `dev1.0.0` (prefix `de`). An inferred prefix must now be empty or end with `/`, `-`, `_` or `.`.
- For a rename, `changes` also reported the path outside the service; it now lists only the relevant paths.
- A push failure during a bulk release could leave tags only partially published.

### Documentation

- SPECS.md aligned with the implementation: `internal/version` package, `remote` and `ignore` keys, required `paths`, `status --verbose` format for unreleased services, `--new` exception for `--version` in bulk mode, exit codes, `ignore` glob syntax, rename handling, prereleases and build metadata.
- README updated with configuration resolution, validation, atomic push, the `--root` prefix rule, prereleases and working tree.

## [0.1.3] - 2026-08-20

### Added

- Per-service `vars` section in `releaser.yml`: a map of string keys and values.
- `get-var <service> <key>` command, which prints only the value and can also be used outside a Git repository.

## [0.1.2] - 2026-08-18

### Added

- `release --new [--version <semver>]`: creates on `HEAD` the first tag (default `0.1.0`) for every configured service that does not have one yet. The command is idempotent.

### Documentation

- README expanded with rationale, installation, configuration and usage examples.

## [0.1.1] - 2026-08-12

### Added

- Prebuilt binaries for macOS (`darwin/amd64` and `darwin/arm64`).

## [0.1.0] - 2026-08-12

First release.

### Added

- Independent versioning of monorepo services through `<service>/v<semver>` Git tags, with strict SemVer parsing and no `VERSION` files.
- `releaser.yml` configuration with per-service `paths`, `dependencies` and `ignore`, global `ignore` and `remote`; `config check` command.
- Query commands `status [service] [--verbose]`, `affected`, `changes <service>` and `plan [--format json]`, with an independent baseline per service.
- Scripting primitives `version-number`, `version-tag`, `next-version-number` and `next-version-tag`.
- `release <service> <patch|minor|major>` and `release <service> --version <semver>`, with annotated tags, `--dry-run` and `--push`.
- Affected check in `release`, which can be bypassed with `--force`.
- Bulk releases with `release --affected <bump>` and `release --all --force <bump>`, fully validated before any tag is created.
- `release --root` for standard repositories, with no configuration, tag prefix inference and `--prefix`.
- Distinct exit codes for validation (1), configuration (2) and Git (3) errors.
- Multi-platform builds with GoReleaser and a GitHub Actions workflow to publish releases.

[Unreleased]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/ivanomatteo/GitReleaser/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/ivanomatteo/GitReleaser/releases/tag/v0.1.0
