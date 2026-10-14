.PHONY: check build test vet fmt fmt-check tidy cover

check: fmt-check vet build test

build:
	go build ./...

test:
	go test -race -cover ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .) || exit $$?; \
	if [ -n "$$out" ]; then \
		echo "The following files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	fi

tidy:
	go mod tidy

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
