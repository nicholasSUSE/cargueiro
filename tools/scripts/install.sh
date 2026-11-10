#!/bin/sh
# install.sh installs cargueiro, a single self-contained binary, into
# /usr/local/bin. Run it directly, or pipe it from a release:
#
#   curl -sfL https://github.com/nicholasSUSE/cargueiro/releases/latest/download/install.sh | sh -
#
# It installs the latest release, or the one named in CARGUEIRO_VERSION, for
# example CARGUEIRO_VERSION=v0.2.0. It verifies the download against
# checksums.txt and installs nothing on a mismatch. It does not touch
# ~/.config/cargueiro/.
#
# CARGUEIRO_INSTALL_BASE_URL and CARGUEIRO_BIN_DIR are internal, used by the
# test only. They are not part of the interface and may change or disappear.

set -eu
# noglob: never let an unquoted expansion turn into a filename glob.
set -o noglob

REPO="nicholasSUSE/cargueiro"
BIN_NAME="cargueiro"

# fail prints a message to stderr and stops, installing nothing.
fail() {
	echo "install: $1" >&2
	exit 1
}

# have reports whether a command exists.
have() {
	command -v "$1" >/dev/null 2>&1
}

# detect_os maps uname -s to the name a release asset uses.
detect_os() {
	os=$(uname -s)
	case "$os" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	FreeBSD) echo freebsd ;;
	*) fail "unsupported operating system: $os" ;;
	esac
}

# detect_arch maps uname -m to the name a release asset uses.
detect_arch() {
	arch=$(uname -m)
	case "$arch" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	*) fail "unsupported architecture: $arch" ;;
	esac
}

# supported fails unless cargueiro is released for os/arch.
supported() {
	case "$1/$2" in
	linux/amd64 | darwin/amd64 | darwin/arm64 | freebsd/amd64) ;;
	*) fail "no cargueiro release for $1/$2" ;;
	esac
}

# download fetches url into file, with curl or fetch (FreeBSD has no curl). It
# retries and caps each attempt, so a flaky network does not fail on one glitch.
download() {
	url=$1
	file=$2
	if have curl; then
		curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 10 --max-time 300 \
			-o "$file" "$url"
	elif have fetch; then
		fetch -q -a -T 300 -o "$file" "$url"
	else
		fail "need curl or fetch to download cargueiro"
	fi
}

# checksum_of prints the sha256 of file, with whatever tool the system has.
# It calls the sha256 tool through command, so the name does not resolve to
# this function.
checksum_of() {
	if have sha256sum; then
		sha256sum "$1" | awk '{print $1}'
	elif have shasum; then
		shasum -a 256 "$1" | awk '{print $1}'
	elif have sha256; then
		command sha256 -q "$1"
	else
		fail "need sha256sum, shasum or sha256 to verify the download"
	fi
}

# verify checks the binary's sha256 against the line for asset in checksums.
verify() {
	binary=$1
	checksums=$2
	asset=$3

	# sha256sum writes "hash  name" in text mode and "hash *name" in binary
	# mode; strip a leading * so either format matches.
	want=$(awk -v name="$asset" '{sub(/^\*/, "", $2)} $2 == name {print $1}' "$checksums")
	[ -n "$want" ] || fail "no checksum for $asset in checksums.txt"

	got=$(checksum_of "$binary")
	[ "$want" = "$got" ] || fail "checksum mismatch for $asset: expected $want, got $got"
}

# install_binary moves src to dest, with sudo when dest's directory is not
# writable and we are not root. When installed with privilege, the binary is
# owned by root (uid/gid 0, which is root on Linux and wheel on macOS/FreeBSD);
# plain mv keeps the invoking user's ownership. An unprivileged writable dir
# (a user bin dir, or the test's) keeps the user, who cannot chown to root.
install_binary() {
	src=$1
	dest=$2
	dir=$(dirname "$dest")

	if [ "$(id -u)" = 0 ]; then
		mkdir -p "$dir"
		mv -f "$src" "$dest"
		chown 0:0 "$dest"
	elif [ -w "$dir" ]; then
		mkdir -p "$dir"
		mv -f "$src" "$dest"
	elif have sudo; then
		sudo mkdir -p "$dir"
		sudo mv -f "$src" "$dest"
		sudo chown 0:0 "$dest"
	else
		fail "need write access to $dir, or sudo, to install cargueiro"
	fi
}

main() {
	os=$(detect_os)
	arch=$(detect_arch)
	supported "$os" "$arch"
	asset="${BIN_NAME}_${os}_${arch}"

	version=${CARGUEIRO_VERSION:-latest}

	base=${CARGUEIRO_INSTALL_BASE_URL:-}
	if [ -z "$base" ]; then
		if [ "$version" = latest ]; then
			base="https://github.com/${REPO}/releases/latest/download"
		else
			base="https://github.com/${REPO}/releases/download/${version}"
		fi
	fi

	bin_dir=${CARGUEIRO_BIN_DIR:-/usr/local/bin}
	dest="${bin_dir}/${BIN_NAME}"

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT INT TERM HUP

	echo "install: downloading $asset ($version)"
	download "${base}/${asset}" "${tmp}/${asset}" || fail "could not download ${base}/${asset}"
	download "${base}/checksums.txt" "${tmp}/checksums.txt" || fail "could not download ${base}/checksums.txt"

	verify "${tmp}/${asset}" "${tmp}/checksums.txt" "$asset"

	chmod +x "${tmp}/${asset}"
	install_binary "${tmp}/${asset}" "$dest"

	echo "install: installed cargueiro to $dest"
	"$dest" version
}

main "$@"
