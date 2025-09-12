# Agent Guidelines for podcasts-sync

## Build/Test Commands
- `just build` - Build the binary
- `just test` - Run all tests
- `just lint` - Run golangci-lint
- `just fmt` - Format code with gofmt
- `just pre-commit` - Run fmt and lint
- `go test ./...` - Run all tests (alternative to `just test`)
- `go test ./internal` - Run tests for internal package only
- `go test ./tui` - Run tests for TUI package only
- `just run` - Build and run the application
- `just debug` - Build and run with DEBUG=true

## Debugging
- This is a bubbletea TUI app - debugging with prints won't work
- Use the debug message system: `addDebugMsg(title, description)` in tui package
- Debug messages are displayed in the debug panel when DEBUG=true

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

## Testing Protocol
When making changes to the codebase, always follow this testing protocol:

1. **Run tests before making changes**: `just test` to establish baseline
2. **Make your changes incrementally** and test frequently
3. **Run specific package tests** during development:
   - `go test ./internal` for internal package changes
   - `go test ./tui` for TUI changes
4. **Run full test suite**: `just test` before finalizing changes
5. **Check code quality**: `just lint` to ensure code standards
6. **Format code**: `just fmt` or `gofumpt -w .` if gofumpt issues
7. **Final validation**: `just pre-commit` runs fmt and lint together

### Test Coverage Areas
- **internal package**: Core business logic (drive detection, podcast parsing, utils)
- **tui package**: User interface logic (model updates, navigation, state management)
- **Integration tests**: End-to-end workflows using teatest framework

### TUI Testing Notes
- Use teatest for integration testing of TUI components
- Avoid `WaitFinished()` due to continuous polling - use sleep + output verification
- Set `model.loading.macPodcasts = false` when testing drive operations
- Remember that `Update()` returns `Model` for direct cases, `*Model` for handlers