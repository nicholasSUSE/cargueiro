# Manual test: `cargueiro list`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login it makes is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. It stamps and packages a copy of the `rancher-compliance` fixture three times and pushes the three versions, one of them a prerelease, to the local registry. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The charts it pushes stay in the local registry until `make registry-down`.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "SETUP: log in to the local registry without a password, and push three versions of a chart"
want "a warning that the registry accepts anonymous requests, 'Login succeeded', and three lines oci://127.0.0.1:5052/manual-list/rancher-compliance:… for 111.0.0, 111.0.1 and 111.1.0 (the last with -rc.1)"
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'
run 'echo x | cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'
run 'cp -r tests/data/charts/rancher-compliance-upstream/src/rancher-compliance .tmp/manual-tests/chart'
run 'for v in 111.0.0+up1.5.4 111.0.1+up1.5.4 111.1.0+up1.5.5-rc.1; do cargueiro stamp .tmp/manual-tests/chart --version $v >/dev/null && cargueiro push "$(cargueiro package .tmp/manual-tests/chart --output-dir .tmp/manual-tests/out)" --registry 127.0.0.1:5052/manual-list; done'

hdr "1. HAPPY: the versions of the chart, newest first"
want "111.1.0+up1.5.5-rc.1, then 111.0.1+up1.5.4, then 111.0.0+up1.5.4, one on each line; exit 0"
run 'cargueiro list 127.0.0.1:5052/manual-list/rancher-compliance --plain-http'

hdr "2. HAPPY: as JSON, with the tag each version is stored under"
want "a list in the same order; every version has its tag, where the + is a _ (for example 111.1.0_up1.5.5-rc.1); exit 0"
run 'cargueiro list 127.0.0.1:5052/manual-list/rancher-compliance --plain-http --output json'

hdr "3. FAILURE: a repository that has nothing"
want "an error that says the chart was not found in the registry; nothing on stdout; exit 1"
run 'cargueiro list 127.0.0.1:5052/manual-list-empty/rancher-compliance --plain-http'

hdr "CLEANUP: forget the login, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
