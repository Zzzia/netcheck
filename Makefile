APP := netcheck

.PHONY: build test vet dist

build:
	go build -o $(APP) ./cmd/netcheck

test:
	go test ./...

vet:
	go vet ./...

dist:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/$(APP)-linux-amd64 ./cmd/netcheck
	GOOS=darwin GOARCH=amd64 go build -o dist/$(APP)-darwin-amd64 ./cmd/netcheck
	GOOS=darwin GOARCH=arm64 go build -o dist/$(APP)-darwin-arm64 ./cmd/netcheck
