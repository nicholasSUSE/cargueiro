# rancher-compliance upstream test chart

The chart as its **upstream repository** keeps it, before any Rancher release step touches it. It is the source of truth for the `stamp` tests: its image tags are real, but its version is the upstream one and not a Rancher version, and it has no `helm.sh/images` annotation.

| | |
|---|---|
| Source repository | https://github.com/rancher/compliance-operator (public) |
| Tag | `v1.5.4` |
| Commit | `0eb587b929ddd25a2503f67fb0cc18b997ad5db2` (2026-10-26T09:08:10Z) |
| Path | `chart/` |
| Chart | `rancher-compliance` |

## What it is, and is not

- `version` is `1.5.4` and `appVersion` is `v1.5.4`. The version released from this tag in `rancher/charts` is `110.3.0+up1.5.4`, so `stamp --version 110.3.0+up1.5.4` is the change it needs.
- The `catalog.cattle.io/rancher-version` annotation is `>= 2.15.0-0 < 2.16.0-0`.
- There is no `helm.sh/images` annotation.
- `values.yaml` has four images, all with a real tag: `rancher/compliance-operator:v1.5.4`, `rancher/security-scan:v0.10.4`, `rancher/mirrored-sonobuoy-sonobuoy:v0.57.5` and `rancher/kuberlr-kubectl:v8.1.2`.
- `apiVersion` is `v1`, and the icon is an `https://charts.rancher.io/…` URL.

## Layout

- `src/rancher-compliance/`: `Chart.yaml` and `values.yaml`, copied unmodified from the tag. They are the only files `stamp` and `check` read, so the rest of the chart is not kept.

| File | sha256 |
|---|---|
| `Chart.yaml` | `da4b2dd80701fda1b8c022c2bbdba82a54be1a9253607e557aeceb9937182364` |
| `values.yaml` | `fb3de716b3da816d045b6026c2a8821df93b352a671e30e8d8dc7c334a3a2c26` |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- The webhook chart in `tests/data/charts/rancher-webhook-upstream` is the opposite case: a placeholder version and placeholder image tags.
- To use a different version, replace the files by hand from the new upstream tag and update this table. There is no refresh script by design.
