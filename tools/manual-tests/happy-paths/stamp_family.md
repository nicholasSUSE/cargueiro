# Manual test: `cargueiro stamp --family`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, and no registry, because the version is given. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The charts it stamps are a copy of the fleet charts in `.tmp/manual-tests/fleet`, with the main chart and the agent changed to charts for Rancher 2.17, because the fleet fixtures are for Rancher 2.15, which `stamp` does not support. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

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

hdr "1. HAPPY: a dry run, with the three folders given out of order"
want "==> fleet, ==> fleet-agent and ==> fleet-crd, each with a diff that changes the version to 112.0.0+up0.16.3; nothing is written; exit 0"
run 'cargueiro stamp --family fleet .tmp/manual-tests/fleet/fleet-crd .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet --version 112.0.0+up0.16.3 --dry-run'

hdr "2. HAPPY: stamp the family"
want "==> fleet, ==> fleet-agent and ==> fleet-crd, each with 'version: 110.0.3+up0.16.3 -> 112.0.0+up0.16.3'; exit 0. Then the three Chart.yaml have 112.0.0+up0.16.3"
run 'cargueiro stamp --family fleet .tmp/manual-tests/fleet/fleet-crd .tmp/manual-tests/fleet/fleet-agent .tmp/manual-tests/fleet/fleet --version 112.0.0+up0.16.3'
run 'grep -H "^version:" .tmp/manual-tests/fleet/fleet/Chart.yaml .tmp/manual-tests/fleet/fleet-agent/Chart.yaml .tmp/manual-tests/fleet/fleet-crd/Chart.yaml'

hdr "3. FAILURE: a chart of the family is missing"
want "error: the family \"fleet\" is missing fleet-agent; nothing on stdout; exit 1"
run 'cargueiro stamp --family fleet .tmp/manual-tests/fleet/fleet .tmp/manual-tests/fleet/fleet-crd --version 112.0.1+up0.16.3'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
