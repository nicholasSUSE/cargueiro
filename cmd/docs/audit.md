# audit

Check that a charts repository agrees with itself and with the registry its versions are pulled from.

`--repo` is the path of the repository. The command reads its `index.yaml` once, asks the registry once for each version of the index, and runs every check on what it read. No check stops the others: the report has one result for each of them, with its name, whether it passed, one sentence about what it looked at, and the errors it found, one to a line, or `no-error`. Nothing is written, in the repository or in the registry. The exit code is 0 when every check passed, and 1 when any did not.

The checks are:

- `urls`: the URL of every version in the index points to the staging registry of the edition of its chart in `rules.yaml`, with the tag the registry stores (a `+` written as `_`) and the digest of the manifest pinned.
- `manifest`: the tag of every version in the registry names the manifest that its URL pins. A version the registry does not have fails here.
- `archive`: the `digest` of every version in the index is the digest of the chart in its manifest in the registry, which is the sha256 of the archive.

The registry is read where `rules.yaml` says the version is, never where its URL says. `--registry` replaces the staging registry, as `host[:port]/path`, for a local one. A version the registry could not give is a failure of `manifest`, with the reason, and `archive` leaves it to that check. A check that could not run, because the repository could not be read, is in the report as not valid, with the reason.

Credentials are optional. Without them the registry is read anonymously, which is right for a public path. To give them, use `--username` (or `CARGUEIRO_USERNAME`) and the password through `CARGUEIRO_PASSWORD`; otherwise the login saved by `cargueiro registry login` is used when it is for this registry. They are sent only to that registry, and over HTTP only with `--plain-http`.

`--output` is `text` (the default) or `json`, a list of the results.

Target-Audience: Release Team and Automation

## Examples

```
# audit the charts repository in the current folder
cargueiro audit --repo .
```

```
# the same report as JSON
cargueiro audit --repo ~/charts --output json
```
