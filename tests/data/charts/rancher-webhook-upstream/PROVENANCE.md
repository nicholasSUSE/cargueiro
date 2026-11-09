# rancher-webhook test chart, derived from upstream and MODIFIED

> **This is not the upstream chart.** It was copied from the upstream repository and then changed by hand, for the tests of `cargueiro stamp`. Do not use it as a reference of what the webhook team ships.

## Where it comes from

| | |
|---|---|
| Source repository | https://github.com/rancher/webhook (public) |
| Tag | `v0.11.4` |
| Commit | `855f361c0277092a4703c5fa6068e5589727a56c` (2026-10-27T00:49:49Z) |
| Path | `charts/rancher-webhook/` |
| Chart | `rancher-webhook` |

## What was modified, and why

One line of `Chart.yaml`, the `catalog.cattle.io/rancher-version` annotation:

```diff
-  catalog.cattle.io/rancher-version: ">= 2.15.0-0 < 2.16.0-0"
+  catalog.cattle.io/rancher-version: ">= 2.16.0-0 < 2.17.0-0"
```

The tests of `stamp` are built around Rancher 2.16, and Rancher 2.15 and before are not allowed (they are the old system). Nothing else was changed. `values.yaml` is as upstream has it.

## What it is for

The case of a chart before the owner's release automation touches it:

- `version` and `appVersion` are the placeholder `"999"`. The released version from this tag, for Rancher 2.15, is `110.0.4+up0.11.4`.
- There is no `helm.sh/images` annotation.
- `values.yaml` has one image, `rancher/rancher-webhook`, with the placeholder tag `latest`, so `stamp` must refuse it as it is.

## Layout

- `src/rancher-webhook/`: `Chart.yaml` (modified) and `values.yaml` (as upstream). They are the only files `stamp` and `check` read, so the rest of the chart is not kept.

| File | sha256 here | sha256 upstream |
|---|---|---|
| `Chart.yaml` | `bbd88b3ebb00ff260f6950fb2fc4047c49008f27be50e8511135fd96dd3496af` | `9d47b73927aaced2c5f3bc7f324887c6caab10285ae519f97223081ec1d31ff0` |
| `values.yaml` | `52edcde4003d34263cfbe1c7e75e863c9e9afda1074d1e63b92c1d79e70b78b2` | the same |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- To use a different version, copy the files by hand from the new upstream tag, repeat the change above, and update this file. There is no refresh script by design.
