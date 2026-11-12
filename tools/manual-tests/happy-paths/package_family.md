# Manual test: `cargueiro package --family`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, and no registry. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The charts it packages are the fleet charts of the repository, which are already stamped. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "1. HAPPY: package the three charts of the family, with the folders given out of order"
want "three lines on stdout, the absolute paths of the archives, in the order of the family: fleet, fleet-agent, fleet-crd, all ending in -110.0.3+up0.16.3.tgz; exit 0. Then ls shows the three archives"
run 'cargueiro package --family fleet tests/data/charts/fleet/src/fleet-crd tests/data/charts/fleet/src/fleet-agent tests/data/charts/fleet/src/fleet --output-dir .tmp/manual-tests/out'
run 'ls .tmp/manual-tests/out'

hdr "2. HAPPY: the three archives are accepted together as a family"
want "three reports in the order fleet, fleet-agent, fleet-crd, all 'valid: true'; exit 0"
run 'cargueiro check archive --family fleet .tmp/manual-tests/out/fleet-110.0.3+up0.16.3.tgz .tmp/manual-tests/out/fleet-agent-110.0.3+up0.16.3.tgz .tmp/manual-tests/out/fleet-crd-110.0.3+up0.16.3.tgz'

hdr "3. FAILURE: one chart is not valid (a copy where fleet-agent has no annotations)"
want "the report of fleet-agent only ('valid: false'), then 'error: fleet-agent: the chart is not valid, so the family was not packaged'; exit 1. Then ls says the output folder is not there (exit 2): no archive was built, not even the ones of the charts that are valid"
run 'cp -r tests/data/charts/fleet/src .tmp/manual-tests/bad && sed -i "/catalog.cattle.io/d" .tmp/manual-tests/bad/fleet-agent/Chart.yaml'
run 'cargueiro package --family fleet .tmp/manual-tests/bad/fleet .tmp/manual-tests/bad/fleet-agent .tmp/manual-tests/bad/fleet-crd --output-dir .tmp/manual-tests/out-bad'
run 'ls .tmp/manual-tests/out-bad'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
