# stamp

Put the final version and the helm.sh/images annotation on a chart directory.

The argument is one chart directory with a `Chart.yaml`, and a `values.yaml` if the chart has one. The chart must be listed in `rules.yaml`, which is compiled into the binary. `--version` is the final version of the chart. It must follow the version rules that `check` applies: the form `1XY.Z.W+upA.B.C` (`1XY.Z.W` for the charts that the rules file marks plain), an optional prerelease after the `+up` part, and a major that matches the lower bound of the chart's `catalog.cattle.io/rancher-version` annotation. A CRD chart has no such annotation, so only its form is checked. Charts for Rancher 2.15 and before are not supported. Whoever gives `--version` owns it: it is not compared with the `appVersion`, nor with the versions that are released.

Without `--version`, the version is worked out. The major comes from the `rancher-version` annotation, the upstream part from the `appVersion` (a leading `v` is dropped, and what is left must look like `A.B.C`, optionally with a prerelease), and the release counter from the versions already released in the staging registry of the chart's edition, read the way `list` reads them. The candidates of one upstream version, and its final, share the counter; any other release takes the next one; the first release of a chart, or a chart that was never pushed, starts at `1XY.0.0`. A tag in that registry that is not a version that follows the rules, or a version that is already released, is an error: clean the registry or give `--version`. A CRD chart has no `rancher-version`, so it needs `--version`. `--registry` reads the released versions from another registry path, as `host[:port]/path` with the name of the chart added, and `--plain-http` talks HTTP to it, both for local registries.

The `helm.sh/images` annotation is made by going through the images in `values.yaml`, each written with a `repository` and a `tag`, one entry per image, named after the last part of the repository. An annotation that exists is kept and extended with the images it does not list yet. A chart with no `values.yaml` has no images, so it gets no annotation. An image with the tag `latest`, or without a tag, is refused, and so is a name that two different images would share.

Only the `version` line and the `helm.sh/images` annotation of `Chart.yaml` change; everything else in it, and `values.yaml`, stay as they are. Every check runs before anything is written: when one fails, nothing is changed, the errors go to stderr and the exit code is 1. What changed is printed to stdout, or `nothing to change` when the chart already has the version and the images.

## Examples

```
# work out the version from the released ones, and list the images
cargueiro stamp charts/fleet
```

```
# set the version by hand
cargueiro stamp charts/fleet --version 111.0.0+up0.17.0
```

```
# stamp a chart, then check it
cargueiro stamp charts/fleet && cargueiro check chart charts/fleet
```

```
# read the released versions from a local registry
cargueiro stamp charts/fleet --registry 127.0.0.1:5050/rancher --plain-http
```
