# check

Check that a chart follows the Rancher chart rules.

`cargueiro check chart` checks a chart directory, `cargueiro check archive` a packaged chart (`.tgz`), and `cargueiro check oci` a chart in an OCI registry, against `rules.yaml`, which is compiled into the binary: the annotations the chart carries and the `helm.sh/images` annotation (HIP-15). Nothing is changed.

## Examples

```
# check one chart directory
cargueiro check chart charts/fleet/110.0.3+up0.16.3
```

```
# check one packaged chart
cargueiro check archive assets/fleet-110.0.3+up0.16.3.tgz
```

```
# check one chart in a registry
cargueiro check oci registry.example.com/rancher/fleet:110.0.3+up0.16.3
```
