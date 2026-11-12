# index

Write the `index.yaml` of a charts repository from its `entries/`.

`--repo` is the path of the repository. Each version of a chart is one file, `entries/<chart>/<version>.yaml`, which holds the metadata of the version and the URL it is pulled from. The index lists every entry that is not hidden, the versions of each chart newest first, and nothing else: it is the same bytes every time for the same entries and archives, so the file can be regenerated and compared. Each line has the `created` date of its entry, the date its chart was pushed, which never changes, and `generated` is the newest of them, so it changes only when a version is added. The time `index` runs is never written.

Nothing is written unless every entry is valid: the chart must be listed in `rules.yaml`, the file must be called after the version it holds, and no version may be in two entries. The entries, the `index.yaml` that is already there and the archives in `assets/<chart>/<chart>-<version>.tgz` must also agree: a published version never loses its entry or its archive, an archive never has no entry, and a new entry has no archive yet. See `docs/INDEX.md`.

A new version is also checked for its URL. It has exactly one, `oci://<registry>/<chart>:<tag>@sha256:<digest>`, with the tag written as the registry stores the version, a `+` as a `_` (`111.0.3_up0.17.3` for `111.0.3+up0.17.3`), the digest of the manifest pinned, and the registry the staging registry of the chart's edition in `rules.yaml`. `--registry` replaces that registry, as `host[:port]/path`, for a local one. A new version must also have the `created` date in its entry, which `ship` writes. A version that is already published is not checked again, and a change to its date is a change to a published version.

The icon of a new version of a main chart must be `file://assets/logos/<file>`, where the file exists in the repository. A `crd` or `extra` chart, and a main chart marked `icon: skip` in `rules.yaml`, are not checked.

A new version is then pulled: the archive of the manifest its URL pins is read from the registry, checked to be the chart its entry describes (its `Chart.yaml` has the same metadata as the entry, except the icon), and written to `assets/<chart>/<chart>-<version>.tgz`. Only new versions are pulled. Every line of the index has the `digest` of its archive, the sha256 of the file, and a published archive is never allowed to change. If any new version fails, nothing is written. The archives are written before the index, and are taken back if the index cannot be written. Credentials are optional. Without them the registry is read anonymously, which is right for a public path. To give them, use `--username` (or `CARGUEIRO_USERNAME`) and the password through `CARGUEIRO_PASSWORD`; otherwise the login saved by `cargueiro registry login` is used when it is for this registry. They are sent only to that registry, and over HTTP only with `--plain-http`. `--plain-http` talks HTTP to the registry, for local ones.

An entry with `hide: true` is a published version that must no longer be offered. It is left out of the index, and its entry and archive stay. Hiding is one way: taking `hide: true` off the entry does not publish the version again, and a version that was never published cannot be hidden.

The command prints what it did, one version to a line: `add <chart> <version>` for each version added to the index, then `hide <chart> <version>` for each one left out of it, or `nothing to change`. `--dry-run` prints the same lines for what it would do, and pulls and writes nothing. It does every check that needs no registry, so it cannot check the manifest digest or the archive of a new version. The exit code is 0, or 1 when a check fails.

Target-Audience: Release Team and Automation

## Examples

```
# write the index of a charts repository
cargueiro index --repo ~/charts
```

```
# see what would be added or hidden, and change nothing
cargueiro index --repo ~/charts --dry-run
```
