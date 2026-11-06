# rancher-monitoring test chart

Real SUSE chart used as the source of truth for the checking e2e tests. The worst case for `check chart`: a 208 KB `values.yaml` with deeply nested image blocks.

| | |
|---|---|
| Source repository | https://github.com/rancher/charts (public) |
| Branch | `release-v2.15` |
| Commit | `be246f515cafb78fbe8d666a519cb2d53b429c85` (2026-10-29T15:50:48Z) |
| Chart | `rancher-monitoring` |
| Version | `110.0.2+up80.9.1-rancher.22` |

## Layout

- `src/rancher-monitoring/`: `Chart.yaml` and `values.yaml`, copied unmodified from `charts/rancher-monitoring/110.0.2+up80.9.1-rancher.22/`. They are the only files `cargueiro check chart` reads, so the rest of the chart is not kept.

| File | sha256 |
|---|---|
| `Chart.yaml` | `767682e177eb4a83c07d92a04e78de28e7bd290293e77e7ca5e6f2006ec2e5aa` |
| `values.yaml` | `44c64ae52efd5335779477aeaf98dbae0e0b4784c17720bbc06edf9eec5dd3b2` |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- To use a different version, replace the files by hand from the new upstream commit and update this table. There is no refresh script by design.
