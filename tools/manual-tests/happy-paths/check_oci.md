# Manual test: `cargueiro check oci`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, python3 and the local registries (`make registry-up`), and it uses the one without a password, on `127.0.0.1:5052`. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login it makes is saved in `.tmp/manual-tests/config`, not in your own configuration. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end. The chart it pushes stays in the local registry until `make registry-down`.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "SETUP: push the fleet chart to the local registry without a password"
want "{} from the registry, a warning that it accepts anonymous requests, 'Login succeeded', and oci://127.0.0.1:5052/manual-check/fleet:110.0.3_up0.16.3"
run 'curl -fsS -w "\n" http://127.0.0.1:5052/v2/'
run 'echo x | XDG_CONFIG_HOME="$PWD/.tmp/manual-tests/config" cargueiro registry login 127.0.0.1:5052 --username x --password-stdin --plain-http'
run 'XDG_CONFIG_HOME="$PWD/.tmp/manual-tests/config" cargueiro push tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz --registry 127.0.0.1:5052/manual-check'

hdr "1. HAPPY: the chart in the registry follows the rules"
want "one report for fleet, all valid, 'valid: true'; exit 0"
run 'cargueiro check oci 127.0.0.1:5052/manual-check/fleet:110.0.3+up0.16.3 --plain-http'

hdr "2. HAPPY: as JSON"
want "one line: fleet main True 110.0.3+up0.16.3; exit 0"
run 'cargueiro check oci 127.0.0.1:5052/manual-check/fleet:110.0.3+up0.16.3 --plain-http --output json | python3 -c "import json,sys; r=json.load(sys.stdin); print(r[\"chart\"], r[\"kind\"], r[\"valid\"], r[\"metadata\"][\"version\"])"'

hdr "3. FAILURE: a version that is not in the registry"
want "error: 127.0.0.1:5052/manual-check/fleet:110.0.9_up0.16.9 was not found in the registry; nothing on stdout; exit 1"
run 'cargueiro check oci 127.0.0.1:5052/manual-check/fleet:110.0.9+up0.16.9 --plain-http'

hdr "CLEANUP: take the binary off the PATH and remove the copies and the login"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
