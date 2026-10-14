#!/usr/bin/env bash
# Runs the pinned golangci-lint, installing it once into .tmp/bin when the version
# on PATH does not match.
#
# The tool is built from source on purpose: a prebuilt release is compiled with an
# older Go than this repository uses, and a type checker can only read export data
# from its own Go release or older (see docs/05-tasks/05-03-pending-work.md, 待办 O).
# If the version here changes, update lintPin in the justfile too.
set -euo pipefail

version="v2.14.0"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bin="$root/.tmp/bin/golangci-lint"

# The caches live in the workspace because a shared toolchain may mount the
# default ones read-only; GOPATH is pinned for the same reason, so the build
# neither reads nor writes the caller's environment.
export GOTOOLCHAIN=go1.27.2
export GOPATH="$root/.tmp/gopath"
export GOBIN="$root/.tmp/bin"
export GOCACHE="${GOCACHE:-$root/.tmp/gocache}"
export GOTMPDIR="${GOTMPDIR:-$root/.tmp/gotmp}"
export GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-$root/.tmp/golangci-cache}"
mkdir -p "$GOBIN" "$GOCACHE" "$GOTMPDIR" "$GOLANGCI_LINT_CACHE"

# `version` prints the bare number ("2.14.0"), the pin carries the v prefix.
want="${version#v}"
if [ ! -x "$bin" ] || ! "$bin" version 2>/dev/null | grep -q " $want "; then
    echo "installing golangci-lint $version (once; cached in .tmp/)" >&2
    cd "$root"
    go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$version"
fi

exec "$bin" "$@"
