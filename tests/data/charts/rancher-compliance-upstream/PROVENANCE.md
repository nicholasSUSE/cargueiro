# rancher-compliance test chart, derived from upstream and MODIFIED

> **This is not the upstream chart.** It was copied from the upstream repository and then changed by hand, for the tests of `cargueiro stamp`. Do not use it as a reference of what the compliance team ships.

## Where it comes from

| | |
|---|---|
| Source repository | https://github.com/rancher/compliance-operator (public) |
| Tag | `v1.5.4` |
| Commit | `0eb587b929ddd25a2503f67fb0cc18b997ad5db2` (2026-10-26T09:08:10Z) |
| Path | `chart/` |
| Chart | `rancher-compliance` |

## What was modified, and why

One line of `Chart.yaml`, the `catalog.cattle.io/rancher-version` annotation:

```diff
-  catalog.cattle.io/rancher-version: '>= 2.15.0-0 < 2.16.0-0'
+  catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'
```

The tests of `stamp` are built around Rancher 2.16, and Rancher 2.15 and before are not allowed (they are the old system). Nothing else was changed. `values.yaml` is as upstream has it.

## What it is for

The opposite case to the webhook chart: a chart whose image tags are real, but whose version is not a Rancher version.

- `version` is `1.5.4` and `appVersion` is `v1.5.4`. For Rancher 2.15 this tag was released as `110.3.0+up1.5.4`.
- There is no `helm.sh/images` annotation.
- `values.yaml` has four images, all with a real tag: `rancher/compliance-operator:v1.5.4`, `rancher/security-scan:v0.10.4`, `rancher/mirrored-sonobuoy-sonobuoy:v0.57.5` and `rancher/kuberlr-kubectl:v8.1.2`.
- `apiVersion` is `v1`, and the icon is an `https://charts.rancher.io/…` URL.

## Layout

- `src/rancher-compliance/`: `Chart.yaml` (modified) and `values.yaml` (as upstream). They are the only files `stamp` and `check` read, so the rest of the chart is not kept.

| File | sha256 here | sha256 upstream |
|---|---|---|
| `Chart.yaml` | `65193b8b5194b865aacd89abe93ab76fa77a45ca5158d980e334870a041ddf0a` | `da4b2dd80701fda1b8c022c2bbdba82a54be1a9253607e557aeceb9937182364` |
| `values.yaml` | `fb3de716b3da816d045b6026c2a8821df93b352a671e30e8d8dc7c334a3a2c26` | the same |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- The webhook chart in `tests/data/charts/rancher-webhook-upstream` is the opposite case: a placeholder version and placeholder image tags.
- To use a different version, copy the files by hand from the new upstream tag, repeat the change above, and update this file. There is no refresh script by design.
