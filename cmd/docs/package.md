# package

Package a Helm chart into a .tgz archive.

The argument is one chart directory containing a `Chart.yaml`. The archive is named `<name>-<version>.tgz` after the chart metadata and written to the output directory (`--output-dir`, default `assets`), which is created if it does not exist.

The chart is not changed, so it must be stamped before: `package` checks it with the rules of `check chart`, nothing ignored, and builds no archive when it finds anything. The chart must be listed in `rules.yaml`. After packaging, the archive is checked with the rules of `check archive`, and the digest of each file in the archive is compared with the same file in the directory; the archive is removed, and the exit code is 1, if it is not valid or if any file differs. With `--debug`, it also logs the files of the directory that are not in the archive.

The archive path is printed to stdout. Logs and errors go to stderr, and a failure makes the exit code non-zero.

With `--family`, the arguments are the folders of every chart of one family in `rules.yaml`, such as `fleet`, `fleet-agent` and `fleet-crd`, and nothing else: a chart that is missing, given twice or from another family is an error, and nothing is built. The folders can be given in any order, and the name of each chart is read from its `Chart.yaml`. Every chart is checked before any archive is built, the report of each one that is not valid is written to stderr, and the charts must all have the same version. The archives are built one after the other into the same output directory, and their paths are printed to stdout, one on each line, in the order of the family, the main chart first. If one of them fails, the archives already built are removed, and so is the output directory when it was made for them.

Dependencies declared in `Chart.yaml` must already be present under the chart's `charts/` directory. Nothing is downloaded.

Target-Audience: Chart Owners and Automation

## Examples

```
# package one chart into ./assets
cargueiro package charts/fleet/110.0.3+up0.16.3
```

```
# choose the output directory
cargueiro package charts/fleet/110.0.3+up0.16.3 --output-dir output
```

```
# package every chart of a family
cargueiro package --family fleet charts/fleet charts/fleet-agent charts/fleet-crd --output-dir output
```
