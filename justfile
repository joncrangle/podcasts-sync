default:
    @just --list

build:
    go build -o dist/podcasts-sync main.go

run: build
    ./dist/podcasts-sync

debug: build
    DEBUG=true ./dist/podcasts-sync

clean:
    rm -rf dist/
    go clean

fmt:
    go fmt ./...

test:
    go test -v ./...

lint:
    golangci-lint run

tidy:
    go mod tidy

# Build for macOS (both Intel and Apple Silicon)
build-all: clean
    GOOS=darwin GOARCH=amd64 go build -o dist/podcasts-sync-intel main.go
    GOOS=darwin GOARCH=arm64 go build -o dist/podcasts-sync-silicon main.go

update-deps:
    go get -u ./...
    go mod tidy

tag version:
    git tag -a v{{version}} -m "Release v{{version}}"
    git push origin v{{version}}

run-race: 
    go run -race main.go

pre-commit: fmt lint

vhs:
    vhs assets/demo.tape
