.PHONY: fmt fmt-check test race vet check release-files release-e2e release-check

GOFILES := $(shell find . -name '*.go' -not -path './.git/*')
RELEASE_GO_FILES := $(shell find \
	core \
	sdk \
	app \
	adapters/onebot11 \
	cmd/anybot \
	internal/scaffold \
	examples \
	-name '*.go' | sort)
RELEASE_FILES := \
	go.mod \
	go.sum \
	README.md \
	docs/users/README.md \
	docs/plugin-developers/README.md \
	docs/maintainers/README.md \
	docs/maintainers/api-index.md \
	assets/waifu.png \
	LICENSE \
	.github/workflows/ci.yaml \
	scripts/release-e2e.sh \
	examples/README.md \
	examples/plugins/hello/README.md \
	examples/plugins/groupmemo/README.md \
	examples/plugins/dialogue/README.md

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
	@for file in $(RELEASE_FILES) $(RELEASE_GO_FILES); do \
		test -s "$$file" || { echo "missing release file: $$file"; exit 1; }; \
	done

release-e2e:
	sh scripts/release-e2e.sh

release-check: check release-files release-e2e
	@test -z "$$(git status --short)"
