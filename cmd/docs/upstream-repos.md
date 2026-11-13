# upstream repos

List the upstream repository of every main chart in `rules.yaml`.

Each line is a main chart and the repository that releases it, as `https://github.com/<organization>/<repository>`, sorted by the name of the chart. A crd or extra chart is released from the repository of its main chart, so it has no line. A main chart that has no `upstream` in `rules.yaml` yet is printed as `needs an upstream`, so the charts that still have to be assigned stand out. Nothing is read from the network or from a repository: the list is the `rules.yaml` compiled into the binary. The exit code is 0.

Target-Audience: Release Team

## Examples

```
# the upstream repository of every main chart
cargueiro upstream-repos
```

```
# only the charts that still need one
cargueiro upstream-repos | grep "needs an upstream"
```
