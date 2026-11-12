# chart version

Print the final version of the next release of a chart.

The argument is one chart directory with a `Chart.yaml`. The chart must be listed in `rules.yaml`, which is compiled into the binary. The version is worked out, and nothing is written: the output is the value to give to `stamp --version`. The major comes from the lower bound of the `catalog.cattle.io/rancher-version` annotation, and the `Chart.yaml` must have it. The upstream part comes from the `appVersion`, which must be present too: a leading `v` is dropped, and what is left must look like `A.B.C`, optionally with a prerelease. A chart that the rules file marks plain has no upstream part, so it does not need an `appVersion`. Charts for Rancher 2.15 and before are not supported, and a CRD chart has no `rancher-version`, so on its own it has no version to work out.

The release counter comes from the versions already released, so the command reads the network: it lists the tags of the chart in the staging registry of the chart's edition, `stgregistry.suse.com`, the way `list` does. The candidates of one upstream version, and its final, share the counter; any other release takes the next one; the first release of a chart, or a chart that was never pushed, starts at `1XY.0.0`. A tag in that registry that is not a version that follows the rules, or a version that is already released, is an error. `--registry` reads the released versions from another registry path, as `host[:port]/path` with the name of the chart added, and `--plain-http` talks HTTP to it, both for local registries.

Credentials are optional. Without them the registry is read anonymously, which is right for a public path. To give them, use `--username` (or `CARGUEIRO_USERNAME`) and the password through `CARGUEIRO_PASSWORD`; otherwise the login saved by `cargueiro registry login` is used when it is for this registry. They are sent only to that registry, and over HTTP only with `--plain-http`.

The version is printed on one line on stdout. The exit code is 0, or 1 when the version cannot be worked out, and then nothing is printed to stdout and the error goes to stderr.

Target-Audience: Chart Owners and Automation

## Examples

```
# print the version of the next release of a chart
cargueiro chart-version charts/fleet
```

```
# stamp a chart with it
cargueiro stamp charts/fleet --version "$(cargueiro chart-version charts/fleet)"
```

```
# read the released versions from a local registry
cargueiro chart-version charts/fleet --registry 127.0.0.1:5050/rancher --plain-http
```
