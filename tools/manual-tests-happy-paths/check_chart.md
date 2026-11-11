# Manual test: `cargueiro check chart`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and python3, and no registry. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "1. HAPPY: a chart that follows the rules"
want "one report for fleet: annotations, hip15-annotations and version all valid; 'valid: true'; exit 0"
run 'cargueiro check chart tests/data/charts/fleet/src/fleet'

hdr "2. HAPPY: as JSON"
want "one line: fleet main True 110.0.3+up0.16.3; exit 0"
run 'cargueiro check chart --output json tests/data/charts/fleet/src/fleet | python3 -c "import json,sys; r=json.load(sys.stdin); print(r[\"chart\"], r[\"kind\"], r[\"valid\"], r[\"metadata\"][\"version\"])"'

hdr "3. FAILURE: a chart that breaks the rules (longhorn)"
want "valid: false, a wrong kube-version and images that are not listed in helm.sh/images, then 'error: chart is not valid'; exit 1"
run 'cargueiro check chart tests/data/charts/longhorn/src/longhorn'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
