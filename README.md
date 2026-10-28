# GitReleaser

*Independent semantic versioning for services in a monorepo, using nothing but Git tags.*

GitReleaser is a single static binary that tells you which services changed since their last release, computes the next version, and creates the tag. No commit conventions required, no version files to keep in sync, no runtime, no CI vendor lock-in.

It also works on standard single-project repositories with `--root`.

## The problem

Your monorepo holds several deployable services. They share one Git history but ship on independent schedules: `api` is at 2.4.1, `worker` at 1.8.3, and a change in `common/` may or may not affect either of them.

The usual answers don't fit everywhere:

- **Commit-driven release tools** (release-please, cocogitto, Nx Release in its default mode) derive versions from Conventional Commits. That breaks down when you don't control how commits are written, for example with an external contractor or a legacy team.
- **File-driven approaches** keep release intent or versions in the source tree: changeset files (Changesets), or `package.json`, `.csproj`, `VERSION`. That's another artifact to maintain alongside every change, and version files in particular drift from what was actually released.
- **Ad-hoc scripts** start as ten lines of `git describe` and slowly accumulate edge cases nobody tests.

## How it works

Each service is a set of paths plus its declared shared dependencies. Its latest tag (`api/v2.4.1`) is its baseline. If anything relevant changed between that tag and `HEAD`, the service is *affected*.

```text
api/v2.4.1..HEAD      -> services/api, common/auth, common/logging
worker/v1.8.3..HEAD   -> services/worker, common/logging
```

Every service has its own baseline, even though they all share the same history. You choose the bump (`patch`, `minor`, `major`); the tool computes the version and creates an annotated tag.

The core job is four things: find the latest release of each service, determine whether it changed, compute the next version, create the tag. On top of that, GitReleaser exposes per-service data (name, paths, dependencies, version, tag, custom `vars`) to your own scripts and templates. It never builds, deploys or talks to a registry itself.

```text
monorepo != mono-version
repository != release unit
```

The repository is the state of the code. Tags are the releases of the individual deployable units.

### Use it if

You have a polyglot monorepo, independently deployed services, no enforced commit convention, and you want release state to be traceable from Git, with server-side tag protection in place.

### Don't use it if

You need changelogs, package publishing, or automatic bumps from commit messages. The tools listed above do that better.

## Installation

