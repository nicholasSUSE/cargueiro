# Manual test: `cargueiro push`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login it makes is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. It stamps and packages a copy of the `rancher-compliance` fixture to have something to push. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The chart it pushes stays in the local registry until `make registry-down`.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "SETUP: log in to the local registry without a password, and make an archive to push"
want "a warning that the registry accepts anonymous requests, 'Login succeeded', and the absolute path of the archive rancher-compliance-111.0.0+up1.5.4.tgz"
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'
run 'echo x | cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'
run 'cp -r tests/data/charts/rancher-compliance-upstream/src/rancher-compliance .tmp/manual-tests/chart && cargueiro stamp .tmp/manual-tests/chart --version 111.0.0+up1.5.4 >/dev/null && cargueiro package .tmp/manual-tests/chart --output-dir .tmp/manual-tests/out'

hdr "1. HAPPY: push the archive to the local registry"
want "one line on stdout, the reference that was pushed: oci://127.0.0.1:5052/manual-push/rancher-compliance:111.0.0_up1.5.4; exit 0"
run 'cargueiro push ".tmp/manual-tests/out/rancher-compliance-111.0.0+up1.5.4.tgz" --registry 127.0.0.1:5052/manual-push'

hdr "2. HAPPY: the chart is in the registry, and it follows the rules there"
want "list prints 111.0.0+up1.5.4; check oci prints a report for rancher-compliance with 'valid: true'; exit 0 for both"
run 'cargueiro list 127.0.0.1:5052/manual-push/rancher-compliance --plain-http'
run 'cargueiro check oci "127.0.0.1:5052/manual-push/rancher-compliance:111.0.0+up1.5.4" --plain-http | head -5'

hdr "3. FAILURE: a push the rules refuse, to the image path of the staging registry of SUSE"
want "error: pushing to this registry is not allowed: rancher-compliance is a rancher-charts chart and can only go to stgregistry.suse.com/rancher/catalog; exit 1. It is decided from rules.yaml, before any connection is made"
run 'cargueiro push ".tmp/manual-tests/out/rancher-compliance-111.0.0+up1.5.4.tgz" --registry stgregistry.suse.com/rancher'

hdr "CLEANUP: forget the login, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
