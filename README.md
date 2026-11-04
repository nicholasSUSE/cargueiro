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
| [`package`](cmd/docs/package.md) | Package Helm charts into .tgz archives. |
| [`version`](cmd/docs/version.md) | Print the cargueiro version. |
<!-- COMMANDS:END -->

## Development

See [docs/DEVELOPING.md](docs/DEVELOPING.md). Contributions made with LLM agents follow [docs/AGENTS.md](docs/AGENTS.md).
