# index

Write the `index.yaml` of a charts repository from its `entries/`.

`--repo` is the path of the repository. Each version of a chart is one file, `entries/<chart>/<version>.yaml`, which holds the metadata of the version and the URL it is pulled from. The index lists every entry, the versions of each chart newest first, and nothing else: it is the same bytes every time for the same entries, so the file can be regenerated and compared. No time of day is written.

Nothing is written unless every entry is valid: the chart must be listed in `rules.yaml`, the file must be called after the version it holds, and no version may be in two entries.

## Examples

```
# write the index of a charts repository
cargueiro index --repo ~/charts
```
