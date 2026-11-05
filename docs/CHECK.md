# `cargueiro check` implementation plan

> **Temporary. Delete this file once `check` is implemented.** The finished feature is documented in `cmd/docs/check.md`.

## Scope

```
cargueiro check [--local=<archive.tgz>] [--registry=<ref>]
```

- Read-only. It writes nothing, anywhere.
- At least one of `--local` or `--registry` is required.
- **No comparison for now.** A separate command will compare two sources later.
- No transport flags: how a registry is reached (plain HTTP, an extra CA) is saved by `registry login` and reused, like the credentials. Only one login is kept.
- Text output only. JSON comes later.
- `--registry` takes a tag. A `@sha256:` suffix is recognized and **rejected** with "digest pinning is not supported yet", so a digest is never silently ignored. Later: resolve the tag first, compare its manifest digest with the given one, and fail on a mismatch.

## Fields

The same fields come back for both sources. A field that does not apply prints `n/a`.

| Field | `--local` | `--registry` |
|-------|-----------|--------------|
| `valid` | yes/no | yes/no |
| `name`, `version`, `appVersion`, `icon` | `Chart.yaml` in the archive | config blob |
| `size` | file size | chart layer descriptor |
| layer digest | sha256 of the file | chart layer descriptor |
| manifest digest | `n/a` (a local archive has none) | digest the tag resolves to |

- The layer digest is the chart `.tgz` bytes. The manifest digest is what the tag points to and what gets pinned in `index.yaml`.
- The manifest digest changes on every push, because Helm stamps the push time into the manifest. It cannot be computed from the `.tgz`; it exists only once the registry has the artifact.
- `valid` (assumption, to confirm): for an archive, a readable gzip tar that loads as a Helm chart with a valid name and semver version. For the registry, a Helm manifest with a Helm config and exactly one chart layer, whose config parses as chart metadata.
- The registry side never downloads the chart layer. It reads the manifest and the small config blob.

## Tag rules

- `+` is converted to `_` in the **tag only**, never in the host or path. A registry stores `110.0.3+up0.16.3` as `110.0.3_up0.16.3`.
- A `_` is left as given. It is never converted back to `+`.
- A chart version cannot contain `_` (`helm package` rejects it), so the mapping is unambiguous.
- No tag is an error. There is no implicit `latest`.
- Accept a leading `oci://`; ignore a trailing slash. The tag starts at the last `:` after the last `/`, so a port is not mistaken for a tag.
- Show both spellings in the output.

## Steps

Each step is small and reviewed before the next.

1. **Reference parsing** (`internal/oci/ref.go`): pure code with a large test table.
2. **Local inspection:** read the archive and report the fields. Tests with the fleet data plus corrupt, empty, directory, no `Chart.yaml` and invalid-version inputs.
3. **Registry inspection** (`internal/oci`): resolve the tag, read the manifest and config blob, validate media types, cap the config size. Typed errors for tag not found, repository not found and unreachable. Tests against fake servers.
4. **The command** (`cmd/check.go`): flags, aligned text output with `n/a`, read-only.
5. **E2E** (`tests/checking`): real binary on the fleet charts and a seeded registry. Digest equals `sha256sum`; manifest digest equals the registry's own header; tag as `+` and `_`; missing tag, repository and file; proof of read-only (manifest digest and working directory unchanged).
6. **Documentation:** `cmd/docs/check.md`, `make docs`, `DEVELOPING.md`.

## Open points

1. **Registries that need no password** but do need plain HTTP or a private CA cannot get their settings saved yet, because `registry login` requires credentials. The anonymous test registry (below) needs this to be read by `check`.
2. **Exit codes.** Proposed `diff` style: 0 fine, 1 failed a check, 2 could not check (not found, unreachable, unreadable). `main` currently exits 1 for every error.
3. **Both flags at once** now that nothing is compared: report both sources, or reject the combination.
4. **An anonymous test registry** (port 5052) is needed, since the existing ones demand a password.
