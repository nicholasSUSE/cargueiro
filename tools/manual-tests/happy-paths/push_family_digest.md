# Manual test: `cargueiro push --family` and the content of a chart that is in the registry

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, `helm` and a network, and it reads the real staging registry of SUSE, `stgregistry.suse.com/rancher/catalog`, anonymously: it has no credentials, so it can not write there, and both pushes end before any upload. It takes the chart `rancher-webhook 112.0.0+up0.13.0-rc.7`, which is already in that registry. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The configuration is an empty one in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), so no login of yours is used. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout, put it first on the PATH of this shell, use an empty configuration, and take the chart from the registry"
run 'mkdir -p .tmp/manual-tests/bin .tmp/manual-tests/config .tmp/manual-tests/chart && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"; unset CARGUEIRO_USERNAME CARGUEIRO_PASSWORD'
run 'cargueiro list stgregistry.suse.com/rancher/catalog/rancher-webhook'
run '(cd .tmp/manual-tests/chart && helm pull oci://stgregistry.suse.com/rancher/catalog/rancher-webhook --version 112.0.0+up0.13.0-rc.7 > /dev/null) && ls .tmp/manual-tests/chart'

hdr "1. HAPPY: push the archive that the registry holds"
want "a warning that rancher-webhook 112.0.0+up0.13.0-rc.7 is in the registry already with the same content, so it was not pushed; one line on stdout, oci://stgregistry.suse.com/rancher/catalog/rancher-webhook:112.0.0_up0.13.0-rc.7; exit 0"
run 'cargueiro push --family rancher-webhook ".tmp/manual-tests/chart/rancher-webhook-112.0.0+up0.13.0-rc.7.tgz" --registry stgregistry.suse.com/rancher/catalog'

hdr "2. FAILURE: push the same version with other content (one line added to its values.yaml)"
want "error: nothing was pushed: …/rancher-webhook:112.0.0+up0.13.0-rc.7 is in the registry already with other content (digest sha256:…, the archive has sha256:…): a version is never overwritten, so give the chart a new version; two different digests; nothing on stdout; exit 1"
run 'mkdir -p .tmp/manual-tests/changed && tar xzf ".tmp/manual-tests/chart/rancher-webhook-112.0.0+up0.13.0-rc.7.tgz" -C .tmp/manual-tests/changed && printf "\n# changed\n" >> .tmp/manual-tests/changed/rancher-webhook/values.yaml && tar czf .tmp/manual-tests/rancher-webhook-changed.tgz -C .tmp/manual-tests/changed rancher-webhook'
run 'cargueiro push --family rancher-webhook .tmp/manual-tests/rancher-webhook-changed.tgz --registry stgregistry.suse.com/rancher/catalog'

hdr "CLEANUP: forget the configuration, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
