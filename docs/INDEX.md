# The index of a charts repository

`cargueiro index` keeps three places of a charts repository in agreement. This page explains the rule it applies to them. The command itself is described in `cmd/docs/index.md`.

## The three places

Every version of a chart is seen from three places. The entry, `entries/<chart>/<version>.yaml`, says the version is meant to be published. The index, the committed `index.yaml`, says it is published. The archive, `assets/<chart>/<chart>-<version>.tgz`, is the chart itself. A version is complete when it is in all three, and the index is only ever extended by versions that are entries and not yet published.

## What is decided for each version

| Entry | In the index | Archive | Result |
|-------|--------------|---------|--------|
| yes | no | no | new: to be published |
| yes | yes | yes | published: nothing to do |
| yes | yes | no | error: the index lists it but the archive is missing |
| yes | no | yes | error: a new entry must not have an archive yet |
| no | yes | any | error: entries are never removed |
| no | no | yes | error: an archive with no entry |

Nothing else is possible, because a version in none of the three places does not exist. When any version is an error, every version at fault is reported and nothing is decided, so a repository is never half processed.

## Permanence

A version that has been published is never removed, and the line it has in the index never changes. This is why a published version without its entry is an error and not a cleanup, and why a new entry that already has an archive is refused instead of being overwritten.
