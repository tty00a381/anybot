.PHONY: fmt fmt-check test race vet check release-files release-check

GOFILES := $(shell find . -name '*.go' -not -path './.git/*')
RELEASE_FILES := \
	README.md \
	docs/index.md \
	docs/getting-started.md \
	docs/core.md \
	docs/anybot.md \
	docs/plugin-development.md \
	docs/configuration.md \
	docs/onebot11.md \
	docs/architecture.md \
	assets/waifu.png \
	LICENSE \
	.github/workflows/ci.yaml \
	core/app.go \
	app/host/app.go \
	cmd/anybot/main.go \
	cmd/anybot/dev.go \
	sdk/doc.go \
	sdk/spec.go \
	sdk/runtime.go \
	sdk/message/message.go \
	examples/ping/main.go \
	examples/reversews/main.go \
	examples/websocket/main.go \
	examples/httpaction/main.go \
	examples/proactive/main.go \
	examples/dialogueplugin/dialogueplugin.go \
	examples/pluginbot/main.go \
	examples/permission/main.go \
	examples/session/main.go \
	examples/media/main.go

fmt:
	gofmt -w $(GOFILES)

fmt-check:
	@test -z "$$(gofmt -l $(GOFILES))"

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

check: fmt-check test race vet

release-files:
	@for file in $(RELEASE_FILES); do \
		test -s "$$file" || { echo "missing release file: $$file"; exit 1; }; \
	done

release-check: check release-files
	@test -z "$$(git status --short)"