Prebuilt binaries for Linux, macOS and Windows are available in the [latest release](https://github.com/ivanomatteo/GitReleaser/releases/latest). The only runtime dependency is Git in the `PATH`.

To build from source (Go 1.26 or later):

```sh
CGO_ENABLED=0 go install github.com/ivanomatteo/GitReleaser/cmd/releaser@latest
# or, from a clone:
CGO_ENABLED=0 go build -o releaser ./cmd/releaser
```

## Quick start

```sh
releaser config check                      # validate releaser.yml
releaser release --new --version 1.0.0     # give every service its first tag
releaser plan                              # later: what changed since each release?
releaser release --affected patch --push   # release only what changed
```

## Configuration

All commands read `releaser.yml` from the repository root.

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
    vars:
      image: registry.example.com/project/worker
      deployment: worker-production
```

| Key | Required | Meaning |
| --- | --- | --- |
| `services.<name>.paths` | yes | the service's own code |
| `services.<name>.dependencies` | no | shared directories that can make the service affected |
| `services.<name>.ignore` | no | globs excluded for this service |
| `services.<name>.vars` | no | string key/value pairs for your scripts and templates |
| `ignore` | no | globs excluded for every service |
| `remote` | no | remote used by `--push` (default `origin`) |

Unknown keys are a configuration error, so a typo such as `dependecies` can't silently disable a dependency. `vars` values must be strings: quote numbers and booleans.

Service names must be valid as a single Git ref component: no `/`, spaces, `~ ^ : ? * [ \`, `..` or `@{`, and they can't start with `.` or `-` or end with `.` or `.lock`.

`ignore` globs are relative to the repository root and support `**` (any number of directories, including none), `*` and `?` (within a single path segment). All other characters are literal.

Added, modified, deleted and renamed files are all detected. For a rename, both the old and the new path are evaluated: the service is affected if at least one of them is relevant.

Global options:

```text
-c, --config string   configuration file (default "releaser.yml")
    --repo string     Git repository directory (default ".")
```

The default `releaser.yml` is looked up in the `--repo` directory, so `releaser --repo ../monorepo status` works from anywhere. An explicit relative `--config` is resolved against the current directory.

## Inspecting state

```sh
releaser status                  # every service: last version, affected
releaser status api --verbose    # one service, with tag and changed files
releaser affected                # names of affected services, one per line
releaser changes api             # files that make api affected
releaser plan                    # table: version, affected, changed files
releaser plan --format json      # same, for pipelines
```

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

A service that has never been released is always affected. It shows `none` as its version in tables and `null` for `lastVersion` and `lastTag` in JSON.

Read-only commands compare commits and ignore uncommitted changes in the working tree. They work in detached HEAD, which is what most CI systems check out.

## Version commands

These write only the requested value to stdout, so they can be used directly in scripts without `awk`, `sed` or `cut`:

```sh
releaser version-number api              # 2.4.1
releaser version-tag api                 # api/v2.4.1
releaser next-version-number api patch   # 2.4.2
releaser next-version-tag api minor      # api/v2.5.0
```

```sh
VERSION=$(releaser version-number api)
IMAGE_TAG=$(releaser next-version-number api patch)
```

The `next-version-*` commands only compute the value; they never create tags. All four fail with exit code 1 if the service has no release yet, or if its latest release isn't an ancestor of `HEAD` (see [Branches and tag ancestry](#branches-and-tag-ancestry)). On failure nothing is written to stdout, so `VERSION=$(...)` never captures a wrong value.

## Releasing

### A single service

```sh
releaser release api patch
releaser release api minor --dry-run   # show the result, create nothing
```

This creates the annotated tag `api/v2.4.2` on `HEAD`, with the message `Release api v2.4.2`.

The release is refused if:

- the working tree has changes to tracked files (untracked files, such as CI artifacts, are ignored);
- the service is not affected, unless you pass `--force`;
- the resulting tag already exists.

`--force` bypasses only the affected check and the explicit-version check described below. It never overwrites an existing tag.

### Explicit versions

```sh
releaser release new-service --version 1.0.0
```

Use a strict SemVer without the `v` prefix: values such as `1.2`, `v1.2.3` and `01.2.3` are rejected. `--version` and a bump can't be combined.

If the service already has a release, the explicit version must be greater than the latest one. With `--force` the tag is created anyway (for example to rebuild a tag missing from the history), but it doesn't become the latest release, which is always the highest SemVer.

### Initializing all services

```sh
releaser release --new                  # <service>/v0.1.0 for every service never released
releaser release --new --version 1.0.0  # custom initial version
```

Services that already have a valid tag are skipped, so the command is idempotent. `--new` supports `--dry-run` and `--push`, and can't be combined with a service name, a bump, `--affected`, `--all`, `--root` or `--force`.

### Prereleases

Prerelease and build-metadata tags are valid (`api/v2.1.0-rc.1`, `api/v1.0.0+build.5`). Prereleases are created only with `--version`: no bump produces one.

A prerelease takes part in SemVer ordering (`2.0.0` < `2.1.0-rc.1` < `2.1.0`). If it's the highest version, it's the latest release, so `version-number` returns `2.1.0-rc.1`: keep this in mind if your CI uses that value to tag images. From `2.1.0-rc.1`, `patch` produces `2.1.0`, `minor` produces `2.2.0` and `major` produces `3.0.0`. Build metadata doesn't affect ordering.

### Pushing

Tags stay local unless you ask otherwise:

```sh
releaser release api patch --push
```

`--push` publishes only the tags created by the command, to the configured remote, in a single atomic push: either all of them reach the remote or none do. If tag creation or the push fails, the local tags created by the command are deleted, so it can be safely rerun. No fetch, pull, build or deploy ever happens implicitly.

### Bulk release

```sh
releaser release --affected patch        # every affected service
releaser release --all --force major     # every configured service
```

The whole batch is validated before the first tag is created: configuration, history, previous releases, tag uniqueness and a clean working tree. A validation error leaves no partial tags. Every service needs a previous release, because explicit versions aren't supported in these modes (use `--new` for first releases). `--dry-run` and `--push` are available.

### Branches and tag ancestry

A service's latest release is its highest SemVer tag, whichever branch it was created on, and it must be an ancestor of `HEAD`. If it isn't, for example because it was released from a hotfix branch that hasn't been merged, every command that uses it fails with exit code 1 instead of computing changes or versions against an unrelated commit:

```text
ERROR: latest release api/v2.4.2 is not an ancestor of HEAD; it was probably created on an unmerged branch (merge it, or run from it)
```

This applies to `status`, `affected`, `changes`, `plan`, `release`, the version commands, `run` and `template`. Commands that cover every service fail if any of them has such a tag; select a single service to work on the others. Merge the branch, or run the tool from it.

Changes are the difference between the tag's tree and `HEAD`'s tree, not the commits in between: a change made and then reverted doesn't make a service affected.

In a shallow clone, missing history is reported as such, with exit code 3, rather than producing a potentially wrong result. This includes the version commands, `run` and `template`: fetching the tags isn't enough, the history between each latest release and `HEAD` must be available too (for example `fetch-depth: 0` in GitHub Actions, or `git fetch --unshallow --tags`). The tool never accesses the network on its own.

### Standard repositories

For a repository that isn't a monorepo, use `--root`. `releaser.yml` isn't read.

```sh
releaser release --root patch
releaser release --root --version 1.0.0
releaser release --root minor --prefix='release-'
```

With no previous release the tag has no prefix, for example `v0.1.5`. Otherwise the prefix is inferred from previous tags. A prefix is recognized only if it's empty or ends with a separator (`/`, `-`, `_`, `.`), so `release-v1.0.0` has the prefix `release-`, while `dev1.0.0` isn't treated as a release tag. If mixed prefixes are found, `--prefix` becomes mandatory; `--prefix=''` explicitly selects unprefixed tags.

If nothing changed since the previous tag, the release requires `--force`. `--dry-run` and `--push` are supported; the push always uses `origin`. `--root` is mutually exclusive with `--affected` and `--all`.

## Reusing `releaser.yml` in your pipeline

`releaser.yml` already describes every service: its paths, its shared dependencies, and any metadata you attach with `vars`. `get-var`, `template` and `run` let you reuse that description instead of duplicating it across scripts and CI files. They add no build or deploy logic: they only feed per-service data to whatever you write.

### Reading a variable

```sh
releaser get-var api image   # registry.example.com/project/api
IMAGE=$(releaser get-var api image)
```

Only the value is printed. The command fails if the service or the key doesn't exist, and it doesn't need a Git repository.

### Generating files from a template

`template` is a thin wrapper around Go's [`text/template`](https://pkg.go.dev/text/template): it renders any text file once per service, with that service's data. It knows nothing about the output format: CI definitions, Kubernetes manifests, `.env` files and plain reports are all just text.

| Field | Value |
| --- | --- |
| `.Name` | service name |
| `.Paths` | list of `paths` |
| `.Deps` | list of `dependencies` |
| `.Version` | latest released version, e.g. `2.4.1` (empty if none) |
| `.Tag` | latest release tag, e.g. `api/v2.4.1` (empty if none) |
| `.Vars` | map of `vars`, with `-` in keys replaced by `_` |

**Example: generating CI jobs.** Instead of writing one build, one test and one production job by hand for each of 30 services, write them once:

```yaml
build-{{.Name}}:
  stage: build
  script:
    - docker build -t "{{.Vars.image}}:$CI_COMMIT_SHORT_SHA" {{index .Paths 0}}
  rules:
    - changes:
{{- range .Paths}}
        - {{.}}/**/*
{{- end}}
{{- range .Deps}}
        - {{.}}/**/*
{{- end}}

deploy-prod-{{.Name}}:
  stage: prod
  script:
    - ./deploy.sh {{.Name}} "$(releaser version-number {{.Name}})"
  when: manual
```

Render it for all services into one file and include it from `.gitlab-ci.yml`:

```sh
releaser template ci/services.yml.tpl > .gitlab/services.yml
```

Adding a service now means adding it to `releaser.yml` and regenerating. Its dependencies drive both release detection and CI rules, from a single place.

If the generated file is committed, add a CI job that fails when it's out of date:

```sh
releaser template ci/services.yml.tpl > .gitlab/services.yml
git diff --exit-code .gitlab/services.yml
```

Alternatively, generate it at runtime as an artifact and run it as a GitLab child pipeline (`trigger: include: artifact:`): nothing to commit, nothing to drift. The job that renders it needs enough history to check each service's latest release, for example `GIT_DEPTH: 0`.

**Other ideas:**

- one file per service with `-o`, for example `releaser template app.env.tpl -o 'deploy/{{.Name}}.env'`;
- Kubernetes or OpenShift manifests with `{{.Vars.image}}:{{.Version}}`;
- a `CODEOWNERS` file generated from each service's `paths`;
- a release report listing every service with its current version and tag.

**Rules.** Without `-o`, the output of all services is concatenated on stdout. With `-o`, the output path is itself a template and must be different for each service; missing directories are created, existing files are overwritten, and each written path is printed. All services are rendered before anything is written, so a failure leaves no partial output. Template and output paths are relative to the current directory.

A missing key such as `{{.Vars.imgae}}` is an error. For optional variables use `index`, which returns an empty string: `{{with index .Vars "port"}}port: {{.}}{{end}}`. Since template field names can't contain `-`, the variable `docker-file` is written `{{.Vars.docker_file}}`; two keys that map to the same name are a configuration error.

### Running a command per service

```sh
releaser run build.sh                       # once per service, alphabetically
releaser run api build.sh                   # only api
releaser run --affected build.sh            # only affected services
releaser run api build.sh -- --push latest  # arguments after -- go to the script
```

Each run gets the inherited environment plus:

| Variable | Value |
| --- | --- |
| `RELEASER_NAME` | service name |
| `RELEASER_PATHS` | space-separated `paths` |
| `RELEASER_DEPS` | space-separated `dependencies` (empty if none) |
| `RELEASER_VERSION` | latest released version (empty if none) |
| `RELEASER_TAG` | latest release tag (empty if none) |
| `RELEASER_VAR_<KEY>` | one per `vars` key, uppercased, `-` replaced by `_` |

```sh
#!/bin/sh
set -e
docker build -t "$RELEASER_VAR_IMAGE:$RELEASER_VERSION" "services/$RELEASER_NAME"
```

The script runs from the repository root. If the given path exists relative to the current directory it's executed directly, otherwise it's looked up in the `PATH`. Shebang scripts work on Unix-like systems; on Windows, or to be explicit, pass the interpreter: `releaser run sh -- ci/build.sh`. Before each run, `==> <service>` is printed to stderr.

All variables are validated before the first run. Execution stops at the first failure, and `releaser` exits with the script's exit code (or 1 if the script can't be started or is killed by a signal). Only `--affected` requires a Git repository. Inside a repository, a selected service whose latest release isn't an ancestor of `HEAD` stops the command before anything runs (see [Branches and tag ancestry](#branches-and-tag-ancestry)); `template` behaves the same way.

## Adopting it in an existing repository

No history rewrite, no commit convention to adopt, no version files to add. Write a `releaser.yml`, run `releaser release --new --version <current>`, and you're done.

Existing tags matching `<service>/v<semver>` are picked up as baselines; tags in other formats are ignored. If you remove the tool later, you're left with plain Git tags.

## Exit codes and errors

Errors go to stderr with the `ERROR:` prefix.

| Code | Meaning |
| ---: | --- |
| 0 | success |
| 1 | invalid input or flags, unknown service or variable, no release, invalid state |
| 2 | missing or invalid configuration |
| 3 | invalid repository, incomplete history, failed Git command (including push) |
| 4 | internal error (reserved, currently unused) |

**Exception:** `run` exits with the exit code of the failing script, so with `run` a non-zero code may come from your script rather than from `releaser`.

## Limitations

- **Change detection is only as good as your declared dependencies.** If a service uses shared code that isn't listed in `releaser.yml`, changes there won't mark it as affected, and the tool won't warn you.
- **Git tags are mutable by default.** Anyone with push rights can delete or move them. For real traceability, protect release tags on the server (in GitLab, a protected tag rule on `*/v*`; in GitHub, a tag ruleset) and consider signed tags.
- **No changelogs, no automatic bumps.** By design: the bump is always your decision.

## Quick reference

```text
releaser config check
releaser status [service] [--verbose]
releaser affected
releaser changes <service>
releaser plan [--format table|json]

releaser version-number <service>
releaser version-tag <service>
releaser next-version-number <service> <patch|minor|major>
releaser next-version-tag <service> <patch|minor|major>

releaser release <service> <patch|minor|major> [--dry-run] [--push] [--force]
releaser release <service> --version <semver> [--dry-run] [--push] [--force]
releaser release --new [--version <semver>] [--dry-run] [--push]
releaser release --affected <patch|minor|major> [--dry-run] [--push]
releaser release --all --force <patch|minor|major> [--dry-run] [--push]
releaser release --root <patch|minor|major> [--prefix=<prefix>] [--dry-run] [--push] [--force]
releaser release --root --version <semver> [--prefix=<prefix>] [--dry-run] [--push]

releaser get-var <service> <key>
releaser template [service] <file> [-o <path-template>]
releaser run [service] <script> [--affected] [-- args...]
```

Use `releaser --help` or `releaser <command> --help` for the built-in help.

## License

MIT