# list

List the versions of a chart in an OCI registry, newest first.

The argument is the repository of one chart, `[oci://]host[:port]/path/chart`, with no tag. The chart must be listed in `rules.yaml`, which is compiled into the binary. One version is printed per line, newest first: by `1XY.Z.W`, then by the upstream version after `+up`, then by the prerelease, and a release is newer than its prereleases. A registry stores a `+` in a version as `_` in the tag, and the versions are printed with the `+`.

`--output json` prints a list of objects with the `version` and the `tag` it is stored under. A repository with no tags prints nothing, or `[]` in JSON.

The registry is read without credentials. `--plain-http` talks HTTP to the registry instead of HTTPS, for local registries.

The exit code is 0, or 1 when the registry cannot be reached, the chart is not found, the registry asks for credentials, the chart is not listed in `rules.yaml`, or a tag is not a version that follows the rules. In every failure nothing is printed to stdout and the error goes to stderr.

## Examples

```
# list the versions of one chart
cargueiro list registry.suse.com/rancher/catalog/fleet
```

```
# print them as JSON
cargueiro list registry.suse.com/rancher/catalog/fleet --output json
```

```
# a local registry over HTTP
cargueiro list 127.0.0.1:5050/rancher/fleet --plain-http
```
