.PHONY: dev run build test check fmt tidy

# The app, rebuilt and restarted as files change.
dev:
	go tool mygo dev

run: dev

# The packaged app in build/<os>-<arch>/.
build:
	go tool mygo build

test:
	go test ./...

# go vet, and mygo's check of native UI build lifetimes.
check:
	go tool mygo vet .

fmt:
	gofmt -w cmd internal

tidy:
	go mod tidy
