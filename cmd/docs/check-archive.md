# check archive

Check one packaged chart (.tgz) against the chart rules.

The argument is one chart archive, such as the ones `package` writes. `Chart.yaml` and `values.yaml` (if the chart has one) are read from it, in memory and without unpacking anything, and then checked, and the result is printed and the exit code decided, exactly as `check chart` does; see `check chart` for the output and the exit codes. Each of the two files may be at most 2 MB.

`--output` and `--no-error-hip-15` work as they do for `check chart`.

When the file is missing or is not a chart archive, or the chart is not listed in `rules.yaml`, nothing is printed to stdout: the error goes to stderr and the exit code is 1.

Target-Audience: Chart Owners, Release Team and Automation

## Examples

```
# check one archive
cargueiro check archive assets/fleet-110.0.3+up0.16.3.tgz
```

```
# print the result as JSON
cargueiro check archive assets/fleet-110.0.3+up0.16.3.tgz --output json
```
