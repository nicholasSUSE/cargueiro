# Manual test: `cargueiro audit`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, `curl` and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login it makes is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. It stamps, packages and pushes a copy of the `rancher-compliance` fixture to the local registry, and writes by hand the `index.yaml` of a repository that lists it, with the digest of the manifest in the registry and the one of the archive. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The chart it pushes stays in the local registry until `make registry-down`.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "SETUP: push a chart to the local registry, and write the index.yaml of a repository that lists it"
want "'Login succeeded', the line oci://127.0.0.1:5052/manual-audit/rancher-compliance:111.0.0_up1.5.4, and the index.yaml with one version whose url pins the digest of its manifest"
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'
run 'echo x | cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'
run 'cp -r tests/data/charts/rancher-compliance-upstream/src/rancher-compliance .tmp/manual-tests/chart && cargueiro stamp .tmp/manual-tests/chart --version 111.0.0+up1.5.4 >/dev/null'
run 'archive=$(cargueiro package .tmp/manual-tests/chart --output-dir .tmp/manual-tests/out) && cargueiro push "$archive" --registry 127.0.0.1:5052/manual-audit'
run 'manifest=$(curl -sI -H "Accept: application/vnd.oci.image.manifest.v1+json" http://127.0.0.1:5052/v2/manual-audit/rancher-compliance/manifests/111.0.0_up1.5.4 | tr -d "\r" | sed -n "s/^[Dd]ocker-[Cc]ontent-[Dd]igest: //p") && archive_sha=$(sha256sum "$archive" | cut -d" " -f1)'
run 'mkdir -p .tmp/manual-tests/repo && printf "apiVersion: v1\nentries:\n  rancher-compliance:\n  - apiVersion: v2\n    name: rancher-compliance\n    version: 111.0.0+up1.5.4\n    digest: %s\n    urls:\n    - oci://127.0.0.1:5052/manual-audit/rancher-compliance:111.0.0_up1.5.4@%s\n" "$archive_sha" "$manifest" > .tmp/manual-tests/repo/index.yaml'

hdr "1. HAPPY: a repository that agrees with the registry"
want "the checks urls, manifest and archive, each with 'valid: true' and 'error: no-error'; exit 0"
run 'cargueiro audit --repo .tmp/manual-tests/repo --registry 127.0.0.1:5052/manual-audit --plain-http'

hdr "2. HAPPY: as JSON"
want "a list of three results with check, valid, description and error; every valid is true; exit 0"
run 'cargueiro audit --repo .tmp/manual-tests/repo --registry 127.0.0.1:5052/manual-audit --plain-http --output json'

hdr "3. FAILURE: the digest of the archive in the index is not the one of the chart in the registry"
want "urls and manifest valid, archive with 'valid: false' and an error that has both digests; exit 1"
run 'sed -i "s/digest: [0-9a-f]\{64\}/digest: 0000000000000000000000000000000000000000000000000000000000000a/" .tmp/manual-tests/repo/index.yaml'
run 'cargueiro audit --repo .tmp/manual-tests/repo --registry 127.0.0.1:5052/manual-audit --plain-http'

hdr "4. FAILURE: the version is not in the registry that is read"
want "urls not valid, manifest not valid with 'was not found in the registry'; exit 1"
run 'cargueiro audit --repo .tmp/manual-tests/repo --registry 127.0.0.1:5052/manual-audit-empty --plain-http'

hdr "CLEANUP: forget the login, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR archive manifest archive_sha; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
