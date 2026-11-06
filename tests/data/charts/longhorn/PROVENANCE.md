# longhorn test chart

Real SUSE chart used as the source of truth for the checking e2e tests. A chart that is expected to fail: its `kube-version` has no upper bound.

| | |
|---|---|
| Source repository | https://github.com/rancher/charts (public) |
| Branch | `release-v2.15` |
| Commit | `be246f515cafb78fbe8d666a519cb2d53b429c85` (2026-10-29T15:50:48Z) |
| Chart | `longhorn` |
| Version | `110.1.0+up1.12.1` |

## Layout

- `src/longhorn/`: `Chart.yaml` and `values.yaml`, copied unmodified from `charts/longhorn/110.1.0+up1.12.1/`. They are the only files `cargueiro check chart` reads, so the rest of the chart is not kept.

| File | sha256 |
|---|---|
| `Chart.yaml` | `ce34d2ed064e1b527ebed9ecaf377140c1b1c02cf2c93065fef9f54edc28f488` |
| `values.yaml` | `178a873079d2514187d4a89404f00c187ca8f2b3dae3ddc41a43e8fcfa0993bc` |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- To use a different version, replace the files by hand from the new upstream commit and update this table. There is no refresh script by design.
