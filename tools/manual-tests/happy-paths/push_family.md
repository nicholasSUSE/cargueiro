# Manual test: `cargueiro push --family`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. The charts it pushes are the packaged fleet charts of the repository. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The charts it pushes stay in the local registry until `make registry-down`.

The retries cannot be shown here, because they wait 30, 60, 120 and 240 seconds: they are tested with a fake registry. On this registry, which the rules do not protect, a chart that is there is replaced, as for a single chart. On the SUSE registries it would be skipped.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "SETUP: log in to the local registry without a password"
want "a warning that the registry accepts anonymous requests, and 'Login succeeded'"
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'
run 'echo x | cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'

hdr "1. HAPPY: push the three charts of the family, with the archives given out of order"
want "three lines on stdout, the references that were pushed, in the order of the family: fleet, fleet-agent, fleet-crd, all oci://127.0.0.1:5052/manual-family/…:110.0.3_up0.16.3; exit 0"
run 'cargueiro push --family fleet tests/data/charts/fleet/published/fleet-crd-110.0.3+up0.16.3.tgz tests/data/charts/fleet/published/fleet-agent-110.0.3+up0.16.3.tgz tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz --registry 127.0.0.1:5052/manual-family'

hdr "2. HAPPY: the three charts are in the registry"
want "each list prints 110.0.3+up0.16.3; exit 0 for the three"
run 'for c in fleet fleet-agent fleet-crd; do cargueiro list 127.0.0.1:5052/manual-family/$c --plain-http; done'

hdr "3. FAILURE: a chart of the family is missing"
want "error: the family \"fleet\" is missing fleet-agent; nothing on stdout; exit 1. Nothing was pushed to the new path: list says the repository does not exist"
run 'cargueiro push --family fleet tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz tests/data/charts/fleet/published/fleet-crd-110.0.3+up0.16.3.tgz --registry 127.0.0.1:5052/manual-family-incomplete'
run 'cargueiro list 127.0.0.1:5052/manual-family-incomplete/fleet --plain-http'

hdr "4. FAILURE: fleet and fleet-crd are at 110.0.3, but fleet-agent is at 110.0.4 (a copy of its archive with another version)"
want "error: the charts of the family \"fleet\" do not have the same version: fleet 110.0.3+up0.16.3, fleet-agent 110.0.4+up0.16.3, fleet-crd 110.0.3+up0.16.3; nothing on stdout; exit 1. Nothing was pushed to the new path: list says the repository does not exist"
run 'mkdir -p .tmp/manual-tests/agent && tar xzf tests/data/charts/fleet/published/fleet-agent-110.0.3+up0.16.3.tgz -C .tmp/manual-tests/agent && sed -i "s/^version: .*/version: 110.0.4+up0.16.3/" .tmp/manual-tests/agent/fleet-agent/Chart.yaml && tar czf ".tmp/manual-tests/fleet-agent-110.0.4+up0.16.3.tgz" -C .tmp/manual-tests/agent fleet-agent'
run 'cargueiro push --family fleet tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz ".tmp/manual-tests/fleet-agent-110.0.4+up0.16.3.tgz" tests/data/charts/fleet/published/fleet-crd-110.0.3+up0.16.3.tgz --registry 127.0.0.1:5052/manual-family-versions'
run 'cargueiro list 127.0.0.1:5052/manual-family-versions/fleet --plain-http'

hdr "5. FAILURE: a push the rules refuse, to the image path of the staging registry of SUSE"
want "one error line for each of the three charts: pushing to this registry is not allowed: … can only go to stgregistry.suse.com/rancher/catalog; exit 1. It is decided from rules.yaml, before any secret is read or any connection is made"
run 'cargueiro push --family fleet tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz tests/data/charts/fleet/published/fleet-agent-110.0.3+up0.16.3.tgz tests/data/charts/fleet/published/fleet-crd-110.0.3+up0.16.3.tgz --registry stgregistry.suse.com/rancher'

hdr "CLEANUP: forget the login, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
