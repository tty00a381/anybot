#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

version="${1:-}"
if [[ -z "$version" ]]; then
	echo "usage: scripts/release-build.sh vX.Y.Z" >&2
	exit 2
fi
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
	echo "invalid version: $version" >&2
	exit 2
fi

export GOCACHE="${GOCACHE:-$ROOT/.local/go-cache}"
mkdir -p "$GOCACHE"

dist="$ROOT/dist/$version"
rm -rf "$dist"
mkdir -p "$dist"

build_one() {
	local goos="$1"
	local goarch="$2"
	local ext=""
	if [[ "$goos" == "windows" ]]; then
		ext=".exe"
	fi
	local name="anybot_${version}_${goos}_${goarch}"
	local bin="$dist/$name/anybot$ext"
	mkdir -p "$(dirname "$bin")"
	GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
		go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$bin" ./cmd/anybot
	cp LICENSE README.md "$dist/$name/"
	cp -R docs examples "$dist/$name/"
	(
		cd "$dist"
		tar -czf "$name.tar.gz" "$name"
		rm -rf "$name"
	)
}

build_one darwin arm64
build_one darwin amd64
build_one linux amd64
build_one linux arm64
build_one windows amd64

(
	cd "$dist"
	shasum -a 256 *.tar.gz > checksums.txt
)

echo "release artifacts:"
ls -1 "$dist"
