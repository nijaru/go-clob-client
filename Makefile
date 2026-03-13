.PHONY: fmt test build check

fmt:
	@files="$$(git ls-files '*.go')"; \
	if [ -z "$$files" ]; then \
		echo "no tracked Go files to format"; \
	else \
		golines --base-formatter gofumpt -w $$files; \
	fi

test:
	go test ./...

build:
	go build ./...

check: test build
