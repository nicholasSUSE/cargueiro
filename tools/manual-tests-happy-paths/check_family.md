# Manual test: `cargueiro check --family`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and python3, and no registry. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

What it prints, in order, for every step:

- a **blue title** (`HAPPY` or `FAILURE`),
- a **yellow line** with what to expect,
- the **command that runs, in cyan**, after a `$`,
- the output of the command, and a bold `=> exit N`.

A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;97;44m %s \033[0m\n' "$1"; }
want() { printf '\033[33mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[36m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "1. HAPPY: the three folders, given out of order"
want "three reports in the order fleet, fleet-agent, fleet-crd, all 'valid: true'; exit 0"
run 'cargueiro check chart --family fleet tests/data/charts/fleet/src/fleet-crd tests/data/charts/fleet/src/fleet-agent tests/data/charts/fleet/src/fleet'

hdr "2. HAPPY: the same from the packaged archives"
want "the same three reports; exit 0"
run 'cargueiro check archive --family fleet tests/data/charts/fleet/published/fleet-crd-110.0.3+up0.16.3.tgz tests/data/charts/fleet/published/fleet-agent-110.0.3+up0.16.3.tgz tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz'

hdr "3. HAPPY: as JSON, one line per chart"
want "fleet main True / fleet-agent extra True / fleet-crd crd True, all 110.0.3+up0.16.3; exit 0"
run 'cargueiro check chart --family fleet --output json tests/data/charts/fleet/src/fleet-crd tests/data/charts/fleet/src/fleet-agent tests/data/charts/fleet/src/fleet | python3 -c "import json,sys; [print(r[\"chart\"], r[\"kind\"], r[\"valid\"], r[\"metadata\"][\"version\"]) for r in json.load(sys.stdin)]"'

hdr "4. HAPPY: without --family, a single chart works as before"
want "one report for fleet; exit 0"
run 'cargueiro check chart tests/data/charts/fleet/src/fleet'

hdr "5. FAILURE: a chart of the family is missing"
want "error: the family \"fleet\" is missing fleet-agent; nothing on stdout; exit 1"
run 'cargueiro check chart --family fleet tests/data/charts/fleet/src/fleet tests/data/charts/fleet/src/fleet-crd'

hdr "6. FAILURE: a chart is given twice"
want "error: chart \"fleet\" is given twice; exit 1"
run 'cargueiro check chart --family fleet tests/data/charts/fleet/src/fleet tests/data/charts/fleet/src/fleet tests/data/charts/fleet/src/fleet-agent tests/data/charts/fleet/src/fleet-crd'

hdr "7. FAILURE: a chart of another family"
want "error: chart \"longhorn\" is not in the family \"fleet\"; exit 1"
run 'cargueiro check chart --family fleet tests/data/charts/fleet/src/fleet tests/data/charts/fleet/src/fleet-agent tests/data/charts/fleet/src/fleet-crd tests/data/charts/longhorn/src/longhorn'

hdr "8. FAILURE: one chart is not valid (a copy where fleet-agent has no annotations)"
want "all three charts reported, only fleet-agent 'valid: false'; error: the family \"fleet\" is not valid: fleet-agent; exit 1"
run 'rm -rf .tmp/manual-tests/bad && cp -r tests/data/charts/fleet/src .tmp/manual-tests/bad && sed -i "/catalog.cattle.io/d" .tmp/manual-tests/bad/fleet-agent/Chart.yaml'
run 'cargueiro check chart --family fleet .tmp/manual-tests/bad/fleet .tmp/manual-tests/bad/fleet-agent .tmp/manual-tests/bad/fleet-crd'

hdr "9. FAILURE: the versions of the family differ (a copy where fleet-agent has another version)"
want "the three reports, then: error: the charts of the family \"fleet\" do not have the same version, with the version of each chart; exit 1"
run 'rm -rf .tmp/manual-tests/diff && cp -r tests/data/charts/fleet/src .tmp/manual-tests/diff && sed -i "s/^version: .*/version: 110.0.4+up0.16.4/" .tmp/manual-tests/diff/fleet-agent/Chart.yaml'
run 'cargueiro check chart --family fleet .tmp/manual-tests/diff/fleet .tmp/manual-tests/diff/fleet-agent .tmp/manual-tests/diff/fleet-crd'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
