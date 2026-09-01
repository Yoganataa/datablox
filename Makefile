.PHONY: run build test vet tidy

run:
	go run ./cmd/bot

build:
	go build -o bin/bot ./cmd/bot

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy