# Updated specifications — Monorepo Microservice Release Tool

## 1. Purpose

Build a CLI tool written in **Go** to manage independent versioning and releases of microservices contained in a single monorepo.

The tool must make it possible to:

* find the latest released version of each microservice;
* determine which services have unreleased changes;
* take shared dependencies into account, for example `common` modules;
* compute the next SemVer version;
* create per-service namespaced Git tags;
* easily expose the version number and the full tag;
* produce output that is easy to consume from CI/CD pipelines;
* keep **Git as the single source of truth for versioning**.

No `VERSION` file must exist.

---

# 2. Source of truth

A service's version is determined exclusively by its Git tags.

Format:

```text
<service>/v<semver>
```

Example:

```text
scraper-service/v0.1.5
api/v2.4.1
worker/v1.8.3
```

Internally:

```text
service = scraper-service
version = 0.1.5
tag     = scraper-service/v0.1.5
```

The `v` prefix belongs to the Git tag convention.

The pure SemVer value is:

```text
0.1.5
```

---

# 3. SemVer

The tool must use Semantic Versioning 2.0.0:

```text
MAJOR.MINOR.PATCH
```

Example:

```text
2.7.4
```

The supported operations are:

```text
patch
minor
major
```

Examples:

```text
2.7.4 + patch -> 2.7.5
2.7.4 + minor -> 2.8.0
2.7.4 + major -> 3.0.0
```

The tool must not determine the bump type automatically.

Choosing between `patch`, `minor` and `major` remains the responsibility of the user or of the release pipeline.

For the first bulk release of the services, the command:

```text
releaser release --new [--version <semver>]
```

must create on `HEAD` the first tag of every configured service that does not yet have a valid tag in its namespace. The default version, when `--version` is not given, is `0.1.0`.

Example:

```text
releaser release --new --version 0.1.0
```

With the services `api`, `worker` and `web`, if only `api` already has a valid tag, the following must be created:

```text
worker/v0.1.0
web/v0.1.0
```

`api` must be skipped regardless of the version of its existing tag. The command must be idempotent: if all services already have a valid release, it creates no tags. `--new` cannot be combined with a service, a bump, `--affected`, `--all`, `--root` or `--force`; it can be used with `--dry-run` and `--push`.

---

# 4. Go SemVer library

Use:

```text
github.com/Masterminds/semver/v3
```

The library must be used at least for:

* parsing;
* validation;
* comparison;
* sorting;
* version bumping.

To parse version numbers coming from tags, preferably use a strict mode.

Conceptually:

```go
semver.StrictNewVersion("1.2.3")
```

must be accepted.

Values such as:

```text
1.2
v1.2
01.2.3
foo
```

must not be accepted as valid SemVer through automatic coercion.

The `v` prefix must be stripped from the tag before passing the value to the SemVer parser.

Example:

```text
api/v2.4.1
      ↓
2.4.1
      ↓
StrictNewVersion(...)
```

---

# 5. Repository structure

Example:

```text
repo/
├── services/
│   ├── api/
│   ├── worker/
│   └── scraper/
│
├── common/
│   ├── auth/
│   ├── logging/
│   └── http/
│
├── tools/
│   └── releaser/
│
└── releaser.yml
```

The tool must not assume this structure.

All paths must be configurable.

---

# 6. Configuration

Default file:

```text
releaser.yml
```

Example:

```yaml
remote: origin

ignore:
  - docs/**

services:

  api:
    paths:
      - services/api
    dependencies:
      - common/auth
      - common/logging
    vars:
      image: registry.example.com/project/api
      deployment: api-production

  worker:
    paths:
      - services/worker
    dependencies:
      - common/logging

  scraper-service:
    paths:
      - services/scraper
    dependencies:
      - common/http
      - common/logging
```

Each service must have `paths` and may have:

```text
dependencies
ignore
vars
```

`vars` is an optional service-specific map of string keys and values. Non-string values are not valid.

