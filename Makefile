.PHONY: fmt fmt-check test race vet check release-files release-check

GOFILES := $(shell find . -name '*.go' -not -path './.git/*')
RELEASE_FILES := \
	README.md \
	docs/users/README.md \
	docs/plugin-developers/README.md \
	docs/maintainers/README.md \
	docs/maintainers/api-index.md \
	assets/waifu.png \
	LICENSE \
	.github/workflows/ci.yaml \
	core/app.go \
	core/file_store.go \
	app/host/app.go \
	app/host/config_store.go \
	app/host/store.go \
	cmd/anybot/main.go \
	cmd/anybot/dev.go \
	internal/scaffold/scaffold.go \
	sdk/access.go \
	sdk/config.go \
	sdk/data_dir.go \
	sdk/doc.go \
	sdk/spec.go \
	sdk/runtime.go \
	sdk/state.go \
	sdk/message/message.go \
	examples/README.md \
	examples/plugins/hello/README.md \
	examples/plugins/hello/hello.go \
	examples/plugins/hello/hello_test.go \
	examples/plugins/groupmemo/README.md \
	examples/plugins/groupmemo/groupmemo.go \
	examples/plugins/groupmemo/groupmemo_test.go \
	examples/plugins/dialogue/README.md \
	examples/plugins/dialogue/dialogue.go \
	examples/plugins/dialogue/dialogue_test.go

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
