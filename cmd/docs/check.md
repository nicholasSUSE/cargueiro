# check

Check that a chart follows the Rancher chart rules.

`cargueiro check chart` checks a chart directory against `rules.yaml`, which is compiled into the binary: the annotations the chart carries and the `helm.sh/images` annotation (HIP-15). Nothing is changed and nothing is downloaded.

## Examples

```
# check one chart directory
cargueiro check chart charts/fleet/110.0.3+up0.16.3
```
