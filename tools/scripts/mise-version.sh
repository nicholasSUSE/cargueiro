#!/usr/bin/env bash
# Prints the version that mise.toml pins for a tool, so the workflows install
# exactly what developers get from `mise install`.
#
#   mise-version.sh <tool> [mise.toml]
set -euo pipefail

tool=${1:?usage: mise-version.sh <tool> [mise.toml]}
file=${2:-mise.toml}

# Takes the first `tool = "version"` line of [tools]. The key may be quoted
# (`"go:github.com/x/y" = "1.2.3"`), and a comment may follow the value.
version=$(awk -v tool="$tool" '
	/^\[/ { in_tools = ($0 == "[tools]"); next }
	!in_tools { next }
	{
		line = $0
		sub(/^[ \t]+/, "", line)
		split(line, kv, "=")
		key = kv[1]
		gsub(/[ \t"]/, "", key)
		if (key != tool) next
		value = substr(line, index(line, "=") + 1)
		sub(/^[ \t]*"/, "", value)
		sub(/".*$/, "", value)
		print value
		exit
	}
' "$file")

if [ -z "$version" ]; then
	echo "mise-version: no version for '$tool' in $file" >&2
	exit 1
fi
echo "$version"
