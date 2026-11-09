# check

Check that a chart follows the Rancher chart rules.

`cargueiro check chart` checks a chart directory, and `cargueiro check oci` checks a chart in an OCI registry, against `rules.yaml`, which is compiled into the binary: the annotations the chart carries and the `helm.sh/images` annotation (HIP-15). Nothing is changed.

## Examples

```
# check one chart directory
cargueiro check chart charts/fleet/110.0.3+up0.16.3
```

```
# check one chart in a registry
cargueiro check oci registry.example.com/rancher/fleet:110.0.3+up0.16.3
```
