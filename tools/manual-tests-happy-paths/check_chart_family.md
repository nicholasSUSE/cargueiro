# Manual test: `cargueiro check chart --family`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, and no registry. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "1. HAPPY: the three folders of the family, given out of order"
want "three reports in the order fleet, fleet-agent, fleet-crd, all 'valid: true'; exit 0"
run 'cargueiro check chart --family fleet tests/data/charts/fleet/src/fleet-crd tests/data/charts/fleet/src/fleet-agent tests/data/charts/fleet/src/fleet'

hdr "2. FAILURE: one chart is not valid (a copy where fleet-agent has no annotations)"
want "all three charts reported, only fleet-agent 'valid: false'; error: the family \"fleet\" is not valid: fleet-agent; exit 1"
run 'rm -rf .tmp/manual-tests/bad && cp -r tests/data/charts/fleet/src .tmp/manual-tests/bad && sed -i "/catalog.cattle.io/d" .tmp/manual-tests/bad/fleet-agent/Chart.yaml'
run 'cargueiro check chart --family fleet .tmp/manual-tests/bad/fleet .tmp/manual-tests/bad/fleet-agent .tmp/manual-tests/bad/fleet-crd'

hdr "3. FAILURE: the versions of the family differ (a copy where fleet-agent has another version)"
want "the three reports, then: error: the charts of the family \"fleet\" do not have the same version, with the version of each chart; exit 1"
run 'rm -rf .tmp/manual-tests/diff && cp -r tests/data/charts/fleet/src .tmp/manual-tests/diff && sed -i "s/^version: .*/version: 110.0.4+up0.16.4/" .tmp/manual-tests/diff/fleet-agent/Chart.yaml'
run 'cargueiro check chart --family fleet .tmp/manual-tests/diff/fleet .tmp/manual-tests/diff/fleet-agent .tmp/manual-tests/diff/fleet-crd'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
