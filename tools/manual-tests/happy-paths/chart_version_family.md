# Manual test: `cargueiro chart-version --family`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`, as the registry to read the released versions from. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login it makes is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. The charts are a copy of the fleet charts in `.tmp/manual-tests/fleet`, with the main chart and the agent changed to charts for Rancher 2.17, because the fleet fixtures are for Rancher 2.15, which `stamp` does not support. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The charts it pushes stay in the local registry until `make registry-down`, so each run uses a new path in it.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout, put it first on the PATH of this shell, and copy the fleet charts"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'
run 'cp -r tests/data/charts/fleet/src .tmp/manual-tests/fleet && sed -i "s/>= 2.15.0-0 < 2.16.0-0/>= 2.17.0-0 < 2.18.0-0/" .tmp/manual-tests/fleet/fleet/Chart.yaml .tmp/manual-tests/fleet/fleet-agent/Chart.yaml'

hdr "SETUP: log in to the local registry without a password"
want "a warning that the registry accepts anonymous requests, and 'Login succeeded'"
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'
run 'echo x | cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'

hdr "SETUP: choose a registry path that was never used, so the test can be run again"
run 'REG="127.0.0.1:5052/manual-family-version-$(date +%s)"; echo "$REG"'

hdr "1. HAPPY: the first version of a family that was never pushed, with the folders given out of order"
want "one line on stdout, 112.0.0+up0.16.3, the version of the main chart for the whole family; exit 0. The three Chart.yaml keep their version 110.0.3+up0.16.3"
run 'cargueiro chart-version --family fleet .tmp/manual-tests/fleet/fleet-crd .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet --registry $REG --plain-http'
run 'grep -H "^version:" .tmp/manual-tests/fleet/fleet/Chart.yaml .tmp/manual-tests/fleet/fleet-agent/Chart.yaml .tmp/manual-tests/fleet/fleet-crd/Chart.yaml'

hdr "2. HAPPY: release the family with that version, and ask again"
want "the three charts are stamped, packaged and pushed (three oci://$REG/… lines for 112.0.0_up0.16.3); then the next version is 112.0.1+up0.16.3; exit 0"
run 'v=$(cargueiro chart-version --family fleet .tmp/manual-tests/fleet/fleet .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet-crd --registry $REG --plain-http) && cargueiro stamp --family fleet .tmp/manual-tests/fleet/fleet .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet-crd --version "$v" >/dev/null && cargueiro push --family fleet $(cargueiro package --family fleet .tmp/manual-tests/fleet/fleet .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet-crd --output-dir .tmp/manual-tests/out) --registry $REG'
run 'cargueiro chart-version --family fleet .tmp/manual-tests/fleet/fleet .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet-crd --registry $REG --plain-http'

hdr "3. FAILURE: the main chart is released alone, so the family has drifted"
want "error: the charts of the family \"fleet\" are not at the same released version: fleet 112.0.1+up0.16.3, fleet-agent 112.0.0+up0.16.3, fleet-crd 112.0.0+up0.16.3; nothing on stdout; exit 1"
run 'cargueiro stamp .tmp/manual-tests/fleet/fleet --version 112.0.1+up0.16.3 >/dev/null && cargueiro push "$(cargueiro package .tmp/manual-tests/fleet/fleet --output-dir .tmp/manual-tests/alone)" --registry $REG'
run 'cargueiro chart-version --family fleet .tmp/manual-tests/fleet/fleet .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet-crd --registry $REG --plain-http'

hdr "CLEANUP: forget the login, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR REG; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
