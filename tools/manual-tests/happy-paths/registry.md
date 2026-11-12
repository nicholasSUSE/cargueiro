# Manual test: `cargueiro registry`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go and the local registries (`make registry-up`), and it uses the one with a password, on `127.0.0.1:5050` (user `admin`, password `cargueiro-test`), so the login is really checked. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. The login is saved in `.tmp/manual-tests/config` (through `CARGUEIRO_CONFIG_DIR`), not in your own configuration. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **green title**, an **orange line** with what to expect, the **command in blue** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;38;2;48;186;120m%s\033[0m\n' "$1"; }
want() { printf '\033[38;2;254;124;63mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[38;2;0;117;168m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout, put it first on the PATH of this shell, and keep the login in a folder of the test"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'
run 'export CARGUEIRO_CONFIG_DIR="$PWD/.tmp/manual-tests/config"'

hdr "1. FAILURE: log in with a wrong password"
want "error: login to 127.0.0.1:5050 failed: … response status code 401: unauthorized; exit 1. Nothing is saved: ls says the config folder is not there (exit 2)"
run 'echo wrong | cargueiro registry login 127.0.0.1:5050 --username admin --password-stdin --plain-http'
run 'ls .tmp/manual-tests/config'

hdr "2. HAPPY: log in with the right password"
want "a warning that the password is saved in plain text, readable only by you, and 'Login succeeded for 127.0.0.1:5050'; exit 0. The file is -rw------- (only you can read it)"
run 'echo cargueiro-test | cargueiro registry login 127.0.0.1:5050 --username admin --password-stdin --plain-http'
run 'ls -l .tmp/manual-tests/config'

hdr "3. FAILURE: log in again before logging out (only one login is kept)"
want "error: already logged in to 127.0.0.1:5050: run \`cargueiro registry logout\` first; exit 1"
run 'echo cargueiro-test | cargueiro registry login 127.0.0.1:5050 --username admin --password-stdin --plain-http'

hdr "4. HAPPY: log out, and log out again"
want "'Removed the saved login for 127.0.0.1:5050' and exit 0; the second time 'No saved login' and exit 0, because logging out when nothing is saved is not an error"
run 'cargueiro registry logout'
run 'cargueiro registry logout'

hdr "CLEANUP: forget the login, take the binary off the PATH and remove the copies"
run 'unset CARGUEIRO_CONFIG_DIR; PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
