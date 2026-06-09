.PHONY: test vet race release-check release-build clean

VERSION ?=

test:
	go test ./...

vet:
	go vet ./...

race:
	go test -race ./sdk ./sdk/testkit ./app/host ./cmd/anybot ./internal/scaffold ./examples/plugins/...

release-check:
	scripts/release-check.sh

release-build:
	scripts/release-build.sh $(VERSION)

clean:
	rm -rf dist .local/go-cache
