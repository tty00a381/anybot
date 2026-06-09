#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export GOCACHE="${GOCACHE:-$ROOT/.local/go-cache}"
mkdir -p "$GOCACHE"

if [[ "${RELEASE_CHECK_REQUIRE_CLEAN:-0}" == "1" ]]; then
	if [[ -n "$(git status --short)" ]]; then
		git status --short >&2
		echo "working tree must be clean" >&2
		exit 1
	fi
fi

echo "==> release files"
required_files=(
	Makefile
	README.md
	docs/users/README.md
	docs/plugin-developers/README.md
	docs/maintainers/README.md
	examples/README.md
	examples/plugins/hello/README.md
	examples/plugins/groupmemo/README.md
	examples/plugins/dialogue/README.md
	scripts/release-check.sh
	scripts/release-build.sh
	LICENSE
	go.mod
	go.sum
)
for file in "${required_files[@]}"; do
	if [[ ! -s "$file" ]]; then
		echo "missing release file: $file" >&2
		exit 1
	fi
done

echo "==> gofmt"
unformatted="$(gofmt -l $(find . -name '*.go' -not -path './.git/*' -not -path './.local/*'))"
if [[ -n "$unformatted" ]]; then
	echo "$unformatted" >&2
	echo "gofmt required" >&2
	exit 1
fi

echo "==> go test"
go test ./...

echo "==> go vet"
go vet ./...

echo "==> race smoke"
go test -race ./sdk ./sdk/testkit ./app/host ./cmd/anybot ./internal/scaffold ./examples/plugins/...

tmp="$(mktemp -d)"
cleanup() {
	rm -rf "$tmp"
}
trap cleanup EXIT

echo "==> generated plugin scaffold"
go run ./cmd/anybot dev plugin release-smoke \
	-dir "$tmp/plugin" \
	-module example.com/anybot-release-smoke \
	-replace "$ROOT"
(cd "$tmp/plugin" && go test ./...)

echo "==> generated bot workspace"
go run ./cmd/anybot init "$tmp/bot"
perl -0pi -e 's/listen: "127\.0\.0\.1:6700"/listen: "127.0.0.1:0"/' "$tmp/bot/anybot.yaml"
go run ./cmd/anybot doctor -config "$tmp/bot/anybot.yaml"
go run ./cmd/anybot plugin add example.com/anybot-release-smoke -replace "$tmp/plugin" -dir "$tmp/bot"
plugin_id="$(go run ./cmd/anybot plugin list -dir "$tmp/bot" | awk 'NR==2 {print $1}')"
go run ./cmd/anybot plugin enable "$plugin_id" -dir "$tmp/bot"
go run ./cmd/anybot build -dir "$tmp/bot"
(cd "$tmp/bot" && ./anybot-bot plugin check)

echo "release check passed"
