#!/usr/bin/env bash
# Runs the pinned golangci-lint, building it once into the tool home when the
# version there does not match.
#
# Why build from source: a prebuilt release is compiled with an older Go than this
# repository uses, and a type checker can only read export data from its own Go
# release or older. The system install (2.13.2, built with go1.27.0) therefore
# fails on this tree (go1.27.2) with "export data version 5 is greater than
# maximum supported version 4".
#
# Why the tool home lives in the repository: this environment leaves no other
# writable place that persists. $HOME/.cache and $GOMODCACHE are read-only, and
# /tmp is a tmpfs that is emptied between invocations, so a tool home there would
# re-download and rebuild the linter every run. Everything lives under
# .tmp/lint (gitignored) and is reused as long as the pin does not change; the Go
# build cache stays shared with the rest of the project.
#
# If LINT_VERSION changes, update lintPin in the justfile too.
set -euo pipefail

version="v2.14.0"
toolchain="go1.27.2"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
home="${LINT_HOME:-$root/.tmp/lint}"
bin="$home/bin/golangci-lint"

export GOTOOLCHAIN="$toolchain"
export GOPATH="$home/gopath"
export GOMODCACHE="$home/gopath/pkg/mod"
export GOBIN="$home/bin"
# The Go build cache and temp dir are shared with the project's other recipes
# (the justfile exports the same defaults), so the linter does not keep a second
# copy of compiled packages.
export GOCACHE="${GOCACHE:-$root/.tmp/gocache}"
export GOTMPDIR="${GOTMPDIR:-$root/.tmp/gotmp}"
export GOLANGCI_LINT_CACHE="$home/lintcache"
mkdir -p "$GOBIN" "$GOCACHE" "$GOTMPDIR" "$GOLANGCI_LINT_CACHE"

# `version` prints the bare number ("2.14.0") while the pin carries the v prefix.
want="${version#v}"
if [ ! -x "$bin" ] || ! "$bin" version 2>/dev/null | grep -q " $want "; then
    echo "installing golangci-lint $version ($toolchain) into $home" >&2
    cd "$root"
    go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$version"
fi

exec "$bin" "$@"
