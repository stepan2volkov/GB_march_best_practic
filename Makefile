BUILD_COMMIT := $(shell git log --format="%H" -n 1)

.PHONY: check
check: install-tools
	./bin/golangci-lint run -c golangci-lint.yaml

.PHONY: install-tools
install-tools:
	if [ ! -f ./bin/golangci-lint ]; then \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.45.2; \
	fi;
	

.PHONY: test
test:
	go test -race ./...

.PHONY: build
build:
	go build -o ./build/findext -ldflags="-X 'main.BuildCommit=$(BUILD_COMMIT)'" ./cmd/findext
