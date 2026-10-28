# Git Releaser

A Go CLI to version and release the microservices of a monorepo independently. Git tags in the `<service>/v<semver>` format are the only source of versions: the project does not use `VERSION` files.

It also supports standard repositories with the `--root` option; in that case its usefulness is obviously smaller, and no configuration file is needed.

## What problem does it solve?

In a monorepo with multiple microservices, the Git repository is shared, but releases are often independent.

One service may be at `2.4.1`, another at `1.8.3`, and a change to a `common` module may require releasing only some of the services.

The problem is therefore to easily understand:

* what the latest release of each service is;
* what changes happened since that release;
* which services are actually `affected`;
* what the next version or the next tag will be.

Many tools manage versioning by also keeping the version in the sources, for example in `package.json`, `pom.xml` or a `VERSION` file.

This project deliberately avoids a second piece of state to keep in sync and uses a single source of truth:

```text id="pazl0w"
Git tag = actual release
```

Each service uses namespaced tags:

```text id="3vctve"
api/v2.4.1
worker/v1.8.3
scraper/v0.6.0
```

To determine whether a service has changed, the tool compares `HEAD` with its latest tag:

```text id="qb4rxb"
api/v2.4.1..HEAD
worker/v1.8.3..HEAD
```

Each service therefore has an independent baseline, even though they all share the same Git history.

Shared dependencies are declared explicitly:

```yaml id="7kzr6a"
services:
  api:
    paths:
      - services/api
    dependencies:
      - common/auth
      - common/logging
```

If a dependency changes, the service is considered `affected` relative to its own latest release.

### Why not use a more complete tool?

There are tools such as Nx Release, Changesets, Lerna or release-please that offer much broader functionality: changelogs, Conventional Commits, publishing, automatic dependency graphs and integration with specific ecosystems.

This project is **deliberately minimal**.

It does not try to handle builds, deployment, package registries, changelogs or Git workflows.

It essentially does four things:

```text id="5bvgm2"
find the latest release
determine whether the service has changed
compute the next version
create the corresponding tag
```

It works only with:

```text id="puqgq7"
Git
paths
declared dependencies
SemVer
```

This is why it stays independent of the language, the build system and the CI/CD platform.

The underlying idea is simple:

```text id="u0ec5a"
monorepo != mono-version
repository != release unit
```

The repository represents the overall state of the code.

Tags, instead, represent the releases of the individual deployable units.


## Requirements and build

Go 1.27 or later and Git available in the `PATH` are required.

