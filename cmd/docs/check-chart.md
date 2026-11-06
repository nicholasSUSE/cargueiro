# check chart

Check one chart directory against the chart rules.

The argument is one chart directory with a `Chart.yaml` and a `values.yaml`. The chart must be listed in `rules.yaml`, which is compiled into the binary and says what kind of chart it is and which annotations it needs. Two groups of checks run, and both always run: the `catalog.cattle.io` annotations of the chart's kind, and the `helm.sh/images` annotation (HIP-15), which must list every image in `values.yaml`.

The result is printed to stdout, one line per error. Annotation values are never printed, except `kubeVersion` in the metadata, which is the `catalog.cattle.io/kube-version` annotation. `--output json` prints the same result as JSON, with `valid` and `errors` for each group.

The exit code is 0 when the chart is valid and 1 when it is not. With `--no-error-hip-15`, errors of the `helm.sh/images` annotation do not make the exit code 1: the checks still run, the result is printed the same way, and `chart is not valid (HIP-15 errors ignored by --no-error-hip-15)` goes to stderr. Annotation errors still exit 1. When the directory cannot be read, a file is not valid YAML, or the chart is not listed in `rules.yaml`, nothing is printed to stdout: the error goes to stderr and the exit code is 1.

## Examples

```
# check one chart
cargueiro check chart charts/fleet/110.0.3+up0.16.3
```

```
# do not fail because of helm.sh/images errors
cargueiro check chart charts/fleet/110.0.3+up0.16.3 --no-error-hip-15
```

```
# print the result as JSON
cargueiro check chart charts/fleet/110.0.3+up0.16.3 --output json
```
