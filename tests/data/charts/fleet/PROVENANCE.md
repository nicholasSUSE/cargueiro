# Fleet test charts

Real SUSE charts used as the source of truth for the packaging e2e tests.

| | |
|---|---|
| Source repository | https://github.com/rancher/charts (public) |
| Branch | `release-v2.15` |
| Commit | `be246f515cafb78fbe8d666a519cb2d53b429c85` (2026-10-29T15:50:48Z) |
| Charts | `fleet`, `fleet-crd`, `fleet-agent` |
| Version | `110.0.3+up0.16.3` |

## Layout

- `src/<chart>/`: the chart directories, copied unmodified from `charts/<chart>/110.0.3+up0.16.3/`.
- `published/<chart>-110.0.3+up0.16.3.tgz`: the archives published in `assets/<chart>/` at the same commit, copied unmodified. They are the expected output of packaging `src/<chart>/`.

| Published archive | sha256 |
|---|---|
| `fleet-110.0.3+up0.16.3.tgz` | `de5ef718a024560f857aad82cb95624067c13d0248455d498a25bea5cb9bc742` |
| `fleet-agent-110.0.3+up0.16.3.tgz` | `6007722309970bd54c5f50c42afae29a37632be4ec1331b092e1dc273f732e2d` |
| `fleet-crd-110.0.3+up0.16.3.tgz` | `720f081383b158f2d38a40248f94773420fbf408db5b42e58d4fc4cb1d71e0d3` |

## Notes

- The published archives were built by charts-build-scripts with Helm v3. Matching them proves parity with what is shipped today, not correctness; the e2e tests also compare against the pinned `helm` CLI.
- `rancher/charts` has no license file. The charts package the Apache-2.0 [rancher/fleet](https://github.com/rancher/fleet) project.
- This data is test-only. Nothing under `tests/` is embedded in the cargueiro binary.
- To use a different version, replace the files by hand from the new upstream commit and update this table. There is no refresh script by design.
