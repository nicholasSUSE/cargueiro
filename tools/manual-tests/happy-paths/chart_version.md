# Manual test: `cargueiro chart-version`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`, as the registry to read the released versions from. Without `--registry` the command reads the staging registry of SUSE. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login it makes is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. It uses a copy of the `rancher-compliance` fixture, and releases it once to the local registry. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The chart it pushes stays in the local registry until `make registry-down`.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout, put it first on the PATH of this shell, and copy the chart"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'
run 'cp -r tests/data/charts/rancher-compliance-upstream/src/rancher-compliance .tmp/manual-tests/chart'

hdr "SETUP: log in to the local registry without a password"
want "a warning that the registry accepts anonymous requests, and 'Login succeeded'"
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'
run 'echo x | cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'

hdr "1. HAPPY: the first release of a chart that was never pushed"
want "one line on stdout, 111.0.0+up1.5.4; exit 0. Chart.yaml is not changed: its version stays 1.5.4"
run 'cargueiro chart-version .tmp/manual-tests/chart --registry 127.0.0.1:5052/manual-version --plain-http'
run 'grep "^version:" .tmp/manual-tests/chart/Chart.yaml'

hdr "2. HAPPY: release it with that version, and ask again"
want "the chart is stamped, packaged and pushed (oci://127.0.0.1:5052/manual-version/rancher-compliance:111.0.0_up1.5.4); then the next version is 111.0.1+up1.5.4; exit 0"
run 'v=$(cargueiro chart-version .tmp/manual-tests/chart --registry 127.0.0.1:5052/manual-version --plain-http) && cargueiro stamp .tmp/manual-tests/chart --version "$v" >/dev/null && cargueiro push "$(cargueiro package .tmp/manual-tests/chart --output-dir .tmp/manual-tests/out)" --registry 127.0.0.1:5052/manual-version'
run 'cargueiro chart-version .tmp/manual-tests/chart --registry 127.0.0.1:5052/manual-version --plain-http'

hdr "3. FAILURE: a chart with no appVersion"
want "an error that says the Chart.yaml has no appVersion; nothing on stdout; exit 1"
run 'sed -i "/^appVersion:/d" .tmp/manual-tests/chart/Chart.yaml'
run 'cargueiro chart-version .tmp/manual-tests/chart --registry 127.0.0.1:5052/manual-version --plain-http'

hdr "CLEANUP: forget the login, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
