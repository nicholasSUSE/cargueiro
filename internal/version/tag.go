package version

import "strings"

// ToOCITag returns the tag a chart version is stored under in an OCI registry: a
// "+" becomes "_", because a tag cannot hold a "+". Every command that sends a
// version to a registry uses it, except Helm's own push, which does the same.
func ToOCITag(version string) string {
	return strings.ReplaceAll(version, "+", "_")
}

// FromOCITag returns the chart version stored under an OCI tag: a "_" becomes "+".
// Every command that receives a tag from a registry uses it. A version never
// holds a "_", so the conversion loses nothing.
func FromOCITag(tag string) string {
	return strings.ReplaceAll(tag, "_", "+")
}
