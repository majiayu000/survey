.PHONY: test test-unit test-e2e test-all

GO := /usr/local/go/bin/go

# Run unit tests only (default)
test: test-unit

# Run unit tests (excluding e2e)
test-unit:
	$(GO) test -v ./...

# Run E2E tests (requires Docker)
test-e2e:
	$(GO) test -v -tags=e2e ./internal/api/e2e_test.go ./internal/api/handlers.go ./internal/api/dto.go ./internal/api/middleware.go ./internal/api/router.go -timeout=5m

# Run all tests (unit + e2e)
test-all:
	$(GO) test -v ./...
	$(GO) test -v -tags=e2e ./internal/api/e2e_test.go ./internal/api/handlers.go ./internal/api/dto.go ./internal/api/middleware.go ./internal/api/router.go -timeout=5m

# Build the API server
build:
	$(GO) build -o bin/survey-api ./cmd/api

# Build the consumer
build-consumer:
	$(GO) build -o bin/survey-consumer ./cmd/consumer

# Run the API server locally
run:
	$(GO) run ./cmd/api

# Run the consumer locally
run-consumer:
	$(GO) run ./cmd/consumer

# Clean build artifacts
clean:
	rm -rf bin/
