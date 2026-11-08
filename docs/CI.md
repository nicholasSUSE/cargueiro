# CI

GitHub Actions checks every pull request and every push to `main`. The workflow lives in `.github/workflows`. It runs the same `make` targets as a developer, so a failure in CI can be reproduced locally with the command named in the failing step.

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

The unit tests start nothing, so they run in a job of their own and do not wait for the e2e tests. `make test-e2e` builds the binary and starts the local registries with the Docker on the runner. It stops them again, even when a test fails.

## Release binaries

`make release-build` builds cargueiro for every release target into `bin/`, next to the `bin/cargueiro` that `make build` writes for the host. `make build` stays a fast build for the host only. The build is a Go program in `tools/build`, which is also the one place that lists the targets. It uses plain `go build` without cgo, with `-trimpath` and a stripped binary, and stamps the version with the same `VERSION` as `make build`, which is `dev` unless you set it. The targets build in parallel, and one that fails does not stop the others.

`make test-release` builds them and then checks each binary from its header: the format and the architecture of its operating system, and that the version reached the one binary the runner can run. The test has a build tag of its own, so `make test` and the e2e tests do not pay for a cross-build. CI builds and checks the binaries on every pull request, in a job of its own. Nothing is published: that is the release pipeline.

## Tool versions

`mise.toml` is the only place with tool versions. The workflow installs them with `jdx/mise-action`, so Go, golangci-lint and Helm are the ones you use locally. There is no `go-version` in the workflow. Helm has to be the version of the `helm.sh/helm/v4` library in `go.mod`, and a test fails if it is not.

The version of mise itself is pinned in the workflow (`version:` of the action), so a new mise release cannot change a run. Raise it by hand when needed.

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
