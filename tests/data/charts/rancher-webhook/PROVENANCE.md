# rancher-webhook test chart

Real SUSE chart used as the source of truth for the checking e2e tests. An auto-installed main chart. Expected to fail: its `kube-version` has no lower bound.

| | |
|---|---|
| Source repository | https://github.com/rancher/charts (public) |
| Branch | `release-v2.15` |
| Commit | `be246f515cafb78fbe8d666a519cb2d53b429c85` (2026-10-29T15:50:48Z) |
| Chart | `rancher-webhook` |
| Version | `110.0.4+up0.11.4` |

## Layout

- `src/rancher-webhook/`: `Chart.yaml` and `values.yaml`, copied unmodified from `charts/rancher-webhook/110.0.4+up0.11.4/`. They are the only files `cargueiro check chart` reads, so the rest of the chart is not kept.

| File | sha256 |
|---|---|
| `Chart.yaml` | `700f95e30dc651629425451c916674f9f169176184967b4e399c99a70d057eeb` |
| `values.yaml` | `56769eed0c626f91b2d693032b414db7d6028f15d5108f262b3a7b7474ecb256` |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- To use a different version, replace the files by hand from the new upstream commit and update this table. There is no refresh script by design.