Prebuilt binaries for Linux, macOS and Windows are available in the [latest release on GitHub](https://github.com/ivanomatteo/GitReleaser/releases/latest).

```sh
CGO_ENABLED=0 go build -o releaser ./cmd/releaser
```

All commands operate on the current repository and read `releaser.yml`. Global options let you change both:

```text
-c, --config string   configuration file (default "releaser.yml")
    --repo string     Git repository directory (default ".")
```

The default `releaser.yml` is looked up in the directory given by `--repo`, so `releaser --repo ../monorepo status` works from any directory. An explicitly passed relative `--config`, instead, is resolved against the current directory.

## Configuration

```yaml
remote: origin

ignore:
  - docs/**
  - "**/*.md"

services:
  api:
    paths:
      - services/api
    dependencies:
      - common/auth
      - common/logging
    ignore:
      - services/api/docs/**
    vars:
      image: registry.example.com/project/api
      deployment: api-production

  worker:
    paths:
      - services/worker
    dependencies:
      - common/logging
```

`paths` identifies the service's code, `dependencies` adds shared directories that can make it affected, `ignore` excludes specific globs and `vars` holds service-specific string key-value pairs. The root-level `ignore` applies to all services. `remote` is used only by `release --push` and defaults to `origin` when omitted.

`paths` is required; the other fields are optional. Unknown keys (for example a typo such as `dependecies`) are a configuration error. Service names must be valid as a Git tag component: no `/`, spaces, `~ ^ : ? * [ \`, `..` or `@{`, and they cannot start with `.` or `-` or end with `.` or `.lock`.

`ignore` globs support `**` (any number of directories), `*` and `?` (within a single path segment); all other characters are literal. For a renamed file both the old and the new path are evaluated: the service is affected if at least one of them belongs to its `paths`/`dependencies` and is not ignored, and `changes` reports only the relevant paths.

Check the configuration with:

```sh
releaser config check
```

Variables can be retrieved without accessing Git, and the command prints only the value, so it can be used in scripts:

```sh
releaser get-var api image
# registry.example.com/project/api

IMAGE=$(releaser get-var api image)
```

The command fails if the service or the key does not exist. `vars` values must be strings, so numbers and booleans must be quoted.

## Version commands

These commands write only the requested value to stdout, so they can be used directly in scripts. They require the service to already have at least one valid tag.

```sh
releaser version-number api
# 2.4.1

releaser version-tag api
# api/v2.4.1

releaser next-version-number api patch
# 2.4.2

releaser next-version-tag api minor
# api/v2.5.0
```

The allowed bumps are `patch`, `minor` and `major`. The `next-version-*` commands only compute the value and do not create tags.

CI example:

```sh
VERSION=$(releaser version-number api)
IMAGE_TAG=$(releaser next-version-number api patch)
```

## Status and changes

`status` compares, for each service, its latest tag with `HEAD` and takes both `paths` and `dependencies` into account:

```sh
releaser status
releaser status api
releaser status api --verbose
```

The compact output shows the service, the latest version and the affected state. Verbose mode also includes the tag and the changed files. A service with no release is considered affected and shows `none` as its version.

To get only the names of the affected services, one per line:

```sh
releaser affected
```

To get only the files that make a service affected:

```sh
releaser changes api
```

Read-only commands compare Git commits and ignore uncommitted changes in the working tree.

## Release plan

```sh
releaser plan
releaser plan --format json
```

The table format shows the version, the affected state and the number of changed files. The JSON format is suitable for pipelines:

```json
{
  "services": [
    {
      "name": "api",
      "lastVersion": "2.4.1",
      "lastTag": "api/v2.4.1",
      "affected": true,
      "changedFiles": 2
    }
  ]
}
```

For a service that has never been released, `lastVersion` and `lastTag` are `null`.

## Creating a release

To bump the latest version and create an annotated tag on `HEAD`:

```sh
releaser release api patch
releaser release api minor
releaser release api major
```

The resulting tag looks like `api/v2.4.2`, with the message `Release api v2.4.2`. The working tree must be clean: no changes to tracked files. Untracked files are ignored.

The service must be affected, that is, it must have relevant changes after its latest release. Otherwise the command exits with an error. To intentionally create a tag even without changes to the service, use `--force`:

```sh
releaser release api patch --force
```

To see the result without creating the tag:

```sh
releaser release api minor --dry-run
```

To bootstrap a service with no tags, or to set a specific version, use a valid SemVer without the `v` prefix:

```sh
releaser release new-service --version 1.0.0
```

To initialize, in a single operation, all configured services that do not have a valid tag yet, use `--new`:

```sh
releaser release --new
# creates <service>/v0.1.0 for every service never released

releaser release --new --version 1.0.0
# uses 1.0.0 as the initial version
```

Services that already have at least one valid tag according to the `<service>/v<semver>` scheme are skipped. If `--version` is not specified, the default initial version is `0.1.0`. `--new` does not accept a service name or a bump and cannot be combined with `--affected`, `--all`, `--root` or `--force`; it does support `--dry-run` and `--push`.

`--version` and a bump cannot be used together. Parsing is strict: values such as `1.2`, `v1.2.3` and `01.2.3` are not accepted. If the service already has a release, the explicit version must be greater than the latest one; with `--force` the tag is created anyway, but it does not become the latest release, because that is always the highest SemVer.

Prereleases and build metadata are valid tags (`api/v2.1.0-rc.1`, `api/v1.0.0+build.5`). A prerelease takes part in SemVer ordering (`2.0.0` < `2.1.0-rc.1` < `2.1.0`), so it can be the latest release returned by `version-number`. From `2.1.0-rc.1`, the `patch` bump produces `2.1.0`, `minor` produces `2.2.0` and `major` produces `3.0.0`.

The tag stays local unless explicitly requested:

```sh
releaser release api patch --push
```

With `--push`, the command publishes only the new tags to the configured remote, with a single atomic push: either they all reach the remote or none do. If creation or push fails, the local tags created by the command are deleted, so the command can be rerun. No implicit fetch, pull, build or deploy is performed.

### Bulk release

To apply the same bump to all and only the affected services:

```sh
releaser release --affected patch
```

To apply it to all configured services, including those that are not affected, you must confirm explicitly with `--force`:

```sh
releaser release --all --force major
```

The command validates the whole batch before creating any tag. Every service must have a previous release, because explicit versions are not supported in bulk mode. `--dry-run` and `--push` are also available for bulk releases; with `--push`, all created tags are published to the configured remote in a single atomic push.

### Standard repository (root)

To release a standard repository, not organized as a monorepo, use `--root`. In this mode `releaser.yml` is not read:

```sh
releaser release --root patch
releaser release --root --version 1.0.0
```

The default tag has no prefix, for example `v0.1.5`. If a release already exists, the prefix is inferred from previous tags; if mixed prefixes are found, `--prefix` becomes mandatory. During inference a prefix is valid only if it is empty or ends with a separator (`/`, `-`, `_`, `.`): `release-v1.0.0` has the prefix `release-`, while `dev1.0.0` is not considered a release tag. An explicitly empty value (`--prefix=''`) selects tags without a prefix:

```sh
releaser release --root patch --prefix=''
releaser release --root minor --prefix='release-'
```

If there are no commits with changes after the previous tag, the release requires `--force`. `--dry-run` and `--push` are supported; the push uses `origin`. `--root` is mutually exclusive with `--affected` and `--all`.

## Running scripts per service

`run` runs a script (or any command) once for each configured service, in alphabetical order, or only for the given service:

```sh
releaser run build.sh           # once per service
releaser run api build.sh       # only for api
releaser run --affected build.sh  # only for affected services
releaser run api build.sh -- --push latest  # arguments passed to the script
```

The script receives these environment variables:

| Variable | Value |
| --- | --- |
| `RELEASER_NAME` | service name |
| `RELEASER_PATHS` | space-separated `paths` |
| `RELEASER_DEPS` | space-separated `dependencies` (empty if none) |
| `RELEASER_VERSION` | latest released version, for example `2.4.1` (empty if not available) |
| `RELEASER_TAG` | latest release tag, for example `api/v2.4.1` (empty if not available) |
| `RELEASER_VAR_<KEY>` | one for each `vars` key, uppercased and with `-` replaced by `_` |

With the example configuration, for `api` the script receives `RELEASER_VAR_IMAGE=registry.example.com/project/api` and `RELEASER_VAR_DEPLOYMENT=api-production`:

```sh
#!/bin/sh
set -e
for p in $RELEASER_PATHS; do echo "path: $p"; done
docker build -t "$RELEASER_VAR_IMAGE" "services/$RELEASER_NAME"
```

The script runs from the repository root (`--repo`); if the given path exists relative to the current directory it is executed directly (it must be executable and have a shebang), otherwise it is looked up in the `PATH`. Stdin, stdout and stderr are those of `releaser`; before each run the line `==> <service>` is printed to stderr.

Execution stops at the first failing script, and `releaser` exits with the same exit code as the script. Before running any script, all variables are validated: a `vars` key that does not produce a valid name (only letters, digits, `_` and `-`), two keys that produce the same name (for example `my-var` and `my_var`) or a path containing spaces are a configuration error. `RELEASER_VERSION` and `RELEASER_TAG` are empty for a service that has never been released or if `--repo` is not a Git repository: in that case `run` still works, except with `--affected`.

## Generating files from templates

`template` executes a Go template ([`text/template`](https://pkg.go.dev/text/template)) once for each configured service, in alphabetical order, or only for the given service:

```sh
releaser template deploy.yaml.tpl                  # all services, concatenated on stdout
releaser template api deploy.yaml.tpl > api.yaml   # only api
releaser template deploy.yaml.tpl -o 'deploy/{{.Name}}.yaml'  # one file per service
```

The template has access to:

| Field | Value |
| --- | --- |
| `.Name` | service name |
| `.Paths` | list of `paths` |
| `.Deps` | list of `dependencies` |
| `.Version` | latest released version, for example `2.4.1` (empty if not available) |
| `.Tag` | latest release tag, for example `api/v2.4.1` (empty if not available) |
| `.Vars` | map of `vars`, with `-` replaced by `_` in the keys |

With the example configuration, the template:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Vars.deployment}}
spec:
  template:
    spec:
      containers:
        - name: {{.Name}}
          image: {{.Vars.image}}:{{.Version}}
---
```

produces for `api`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-production
spec:
  template:
    spec:
      containers:
        - name: api
          image: registry.example.com/project/api:2.4.1
---
```

Lists are iterated with `range`, for example `{{range .Paths}}{{.}} {{end}}`. A missing key in `.Vars` (for example a typo such as `{{.Vars.imgae}}`) is an error; for an optional variable use `index`, which returns an empty string: `{{with index .Vars "port"}}port: {{.}}{{end}}`.

In templates a name after the dot cannot contain `-`, so in `.Vars` keys every `-` is replaced by `_`: the variable `docker-file` is written `{{.Vars.docker_file}}`. Two keys that produce the same name (for example `docker-file` and `docker_file`) are a configuration error.

Without `-o`, the output of all services is written to stdout, one after the other. With `-o`/`--output`, each service is written to a file; the output path is itself a template with the same fields and must be different for each service (it typically contains `{{.Name}}`). Missing directories are created, existing files are overwritten, and the path of each written file is printed to stdout. The template path and the output path are relative to the current directory.

All services are rendered before writing: if the template fails for one service, nothing is written. `.Version` and `.Tag` are empty for a service that has never been released or if `--repo` is not a Git repository.

## Exit codes and errors

Errors are written to stderr with the `ERROR:` prefix. The exit codes are:

| Code | Meaning |
| ---: | --- |
| 0 | success |
| 1 | invalid input, service/version not available or invalid state |
| 2 | unreadable or invalid configuration |
| 3 | repository, history or Git operation not available |

In a shallow clone, the required tags and history must be made available before running the tool; `releaser` does not access the network automatically. If the history between a tag and `HEAD` is missing, the command fails with exit code 3 instead of reporting a potentially wrong result.

## Quick reference

```text
releaser config check
releaser status [service] [--verbose]
releaser affected
releaser changes <service>
releaser get-var <service> <key>
releaser run [service] <script> [--affected] [-- args...]
releaser template [service] <file> [-o <path-template>]
releaser version-number <service>
releaser version-tag <service>
releaser next-version-number <service> <patch|minor|major>
releaser next-version-tag <service> <patch|minor|major>
releaser plan [--format table|json]
releaser release <service> <patch|minor|major> [--dry-run] [--push] [--force]
releaser release <service> --version <semver> [--dry-run] [--push] [--force]
releaser release --affected <patch|minor|major> [--dry-run] [--push]
releaser release --all --force <patch|minor|major> [--dry-run] [--push]
releaser release --root <patch|minor|major> [--prefix=<prefix>] [--dry-run] [--push] [--force]
releaser release --root --version <semver> [--prefix=<prefix>] [--dry-run] [--push]
```

Use `releaser --help` or `releaser <command> --help` for the built-in help.
