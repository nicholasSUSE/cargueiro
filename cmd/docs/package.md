# package

Package a Helm chart into a .tgz archive.

The argument is one chart directory containing a `Chart.yaml`. The archive is named `<name>-<version>.tgz` after the chart metadata and written to the output directory (`--output-dir`, default `assets`), which is created if it does not exist.

The chart is not changed, so it must be stamped before: `package` checks it with the rules of `check chart`, nothing ignored, and builds no archive when it finds anything. The chart must be listed in `rules.yaml`. After packaging, the archive is checked with the rules of `check archive`, and the digest of each file in the archive is compared with the same file in the directory; the archive is removed, and the exit code is 1, if it is not valid or if any file differs. With `--debug`, it also logs the files of the directory that are not in the archive.

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
