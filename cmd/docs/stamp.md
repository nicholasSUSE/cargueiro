# stamp

Put the final version and the helm.sh/images annotation on a chart directory.

The argument is one chart directory with a `Chart.yaml`, and a `values.yaml` if the chart has one. The chart must be listed in `rules.yaml`, which is compiled into the binary. `--version` is the final version of the chart, and it is required: cargueiro does not work it out, the chart owner's automation passes it in. It must follow the version rules that `check` applies: the form `1XY.Z.W+upA.B.C` (`1XY.Z.W` for the charts that the rules file marks plain), an optional prerelease after the `+up` part, and a major that matches the lower bound of the chart's `catalog.cattle.io/rancher-version` annotation. A CRD chart has no such annotation, so only its form is checked. Charts for Rancher 2.15 and before are not supported.

The `helm.sh/images` annotation is made by going through the images in `values.yaml`, each written with a `repository` and a `tag`, one entry per image, named after the last part of the repository. An annotation that exists is kept and extended with the images it does not list yet. A chart with no `values.yaml` has no images, so it gets no annotation. An image with the tag `latest`, or without a tag, is refused, and so is a name that two different images would share.

Only the `version` line and the `helm.sh/images` annotation of `Chart.yaml` change; everything else in it, and `values.yaml`, stay as they are. Every check runs before anything is written: when one fails, nothing is changed, the errors go to stderr and the exit code is 1. What changed is printed to stdout, or `nothing to change` when the chart already has the version and the images.

## Examples

```
# set the version and list the images of a chart
cargueiro stamp charts/fleet --version 111.0.0+up0.17.0
```

```
# stamp a chart, then check it
cargueiro stamp charts/fleet --version 111.0.0+up0.17.0 && cargueiro check chart charts/fleet
```
