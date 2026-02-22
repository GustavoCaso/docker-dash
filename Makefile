.PHONY: build test test-race lint generate-test-coverage clean install help

.DEFAULT_GOAL := help

build:
	go build -o docker-dash ./cmd/docker-dash

test:
	go test ./...
	
test-race:
	go test ./... -race

lint:
	golangci-lint run --fix
	
format:
	golangci-lint fmt .
	
install:
	go install ./cmd/docker-dash/.

generate-test-coverage:
	@echo "Generating coverage report"
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html
	
clean:
	rm -f docker-dash coverage.out coverage.html

help:
	@echo "Available targets:"
	@echo "  build                  - Build the CLI binary"
	@echo "  test                   - Run all tests"
	@echo "  test-race              - Run all tests with the race detector"
	@echo "  generate-test-coverage - Generate coverage report"
	@echo "  lint                   - Run golangci-lint"
	@echo "  clean                  - Clean build artifacts"
	@echo "  help                   - Show this help message"
