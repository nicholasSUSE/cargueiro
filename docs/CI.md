# CI

GitHub Actions checks every pull request and every push to `main`. The workflow is `.github/workflows/ci.yml`. It runs the same `make` targets as a developer, so a failure in CI can be reproduced locally with the command named in the failing step.

## What runs

Three jobs run in parallel on `ubuntu-latest`. Linux is the only platform checked.

| Job | Step | Command | Fails when |
|-----|------|---------|------------|
| `static` | Format | `make fmt`, then `git diff --exit-code` | formatting changed any file |
| `static` | Lint | `make lint` | golangci-lint reports a finding |
| `static` | Vet | `make vet` | `go vet` reports a finding (with the `e2e` tag) |
| `static` | Docs | `make docs`, then `git diff --exit-code` | the README commands table is stale |
| `unit` | Unit tests | `make test-unit` | a test fails (race detector on) |
| `e2e` | E2E tests | `make test-e2e` | an e2e test fails |

The unit tests start nothing, so they have a job of their own and finish while the e2e tests are still running. `make test-e2e` builds the binary and starts the two local `registry:2` registries with the Docker on the runner. It stops them again, even when a test fails.

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
```

Start from a clean tree, so that `git diff` shows only what the commands changed. `make test-e2e` needs Docker, and the pinned `helm` first on `PATH` (see "Editor (VSCode)" in [DEVELOPING.md](DEVELOPING.md) if another Helm shadows it).

## Branch protection

CI reports a result; it does not stop a merge. To require it, mark the `static`, `unit` and `e2e` checks as required in the repository's branch protection rules for `main`. That is a repository setting, not part of the workflow.
