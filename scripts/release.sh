#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
	echo "usage: scripts/release.sh <version>" >&2
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
	usage
	exit 0
fi

version="${1:-}"
if [[ -z "$version" || $# -ne 1 ]]; then
	usage
	exit 2
fi
if [[ "$version" =~ [^0-9A-Za-z._+-] ]]; then
	echo "invalid version: $version" >&2
	exit 2
fi

export GOCACHE="${GOCACHE:-$ROOT/.local/go-cache}"
mkdir -p "$GOCACHE"

echo "==> gofmt"
unformatted="$(gofmt -l $(find . -name '*.go' -not -path './.git/*' -not -path './.local/*' -not -path './dist/*'))"
if [[ -n "$unformatted" ]]; then
	echo "$unformatted" >&2
	echo "gofmt required" >&2
	exit 1
fi

echo "==> go test"
go test ./...

echo "==> go vet"
go vet ./...

dist="$ROOT/dist/$version"
rm -rf "$dist"
mkdir -p "$dist"

copy_release_files() {
	local target="$1"
	cp LICENSE README.md "$target/"
	for path in assets examples docs; do
		if [[ -e "$path" ]]; then
			cp -R "$path" "$target/"
		fi
	done
}

build_one() {
	local goos="$1"
	local goarch="$2"
	local ext=""
	if [[ "$goos" == "windows" ]]; then
		ext=".exe"
	fi

	local name="anybot_${version}_${goos}_${goarch}"
	local dir="$dist/$name"

	echo "==> build $goos/$goarch"
	mkdir -p "$dir"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$dir/anybot$ext" ./cmd/anybot
	copy_release_files "$dir"
	tar -C "$dist" -czf "$dist/$name.tar.gz" "$name"
	rm -rf "$dir"
}

for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do
	build_one "${target%/*}" "${target#*/}"
done

echo "==> checksums"
(
	cd "$dist"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum *.tar.gz > checksums.txt
	else
		shasum -a 256 *.tar.gz > checksums.txt
	fi
)

echo "release artifacts: $dist"
