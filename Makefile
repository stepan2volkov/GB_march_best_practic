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