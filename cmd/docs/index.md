# index

Write the `index.yaml` of a charts repository from its `entries/`.

`--repo` is the path of the repository. Each version of a chart is one file, `entries/<chart>/<version>.yaml`, which holds the metadata of the version and the URL it is pulled from. The index lists every entry, the versions of each chart newest first, and nothing else: it is the same bytes every time for the same entries, so the file can be regenerated and compared. No time of day is written.

Nothing is written unless every entry is valid: the chart must be listed in `rules.yaml`, the file must be called after the version it holds, and no version may be in two entries. The entries, the `index.yaml` that is already there and the archives in `assets/<chart>/<chart>-<version>.tgz` must also agree: a published version never loses its entry or its archive, an archive never has no entry, and a new entry has no archive yet. See `docs/INDEX.md`.

A new version is also checked for its URL. It has exactly one, `oci://<registry>/<chart>:<version>@sha256:<digest>`, with the version as it is in `Chart.yaml`, the digest of the manifest pinned, and the registry the staging registry of the chart's edition in `rules.yaml`. `--registry` replaces that registry, as `host[:port]/path`, for a local one. A version that is already published is not checked again.

The icon of a new version of a main chart must be `file://assets/logos/<file>`, where the file exists in the repository. A `crd` or `extra` chart, and a main chart marked `icon: skip` in `rules.yaml`, are not checked.

A new version is then pulled: the archive of the manifest its URL pins is read from the registry, without credentials, checked to be the chart its entry describes (its `Chart.yaml` has the same metadata as the entry, except the icon), and written to `assets/<chart>/<chart>-<version>.tgz`. Only new versions are pulled. Every line of the index has the `digest` of its archive, the sha256 of the file, and a published archive is never allowed to change. If any new version fails, nothing is written, and the archives already written are taken back if the index cannot be written. `--plain-http` talks HTTP to the registry, for local ones.

An entry with `hide: true` is a published version that must no longer be offered. It is left out of the index, and its entry and archive stay. Hiding is one way: taking `hide: true` off the entry does not publish the version again, and a version that was never published cannot be hidden.

Target-Audience: Release Team and Automation

## Examples

```
# write the index of a charts repository
cargueiro index --repo ~/charts
```
