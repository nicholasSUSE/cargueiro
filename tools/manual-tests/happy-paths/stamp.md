# Manual test: `cargueiro stamp`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, and no registry, because the version is given. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The chart it stamps is a copy of the `rancher-compliance` fixture in `.tmp/manual-tests/compliance`. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout, put it first on the PATH of this shell, and copy the chart"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'
run 'cp -r tests/data/charts/rancher-compliance-upstream/src/rancher-compliance .tmp/manual-tests/compliance'

hdr "1. HAPPY: a dry run"
want "a diff that changes 'version: 1.5.4' to 'version: 111.0.0+up1.5.4' and adds helm.sh/images with four images; nothing is written; exit 0"
run 'cargueiro stamp .tmp/manual-tests/compliance --version 111.0.0+up1.5.4 --dry-run'
run 'grep -n "^version:" .tmp/manual-tests/compliance/Chart.yaml'

hdr "2. HAPPY: stamp the chart, and stamp it again"
want "'version: 1.5.4 -> 111.0.0+up1.5.4' and 'helm.sh/images: added kuberlr-kubectl, compliance-operator, security-scan, mirrored-sonobuoy-sonobuoy'; exit 0. The second run prints 'nothing to change'; exit 0"
run 'cargueiro stamp .tmp/manual-tests/compliance --version 111.0.0+up1.5.4'
run 'cargueiro stamp .tmp/manual-tests/compliance --version 111.0.0+up1.5.4'

hdr "3. FAILURE: a version the chart cannot have (the chart is for Rancher 2.16, whose charts have the major 111)"
want "an error that says the major 112 is not the one of Rancher 2.16; nothing on stdout; exit 1. The chart keeps version 111.0.0+up1.5.4"
run 'cargueiro stamp .tmp/manual-tests/compliance --version 112.0.0+up1.5.4'
run 'grep -n "^version:" .tmp/manual-tests/compliance/Chart.yaml'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
