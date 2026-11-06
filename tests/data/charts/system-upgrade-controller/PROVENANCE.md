# system-upgrade-controller test chart

Real SUSE chart used as the source of truth for the checking e2e tests. An auto-installed main chart.

| | |
|---|---|
| Source repository | https://github.com/rancher/charts (public) |
| Branch | `release-v2.15` |
| Commit | `be246f515cafb78fbe8d666a519cb2d53b429c85` (2026-10-29T15:50:48Z) |
| Chart | `system-upgrade-controller` |
| Version | `110.0.0` |

## Layout

- `src/system-upgrade-controller/`: `Chart.yaml` and `values.yaml`, copied unmodified from `charts/system-upgrade-controller/110.0.0/`. They are the only files `cargueiro check chart` reads, so the rest of the chart is not kept.

| File | sha256 |
|---|---|
| `Chart.yaml` | `2fea47407e88532ad7afeed0269e8dde7668d7076988345ab1b6dafa0222b41a` |
| `values.yaml` | `909e6a77999061664d455e8aa870facd4278a1594d1d989044a46b94b1022fa8` |

## Notes

- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- To use a different version, replace the files by hand from the new upstream commit and update this table. There is no refresh script by design.
