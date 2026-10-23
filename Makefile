.PHONY: build lint test check hooks

build:
	go build ./...

lint:
	golangci-lint run ./...

test:
	go test -race ./...

# Same gates as CI. Run before pushing.
check: build lint test

# Run `make check` on every git push.
hooks:
	git config core.hooksPath .githooks
