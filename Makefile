.PHONY: test vet build lint tidy

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./...

tidy:
	go mod tidy

lint: vet test
