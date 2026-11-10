# ship

Write the entry and the logo of a chart in a registry to a charts repository.

The argument is one chart version in a registry, `[oci://]host[:port]/path/chart:tag`, and `--repo` is the absolute path of a checkout of the charts repository. The tag is turned into the digest of its manifest and the chart is read by that digest, so the chart and the digest cannot disagree. The chart must follow the same rules `check oci` applies, and the version in the tag must be the version of the chart. A chart that does not follow them is reported on stderr and refused.

It writes `entries/<chart>/<version>.yaml`: the `Chart.yaml` of the chart as it is, with only the `icon` changed and a `urls` added, whose only item is `oci://<registry>/<chart>:<version>@sha256:<digest of the manifest>`. A chart that is not exempt gets `icon: file://assets/logos/<chart>.<extension>`. The logo is the one the repository has, or the one downloaded from the icon of the chart: it must be an image of a known type of at most 1 MiB. A chart of kind `crd` or `extra`, and a main chart marked `icon: skip` in `rules.yaml`, is shipped with its icon as it is and no logo. A chart with no icon that needs one, or whose icon is a `file://` already, is refused.

`ship` never replaces anything: an entry that is there already is refused even if it would be the same, and so is a logo of the chart in any extension when one has to be added. The entry and the logo are written together or not at all. It does not write the index or `assets/<chart>/`: `cargueiro index` does.

It prints the files it wrote, relative to the repository, one to a line. `--dry-run` prints the same lines for what it would write and writes nothing. The registry is read without credentials, and `--plain-http` talks HTTP to it, for local registries. The exit code is 0, or 1 when anything fails.

Target-Audience: Chart Owners and Automation

## Examples

```
# ship a chart that was pushed to the staging registry
cargueiro ship stgregistry.suse.com/rancher/catalog/fleet:110.0.3+up0.16.3 --repo ~/charts
```

```
# see which files it would write, and write nothing
cargueiro ship stgregistry.suse.com/rancher/catalog/fleet:110.0.3+up0.16.3 --repo ~/charts --dry-run
```

```
# a chart in a local registry
cargueiro ship 127.0.0.1:5052/e2e/fleet:110.0.3+up0.16.3 --repo ~/charts --plain-http
```
