#!/usr/bin/env bash
# Tests mise-version.sh against a throwaway mise.toml.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/mise-version.sh"
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT

cat > "$dir/mise.toml" <<'TOML'
# comment = "0.0.0"
[settings]
go = "9.9.9"

[tools]
golangci-lint = "2.15.0"
shellcheck = "0.11.0"  # lints install.sh
"go:github.com/go-delve/delve/cmd/dlv" = "1.27.2"
helm = "4.3.1"      # keep equal to go.mod
go = "1.27.2"
TOML

failed=0
expect() {
	local want=$1 tool=$2 got
	got=$("$script" "$tool" "$dir/mise.toml" 2>/dev/null) || got="<error>"
	if [ "$got" != "$want" ]; then
		echo "FAIL $tool: want '$want', got '$got'" >&2
		failed=1
	fi
}

expect 2.15.0 golangci-lint
expect 0.11.0 shellcheck
expect 1.27.2 "go:github.com/go-delve/delve/cmd/dlv"
expect 4.3.1 helm
expect 1.27.2 go
expect "<error>" oras
expect "<error>" delve

exit "$failed"
