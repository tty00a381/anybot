#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
root=$(mktemp -d "${TMPDIR:-/tmp}/anybot-release-e2e.XXXXXX")
trap 'rm -rf "$root"' EXIT INT TERM

cli="$root/anybot"
plugin_dir="$root/buddy"
core_dir="$root/corebot"
bot_dir="$root/bot"

cd "$repo_root"
go build -o "$cli" ./cmd/anybot

"$cli" dev plugin buddy -dir "$plugin_dir" -module example.com/anybot-plugin/buddy
(cd "$plugin_dir" && go mod tidy && go test ./...)

"$cli" dev init -dir "$core_dir" -module example.com/anybot-corebot
awk '
	/listen: "127\.0\.0\.1:6700"/ { print "  listen: \"127.0.0.1:0\""; next }
	{ print }
' "$core_dir/core.yaml" > "$core_dir/core.yaml.tmp"
mv "$core_dir/core.yaml.tmp" "$core_dir/core.yaml"
(
	cd "$core_dir"
	go mod tidy
	"$cli" dev doctor
	go run . --help
)

"$cli" init -dir "$bot_dir"
awk '
	/listen: "127\.0\.0\.1:6700"/ { print "    listen: \"127.0.0.1:0\""; next }
	{ print }
' "$bot_dir/anybot.yaml" > "$bot_dir/anybot.yaml.tmp"
mv "$bot_dir/anybot.yaml.tmp" "$bot_dir/anybot.yaml"
"$cli" doctor -config "$bot_dir/anybot.yaml"
"$cli" plugin add example.com/anybot-plugin/buddy -replace "$plugin_dir" -dir "$bot_dir"
"$cli" plugin status -dir "$bot_dir"

plugin_id=$(
	awk '
		/^[[:space:]]*-?[[:space:]]*id:/ { id = $NF }
		/^[[:space:]]*module:[[:space:]]*example\.com\/anybot-plugin\/buddy[[:space:]]*$/ { print id; exit }
	' "$bot_dir/anybot.lock"
)
if [ -z "$plugin_id" ]; then
	echo "release e2e: external plugin id not found" >&2
	exit 1
fi

"$cli" plugin enable "$plugin_id" -dir "$bot_dir"

cat > "$bot_dir/plugins.d/$plugin_id.yaml" <<EOF
enabled: true
config:
  command: ""
EOF
run_log="$root/run.log"
if "$cli" run -dir "$bot_dir" >"$run_log" 2>&1; then
	echo "release e2e: anybot run should reject invalid generated-host plugin config" >&2
	cat "$run_log" >&2
	exit 1
fi
test -x "$bot_dir/anybot-bot" || {
	echo "release e2e: anybot run did not build generated host" >&2
	cat "$run_log" >&2
	exit 1
}
grep -q "插件配置检查失败" "$run_log" || {
	echo "release e2e: anybot run did not reach generated-host plugin check" >&2
	cat "$run_log" >&2
	exit 1
}

cat > "$bot_dir/plugins.d/$plugin_id.yaml" <<EOF
enabled: true
config:
  command: buddy
EOF

(
	cd "$bot_dir"
	./anybot-bot plugin sync
	./anybot-bot plugin check
	./anybot-bot plugin inspect "$plugin_id"
)
