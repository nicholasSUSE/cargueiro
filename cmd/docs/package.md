# package

Package a Helm chart into a .tgz archive.

The argument is one chart directory containing a `Chart.yaml`. The archive is named `<name>-<version>.tgz` after the chart metadata and written to the output directory (`--output-dir`, default `assets`), which is created if it does not exist.

The archive path is printed to stdout. Logs and errors go to stderr, and a failure makes the exit code non-zero.

Dependencies declared in `Chart.yaml` must already be present under the chart's `charts/` directory. Nothing is downloaded.

## Examples

```
# package one chart into ./assets
cargueiro package charts/fleet/110.0.3+up0.16.3
```

```
# choose the output directory
cargueiro package charts/fleet/110.0.3+up0.16.3 --output-dir output
```
