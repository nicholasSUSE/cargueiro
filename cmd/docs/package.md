# package

Package Helm charts into .tgz archives.

Each argument is a chart directory containing a `Chart.yaml`. The archive is named `<name>-<version>.tgz` after the chart metadata and written to the output directory (`--output-dir`, default `dist`), which is created if it does not exist.

Archive paths are printed to stdout, one per line, in argument order, so the output can be piped into other tools. Logs go to stderr.

Every chart is attempted. If any fails, the remaining charts are still packaged, every error is reported and the exit code is non-zero.

Dependencies declared in `Chart.yaml` must already be present under the chart's `charts/` directory. Nothing is downloaded.

## Examples

```
# package one chart into ./dist
cargueiro package charts/fleet/110.0.3+up0.16.3
```

```
# choose the output directory
cargueiro package charts/fleet/110.0.3+up0.16.3 --output-dir output
```

```
# package a chart family in one call
cargueiro package \
  charts/fleet-crd/110.0.3+up0.16.3 \
  charts/fleet/110.0.3+up0.16.3 \
  charts/fleet-agent/110.0.3+up0.16.3 \
  --output-dir output
```
