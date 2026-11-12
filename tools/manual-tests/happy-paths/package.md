# Manual test: `cargueiro package`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, and no registry. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The charts it packages are copies of the `rancher-compliance` fixture in `.tmp/manual-tests`. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout, put it first on the PATH of this shell, and stamp a copy of the chart"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'
run 'cp -r tests/data/charts/rancher-compliance-upstream/src/rancher-compliance .tmp/manual-tests/stamped && cargueiro stamp .tmp/manual-tests/stamped --version 111.0.0+up1.5.4'

hdr "1. HAPPY: package the stamped chart"
want "one line on stdout, the absolute path of the archive, ending in .tmp/manual-tests/out/rancher-compliance-111.0.0+up1.5.4.tgz; exit 0. Then ls shows the archive"
run 'cargueiro package .tmp/manual-tests/stamped --output-dir .tmp/manual-tests/out'
run 'ls -l .tmp/manual-tests/out'

hdr "2. HAPPY: the archive follows the chart rules"
want "one report for rancher-compliance with 'valid: true' and the version 111.0.0+up1.5.4; exit 0"
run 'cargueiro check archive ".tmp/manual-tests/out/rancher-compliance-111.0.0+up1.5.4.tgz"'

hdr "3. FAILURE: a chart that was never stamped (a fresh copy)"
want "a report with 'valid: false' (helm.sh/images does not list the images, and the version 1.5.4 must look like 1XY.Z.W+upA.B.C), then 'error: the chart is not valid, so it was not packaged'; exit 1. Then ls says the output folder is not there (exit 2): nothing was made"
run 'cp -r tests/data/charts/rancher-compliance-upstream/src/rancher-compliance .tmp/manual-tests/plain'
run 'cargueiro package .tmp/manual-tests/plain --output-dir .tmp/manual-tests/out2'
run 'ls .tmp/manual-tests/out2'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
