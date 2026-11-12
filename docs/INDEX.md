# The index of a charts repository

`cargueiro index` keeps three places of a charts repository in agreement. This page explains the rule it applies to them. The command itself is described in `cmd/docs/index.md`.

## The three places

Every version of a chart is seen from three places. The entry, `entries/<chart>/<version>.yaml`, says the version is meant to be published. The index, the committed `index.yaml`, says it is published. The archive, `assets/<chart>/<chart>-<version>.tgz`, is the chart itself. A version is complete when it is in all three, and the index is only ever extended by versions that are entries and not yet published.

## What is decided for each version

| Entry | Hidden | In the index | Archive | Result |
|-------|--------|--------------|---------|--------|
| yes | no | no | no | new: to be published |
| yes | no | yes | yes | published: nothing to do |
| yes | no | yes | no | error: the index lists it but the archive is missing |
| yes | no | no | yes | error: a new entry must not have an archive yet |
| yes | yes | yes | yes | to be hidden: taken out of the index |
| yes | yes | no | yes | hidden: nothing to do |
| yes | yes | yes | no | error: the index lists it but the archive is missing |
| yes | yes | no | no | error: only a published version can be hidden |
| no | | yes | any | error: entries are never removed |
| no | | no | yes | error: an archive with no entry |

Nothing else is possible, because a version in none of the three places does not exist. When any version is an error, every version at fault is reported and nothing is decided, so a repository is never half processed.

## The digest

Every line of the index has a `digest`: the sha256 of the archive of its version. It is worked out from the file in `assets/` every time, for a new version from the archive that was pulled. A published archive is never allowed to change, so when the digest of a published line is not the one of its file, `index` fails and says which archive changed and what the two digests are.

## Order of work

The checks that need no registry come first: the table above, the digest of every published archive, that no published entry changed, and the URL and icon of every new version. Only then are the new versions pulled, each from the manifest its URL pins, and checked to be the chart its entry describes. The archives are written to `assets/` and the index is written last. If a pull or a check fails nothing is written, and if the index cannot be written the archives are taken back. With `--dry-run` it stops after the checks that need no registry.

## Hiding

An entry with `hide: true` is a version that has been published and must no longer be offered, for example because of a critical CVE. It is taken out of the index and nothing else is removed: the entry and the archive stay. Hiding is the only way a line leaves the index, and it is one way. Taking `hide: true` off an entry does not publish the version again: the version is then a new entry that already has an archive, which is an error.

## Permanence

A version that has been published is never removed, and the line it has in the index never changes, its `created` date included, except that hiding takes it out. Its archive never changes either. This is why a published version without its entry is an error and not a cleanup, and why a new entry that already has an archive is refused instead of being overwritten.
