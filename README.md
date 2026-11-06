# cargueiro

Small, single-purpose tools to package Helm charts and push them to OCI registries.

## Build

```
make build
./bin/cargueiro --help
```

## Commands

Each command is documented in its own file under `cmd/docs/`. That file is the source of truth for `cargueiro <command> --help` and for this table.

<!-- COMMANDS:START -->
| Command | Description |
|---------|-------------|
| [`check`](cmd/docs/check.md) | Check that a chart follows the Rancher chart rules. |
| [`check chart`](cmd/docs/check-chart.md) | Check one chart directory against the chart rules. |
| [`package`](cmd/docs/package.md) | Package a Helm chart into a .tgz archive. |
| [`push`](cmd/docs/push.md) | Push a packaged Helm chart to an OCI registry. |
| [`registry`](cmd/docs/registry.md) | Manage the registry login cargueiro saves. |
| [`registry login`](cmd/docs/registry-login.md) | Save credentials and connection settings for a registry. |
| [`registry logout`](cmd/docs/registry-logout.md) | Forget the saved login. |
| [`version`](cmd/docs/version.md) | Print the cargueiro version. |
<!-- COMMANDS:END -->

## Development

See [docs/DEVELOPING.md](docs/DEVELOPING.md).

Contributions made with LLM agents follow [docs/AGENTS.md](docs/AGENTS.md).

LLM Policy notice: All contributions need a human reviewer responsible for it.
