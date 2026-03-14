.PHONY: dev build test migrate-up migrate-down lint

# Development: run with hot reload (requires air)
dev:
	air

# Build the binary
build:
	cd web && npm ci && npm run build
	go build -o freereps ./cmd/freereps

# Run all tests
test:
	go test ./...

# Run integration tests (requires running TimescaleDB)
test-integration:
	go test -tags integration ./...

# Run database migrations up
migrate-up:
	go run ./cmd/freereps -migrate-only

# Run database migrations down
migrate-down:
	@echo "Use golang-migrate CLI: migrate -path migrations -database postgres://... down"

# Lint
lint:
	golangci-lint run ./...
