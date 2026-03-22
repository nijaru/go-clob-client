.PHONY: fmt vet test build tidy check

fmt:
	@files="$$(git ls-files '*.go')"; \
	if [ -z "$$files" ]; then \
		echo "no tracked Go files to format"; \
	else \
		goimports -w $$files; \
		golines --base-formatter gofumpt -w $$files; \
	fi

vet:
	@go vet ./...

test:
	@go test ./...

build:
	@go build ./...

tidy:
	@go mod tidy

check: fmt vet test build
