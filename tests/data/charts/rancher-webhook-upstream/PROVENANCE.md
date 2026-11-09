# rancher-webhook upstream test chart

The chart as its **upstream repository** keeps it, before any Rancher release step touches it. It is the source of truth for the `stamp` tests: it has no real version and no `helm.sh/images` annotation.

| | |
|---|---|
| Source repository | https://github.com/rancher/webhook (public) |
| Tag | `v0.11.4` |
| Commit | `855f361c0277092a4703c5fa6068e5589727a56c` (2026-10-27T00:49:49Z) |
| Path | `charts/rancher-webhook/` |
| Chart | `rancher-webhook` |

## What it is, and is not

- `version` and `appVersion` are the placeholder `"999"`. The webhook team's release automation writes the real version, so a chart owner in this situation passes the whole Rancher version to `stamp`. The version released from this tag in `rancher/charts` is `110.0.4+up0.11.4`.
- The `catalog.cattle.io/rancher-version` annotation is `>= 2.15.0-0 < 2.16.0-0`.
- There is no `helm.sh/images` annotation.
- `values.yaml` has one image, `rancher/rancher-webhook`, with the placeholder tag `latest`.

## Layout

- `src/rancher-webhook/`: `Chart.yaml` and `values.yaml`, copied unmodified from the tag. They are the only files `stamp` and `check` read, so the rest of the chart is not kept.

| File | sha256 |
|---|---|
| `Chart.yaml` | `9d47b73927aaced2c5f3bc7f324887c6caab10285ae519f97223081ec1d31ff0` |
| `values.yaml` | `52edcde4003d34263cfbe1c7e75e863c9e9afda1074d1e63b92c1d79e70b78b2` |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- The released chart is kept next to it in `tests/data/charts/rancher-webhook`.
- To use a different version, replace the files by hand from the new upstream tag and update this table. There is no refresh script by design.
