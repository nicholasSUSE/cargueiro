# CI

GitHub Actions checks every pull request, and runs the same checks again for the commit of every release. A change reaches `main` only through a pull request whose checks passed, so a push to `main` is not checked a second time. The workflow lives in `.github/workflows`. It runs the same `make` targets as a developer, so a failure in CI can be reproduced locally with the command named in the failing step.

## What runs

The checks run as separate jobs, in parallel, on a Linux runner. Linux is the only platform checked.

| Step | Command | Fails when |
|------|---------|------------|
| Format | `make fmt`, then `git diff --exit-code` | formatting changed any file |
| Lint | `make lint` | golangci-lint reports a finding |
| Vet | `make vet` | `go vet` reports a finding (with the build tags of the e2e and release tests) |
| Docs | `make docs`, then `git diff --exit-code` | the README commands table is stale |
| Unit tests | `make test-unit` | a test fails (race detector on) |
| E2E tests | `make test-e2e` | an e2e test fails |
| Release binaries | `make test-release` | a release target does not build, or a binary has the wrong format or architecture, or does not carry the version |
| Shell checks | `make lint-sh` | shellcheck reports a finding in `install.sh` |
| Install test | `make test-install` | `install.sh` fails against a local fake release, ignores the version pin, or installs on a wrong checksum |

The unit tests start nothing, so they run in a job of their own and do not wait for the e2e tests. `make test-e2e` builds the binary and starts the local registries with the Docker on the runner. It stops them again, even when a test fails.

## Release binaries

`make release-build` builds cargueiro for every release target into `bin/`, next to the `bin/cargueiro` that `make build` writes for the host. `make build` stays a fast build for the host only. The build is a Go program in `tools/build`, which is also the one place that lists the targets. It uses plain `go build` without cgo, with `-trimpath`, no VCS stamp and a stripped binary, so the same source gives the same bytes whatever the directory or the git state, and stamps the version with the same `VERSION` as `make build`, which is `dev` unless you set it. The targets build in parallel, and one that fails does not stop the others.

`make test-release` builds them and then checks each binary from its header: the format and the architecture of its operating system, and that the version reached the one binary the runner can run. The test has a build tag of its own, so `make test` and the e2e tests do not pay for a cross-build. CI builds and checks the binaries on every pull request, in a job of its own. Nothing is published: that is the release pipeline.

## Releasing

A release is started by a person, never by pushing a tag. Run the release workflow from `main` (the Actions tab, or `gh workflow run Release -f version=v0.0.1` from a terminal) and type the version.

- **Accepted versions.** `v1.2.3`, or `v1.2.3-rc.1` for a release candidate, which is published as a prerelease. Anything else is stopped at the start, before any build, and so is a version whose tag already exists or a run that was not started from `main`.
- **What happens, in order.** The guard checks the branch and the version. The CI jobs then run again for that commit. Only if they all pass does one job build the release binaries, check them (`make test-release`, with the typed version) and publish those same files: it creates a draft release, attaches the files, and publishes it. The tag is created by the publish, on the commit that was checked, so a tag never exists for a commit that failed.
- **What is published.** The binaries for every release target, a `checksums.txt`, the installer `tools/scripts/install.sh` as `install.sh`, and the release notes that GitHub generates from the merged pull requests. The asset names carry no version, so the download link of the latest release does not change from one release to the next.
- **If it fails.** A release that lacks `install.sh` is not published, because the install command in the README downloads it from the latest release. Nothing is published, and a draft release that the run created is removed. Fix the cause and run the workflow again with the same version.
- **Verifying a download.** Put the binary and `checksums.txt` in one directory and run `sha256sum --ignore-missing -c checksums.txt`.
- **The version a binary reports.** A release prints its tag. Every other build prints `dev`, whatever commit it was built from.

Only the publishing job can write to the repository. The input is passed to the scripts as an environment variable and is checked before it is used.

## Tool versions

`mise.toml` is the only place with tool versions. A local action, `.github/actions/setup`, reads them with `tools/scripts/mise-version.sh` and installs Go, golangci-lint and Helm with their own setup actions, so the ones in CI are the ones you use locally. There is no version in the workflows. Helm has to be the version of the `helm.sh/helm/v4` library in `go.mod`, and a test fails if it is not.

shellcheck is downloaded from its release and checked against a sha256 kept next to the action. The checksum names the archive of one version, so changing the pin in `mise.toml` without the checksum fails the job. The actions the setup uses are pinned by commit, and the organization only allows some actions, which is why mise has no action of its own here.

## Pinned actions

Every `uses:` is pinned to a full commit SHA, with the release in a comment. Tags can be moved, commits cannot. To update an action, look up the SHA of the release you want and change both the SHA and the comment:

```
gh api repos/<owner>/<action>/git/ref/tags/<tag>
```

If the tag is an annotated one, the result is a tag object: follow it with `gh api repos/<owner>/<action>/git/tags/<sha>` to get the commit.

The workflow has `permissions: contents: read` and nothing else, and a run is cancelled when a newer commit is pushed to the same branch.

## Run it locally

```
make fmt && git diff --exit-code
make lint
make vet
make docs && git diff --exit-code
make test-unit
make test-e2e
make test-release
```

Start from a clean tree, so that `git diff` shows only what the commands changed. `make test-e2e` needs Docker, and the pinned `helm` first on `PATH` (see "Editor (VSCode)" in [DEVELOPING.md](DEVELOPING.md) if another Helm shadows it).

## Branch protection

CI reports a result; it does not stop a merge. To require it, mark the workflow's checks as required in the repository's rules for `main`. That is a repository setting, not part of the workflow.

A required check is matched by the name of its job. Renaming a job in the workflow means updating the rule, or the old name stays required and no pull request can merge.

## Settings for releases

These are repository settings, applied by hand, not by a workflow.

### Tag rules

A tag ruleset for the release tags (`v*`) restricts updating, deleting and force-pushing them, with an empty bypass list, so a published version's tag cannot be moved or removed. Creation is not restricted: the release workflow's own token is not exempt from a tag ruleset, and publishing fails with `Cannot create ref due to creations being restricted`. To restrict it too, put the GitHub Actions app in the bypass list, or create the tag with the token of a small GitHub App that is.

### Immutable releases

With the repository setting on, a release published afterwards is locked: its tag and its files cannot be changed or deleted, and a deleted release's tag name can never be reused, release candidates included. This is why the workflow creates a draft, attaches every file, and only then publishes. `gh release view <tag> --json isImmutable` shows whether a release came out immutable.

### Checking a change

After changing one of these settings, run the release workflow with a release candidate. A failed publish names the rule that stopped it, and the draft is removed.
