# Developing

## Setup

Tool versions are pinned in `mise.toml`: Go, golangci-lint, dlv, helm, oras, crane and the Docker CLI.

1. Install [mise](https://mise.jdx.dev/).
2. Activate it in your shell (zsh shown; add after your `PATH` setup):
   ```
   eval "$(mise activate zsh)"
   ```
3. From the repo root, run `mise trust` and `mise install`.
4. Have a Docker daemon running. mise pins the Docker CLI only; the daemon comes from the host.

Check that the pinned tools win over anything else on your `PATH`:

```
mise doctor | grep activated   # activated: yes
mise which helm golangci-lint  # paths under ~/.local/share/mise/installs
```

If a pinned tool is shadowed, `mise which <tool>` shows the right one and `command -v <tool>` the one actually used. Two common causes:

- mise is not activated, so system or Homebrew binaries come first. Activate it as above.
- An editor installed a stale copy into `GOBIN`. `mise.toml` lists `go` last so pinned tools take precedence; remove the stale binary if it still wins.

The pinned `helm` must match `helm.sh/helm/v4` in `go.mod`.

## What a feature is

cargueiro follows the Unix philosophy: each command does one thing, does it well, and prints results that other tools can consume. A **feature** is one such command, delivered end to end. It is **feature complete** only when all five layers exist:

| # | Layer | Where | Done when |
|---|-------|-------|-----------|
| 1 | Command | `cmd/<name>.go` | The cobra command parses arguments and flags, calls the implementation and reports. |
| 2 | Implementation | `internal/<area>/` | The logic lives here, with no cobra and no printing. |
| 3 | Unit tests | next to the code | Every path is covered without external tools. |
| 4 | E2E tests | `tests/<name>/` | The real binary is proven correct on real data. |
| 5 | Documentation | `cmd/docs/<name>.md` | `--help`, the README table and the doc page agree. |

A change that skips a layer is not a finished feature, whatever the other layers look like.

### 1. Command

- One command, one job. If a command needs "and" to describe it, split it.
- One input. A command never takes several; a script calls it once per input.
- Keep it thin: validate arguments, call `internal/`, write results. No business logic.
- Results go to **stdout**, one item per line, so another command can read them. Logs and errors go to **stderr**.
- Return errors from `RunE` ([error handling](#error-handling)).
- Inject dependencies (the implementation as a function or small interface) so the command is testable without the real thing.

### 2. Implementation

- Put it under `internal/<area>/`. It must not import cobra.
- `context.Context` is the first parameter. Errors follow [Error handling](#error-handling).
- Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md); `make lint` enforces most of it.
- Nothing is fetched or mutated beyond what the command's doc promises.

### 3. Unit tests

- Table-driven, next to the code (`*_test.go`), run by `make test-unit`.
- Cover success, every error path and the edge cases. Use fakes for boundaries (the command tests use a fake packager).
- No network, no Docker, no external binaries. Small fixtures go in a local `testdata/`.

### 4. E2E tests

- Live in `tests/<name>/`, behind the `e2e` build tag, run by `make test-e2e`.
- Run the **real binary** as a black box through `tests/internal/harness`: arguments in, stdout, stderr and exit code out.
- Use real data committed under `tests/data/` with a `PROVENANCE.md` saying where it came from. Never fetch data in a test.
- Compare against an independent source of truth wherever one exists. For packaging that is the `helm` CLI and the archives published in `rancher/charts`; comparing against our own output proves nothing.
- Cover the happy path, defaults, failures, that inputs are not modified, and determinism.
- Break something on purpose once to see the test fail, so you know it can.
- Features that need a registry get it from `registry:2`; tests read its address from the environment and never start it themselves.

### 5. Documentation

- Write `cmd/docs/<name>.md`: a title, a one-line summary, a longer description and an `## Examples` section. The layout is described in [Command documentation](#command-documentation).
- Mention every local flag of the command in that file. A test fails if one is missing.
- Run `make docs` to refresh the README table. A test fails if the table is stale.
- Update this file when the feature changes how people work: new tools go in `mise.toml` and in Setup, new test data in its `PROVENANCE.md`.

### Checklist

A feature is complete when all of this is true:

- [ ] The command, its implementation, unit tests, e2e tests and docs all exist.
- [ ] `make fmt`, `make lint` and `make vet` report nothing.
- [ ] `make test` passes (unit and e2e).
- [ ] `make docs` leaves no diff.
- [ ] New tools are pinned in `mise.toml`.
- [ ] A human reviewed it, as required by [AGENTS.md](AGENTS.md).

Commit by layer, with Conventional Commit messages, so the history reads like this list. For `package` that was `feat(chart)` for the implementation, `feat(cmd)` for the command, `test(e2e)` for the test data and tests, and `feat(docs)` for the documentation.

### What already exists

Later features reuse these and only add their own pieces:

- The docs pipeline: the `cmd/docs` loader, the guard tests and `make docs`.
- The e2e harness (`tests/internal/harness`): building and running the binary, running `helm`, comparing archives by content (`internal/chart/archivetest`).
- The tooling: mise pins, lint config, the lint hook and the Make targets.

## Error handling

| # | Rule |
|---|------|
| 1 | Wrap with context: `fmt.Errorf("verb noun: %w", err)`. Lowercase, no trailing punctuation. Add context only when the inner error lacks it. |
| 2 | Use `fmt.Errorf` or `errors.New`. Add an error type only when a caller must branch on it. |
| 3 | Sentinels (`var ErrX = errors.New(...)`, matched with `errors.Is`) name stable conditions. Declare them next to the code that returns them. |
| 4 | Custom types (matched with `errors.As`) only when the caller needs data from the error. |
| 5 | Handle an error once: log it or return it, never both. Only `main` prints errors and exits. |
| 6 | Do not leak internals: wrap with `%w` only where callers may depend on the cause, otherwise `%v`. |
| 7 | Advice for the user belongs in `cmd/`. `internal/` returns facts. |
| 8 | Exit codes: 0 success, 1 failure. |
| 9 | No package-wide `errors.go`. A dedicated file is only for an error type with methods. |

## Chart annotation rules

`rules.yaml` at the repository root lists every chart cargueiro checks, the kind of each, and the annotations each kind requires. `internal/rules` parses it and `internal/annotations` validates a chart's annotations against it. A chart the file does not list is an error. The validators take parsed data and do no file I/O.

| Kind | Chart | Required annotations (under `catalog.cattle.io/`) |
|------|-------|---------------------------------------------------|
| `main` | a product chart, with or without a `-crd` sibling | `release-name`, `certified`, `rancher-version`, `kube-version`, `permits-os` |
| `crd` | a CRD chart | `release-name`, `certified`, `hidden` |
| `extra` | a companion or add-on of a product chart | `certified`, `rancher-version`, `kube-version`, `permits-os` |

`namespace` is optional for every kind. Space around a value is ignored. Besides being present, a value must be well formed:

| Annotation | Valid value | Kinds |
|------------|-------------|-------|
| `certified` | `rancher` | all |
| `release-name` | the chart name, or the override in `rules.yaml` | `main`, `crd` |
| `hidden` | `true` | `crd` |
| `kube-version`, `rancher-version` | both bounds, written `>= X.Y.Z-0 < X.Y.Z-0`, lower below upper | `main`, `extra` |
| `permits-os` | `linux`, optionally with `windows`, comma separated | `main`, `extra` |

Each kind has one function in `internal/annotations` (`ValidateMainChart`, `ValidateCRD`, `ValidateExtraChart`) that lists its checks. Each annotation has its own file with a `check<Name>` function that reads the annotation and a function that validates the value. To change a rule, edit that file. To require another annotation of a kind, list it under that kind in `rules.yaml`; to also check its value, add a line to the kind's function.

### helm.sh/images

The `helm.sh/images` annotation follows [Helm HIP-15](https://github.com/helm/community/blob/main/hips/hip-0015.md), a draft, and adds Rancher policy on top. `ValidateHelmImages(annotations, values)` is meant to run for every kind, next to the kind's own validator, and takes the parsed `Chart.yaml` annotations and `values.yaml`. A chart whose `values.yaml` has no images needs no annotation.

| Rule | Source |
|------|--------|
| The value is a YAML list; each entry has a `name` and an `image`; other properties are allowed | HIP-15 |
| `dependency` is optional and names the subchart an image belongs to | HIP-15 |
| `name` is unique in the list | Rancher policy (HIP-15 allows repeats) |
| `image` is written `repository:tag` | Rancher policy (HIP-15 only says "a location") |
| An image in `values.yaml` is a map with a `repository` key. It needs a non-empty string `repository` and `tag` (quote a numeric tag: `tag: 1.10` is rejected); both empty means no image | Rancher policy |
| A string value under any key, including one called `image`, is not an image and is ignored | Rancher policy |
| Every image in `values.yaml` is listed in the annotation, compared as `repository:tag` | Rancher policy |
| An entry that is not in `values.yaml` has a non-empty `dependency`, whose value is not checked | Rancher policy |

The `registry` key beside a `repository` in `values.yaml` is never part of a comparison, because the release process switches it between registries. An annotation image is compared as written, so `registry.suse.com/rancher/a:1` does not match `repository: rancher/a` with `tag: "1"`.

Known limits: a chart repository URL under a `repository` key is read as an image, `repo@sha256:...` passes the image format, and a null item in the annotation list is skipped by the YAML decoder, so entry numbers can shift.

## Linting

`.golangci.yml` is the single source of truth. Every runner (editor, agent hook, CI) uses the same config and the same pinned golangci-lint version from `mise.toml`.

| Runner | How |
|--------|-----|
| Local | `make lint` / `make fmt` |
| Editor (VSCode) | `go.lintTool: golangci-lint-v2`, picks up `.golangci.yml` |
| Agent hook | `scripts/lint-hook.sh`, runs after every `.go` edit |
| CI (GHA) | `jdx/mise-action`, reads the same `mise.toml` |

### Version pinning

The version lives once, in `mise.toml` (`golangci-lint = "2.15.0"`). `make lint` and `make fmt` call `golangci-lint` from `PATH`, so they use whatever mise provides. To upgrade, change that line and run `mise install`.

The pinned binary is a prebuilt release built with a specific Go version. If `go.mod` moves to a newer Go minor before golangci-lint publishes a matching build, `make lint` fails with "the Go language version used to build golangci-lint is lower than the targeted Go version". Bump golangci-lint, or install it from source with mise's `go:` backend (`"go:github.com/golangci/golangci-lint/v2/cmd/golangci-lint"`).

### Linters

Uber's minimum set (`errcheck`, `goimports`, `revive`, `govet`, `staticcheck`) plus linters for the remaining [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) rules: `gochecknoinits`, `gochecknoglobals`, `errname`, `exhaustive`, `forcetypeassert`, `errorlint`, `predeclared`.

Suppress a finding only with a reason: `//nolint:gochecknoglobals // set via ldflags`.

### Agent hook

A `PostToolUse` hook on `Edit|Write|MultiEdit` runs `scripts/lint-hook.sh`:

1. Reads the edited path from the hook's stdin JSON and skips non-`.go` files.
2. Runs `make fmt`, then lints only the edited package (`./<dir>`).
3. On findings, prints them to stderr and exits `2`, so the agent sees them and fixes them.

The script is committed. The hook registration lives in `.claude/settings.json`, which is gitignored, so each developer registers it locally:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [{ "type": "command", "command": "scripts/lint-hook.sh" }]
      }
    ]
  }
}
```

The hook is fast local feedback only. CI is the enforcing check.

## Command documentation

Each command is documented once, in `cmd/docs/<command>.md`. That file feeds `cargueiro <command> --help` and the commands table in `README.md`.

Layout of a command doc:

````markdown
# <command>

<first paragraph: short description>

<long description>

## Examples

```
cargueiro <command> ...
```
````

Flags stay defined in Go and show up in `--help` automatically. Mention each local flag (for example `--output-dir`) in the doc; a test fails if one is missing.

After changing a doc, run `make docs` to refresh the README table. `make test` fails if the table is stale, if a command has no doc, or if a doc has no command.

## Editor (VSCode)

`.vscode/` is gitignored. Create `.vscode/settings.json` locally so the editor sees the e2e tests and uses the repo's linter:

```json
{
  "go.buildTags": "e2e",
  "go.lintTool": "golangci-lint-v2"
}
```

Without `go.buildTags`, running an e2e test from the editor fails with `build constraints exclude all Go files`, because the e2e files carry the `e2e` build tag. E2E tests also need the pinned `helm` on `PATH`, so start VSCode from a shell where mise is activated (for example `code .`).

## Debugging (VSCode)

Create `.vscode/launch.json` locally from this template (requires [Delve](https://github.com/go-delve/delve)):

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "cargueiro version",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}",
      "args": ["version"],
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "cargueiro version (--debug)",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}",
      "args": ["--debug", "version"],
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "Test current package",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${fileDirname}"
    }
  ]
}
```

Open `cargueiro/` as the workspace root. Change `args` to debug other commands. Debug runs skip the Makefile `-ldflags`, so the version output shows `dev (commit none)`.

## Testing

| Target | What it runs |
|--------|--------------|
| `make test-unit` | Unit tests (`go test -race ./...`), no external tools |
| `make test-e2e` | E2E tests under `tests/` (`-tags e2e`), against the real binary |
| `make test` | Both |

E2E tests build `bin/cargueiro` themselves (or use the binary in `CARGUEIRO_BIN`) and run it as a black box.

`tests/packaging` packages the real SUSE fleet charts in `tests/data/charts/fleet` and compares three archives per chart by content (mode, owner, size, sha256 per file; gzip headers and mtimes ignored): the one built by cargueiro, the one built by the `helm` CLI, and the one published in `rancher/charts`. It needs the `helm` CLI from `mise.toml` on `PATH`, at the same version as `helm.sh/helm/v4` in `go.mod`; a test fails with a clear message if it is not.

`tests/pushing` pushes the published fleet archives to the local registries over HTTP and over TLS with a self-signed CA, then pulls them back with the `helm` CLI and compares the bytes. Each test logs in first, because `push` has no transport flags. It also covers a wrong explicit password, a push without any login, a corrupt archive, and `package` followed by `push`.

`tests/login` logs in and out of the local registries over HTTP and TLS and checks what `push` does with what was saved: a second login is refused until logout, explicit credentials win, a wrong password saves nothing, logout forgets everything, and the file is private.

Test data is committed by hand, never fetched by a script. See `tests/data/charts/fleet/PROVENANCE.md` for where it came from. Nothing under `tests/` is embedded in the binary.

## Local registries

E2E tests for `push` use two throwaway `registry:2` containers, both bound to `127.0.0.1` only and both requiring basic auth, like the production registries:

| Name | Address | Transport |
|------|---------|-----------|
| `cargueiro-registry` | `127.0.0.1:5050` | HTTP |
| `cargueiro-registry-tls` | `127.0.0.1:5051` | HTTPS, self-signed CA |

Credentials: `admin` / `cargueiro-test`. They are test-only; the htpasswd file is `tests/data/registry/htpasswd`.

```
make registry-up     # start both (needs a running Docker daemon)
make registry-down   # stop and remove both
```

`make test-e2e` starts them and stops them again, even when tests fail. Run `make registry-up` yourself to run a single e2e test from the editor or with `go test -tags e2e`; the tests fail with a hint if the registries are not reachable.

The TLS certificates are generated by `tests/gencerts` on every `make registry-up` into `.tmp/registry/certs/` (gitignored); `ca.crt` there is the CA to save with `registry login --ca-file`. The registry image is pinned in the `Makefile` (`REGISTRY_IMAGE`) and the ports can be overridden (`REGISTRY_PORT`, `REGISTRY_TLS_PORT`, and `CARGUEIRO_TEST_REGISTRY_HTTP` / `CARGUEIRO_TEST_REGISTRY_TLS` for the tests). Port 5000 is deliberately not used.

To use them by hand, log in first. Only one login is kept, so log out before switching registries:

```
echo cargueiro-test | cargueiro registry login 127.0.0.1:5050 --username admin --password-stdin --plain-http
echo cargueiro-test | cargueiro registry login 127.0.0.1:5051 --username admin --password-stdin --ca-file .tmp/registry/certs/ca.crt
```

After that, `push` needs no credentials and no transport flags. How login works is described in `cmd/docs/registry-login.md`. The `helm` CLI needs its own `--plain-http` or `--ca-file`. Chart versions containing `+` are stored with `_` in the tag (`110.0.3_up0.16.3`).

E2E tests run every command with its own throwaway `CARGUEIRO_CONFIG_DIR`, so they never read or write your real saved login.

## Before pushing

```
make test
make vet
make lint
```
