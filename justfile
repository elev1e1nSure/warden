# warden — project task runner
# https://github.com/casey/just

set positional-arguments := true

default:
    @just --list

VERSION := env_var_or_default("TAG", "dev")

build:
    go build -ldflags="-s -w -X 'github.com/elev1e1nSure/warden.wardenVersion={{VERSION}}'" -o warden.exe ./cmd/warden

build-check:
    go build ./...

test *args:
    go test ./... {{args}}

test-cov:
    go test ./... -coverprofile=cover.out

lint:
    go vet ./...

lint-go:
    go vet ./...

fmt:
    gofmt -d -l .

fmt-write:
    gofmt -w .
