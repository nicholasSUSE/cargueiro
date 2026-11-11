# Manual test: `cargueiro check archive`

Run it from the root of the repository, in bash or zsh, and paste the whole block. It needs Go, and no registry. The setup builds the binary of the current checkout into `.tmp/manual-tests/bin` and puts it first on the `PATH` of your shell, so every command is a plain `cargueiro …` with real paths. Everything the test creates is in `.tmp/manual-tests`, which git ignores, and the block removes it at the end.

For every step it prints a **blue title**, a **yellow line** with what to expect, the **command in cyan** after a `$`, the output, and a bold `=> exit N`. A command is the text inside `run '…'`.

```bash
hdr()  { printf '\n\033[1;97;44m %s \033[0m\n' "$1"; }
want() { printf '\033[33mexpect: %s\033[0m\n' "$1"; }
run()  { printf '\033[36m$ %s\033[0m\n' "$1"; eval "$1"; printf '\033[1m=> exit %s\033[0m\n' "$?"; }

hdr "SETUP: build the binary of this checkout and put it first on the PATH of this shell"
run 'mkdir -p .tmp/manual-tests/bin && go build -o .tmp/manual-tests/bin/cargueiro .'
run 'PATH="$PWD/.tmp/manual-tests/bin:$PATH"'
run 'command -v cargueiro && cargueiro version'

hdr "1. HAPPY: a packaged chart that follows the rules"
want "one report for fleet, all valid, 'valid: true'; exit 0"
run 'cargueiro check archive tests/data/charts/fleet/published/fleet-110.0.3+up0.16.3.tgz'

hdr "2. FAILURE: an archive that is not there"
want "error: tests/data/charts/fleet/published/nope.tgz not found; nothing on stdout; exit 1"
run 'cargueiro check archive tests/data/charts/fleet/published/nope.tgz'

hdr "3. FAILURE: a file that is not a chart archive"
want "error: README.md: the archive is not gzip: gzip: invalid header; exit 1"
run 'cargueiro check archive README.md'

hdr "CLEANUP: take the binary off the PATH and remove the copies"
run 'PATH=${PATH#"$PWD/.tmp/manual-tests/bin:"}; rm -rf .tmp/manual-tests'
unset -f hdr want run
```
