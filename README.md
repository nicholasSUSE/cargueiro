# cargueiro

Small, single-purpose tools to package Helm charts and push them to OCI registries.

## Install

cargueiro is one self-contained binary. Install the latest release with:

```
curl -sfL https://github.com/rancher/cargueiro/releases/latest/download/install.sh | sh -
```

It detects your system, downloads the right binary, verifies it against
`checksums.txt`, and installs it to `/usr/local/bin/cargueiro` (with `sudo` when
you are not root). Run it again to update. Supported systems: Linux (amd64),
macOS (amd64 and arm64) and FreeBSD (amd64).

Pin a version with `CARGUEIRO_VERSION`:

```
curl -sfL https://github.com/rancher/cargueiro/releases/latest/download/install.sh | CARGUEIRO_VERSION=v0.2.0 sh -
```

If you would rather not pipe `curl` into `sh`, download the script, read it,
verify it and run it:

```
curl -sfLO https://github.com/rancher/cargueiro/releases/latest/download/install.sh
less install.sh
sh install.sh
```

### Uninstall

Remove the binary, and the state file cargueiro writes for your saved registry
login:

```
sudo rm -f /usr/local/bin/cargueiro
rm -f ~/.config/cargueiro/cargueiro.json
```

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
| [`check archive`](cmd/docs/check-archive.md) | Check one packaged chart (.tgz) against the chart rules. |
| [`check chart`](cmd/docs/check-chart.md) | Check one chart directory against the chart rules. |
| [`check oci`](cmd/docs/check-oci.md) | Check one chart in an OCI registry against the chart rules. |
| [`index`](cmd/docs/index.md) | Write the `index.yaml` of a charts repository from its `entries/`. |
| [`list`](cmd/docs/list.md) | List the versions of a chart in an OCI registry, newest first. |
| [`package`](cmd/docs/package.md) | Package a Helm chart into a .tgz archive. |
| [`push`](cmd/docs/push.md) | Push a packaged Helm chart to an OCI registry. |
| [`registry`](cmd/docs/registry.md) | Manage the registry login cargueiro saves. |
| [`registry login`](cmd/docs/registry-login.md) | Save credentials and connection settings for a registry. |
| [`registry logout`](cmd/docs/registry-logout.md) | Forget the saved login. |
| [`ship`](cmd/docs/ship.md) | Write the entry and the logo of a chart in a registry to a charts repository. |
| [`stamp`](cmd/docs/stamp.md) | Put the final version and the helm.sh/images annotation on a chart directory. |
| [`version`](cmd/docs/version.md) | Print the cargueiro version. |
<!-- COMMANDS:END -->

## Documentation

The guides in `docs/`:

| Doc | Details |
|-----|---------|
| [`CHART_OWNER.md`](docs/CHART_OWNER.md) | Releasing a chart, for chart owners: stamp, check, package and push a chart from your repository, send it to `rancher/charts`, and what your repository needs for that. |
| [`DEVELOPING.md`](docs/DEVELOPING.md) | Development, for contributors: setup with mise, how a feature is built and tested, the chart rules, linting, the local registries, and what to run before pushing. |
| [`DEV_ENV.md`](docs/DEV_ENV.md) | VSCode `settings.json` and `launch.json` snippets to test, lint and debug with the tools that mise pins. |
| [`AGENTS.md`](docs/AGENTS.md) | Rules for LLM agents. They ask whether the user is a chart owner or a contributor, and contributions made with agents follow its review, style and commit rules. |
| [`CI.md`](docs/CI.md) | What GitHub Actions checks on every pull request, how a release is built and published, how tool versions and actions are pinned, and the repository settings it relies on. |
| [`INDEX.md`](docs/INDEX.md) | The rule `cargueiro index` applies to the entries, the `index.yaml` and the archives of a charts repository: what is decided for each version, the digests, hiding and permanence. |

LLM Policy notice: All contributions need a human reviewer responsible for it.
