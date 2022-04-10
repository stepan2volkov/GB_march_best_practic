check:
	./bin/golangci-lint run -c golangci-lint.yaml

install-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.45.2