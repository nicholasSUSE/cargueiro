# Manual test: `cargueiro ship`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, `git` and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`. `ship` writes into a checkout of the charts repository, so the setup makes one in `.tmp/manual-tests`: a bare repository as the remote, with a `staging` branch that has an empty `index.yaml`, and a clone of it, as `rancher/charts` has. The git variables it exports make git ignore your own configuration and commit as `test`. The setup also builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login it makes is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. The chart is the published `fleet-crd`, a CRD chart, which needs no logo. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The chart it pushes stays in the local registry until `make registry-down`, so each run uses a new path in it.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "SETUP: make a checkout like the staging branch of rancher/charts, with a bare repository as its remote"
run 'export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com GIT_CONFIG_GLOBAL=/dev/null'
run 'git init -q --bare --initial-branch=staging .tmp/manual-tests/charts.git && git init -q --initial-branch=staging .tmp/manual-tests/seed && : > .tmp/manual-tests/seed/index.yaml && git -C .tmp/manual-tests/seed add index.yaml && git -C .tmp/manual-tests/seed commit -q -m "an empty index" && git -C .tmp/manual-tests/seed remote add origin "$PWD/.tmp/manual-tests/charts.git" && git -C .tmp/manual-tests/seed push -q origin staging && git clone -q --branch staging "$PWD/.tmp/manual-tests/charts.git" .tmp/manual-tests/charts'

hdr "SETUP: log in to the local registry without a password, and push the fleet-crd chart to a path that was never used"
want "a warning that the registry accepts anonymous requests, 'Login succeeded', the path, and the reference oci://…/fleet-crd:110.0.3_up0.16.3 that was pushed"
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'
run 'echo x | cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'
run 'REG="127.0.0.1:5052/manual-ship-$(date +%s)"; echo "$REG"'
run 'cargueiro push tests/data/charts/fleet/published/fleet-crd-110.0.3+up0.16.3.tgz --registry $REG'

hdr "1. HAPPY: ship the chart to the checkout"
want "one line on stdout, entries/fleet-crd/110.0.3+up0.16.3.yaml; exit 0. The entry has 'version: 110.0.3+up0.16.3', with the +, the 'created' date, and a url whose tag has a _: …/fleet-crd:110.0.3_up0.16.3@sha256:…. The date is the same one the registry gives for the chart"
run 'cargueiro ship $REG/fleet-crd:110.0.3+up0.16.3 --repo "$PWD/.tmp/manual-tests/charts" --plain-http'
run 'grep -E "^version:|^created:|^urls:|^- oci://" ".tmp/manual-tests/charts/entries/fleet-crd/110.0.3+up0.16.3.yaml"'
run 'curl -s -H "Accept: application/vnd.oci.image.manifest.v1+json" "http://127.0.0.1:5052/v2/${REG#127.0.0.1:5052/}/fleet-crd/manifests/110.0.3_up0.16.3" | grep -o "\"org.opencontainers.image.created\":\"[^\"]*\""'

hdr "2. HAPPY: index it"
want "one line on stdout, add fleet-crd 110.0.3+up0.16.3; exit 0. Then index.yaml has the version with its +, the url with the _, the same created date as the entry, and generated with that date too, and the archive is in assets/fleet-crd"
run 'cargueiro index --repo "$PWD/.tmp/manual-tests/charts" --registry $REG --plain-http'
run 'grep -E "^    version:|^    created:|^    - oci://|^generated:" .tmp/manual-tests/charts/index.yaml; ls .tmp/manual-tests/charts/assets/fleet-crd'

hdr "3. FAILURE: ship the same version again, after the result is committed and pushed, as rancher/charts has it"
want "an error that says the entry is there already and is never replaced; nothing on stdout; exit 1. Without the commit, ship refuses first because the checkout has changes that are not committed"
run 'git -C .tmp/manual-tests/charts add -A && git -C .tmp/manual-tests/charts commit -q -m "ship and index fleet-crd" && git -C .tmp/manual-tests/charts push -q origin HEAD'
run 'cargueiro ship $REG/fleet-crd:110.0.3+up0.16.3 --repo "$PWD/.tmp/manual-tests/charts" --plain-http'

hdr "CLEANUP: forget the login and the git settings, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR REG GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL GIT_CONFIG_GLOBAL; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
