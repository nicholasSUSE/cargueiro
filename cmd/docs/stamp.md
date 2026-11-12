# stamp

Put the final version and the helm.sh/images annotation on a chart directory.

The argument is one chart directory with a `Chart.yaml`, and a `values.yaml` if the chart has one. The chart must be listed in `rules.yaml`, which is compiled into the binary. `--version` is the final version of the chart, and it is required: `chart-version` works it out. `stamp` reads nothing from the network. It must follow the version rules that `check` applies: the form `1XY.Z.W+upA.B.C` (`1XY.Z.W` for the charts that the rules file marks plain), an optional prerelease after the `+up` part, and a major that matches the lower bound of the chart's `catalog.cattle.io/rancher-version` annotation. A CRD chart has no such annotation, so only its form is checked. Charts for Rancher 2.15 and before are not supported. Whoever gives `--version` owns it: it is not compared with the `appVersion`, nor with the versions that are released.

The `helm.sh/images` annotation is made by going through the images in `values.yaml`, each written with a `repository` and a `tag`, one entry per image, named after the last part of the repository. An annotation that exists is kept and extended with the images it does not list yet. A chart with no `values.yaml` has no images, so it gets no annotation. An image with the tag `latest`, or without a tag, is refused, and so is a name that two different images would share.

Only the `version` line and the `helm.sh/images` annotation of `Chart.yaml` change; everything else in it, and `values.yaml`, stay as they are. Every check runs before anything is written: when one fails, nothing is changed, the errors go to stderr and the exit code is 1. What changed is printed to stdout, or `nothing to change` when the chart already has the version and the images. With `--dry-run` every check runs, and what would change is printed as a diff, but nothing is written.

With `--family`, the arguments are the folders of every chart of one family in `rules.yaml`, such as `fleet`, `fleet-agent` and `fleet-crd`, and nothing else: a chart that is missing, given twice or from another family is an error, and nothing is changed. The folders can be given in any order, and the name of each chart is read from its `Chart.yaml`. The version given is the version of the family. Every chart takes it, and it is checked for that chart like any other version: a CRD chart for its form, and any other chart also against its own `rancher-version`. What is printed for each chart starts with a line `==> <chart>`, in the order of the family, the main chart first.

Every chart is checked before any is written, and the problem of every chart is reported. If a write fails, the files already written are put back; if that fails too, the error says that the charts are half stamped and that the changes should be discarded with `git reset --hard` before running it again.

Target-Audience: Chart Owners and Automation

## Examples

```
# put the version and the images on a chart
cargueiro stamp charts/fleet --version 111.0.0+up0.17.0
```

```
# stamp a chart, then check it
cargueiro stamp charts/fleet --version 111.0.0+up0.17.0 && cargueiro check chart charts/fleet
```

```
# see what would change, and write nothing
cargueiro stamp charts/fleet --version 111.0.0+up0.17.0 --dry-run
```

```
# stamp every chart of a family with the version of its main chart
cargueiro stamp --family fleet charts/fleet charts/fleet-agent charts/fleet-crd --version 110.0.3+up0.16.3
```
