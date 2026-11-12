# ship

Write the entry and the logo of a chart in a registry to a charts repository.

Requirements:
- The chart was pushed to the registry, and the reference names its version.
- The version is not in the `staging` branch of `rancher/charts` yet: there is no `entries/<chart>/<version>.yaml` for it.
- `--repo` is a clean, up to date checkout of the `staging` branch of `rancher/charts`.

Before anything else it checks the checkout. `--repo` must be the root of a git checkout that has a remote, is on the branch `staging` (`--branch` names another), has nothing uncommitted, follows that branch of its remote, and is at the latest commit of that branch there. The remote is asked for it without fetching, and the checkout is never changed. It needs the `git` program. A checkout that is not right is refused, and the error says what to do about it.

The argument is one chart version in a registry, `[oci://]host[:port]/path/chart:tag`, and `--repo` is the absolute path of a checkout of the charts repository. The tag is turned into the digest of its manifest and the chart is read by that digest, so the chart and the digest cannot disagree. The chart must follow the same rules `check oci` applies, and the version in the tag must be the version of the chart. A chart that does not follow them is reported on stderr and refused.

It writes `entries/<chart>/<version>.yaml`: the `Chart.yaml` of the chart as it is, with only the `icon` changed and a `urls` added, whose only item is `oci://<registry>/<chart>:<tag>@sha256:<digest of the manifest>`, where the tag is the version written as the registry stores it, a `+` as a `_`. The `version` of the entry keeps its `+`. It also gets a `created` with the date the chart was pushed, which is the `org.opencontainers.image.created` annotation of its manifest, in UTC, so shipping the same version again gives the same entry; a manifest without that annotation is refused. A chart that is not exempt gets `icon: file://assets/logos/<chart>.<extension>`. The logo is the one the repository has, or the one downloaded from the icon of the chart: it must be an image of a known type of at most 1 MiB. A chart of kind `crd` or `extra`, and a main chart marked `icon: skip` in `rules.yaml`, is shipped with its icon as it is and no logo. A chart with no icon that needs one, or whose icon is a `file://` already, is refused.

`ship` never replaces anything: an entry that is there already is refused even if it would be the same, and so is a logo of the chart in any extension when one has to be added. The entry and the logo are written together or not at all. It does not write the index or `assets/<chart>/`: `cargueiro index` does.

With `--family` it ships every chart of a family at once, instead of one reference. `--family` names the family in `rules.yaml`, `--version` is the version they all have, and `--registry` is where they were pushed, `host[:port]/path` with no chart name. The references are made from them, the main chart first and then the order of `rules.yaml`, and no reference is given. The checkout is checked once. Every chart is read and checked before anything is written, all the problems are reported, and the entries and logos of the whole family are written together or not at all. A chart that is not in the registry, that does not follow the rules, or is of another version, stops the whole family. Nothing else changes.

It prints the files it wrote, relative to the repository, one to a line. `--dry-run` prints the same lines for what it would write and writes nothing. Credentials are optional. Without them the registry is read anonymously, which is right for a public path. To give them, use `--username` (or `CARGUEIRO_USERNAME`) and the password through `CARGUEIRO_PASSWORD`; otherwise the login saved by `cargueiro registry login` is used when it is for this registry. They are sent only to that registry, and over HTTP only with `--plain-http`. `--plain-http` talks HTTP to the registry, for local registries. The exit code is 0, or 1 when anything fails.

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

```
# ship every chart of a family
cargueiro ship --family fleet --version 110.0.3+up0.16.3 --registry stgregistry.suse.com/rancher/catalog --repo ~/charts
```
