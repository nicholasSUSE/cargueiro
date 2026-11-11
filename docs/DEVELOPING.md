# Developing

## Setup

Tool versions are pinned in `mise.toml`: Go, golangci-lint, dlv, helm, oras, crane and the Docker CLI.

1. Install [mise](https://mise.jdx.dev/).
2. Activate it in your shell (zsh shown; add after your `PATH` setup):
   ```
   eval "$(mise activate zsh)"
   ```
3. From the repo root, run `mise trust` and `mise install`.
4. Have a Docker daemon running. mise pins the Docker CLI only; the daemon comes from the host.

Check that the pinned tools win over anything else on your `PATH`:

```
mise doctor | grep activated   # activated: yes
mise which helm golangci-lint  # paths under ~/.local/share/mise/installs
```

If a pinned tool is shadowed, `mise which <tool>` shows the right one and `command -v <tool>` the one actually used. Two common causes:

- mise is not activated, so system or Homebrew binaries come first. Activate it as above.
- An editor installed a stale copy into `GOBIN`. `mise.toml` lists `go` last so pinned tools take precedence; remove the stale binary if it still wins.

The pinned `helm` must match `helm.sh/helm/v4` in `go.mod`.

## What a feature is

cargueiro follows the Unix philosophy: each command does one thing, does it well, and prints results that other tools can consume. A **feature** is one such command, delivered end to end. It is **feature complete** only when all five layers exist:

| # | Layer | Where | Done when |
|---|-------|-------|-----------|
| 1 | Command | `cmd/<name>.go` | The cobra command parses arguments and flags, calls the implementation and reports. |
| 2 | Implementation | `internal/<area>/` | The logic lives here, with no cobra and no printing. |
| 3 | Unit tests | next to the code | Every path is covered without external tools. |
| 4 | E2E tests | `tests/<name>/` | The real binary is proven correct on real data. |
| 5 | Documentation | `cmd/docs/<name>.md` | `--help`, the README table and the doc page agree. |

A change that skips a layer is not a finished feature, whatever the other layers look like.

### 1. Command

- One command, one job. If a command needs "and" to describe it, split it.
- One input. A command never takes several; a script calls it once per input.
- Keep it thin: validate arguments, call `internal/`, write results. No business logic.
- Results go to **stdout**, one item per line, so another command can read them. Logs and errors go to **stderr**.
- Return errors from `RunE` ([error handling](#error-handling)).
- Inject dependencies (the implementation as a function or small interface) so the command is testable without the real thing.

### 2. Implementation

- Put it under `internal/<area>/`. It must not import cobra.
- `context.Context` is the first parameter. Errors follow [Error handling](#error-handling).
- Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md); `make lint` enforces most of it.
- Nothing is fetched or mutated beyond what the command's doc promises.

### 3. Unit tests

- Table-driven, next to the code (`*_test.go`), run by `make test-unit`.
- Cover success, every error path and the edge cases. Use fakes for boundaries (the command tests use a fake packager).
- No network, no Docker, no external binaries. Small fixtures go in a local `testdata/`.

### 4. E2E tests

- Live in `tests/<name>/`, behind the `e2e` build tag, run by `make test-e2e`.
- Run the **real binary** as a black box through `tests/internal/harness`: arguments in, stdout, stderr and exit code out.
- Use real data committed under `tests/data/` with a `PROVENANCE.md` saying where it came from. Never fetch data in a test.
- Compare against an independent source of truth wherever one exists. For packaging that is the `helm` CLI and the archives published in `rancher/charts`; comparing against our own output proves nothing.
- Cover the happy path, defaults, failures, that inputs are not modified, and determinism.
- Break something on purpose once to see the test fail, so you know it can.
- Features that need a registry get it from `registry:2`; tests read its address from the environment and never start it themselves.

### 5. Documentation

- Write `cmd/docs/<name>.md`: a title, a one-line summary, a longer description and an `## Examples` section. The layout is described in [Command documentation](#command-documentation).
- Mention every local flag of the command in that file. A test fails if one is missing.
- Run `make docs` to refresh the README table. A test fails if the table is stale.
- Update this file when the feature changes how people work: new tools go in `mise.toml` and in Setup, new test data in its `PROVENANCE.md`.

### Checklist

A feature is complete when all of this is true:

- [ ] The command, its implementation, unit tests, e2e tests and docs all exist.
- [ ] `make fmt`, `make lint` and `make vet` report nothing.
- [ ] `make test` passes (unit and e2e).
- [ ] `make docs` leaves no diff.
- [ ] New tools are pinned in `mise.toml`.
- [ ] A human reviewed it, as required by [AGENTS.md](AGENTS.md).

Commit by layer, with Conventional Commit messages, so the history reads like this list. For `package` that was `feat(chart)` for the implementation, `feat(cmd)` for the command, `test(e2e)` for the test data and tests, and `feat(docs)` for the documentation.

### What already exists

Later features reuse these and only add their own pieces:

- The docs pipeline: the `cmd/docs` loader, the guard tests and `make docs`.
- The e2e harness (`tests/internal/harness`): building and running the binary, running `helm`, comparing archives by content (`internal/chart/archivetest`).
- The tooling: mise pins, lint config, the lint hook and the Make targets.

## Error handling

| # | Rule |
|---|------|
| 1 | Wrap with context: `fmt.Errorf("verb noun: %w", err)`. Lowercase, no trailing punctuation. Add context only when the inner error lacks it. |
| 2 | Use `fmt.Errorf` or `errors.New`. Add an error type only when a caller must branch on it. |
| 3 | Sentinels (`var ErrX = errors.New(...)`, matched with `errors.Is`) name stable conditions. Declare them next to the code that returns them. |
| 4 | Custom types (matched with `errors.As`) only when the caller needs data from the error. |
| 5 | Handle an error once: log it or return it, never both. Only `main` prints errors and exits. |
| 6 | Do not leak internals: wrap with `%w` only where callers may depend on the cause, otherwise `%v`. |
| 7 | Advice for the user belongs in `cmd/`. `internal/` returns facts. |
| 8 | Exit codes: 0 success, 1 failure. |
| 9 | No package-wide `errors.go`. A dedicated file is only for an error type with methods. |

## Chart annotation rules

`rules.yaml` at the repository root lists every chart cargueiro checks, the kind of each, and the annotations each kind requires. `internal/rules` parses it and `internal/annotations` validates a chart's annotations against it. A chart the file does not list is an error. The validators take parsed data and do no file I/O.

The file is compiled into the binary: `main.go` embeds it with `//go:embed` and hands the bytes to `cmd`. Changing a rule means rebuilding and releasing cargueiro. `cargueiro check chart` is what runs the rules.

Every chart also has an `edition`, `rancher-charts` or `prime-charts`, which says which repository it is released from; the kinds mean the same in both. A family never mixes editions. The `registries` section names the chart repositories of each edition and the image repositories, as `host/path` with no scheme and no trailing slash, and the parser rejects a missing or malformed one.

`push` builds its guard from these. On the SUSE registries named there, a listed chart may only go to the repositories of its edition, anything else is an image dependency that may only go to the image path, and nothing that already exists is overwritten. The check and the upload are separate calls, so only the registry's own immutable tags could close the race between two pushes of the same new tag.

A family is the set of charts that are released together, at one version, such as `fleet`, `fleet-agent` and `fleet-crd`. It has exactly one `main` chart, which sets that version for the others, and `Config.Family` lists its charts with the main chart first. `internal/family.Resolve` takes what a command was given, folders or archives, checks that they are exactly the charts of one family, each once, and puts them in that order. A command that works on a family calls it first.

A chart version is `1XY.Z.W+upA.B.C`, with an optional prerelease after the `+up` part such as `-rc.1`; a chart marked `version: plain` is `1XY.Z.W` and has neither. The `versions` section of the file lists the prerelease names that are allowed and ties each Rancher version to the major (`1XY`) its charts carry. `internal/version` applies it: `Policy.Validate` checks the form, and `Policy.CheckMajor` compares the major with the lower bound of the chart's `rancher-version` annotation. The `check` commands call them.

| Kind | Chart | Required annotations (under `catalog.cattle.io/`) |
|------|-------|---------------------------------------------------|
| `main` | a product chart, with or without a `-crd` sibling | `release-name`, `certified`, `rancher-version`, `kube-version`, `permits-os` |
| `crd` | a CRD chart | `release-name`, `certified`, `hidden` |
| `extra` | a companion or add-on of a product chart | `certified`, `rancher-version`, `kube-version`, `permits-os` |

`namespace` is optional for every kind. Space around a value is ignored. Besides being present, a value must be well formed:

| Annotation | Valid value | Kinds |
|------------|-------------|-------|
| `certified` | `rancher` | all |
| `release-name` | the chart name, or the override in `rules.yaml` | `main`, `crd` |
| `hidden` | `true` | `crd` |
| `kube-version`, `rancher-version` | both bounds, written `>= X.Y.Z-0 < X.Y.Z-0`, lower below upper | `main`, `extra` |
| `permits-os` | `linux`, optionally with `windows`, comma separated | `main`, `extra` |

Each kind has one function in `internal/annotations` (`ValidateMainChart`, `ValidateCRD`, `ValidateExtraChart`) that lists its checks. Each annotation has its own file with a `check<Name>` function that reads the annotation and a function that validates the value. To change a rule, edit that file. To require another annotation of a kind, list it under that kind in `rules.yaml`; to also check its value, add a line to the kind's function.

### check chart

`cargueiro check chart <chart-dir>` ties the pieces together: `internal/chart` reads `Chart.yaml` and `values.yaml` into memory (nothing else in the directory is read; a chart with no `values.yaml` has no values and no images), `internal/check` finds the chart in the rules and runs the annotation checks, the `helm.sh/images` checks and the version checks, and `cmd` prints the result.

| Outcome | stdout | stderr | Exit |
|---------|--------|--------|------|
| Valid chart | the result | | 0 |
| Violations | the result | `chart is not valid` | 1 |
| Only `helm.sh/images` violations, with `--no-error-hip-15` | the result, unchanged | `chart is not valid (HIP-15 errors ignored by --no-error-hip-15)` | 0 |
| Unreadable directory or file, bad YAML, chart not in `rules.yaml`, unknown `--output` | | the error | 1 |

The result has the same fields as text and as JSON (`--output json`): `valid`, three sections, `annotations`, `hip15-annotations` and `version`, each with its own `valid` and an `errors` list, and then the chart, its kind and family, and a few `Chart.yaml` fields. All sections always run. `version` holds the version rule errors: the form, and the major against the `rancher-version` annotation. `check oci` validates the version in the reference, and the other two the one in `Chart.yaml`. Annotation values are never printed, except `kubeVersion` in the metadata, which is the `catalog.cattle.io/kube-version` annotation and not Helm's own `kubeVersion` field. In `internal/check`, `Result` carries the JSON tags, and `Metadata` is its own struct so that the loader's type does not decide the output.

### check archive

`cargueiro check archive <chart.tgz>` checks a packaged chart with the same rules, output and exit codes as `check chart`. `chart.LoadArchive` takes `Chart.yaml` and `values.yaml` from the archive in memory, and the shared `report` does the rest, so the same files give the same report from an archive, a directory and a registry.

### check oci

`cargueiro check oci <registry/path/chart:tag>` checks a chart that is already in a registry, with the same rules, output and exit codes as `check chart`: only the source of `Chart.yaml` and `values.yaml` differs. `internal/oci` resolves the tag with the credentials it is given or anonymously, requires a Helm chart manifest with exactly one chart layer, reads that layer in full (at most 2 MB, verified against its size and digest), and `chart.LoadArchive` takes the two files from it. Reading the registry is untrusted input, so it is bounded. Every command that reads takes the credentials of `push` (`cmd.credentialFlags`), and they are asked for the host that is read. A failure to read the chart prints nothing on stdout and exits 1.

### list

`cargueiro list <registry/path/chart>` prints the versions of a chart that is in a registry, newest first. `internal/oci` reads every tag, anonymously or with credentials, following the registry's pages, and `cmd` turns each tag back into a version with `version.FromOCITag`. `version.Policy.SortNewestFirst` orders them: by `1XY.Z.W`, then by the upstream version after `+up`, then by the prerelease, where a release is newer than its prereleases. The numbers are compared with the semver library. The rank of the prerelease names is the order of `versions.prereleases` in `rules.yaml`, not an alphabetical one. The chart must be listed in `rules.yaml`, which says whether its versions have a `+up` part. A tag that is not a version that follows the rules is an error, and nothing is printed.

### stamp

`cargueiro stamp <chart-dir>` puts the final version and the `helm.sh/images` annotation on a chart before it is packaged. `internal/stamp` builds a plan that validates everything and writes nothing: the chart must be in `rules.yaml`, the version goes through the version policy, the chart must be for a Rancher version the new system supports, and the images in `values.yaml` become the annotation (`annotations.GenerateHelmImages`, which reuses the scan of the check commands). Only then does `Plan.Write` replace `Chart.yaml`, through a temporary file. `internal/chart` changes the version line and the annotation as text, so the rest of the file stays byte for byte. `--version` overrides the calculation: it is not compared with anything, so whoever gives it owns it. Without it, `version.Next` works the version out from the `appVersion`, the `rancher-version` and the versions already released, read the way `list` reads them; a registry error, a tag that is not a version, or a version already released is an error. `--dry-run` prints `Plan.Diff` and writes nothing.

### package

`cargueiro package <chart-dir>` makes the archive of a chart that was stamped before; it does not change the chart. It runs the check of `check chart` on the directory and builds nothing if the chart is not valid, nothing ignored. Then `internal/chart` packages it with Helm, the check of `check archive` runs on the archive, and the digest of each file is compared with the one in the directory (`internal/integrity`). An archive that is not valid, or that differs, is removed. The file lists come from `chart.DirManifest` and `chart.ArchiveManifest`, which read a chart the way Helm does: `.helmignore` applies, `Chart.yaml` is taken as the metadata because Helm writes it again, and a dependency, a directory or a packaged `.tgz`, is read as a chart of its own because Helm writes it as a directory. `integrity` itself only knows digests of files and manifests of names with digests, and how two of them differ. The digest of the archive file changes with the modification time of the files it was made from, so it is not compared between builds.

### index

`cargueiro index --repo <dir>` builds `index.yaml` and `assets/<chart>/` of a charts repository from `entries/<chart>/<version>.yaml`. `internal/index.Write` works in an order that puts everything that needs no registry first, and writes nothing unless every step passed. `readEntries` reads each entry strictly, as Helm's index line plus `hide`, and needs the file to be called after its version, the folder after its chart, the chart to be in `rules.yaml`, and no version twice; the versions of a chart are ordered by `version.SortNewestFirst`, and `created` and `generated` are the zero time, so the same entries and archives give the same bytes. `readPublished` reads the committed `index.yaml` strictly and `archiveRefs` lists `assets/<chart>/<chart>-<version>.tgz`, where anything else is an error. `State.Plan` compares the entries, the index and the archives and decides which versions to add and which to hide, or fails naming every version at fault; the table is in `docs/INDEX.md`. `archiveDigests` takes the sha256 of every archive on disk, `checkArchivesUnchanged` fails with the archive and both digests when a published one is not the one in the index, and `checkUnchanged` compares each published entry with its line, written as YAML. For the new versions `checkURLs` wants one URL, `oci://<repository>:<version>@sha256:<digest>`, where the repository is `Config.StagingRepository` (the same one `stamp` reads, `--registry` replaces it), and `checkIcons` wants a logo that exists unless the chart is exempt. Only then `pullAdded` reads each archive by the digest its URL pins with `oci.PullArchive`, and `checkArchive` compares the whole `Chart.yaml` in it, read by `chart.ArchiveChartYAML`, with the entry, icon aside. `writeArchives` writes the archives through `fsutil.Batch`, then the index is written, and if that fails the archives are taken back. A version with `hide: true` is left out of `entries.visible`, which is the only way a line leaves the index. `Plan.Report` prints what was done, and `--dry-run` stops before the pull.

### ship

`cargueiro ship <registry/path/chart:tag> --repo <dir>` leaves the entry and the logo of a chart that is in a registry in a checkout of the charts repository. `cmd` first has `internal/checkout` verify the checkout, by running `git` for reads only: the root of a checkout that has a remote, on `staging` (or `--branch`), with nothing uncommitted, following that branch, and at the latest commit that `git ls-remote` gives for it, which does not need a fetch. Each failure says what to do about it, and a password in the address of a remote is never printed. Then `internal/ship.Prepare` works out a `Shipment` and writes nothing: `oci.ResolveDigest` turns the tag into the digest of the manifest, `oci.PullArchive` reads the archive by that digest, so the chart and the digest cannot disagree, and `check.Chart` applies the rules, as `check oci` does. A chart that fails is an `InvalidChartError`, which `cmd` prints as the same report. The icon is made local: an exempt chart (`crd`, `extra`, or `icon: skip` in `rules.yaml`) keeps it, any other uses the logo the repository has or the one `FetchLogo` downloads, whose extension comes from a fixed list of content types and which is at most 1 MiB, what Rancher reads. `BuildEntry` edits the text of `Chart.yaml` (`chart.SetIcon`, and a `urls` item at the end) and refuses what `index` would not read. `Shipment.Write` writes the entry and the logo through `fsutil.Batch`, both or neither, and replaces nothing. `ship` writes neither the index nor `assets/<chart>/`: `index` does that, by pulling the archive the URL of the entry pins. `--dry-run` prints the same files and writes nothing.

### helm.sh/images

The `helm.sh/images` annotation follows [Helm HIP-15](https://github.com/helm/community/blob/main/hips/hip-0015.md), a draft, and adds Rancher policy on top. `ValidateHelmImages(annotations, values)` is meant to run for every kind, next to the kind's own validator, and takes the parsed `Chart.yaml` annotations and `values.yaml`. A chart whose `values.yaml` has no images needs no annotation.

| Rule | Source |
|------|--------|
| The value is a YAML list; each entry has a `name` and an `image`; other properties are allowed | HIP-15 |
| `dependency` is optional and names the subchart an image belongs to | HIP-15 |
| `name` is unique in the list | Rancher policy (HIP-15 allows repeats) |
| `image` is written `repository:tag` | Rancher policy (HIP-15 only says "a location") |
| An image in `values.yaml` is a map with a `repository` key. It needs a non-empty string `repository` and `tag` (quote a numeric tag: `tag: 1.10` is rejected); both empty means no image | Rancher policy |
| A string value under any key, including one called `image`, is not an image and is ignored | Rancher policy |
| Every image in `values.yaml` is listed in the annotation, compared as `repository:tag` | Rancher policy |
| An entry that is not in `values.yaml` has a non-empty `dependency`, whose value is not checked | Rancher policy |

The `registry` key beside a `repository` in `values.yaml` is never part of a comparison, because the release process switches it between registries. An annotation image is compared as written, so `registry.suse.com/rancher/a:1` does not match `repository: rancher/a` with `tag: "1"`.

Known limits: a chart repository URL under a `repository` key is read as an image, and `repo@sha256:...` passes the image format.

## Linting

`.golangci.yml` is the single source of truth. Every runner (editor, agent hook, CI) uses the same config and the same pinned golangci-lint version from `mise.toml`.

| Runner | How |
|--------|-----|
| Local | `make lint` / `make fmt` |
| Editor (VSCode) | `go.lintTool: golangci-lint-v2`, picks up `.golangci.yml` |
| Agent hook | `tools/scripts/lint-hook.sh`, runs after every `.go` edit |
| CI (GHA) | `jdx/mise-action`, reads the same `mise.toml` |

### Version pinning

The version lives once, in `mise.toml` (`golangci-lint = "2.15.0"`). `make lint` and `make fmt` call `golangci-lint` from `PATH`, so they use whatever mise provides. To upgrade, change that line and run `mise install`.

The pinned binary is a prebuilt release built with a specific Go version. If `go.mod` moves to a newer Go minor before golangci-lint publishes a matching build, `make lint` fails with "the Go language version used to build golangci-lint is lower than the targeted Go version". Bump golangci-lint, or install it from source with mise's `go:` backend (`"go:github.com/golangci/golangci-lint/v2/cmd/golangci-lint"`).

### Linters

Uber's minimum set (`errcheck`, `goimports`, `revive`, `govet`, `staticcheck`) plus linters for the remaining [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) rules: `gochecknoinits`, `gochecknoglobals`, `errname`, `exhaustive`, `forcetypeassert`, `errorlint`, `predeclared`.

Suppress a finding only with a reason: `//nolint:gochecknoglobals // set via ldflags`.

### Agent hook

A `PostToolUse` hook on `Edit|Write|MultiEdit` runs `tools/scripts/lint-hook.sh`:

1. Reads the edited path from the hook's stdin JSON and skips non-`.go` files.
2. Runs `make fmt`, then lints only the edited package (`./<dir>`).
3. On findings, prints them to stderr and exits `2`, so the agent sees them and fixes them.

The script is committed. The hook registration lives in `.claude/settings.json`, which is gitignored, so each developer registers it locally:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [{ "type": "command", "command": "tools/scripts/lint-hook.sh" }]
      }
    ]
  }
}
```

The hook is fast local feedback only. CI is the enforcing check.

## Command documentation

Each command is documented once, in `cmd/docs/<command>.md`. That file feeds `cargueiro <command> --help` and the commands table in `README.md`.

Layout of a command doc:

````markdown
# <command>

<first paragraph: short description>

<long description>

## Examples

```
cargueiro <command> ...
```
````

Flags stay defined in Go and show up in `--help` automatically. Mention each local flag (for example `--output-dir`) in the doc; a test fails if one is missing.

After changing a doc, run `make docs` to refresh the README table. `make test` fails if the table is stale, if a command has no doc, or if a doc has no command.

## Editor (VSCode)

The `settings.json` and `launch.json` to test, lint and debug from VSCode with the tools that mise pins are in [`DEV_ENV.md`](DEV_ENV.md).

## Testing

| Target | What it runs |
|--------|--------------|
| `make test-unit` | Unit tests (`go test -race ./...`), no external tools |
| `make test-e2e` | E2E tests under `tests/` (`-tags e2e`), against the real binary |
| `make test` | Both |
| `make test-release` | Builds every release target (`make release-build`), then checks the binaries (`-tags release`). Not part of `make test`, because a cross-build is slow |

E2E tests build `bin/cargueiro` themselves (or use the binary in `CARGUEIRO_BIN`) and run it as a black box.

`tests/packaging` packages the real SUSE fleet charts in `tests/data/charts/fleet` and compares three archives per chart by content (mode, owner, size, sha256 per file; gzip headers and mtimes ignored): the one built by cargueiro, the one built by the `helm` CLI, and the one published in `rancher/charts`. It needs the `helm` CLI from `mise.toml` on `PATH`, at the same version as `helm.sh/helm/v4` in `go.mod`; a test fails with a clear message if it is not.

`tests/pushing` pushes the published fleet archives to the local registries over HTTP and over TLS with a self-signed CA, then pulls them back with the `helm` CLI and compares the bytes. Each test logs in first, because `push` has no transport flags. It also covers a wrong explicit password, a push without any login, a corrupt archive, and `package` followed by `push`.

`tests/login` logs in and out of the local registries over HTTP and TLS and checks what `push` does with what was saved: a second login is refused until logout, explicit credentials win, a wrong password saves nothing, logout forgets everything, and the file is private.

`tests/checking` runs `check chart`, `check archive` and `check oci` on real SUSE charts and checks the exact output; `check oci` is run against the local registry that asks for no password, after pushing the charts to it, and its report must equal the one for the source directory. The fleet charts pass. `rancher-monitoring` (the largest `values.yaml`), `rancher-monitoring-crd`, `longhorn`, `rancher-webhook` and `system-upgrade-controller` fail, because they carry no `helm.sh/images` annotation, and `longhorn` and `rancher-webhook` also have a `kube-version` without both bounds. Their fixtures keep only `Chart.yaml` and `values.yaml`, the two files `check` reads. It also covers the text output and every case that prints nothing on stdout.

`tests/stamping` runs `stamp` on copies of two charts as their upstream keeps them, and follows the result through `check chart`, `package` and `check archive`. It also stamps against the local registry release after release, for a first version, the candidates and the final of the next one, and checks the order with `list`. Without a registry given, `stamp` reads the staging registry of SUSE, so the tests that leave the version out run behind a dead proxy and cannot reach it.

`tests/listing` builds its archives with the Helm CLI, because the versions it uses test the order and `package` refuses a version that does not match the `rancher-version` of the chart. It pushes real charts, with only the version changed, to the local registry that asks for no password, in an order that is not the right one, and checks that `list` puts them newest first, as text and JSON. It compares the tags with the ones the registry itself gives, and covers every case that prints nothing on stdout.

`tests/indexing` works on `tests/data/rancher-charts`, a charts repository of hand-written entries, logos and the expected `index.yaml`. The tests that do not pull use it as a repository that is already published: the committed index gets the digest of an empty archive, and every entry gets an empty archive of that name, because only the names and the digests are looked at. They cover each case of the table in `docs/INDEX.md`, hiding, an entry or an archive that changed, and the checks of the URL and the icon of a new version. The tests that pull build the chart of every entry with the Helm CLI, with the time of the files fixed so that the archive is the same on every run, push them to the local registry that asks for no password, and point each entry at what was pushed, with the manifest digest that `helm push` prints. They compare the result with the expected index, where each version has the digest of its archive and the URL it was pushed to, and check that a second run pulls nothing, by giving it a registry that does not exist.

`tests/shipping` makes a chart the way a chart owner does, with `stamp`, `package` and `push` to the local registry that asks for no password, and ships it with `ship` into a real git checkout built like the staging branch of `rancher/charts`: a clone of a bare repository with an empty `index.yaml`, in `harness.ChartsCheckout`. A small web server in the test serves the logo. It then runs `index` on what `ship` wrote, to show that the two commands agree. It covers an entry that is never replaced once merged, a logo the repository already has, a logo that answers 404, a version that is not in the registry, an exempt chart, a dry run, another branch, and every way the checkout can be wrong: uncommitted changes, a commit not pushed, behind the remote, another branch, no upstream, no remote and not a checkout.

Test data is committed by hand, never fetched by a script. See the `PROVENANCE.md` next to each chart in `tests/data/charts/` for where it came from. Nothing under `tests/` is embedded in the binary.

## Local registries

E2E tests use three throwaway `registry:2` containers, all bound to `127.0.0.1` only. Two require basic auth, like the production registries, and are used by `push`. The third needs no password and is used by the commands that read anonymously, such as `check oci`:

| Name | Address | Transport |
|------|---------|-----------|
| `cargueiro-registry` | `127.0.0.1:5050` | HTTP |
| `cargueiro-registry-tls` | `127.0.0.1:5051` | HTTPS, self-signed CA |
| `cargueiro-registry-open` | `127.0.0.1:5052` | HTTP, no password |

Credentials of the first two: `admin` / `cargueiro-test`. They are test-only; the htpasswd file is `tests/data/registry/htpasswd`.

```
make registry-up       # start them (needs a running Docker daemon)
make registry-down     # stop and remove them
make registry-status   # are they up? prints the credentials and the login commands
```

`make test-e2e` starts them and stops them again, even when tests fail. Run `make registry-up` yourself to run a single e2e test from the editor or with `go test -tags e2e`; the tests fail with a hint if the registries are not reachable.

The TLS certificates are generated by `tests/gencerts` on every `make registry-up` into `.tmp/registry/certs/` (gitignored); `ca.crt` there is the CA to save with `registry login --ca-file`. The registry image is pinned in the `Makefile` (`REGISTRY_IMAGE`) and the ports can be overridden (`REGISTRY_PORT`, `REGISTRY_TLS_PORT`, and `CARGUEIRO_TEST_REGISTRY_HTTP` / `CARGUEIRO_TEST_REGISTRY_TLS` for the tests). Port 5000 is deliberately not used.

To use them by hand, log in first. Only one login is kept, so log out before switching registries:

```
echo cargueiro-test | cargueiro registry login 127.0.0.1:5050 --username admin --password-stdin --plain-http
echo cargueiro-test | cargueiro registry login 127.0.0.1:5051 --username admin --password-stdin --ca-file .tmp/registry/certs/ca.crt
```

After that, `push` needs no credentials and no transport flags. How login works is described in `cmd/docs/registry-login.md`. The `helm` CLI needs its own `--plain-http` or `--ca-file`. Chart versions containing `+` are stored with `_` in the tag (`110.0.3_up0.16.3`).

E2E tests run every command with its own throwaway `CARGUEIRO_CONFIG_DIR`, so they never read or write your real saved login.

## Before pushing

```
make test
make vet
make lint
```
