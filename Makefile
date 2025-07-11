.PHONY: test bench demo lint coverage clean all

all: test lint

test:
	@echo "Running tests..."
	@go test -v -race ./...
	@for dir in examples/*/; do \
		echo "Testing $$dir"; \
		(cd "$$dir" && go test -v -race); \
	done

bench:
	@echo "Running benchmarks..."
	@cd benchmarks && go test -bench=. -benchmem -benchtime=10s

demo:
	@echo "Running demos..."
	@cd demos && go run . all

lint:
	@echo "Running linters..."
	@golangci-lint run --config=.golangci.yml

lint-security:
	@echo "Running security-focused linters..."
	@golangci-lint run --config=.golangci.yml --enable=gosec,errorlint,noctx,bodyclose,sqlclosecheck

lint-fix:
	@echo "Running linters with auto-fix..."
	@golangci-lint run --config=.golangci.yml --fix

coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean:
	@echo "Cleaning..."
	@rm -f coverage.out coverage.html
	@find . -name "*.test" -delete
	@find . -name "*.prof" -delete

install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest