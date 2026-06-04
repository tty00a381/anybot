.PHONY: fmt fmt-check test race vet check release-files release-check

GOFILES := $(shell find . -name '*.go' -not -path './.git/*')
RELEASE_FILES := \
	README.md \
	assets/waifu.png \
	LICENSE \
	.github/workflows/ci.yml \
	cmd/anybot/main.go \
	examples/ping/main.go \
	examples/reversews/main.go \
	examples/websocket/main.go \
	examples/httpaction/main.go \
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
