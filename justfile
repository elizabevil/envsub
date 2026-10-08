set shell := ["bash", "-uc"]

# Some shared toolchains mount the default GOCACHE read-only. Pointing it at a
# workspace-local directory keeps `just` recipes working there; override the
# variable in the environment to use a different cache.
export GOCACHE := env("GOCACHE", justfile_directory() / ".tmp/gocache")
export GOTMPDIR := env("GOTMPDIR", justfile_directory() / ".tmp/gotmp")

bin := "bin/envsub"

# Build metadata is injected through the same variables internal/version
# exposes, so `envsub version` reports something useful in release artifacts.
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
commit := `git rev-parse --short HEAD 2>/dev/null || echo none`
build_date := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X github.com/elizabevil/envsub/internal/version.Version=" + version + " -X github.com/elizabevil/envsub/internal/version.Commit=" + commit + " -X github.com/elizabevil/envsub/internal/version.BuildDate=" + build_date

build:
    go build -trimpath -ldflags "{{ ldflags }}" -o {{ bin }} ./cmd/envsub

fmt:
    go fmt ./...

fmt-check:
    test -z "$(gofmt -l ./cmd ./internal ./test)"

vet:
    go vet ./...

lint:
    golangci-lint run ./...

tidy:
    go mod tidy

test:
    go test -count=1 ./...

test-fixtures:
    cd internal/engine/extended && just test-fixtures

test-engine: test-fixtures

test-race:
    go test -race -count=1 ./...

test-e2e:
    cd test && just test-e2e

test-all: test test-e2e

fixtures-list:
    cd internal/engine/extended && just fixtures-list

fixtures-tree:
    cd internal/engine/extended && just fixtures-tree

e2e-list:
    cd test && just e2e-list

test-update:
    UPDATE_GOLDEN=1 go test -count=1 ./internal/engine/extended

run *args:
    go run ./cmd/envsub {{ args }}

bench:
    go test -run XXX -bench=. -benchmem ./...

check: fmt-check vet test test-e2e

build-linux-amd64:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/envsub-linux-amd64 ./cmd/envsub

build-linux-arm64:
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/envsub-linux-arm64 ./cmd/envsub

build-darwin-amd64:
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/envsub-darwin-amd64 ./cmd/envsub

build-darwin-arm64:
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/envsub-darwin-arm64 ./cmd/envsub

build-windows-amd64:
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/envsub-windows-amd64.exe ./cmd/envsub
