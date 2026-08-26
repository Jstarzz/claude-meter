.PHONY: build test vet check

build:
	CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o claude-meter ./cmd/claude-meter

test:
	CGO_ENABLED=1 go test ./...

vet:
	CGO_ENABLED=1 go vet ./...

check: test vet build
