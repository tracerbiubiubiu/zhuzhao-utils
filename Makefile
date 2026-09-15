.PHONY: test vet build lint tidy fmt

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .

# 对齐生态门禁规格：vet + gofmt 漂移检查（standards §4）
lint: vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt 漂移：" && gofmt -l . && exit 1)
	$(MAKE) test
