# rancher-monitoring-crd test chart

Real SUSE chart used as the source of truth for the checking e2e tests. The CRD chart of the `rancher-monitoring` family.

| | |
|---|---|
| Source repository | https://github.com/rancher/charts (public) |
| Branch | `release-v2.15` |
| Commit | `be246f515cafb78fbe8d666a519cb2d53b429c85` (2026-10-29T15:50:48Z) |
| Chart | `rancher-monitoring-crd` |
| Version | `110.0.2+up80.9.1-rancher.22` |

## Layout

- `src/rancher-monitoring-crd/`: `Chart.yaml` and `values.yaml`, copied unmodified from `charts/rancher-monitoring-crd/110.0.2+up80.9.1-rancher.22/`. They are the only files `cargueiro check chart` reads, so the rest of the chart is not kept.

| File | sha256 |
|---|---|
| `Chart.yaml` | `7d6b49575b9a7281d30d7451953c70a2743088fab107223e142a91849850bdd4` |
| `values.yaml` | `9038f399297480d436e0b7835de568dff39b237514826b6aa3c8cb76a42d22cb` |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- To use a different version, replace the files by hand from the new upstream commit and update this table. There is no refresh script by design.
