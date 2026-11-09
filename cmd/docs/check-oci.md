# check oci

Check one chart in an OCI registry against the chart rules.

The argument is a chart reference, `[oci://]host[:port]/path/chart:tag`. A `+` in the tag can be written as it is in the chart's version: the registry stores it as `_`. A reference without a tag, or pinned to a digest (`@sha256:`), is refused. The version checked is the one in the reference, not the one in the chart's `Chart.yaml`, and the two are not compared.

The registry is read without credentials. `Chart.yaml` and `values.yaml` are taken from the chart's layer, which is read in full (at most 2 MB) and checked against its digest. They are then checked, and the result is printed and the exit code decided, exactly as `check chart` does; see `check chart` for the output and the exit codes.

`--plain-http` talks HTTP to the registry instead of HTTPS, for local registries. `--output` and `--no-error-hip-15` work as they do for `check chart`.

When the registry cannot be reached, the chart is not found, the registry asks for credentials, or what is there is not a Helm chart, nothing is printed to stdout: the error goes to stderr and the exit code is 1.

## Examples

```
# check one chart
cargueiro check oci registry.example.com/rancher/fleet:110.0.3+up0.16.3
```

```
# print the result as JSON
cargueiro check oci registry.example.com/rancher/fleet:110.0.3+up0.16.3 --output json
```

```
# a registry on this machine, over HTTP
cargueiro check oci 127.0.0.1:5050/rancher/fleet:110.0.3+up0.16.3 --plain-http
```