At the root level `services`, `ignore` (section 31) and `remote` are allowed, the latter being the remote used by `--push` (default `origin`, section 28). Unknown keys are a configuration error, so that a typo does not silently disable a dependency.

A service name must be usable as a single Git ref component: it cannot be empty, contain `/`, spaces, control characters, `~ ^ : ? * [ \`, `..` or `@{`, start with `.` or `-`, or end with `.` or `.lock`.

The default file is looked up in the directory given by `--repo`. An explicitly passed relative `--config` is resolved against the current directory.

---

# 7. Affected service

A service is `affected` when there is at least one change not included in its latest release.

The comparison is between:

```text
<latest tag of the service>
```

and:

```text
HEAD
```

Example:

```text
api/v2.4.1..HEAD
```

The following must not be used:

```text
HEAD~1..HEAD
```

to determine the release state.

---

# 8. Independent baseline per service

Each service has its own baseline.

Example:

```text
api     -> api/v2.4.1
worker  -> worker/v1.8.3
scraper -> scraper-service/v0.1.5
```

The tool must logically perform:

```text
git diff api/v2.4.1..HEAD
git diff worker/v1.8.3..HEAD
git diff scraper-service/v0.1.5..HEAD
```

Baselines are not global.

The comparison is between the tree of the tag and the tree of `HEAD`, not the list of commits in between: a change made and then reverted after the tag does not make the service affected.

---

# 9. Finding the latest release

For a service:

```text
scraper-service
```

look for:

```text
scraper-service/v*
```

Example:

```bash
git tag --list 'scraper-service/v*'
```

Assuming:

```text
scraper-service/v0.1.3
scraper-service/v0.1.4
scraper-service/v0.1.5
```

the latest release is:

```text
scraper-service/v0.1.5
```

Sorting must be done by SemVer, not lexicographically.

The latest release is the highest SemVer among all the service's tags, regardless of the branch they were created on. It must be an ancestor of `HEAD`: if it is not, for example because it was created on an unmerged hotfix branch, `status`, `affected`, `changes`, `plan`, `release`, the version commands (sections 11–14), `run` and `template` (sections 11.2 and 11.3) must fail with exit code `1`, without writing anything to `stdout`, instead of computing changes or versions against an unrelated commit. The error must name the tag:

```text
ERROR: latest release api/v2.4.2 is not an ancestor of HEAD; it was probably created on an unmerged branch (merge it, or run from it)
```

Commands that evaluate every service fail if any of them has such a tag. In a shallow clone the same condition is reported as missing history (section 35).

---

# 10. Valid tags

Valid format:

```text
<service>/v<semver>
```

Valid examples:

```text
api/v1.0.0
api/v1.10.0
api/v2.0.0
```

Invalid examples:

```text
api/v1
api/v1.2
api/1.2.3
api/v01.2.3
api/foo
```

Prereleases and build metadata according to SemVer 2.0.0 are also valid, for example `api/v2.1.0-rc.1` or `api/v1.0.0+build.5`.

Prereleases take part in SemVer ordering: `2.1.0-rc.1` precedes `2.1.0` but follows `2.0.0`. If it is the highest version, a prerelease is the service's latest release and is returned by `version-number` and `version-tag`. Bumping from a prerelease follows the SemVer library:

```text
2.1.0-rc.1 + patch -> 2.1.0
2.1.0-rc.1 + minor -> 2.2.0
2.1.0-rc.1 + major -> 3.0.0
```

Build metadata does not affect ordering: `1.0.0+a` and `1.0.0` have the same precedence.

Malformed tags can normally be ignored and reported with a warning in verbose mode.

A future `--strict-tags` mode may treat them as errors.

---

# 11. `version-number` command

Syntax:

```bash
releaser version-number <service>
```

Example:

```bash
releaser version-number scraper-service
```

Output:

```text
0.1.5
```

If the latest tag is:

```text
scraper-service/v0.1.5
```

the command returns only:

```text
0.1.5
```

No additional text must be written to `stdout`.

This command is designed for scripting and CI/CD.

Example:

```bash
VERSION=$(releaser version-number scraper-service)
```

---

# 11.1 `get-var` command

Syntax:

```bash
releaser get-var <service> <key>
```

The command reads the value from the service's `vars` section and writes it alone to `stdout`, a trailing newline being allowed. It does not require the directory given by `--repo` to be a Git repository.

Example:

```bash
releaser get-var api image
```

Output:

```text
registry.example.com/project/api
```

A non-existent service or key produces an error and a non-zero exit code. The command is side-effect free and suitable for scripts and pipelines.

---

# 11.2 `run` command

Syntax:

```bash
releaser run [service] <script> [--affected] [-- args...]
```

Runs `<script>` once for each configured service, in alphabetical order. With `service` it runs only for that service; with `--affected` only for the affected services (cannot be combined with `service`). Arguments after `--` are passed to the script.

On each run the inherited environment is extended with:

* `RELEASER_NAME`: service name;
* `RELEASER_PATHS`: space-separated `paths`;
* `RELEASER_DEPS`: space-separated `dependencies`, empty if none;
* `RELEASER_VERSION`: latest released version of the service, without the `v` prefix;
* `RELEASER_TAG`: tag of the latest release;
* `RELEASER_VAR_<KEY>`: one per `vars` key, with `<KEY>` uppercased and `-` replaced by `_`.

Any variables with the same names present in `releaser`'s environment are not inherited. Before running any script, the variables of all selected services are validated: a key that does not produce a name made only of `A-Z`, `0-9` and `_`, two keys that produce the same name or a path containing spaces produce an error with exit code `2`.

The script runs from the `--repo` directory. If `<script>` exists as a file relative to the current directory it is executed through its absolute path, otherwise it is looked up in the `PATH`. Stdin, stdout and stderr are inherited; before each run the line `==> <service>` is written to stderr.

Execution stops at the first failure: `releaser` exits with the script's exit code, or `1` if the script cannot be started or is terminated by a signal. `RELEASER_VERSION` and `RELEASER_TAG` are empty if the service has no release or if `--repo` is not a Git repository; only `--affected` requires a Git repository. Inside a Git repository the latest release of every selected service must be an ancestor of `HEAD` (section 9); otherwise the command fails before running any script.

---

# 11.3 `template` command

Syntax:

```bash
releaser template [service] <file> [-o|--output <path-template>]
```

Executes the Go template (`text/template`, option `missingkey=error`) contained in `<file>` once for each configured service, in alphabetical order, or only for `service`. The template receives the fields `.Name`, `.Paths`, `.Deps` (lists), `.Version`, `.Tag` (empty strings if the service has no release or if `--repo` is not a Git repository) and `.Vars` (map of `vars`, empty if none, with every `-` in the keys replaced by `_`, so that `docker-file` is accessible as `.Vars.docker_file`; two keys that produce the same name are a configuration error with exit code `2`). Accessing a non-existent `.Vars` key with the `.Vars.key` syntax is an error.

Without `--output`, the output of all services is written to stdout, concatenated. With `--output`, the value is itself a template executed with the same data, which produces the path of the service's file; two services producing the same path, or an empty path, are an error. Missing directories are created, existing files are overwritten, and one path per line is written to stdout. All services are rendered before any output is written: an error leaves no partial output. Template read, parse or execution errors produce exit code `1`. As for `run`, inside a Git repository the latest release of every selected service must be an ancestor of `HEAD` (section 9); otherwise the command fails without writing any output.

---

# 12. `version-tag` command

Syntax:

```bash
releaser version-tag <service>
```

Example:

```bash
releaser version-tag scraper-service
```

Output:

```text
scraper-service/v0.1.5
```

Here too, `stdout` must contain only the value.

---

# 13. `next-version-number` command

Syntax:

```bash
releaser next-version-number <service> <patch|minor|major>
```

Example:

```bash
releaser next-version-number scraper-service patch
```

Output:

```text
0.1.6
```

Other examples:

```bash
releaser next-version-number scraper-service minor
```

Output:

```text
0.2.0
```

```bash
releaser next-version-number scraper-service major
```

Output:

```text
1.0.0
```

The command must be side-effect free.

---

# 14. `next-version-tag` command

Syntax:

```bash
releaser next-version-tag <service> <patch|minor|major>
```

Example:

```bash
releaser next-version-tag scraper-service patch
```

Output:

```text
scraper-service/v0.1.6
```

The command must not create the tag.

---

# 15. Removal of the `next` command

The generic command must not exist:

```text
releaser next
```

To obtain versions and tags, the CLI must expose only:

```text
version-number
version-tag
next-version-number
next-version-tag
```

This avoids ambiguity about the returned format. The full list of commands is in section 42.

---

# 16. Output of the version commands

The four commands:

```text
version-number
version-tag
next-version-number
next-version-tag
```

must be scripting primitives.

Rules:

* value on `stdout`;
* errors on `stderr`;
* no header;
* no decoration;
* no descriptive prefix;
* trailing newline allowed;
* `exit 0` on success;
* non-zero exit code on error, with nothing written to `stdout`.

The value is written only if the latest release is an ancestor of `HEAD` (section 9). On error the tag that was found may appear in the message on `stderr`, never on `stdout`, so that `VERSION=$(...)` never captures a value from a failed command.

Example:

```bash
IMAGE_TAG=$(releaser version-number api)
```

must work without `awk`, `sed`, `grep` or `cut`.

---

# 17. Service without a release

If no tags exist for a service:

```bash
releaser version-number new-service
```

must fail.

`stderr`:

```text
ERROR: new-service has no released version
```

Exit code:

```text
1
```

The same applies to:

```text
version-tag
next-version-number
next-version-tag
```

because there is no previous version to bump.

Bootstrap is handled through a release with an explicit version.

---

# 18. `status` command

```bash
releaser status
```

Output:

```text
SERVICE           LAST VERSION    AFFECTED
api               2.4.1           yes
worker            1.8.3           no
scraper-service   0.1.5           yes
```

Single service:

```bash
releaser status scraper-service
```

---

# 19. Verbose status

```bash
releaser status scraper-service --verbose
```

Example:

```text
Service: scraper-service

Last version:
  0.1.5

Last tag:
  scraper-service/v0.1.5

Changed files:

  services/scraper/parser.go
  common/http/client.go

Affected:
  yes
```

---

# 20. `affected` command

```bash
releaser affected
```

Output:

```text
api
scraper-service
```

It must return only the affected services.

---

# 21. `changes` command

```bash
releaser changes scraper-service
```

Output:

```text
services/scraper/parser.go
common/http/client.go
```

It must show only the files that make the service affected.

---

# 22. `plan` command

```bash
releaser plan
```

Example:

```text
SERVICE           VERSION    AFFECTED    FILES
api               2.4.1      yes         4
worker            1.8.3      no          0
scraper-service   0.1.5      yes         2
```

Support:

```bash
releaser plan --format json
```

Example:

```json
{
  "services": [
    {
      "name": "scraper-service",
      "lastVersion": "0.1.5",
      "lastTag": "scraper-service/v0.1.5",
      "affected": true,
      "changedFiles": 2
    }
  ]
}
```

---

# 23. `release` command

Syntax:

```bash
releaser release <service> <patch|minor|major>
```

Example:

```bash
releaser release scraper-service patch
```

With latest release:

```text
scraper-service/v0.1.5
```

it computes:

```text
0.1.6
```

and creates:

```text
scraper-service/v0.1.6
```

on the current commit.

The command must check the service's affected state before the dry-run or the tag creation. If the service is not affected it must exit with exit code `1` and an error, without creating any tag.

To explicitly allow a release without relevant changes, support:

```bash
releaser release scraper-service patch --force
```

`--force` bypasses only the affected check and the check on an explicit version not greater than the latest one (section 24); all other checks still apply, including version validity, tag uniqueness and a clean working tree.

## 23.1 Bulk release

To release all affected services with the same bump:

```bash
releaser release --affected <patch|minor|major>
```

To release all configured services, even if not affected:

```bash
releaser release --all --force <patch|minor|major>
```

`--affected` and `--all` are mutually exclusive. `--all` always requires `--force`, so that bypassing the affected check stays explicit. The `--affected` and `--all` modes do not support `--version`: each service must have a previous release from which to compute the bump. The only bulk mode that accepts `--version` is `--new` (section 3), which concerns only services that have never been released.

Before creating the first tag, the command must validate the whole batch: service configuration, history state, bump, presence of a previous release, uniqueness of all new tags and a clean working tree. A validation error must therefore not leave partial tags. Processing order is the alphabetical order of the services. `--dry-run` must show all tags without modifying them; `--push` must publish all created tags to the configured remote with a single atomic push (`git push --atomic`). If creating a tag or the push fails, the local tags created by the command must be deleted: the command leaves no partial releases either locally or on the remote and can be rerun.

## 23.2 Releasing the root of a standard repository

For non-monorepo repositories, the release operates on the whole root without loading or requiring `releaser.yml`:

```bash
releaser release --root <patch|minor|major> [--dry-run] [--push]
releaser release --root --version <Initial SemVer> [--dry-run] [--push]
```

The tag has the form `<prefix>v<semver>`. The default prefix is inferred from previous release tags; during inference a prefix is valid only if it is empty or ends with a separator (`/`, `-`, `_`, `.`), so that tags such as `dev1.0.0` are not interpreted as releases with prefix `de`. With no releases the prefix is empty and produces, for example, `v0.1.5`. `--prefix=''` explicitly requests an empty prefix. If the release tags found have mixed prefixes, the command must fail and make `--prefix` mandatory.

A bump requires a previous release; bootstrap uses `--version` and a strict SemVer. As in section 24, a `--version` not greater than the latest release requires `--force`. If there are no changes between the selected previous tag and `HEAD`, the command fails unless `--force` is used explicitly. Tag ancestry, uniqueness of the new tag and a clean working tree remain mandatory. `--dry-run` creates no tags; `--push` publishes the tag to `origin`.

`--root`, `--affected` and `--all` are mutually exclusive modes.

---

# 24. Release with an explicit version

Support:

```bash
releaser release new-service --version 1.0.0
```

It is mainly meant for:

* bootstrap;
* migration of existing repositories;
* administrative cases.

The explicit version must be validated through strict SemVer.

If the service already has a release, the explicit version must be strictly greater than the latest one; otherwise the command fails with exit code `1`. With `--force` the tag is created anyway (for example to rebuild a tag missing from the history), but it does not become the latest release, because that is always the highest SemVer version: a "reset" to a lower version is therefore not possible.

---

# 25. Bootstrap

Service without tags:

```text
new-service
```

`status new-service --verbose`, with the same format as section 19:

```text
Service: new-service

Last version:
  none

Last tag:
  none

Changed files:

Affected:
  yes
```

In the `status` and `plan` tables the version appears as `none`; in `plan --format json` `lastVersion` and `lastTag` are `null`.

First release:

```bash
releaser release new-service --version 0.1.0
```

or:

```bash
releaser release new-service --version 1.0.0
```

From then on, the following become available:

```text
version-number
version-tag
next-version-number
next-version-tag
release patch
release minor
release major
```

---

# 26. Dry-run

```bash
releaser release scraper-service minor --dry-run
```

Output:

```text
Service: scraper-service
Current version: 0.1.5
Next version: 0.2.0
Tag: scraper-service/v0.2.0

No changes have been made.
```

---

# 27. Tag creation

Prefer annotated tags.

Example:

```text
scraper-service/v0.2.0
```

Message:

```text
Release scraper-service v0.2.0
```

---

# 28. Pushing tags

Optionally:

```bash
releaser release scraper-service patch --push
```

Default remote:

```text
origin
```

Configurable through the `remote` key of `releaser.yml` (section 6). `release --root` does not read the configuration and always uses `origin`.

The tool must not push implicitly without the flag.

---

# 29. Common

Example:

```text
common/logging
      │
      ├── api
      ├── worker
      └── scraper-service
```

If this changes:

```text
common/logging/logger.go
```

all three may become affected.

The check must, however, be done against each service's specific baseline.

---

# 30. Independent baselines example

History:

```text
A -- B -- C -- D -- E
     ↑       ↑       ↑
 api/v2.4.1  │      HEAD
          worker/v1.8.3
```

If `common/logging` changed in `C`:

```text
api
```

may be affected because the change comes after its release.

```text
worker
```

may be clean because the change is already included in the `worker/v1.8.3` tag.

So each service must be evaluated separately.

---

# 31. Ignore

Global configuration:

```yaml
ignore:
  - docs/**
  - "**/*.md"
```

Per-service configuration:

```yaml
services:
  api:
    paths:
      - services/api

    ignore:
      - services/api/docs/**
```

Patterns are relative to the repository root and support:

* `**`: any number of directories, including none;
* `*`: any sequence of characters within a path segment;
* `?`: a single character within a path segment.

All other characters, including square brackets, are literal. The global ignore and the service ignore are combined.

---

# 32. Rename and delete

The detector must handle:

* added;
* modified;
* deleted;
* renamed.

A deleted file can make the service affected.

For a rename, both the old path and the new path must be evaluated. Each path is relevant if it belongs to the service's `paths` or `dependencies` and is not ignored; the service is affected if at least one of the two is relevant, and `changes` reports only the relevant paths.

Paths must be read unambiguously (for example with `git diff-tree -z`), so that names with non-ASCII characters, spaces or quotes are not altered by Git's quoting.

---

# 33. Working tree

Read-only commands:

```text
status
affected
changes
plan
version-number
version-tag
next-version-number
next-version-tag
```

must operate on:

```text
HEAD
```

and ignore uncommitted changes.

The command:

```text
release
```

must require a clean working tree: no changes, staged or not, to tracked files. Untracked files do not block the release, because they cannot be part of the tag created on `HEAD` (for example artifacts downloaded in CI).

---

# 34. Detached HEAD

The tool must work correctly in detached HEAD.

This is essential for CI/CD.

---

# 35. Shallow clone

If the required history is not available, the tool must fail explicitly.

It must not silently produce a potentially wrong `affected` result.

In particular, if in a shallow clone a tag is not an ancestor of `HEAD`, the error must indicate missing history (exit code `3`) and not a diverging tag.

This also applies to the version commands (sections 11–14), `run` and `template` (sections 11.2 and 11.3): since they verify ancestry, the tags alone are not enough and the history between the latest release and `HEAD` must be available.

---

# 36. No implicit network access

The tool must not automatically run:

```text
git fetch
git fetch --tags
git pull
git push
```

except `push` when explicitly requested through a flag.

The pipeline is responsible for providing sufficient history and tags.

---

# 37. Exit codes

```text
0 = success
1 = validation failure / state not available
2 = configuration error
3 = Git error
4 = internal error (reserved, currently unused)
```

Code `1` also covers invalid CLI arguments or flags, unknown service, unknown variable, invalid bump, service without a release and a latest release that is not an ancestor of `HEAD`. Code `2` covers a missing configuration file, invalid YAML, unknown keys and invalid names or paths. Code `3` covers an invalid repository, incomplete history and failed Git commands, including push.

---

# 38. Tag formatter

Default:

```text
{{service}}/v{{version}}
```

Internally keep separate:

```text
service
version
tag
```

Never store:

```text
v0.1.5
```

as a SemVer value.

The `v` is the sole responsibility of the tag formatter.

---

# 39. Go architecture

Suggested structure:

```text
cmd/
└── releaser/
    └── main.go

internal/
├── config/
├── git/
├── version/
├── service/
├── changes/
├── release/
└── output/
```

---

# 40. `version` package

The internal package:

```text
internal/version
```

(named `version` rather than `semver` to avoid confusion with the library it wraps)

must act as a wrapper around the library:

```text
github.com/Masterminds/semver/v3
```

The rest of the application must not depend directly on the external library.

Indicative internal API:

```go
type Version struct {
    value *semver.Version
}

func Parse(value string) (Version, error)
func Compare(a, b Version) int

func (v Version) String() string
func (v Version) BumpPatch() Version
func (v Version) BumpMinor() Version
func (v Version) BumpMajor() Version
```

This makes it possible to replace the library in the future without changing the rest of the tool.

---

# 41. Git abstraction

Use the Git installed on the system through:

```go
os/exec
```

Indicative interface:

```go
type Git interface {
    Tags(pattern string) ([]string, error)

    Resolve(ref string) (string, error)

    DiffFiles(from, to string) ([]ChangedFile, error)

    IsAncestor(commit, descendant string) (bool, error)

    IsClean() (bool, error)

    IsShallow() (bool, error)

    CreateTag(tag, commit, message string) error

    DeleteTag(tag string) error

    PushTags(remote string, tags ...string) error // atomic push
}
```

---

# 42. CLI

Prefer **Cobra** for subcommands.

Planned commands:

```text
releaser status
releaser affected
releaser changes
releaser get-var

releaser version-number
releaser version-tag

releaser next-version-number
releaser next-version-tag

releaser plan
releaser release

releaser config check
```

The following must not exist:

```text
releaser next
```

---

# 43. External dependencies

Planned dependencies:

```text
Cobra
YAML parser
Masterminds/semver/v3
```

Git is invoked through its CLI.

Goal:

```text
single binary
no runtime dependencies other than Git
```

---

# 44. CI/CD

Example:

```bash
VERSION=$(releaser version-number scraper-service)
```

Docker:

```bash
docker build \
  -t registry.example.com/scraper-service:${VERSION} \
  services/scraper
```

Or:

```bash
NEXT_VERSION=$(releaser next-version-number scraper-service patch)
```

Or:

```bash
TAG=$(releaser version-tag scraper-service)
```

---

# 45. No implicit build and deploy

The tool handles:

```text
Git
versioning
affected detection
release tagging
```

It does not handle:

```text
Docker build
Maven
Gradle
npm
Composer
deployment
Kubernetes
Helm
OpenShift
```

It stays completely polyglot.

---

# 46. No VERSION file

Files such as:

```text
VERSION
version.txt
.release-version
```

or equivalents must not be used.

The version exists only as a property of the Git release.

This removes synchronization problems between:

```text
local file
Git tag
published artifact
```

---

# 47. Main invariant

For each service:

```text
service
   ↓
find latest <service>/v*
   ↓
parse SemVer
   ↓
latest release
   ↓
diff tag..HEAD
   ↓
paths + dependencies
   ↓
affected
```

To get the version:

```text
version-number
      ↓
0.1.5
```

To get the tag:

```text
version-tag
      ↓
scraper-service/v0.1.5
```

To compute the next version:

```text
next-version-number scraper-service patch
      ↓
0.1.6
```

To compute the next tag:

```text
next-version-tag scraper-service patch
      ↓
scraper-service/v0.1.6
```

---

# 48. Final philosophy

The model must remain:

```text
Git history
    =
state of the code

namespaced Git tag
    =
microservice release

releaser.yml
    =
dependency graph

SemVer
    =
identity and meaning of the release
```

A monorepo shares its Git history, but not necessarily its release lifecycle.

Each service keeps:

* an independent version;
* independent tags;
* independent releases;
* an independent build;
* an independent deployment.

The tool must add as little state as possible and leave Git as the single authoritative source of versioning.
