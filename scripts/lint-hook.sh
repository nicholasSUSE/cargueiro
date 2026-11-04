#!/usr/bin/env bash
# PostToolUse hook: format and lint the package of an edited .go file.
# Reads the hook JSON from stdin. Exits 2 with findings on stderr so the
# agent sees them; exits 0 otherwise.
set -euo pipefail

file=$(jq -r '.tool_input.file_path // empty')

case "$file" in
*.go) ;;
*) exit 0 ;;
esac

root=$(git -C "$(dirname "$file")" rev-parse --show-toplevel)
cd "$root"

rel=$(realpath --relative-to="$root" "$(dirname "$file")")
pkg="./$rel"
[ "$rel" = "." ] && pkg="."

make --no-print-directory fmt PKG="$pkg" >&2

if ! out=$(make --no-print-directory lint PKG="$pkg" 2>&1); then
	printf 'golangci-lint findings in %s:\n%s\n' "$pkg" "$out" >&2
	exit 2
fi
