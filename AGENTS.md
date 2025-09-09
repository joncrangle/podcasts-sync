# Agent Guidelines for podcasts-sync

## Build/Test Commands
- `just build` - Build the binary
- `just lint` - Run golangci-lint
- `just fmt` - Format code with gofmt
- `just pre-commit` - Run fmt and lint
- `go test ./...` - Run all tests
- `go test ./internal` - Run tests for internal package
- `just run` - Build and run the application
- `just debug` - Build and run with DEBUG=true

## Code Style
- Use `gofumpt` and `goimports` for formatting
- Follow standard Go naming conventions (CamelCase for exports, camelCase for private)
- Import order: stdlib, third-party, local packages
- Use short variable names in small scopes (e.g., `err`, `ch`, `i`)
- Always handle errors explicitly, never ignore with `_`
- Use meaningful struct and interface names (e.g., `USBDrive`, `PodcastScanner`)
- Keep functions under 50 lines when possible
- Use channels for communication between goroutines
- Prefer composition over inheritance
- Use context for cancellation and timeouts where appropriate

## Error Handling
- Return errors as the last return value
- Wrap errors with context using `fmt.Errorf("operation failed: %w", err)`
- Use early returns to reduce nesting