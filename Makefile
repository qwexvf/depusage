.PHONY: fmt lint test tidy check

fmt:
	gofumpt -w .
	goimports -w -local github.com/qwexvf/depusage .

lint:
	golangci-lint run ./...

test:
	go test -race ./...

tidy:
	go mod tidy

check: lint test
