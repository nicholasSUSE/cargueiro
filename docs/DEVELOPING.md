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

Test data is committed by hand, never fetched by a script. See `tests/data/charts/fleet/PROVENANCE.md` for where it came from. Nothing under `tests/` is embedded in the binary.

## Before pushing

```
make test
make vet
make lint
```
