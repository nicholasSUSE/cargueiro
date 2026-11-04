# Developing

## Linting

`.golangci.yml` is the single source of truth. Every runner (editor, agent hook, CI) uses the same config and the same pinned golangci-lint version.

| Runner | How |
|--------|-----|
| Local | `make lint` / `make fmt` |
| Editor (VSCode) | `go.lintTool: golangci-lint-v2`, picks up `.golangci.yml` |
| Agent hook | `scripts/lint-hook.sh`, runs after every `.go` edit |
| CI (GHA) | `golangci/golangci-lint-action`, same version |

### Version pinning

The version is set once in the `Makefile`:

```make
GOLANGCI_VERSION ?= v2.15.0
```

`make lint` and `make fmt` run `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)`. It compiles with the local Go toolchain, is cached after the first run and adds nothing to `go.mod`. CI reads the same version. To upgrade, change that one variable.

### Linters

Uber's minimum set (`errcheck`, `goimports`, `revive`, `govet`, `staticcheck`) plus linters for the remaining [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) rules: `gochecknoinits`, `gochecknoglobals`, `errname`, `exhaustive`, `forcetypeassert`, `errorlint`, `predeclared`.

Suppress a finding only with a reason: `//nolint:gochecknoglobals // set via ldflags`.

### Agent hook

A `PostToolUse` hook on `Edit|Write|MultiEdit` runs `scripts/lint-hook.sh`:

1. Reads the edited path from the hook's stdin JSON and skips non-`.go` files.
2. Runs `make fmt`, then lints only the edited package (`./<dir>/...`).
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

## Before pushing

```
make test
make vet
make lint
```
