#!/usr/bin/env bash
# Runs the pinned golangci-lint, building it once with `go install` when no
# matching binary is available yet.
#
# Why build from source: a prebuilt release is compiled with an older Go than this
# repository uses, and a type checker can only read export data from its own Go
# release or older. The system install (2.13.2, built with go1.27.0) therefore
# fails on this tree (go1.27.2) with "export data version 5 is greater than
# maximum supported version 4".
#
# Where things go: the standard locations, exactly as a manual `go install` would
# use them ($GOBIN or $GOPATH/bin for the binary, ~/.cache/golangci-lint for its
# cache, the usual module and build caches). A restricted runner that mounts those
# read-only gets a shared scratch home instead (<main worktree>/.tmp/lint, shared
# by every worktree so nothing is downloaded twice). The module cache is only
# relocated when the default one is not writable, so a warm ~/go/pkg/mod is always
# reused. LINT_HOME forces the scratch form.
#
# If LINT_VERSION changes, update lintPin in the justfile too.
set -euo pipefail

version="v2.14.0"
toolchain="go1.27.2"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

export GOTOOLCHAIN="$toolchain"

# matches reports whether a binary is the pinned version. `version` prints the
# bare number ("2.14.0") while the pin carries the v prefix.
want="${version#v}"
matches() {
    [ -n "${1:-}" ] && [ -x "$1" ] && "$1" version 2>/dev/null | grep -q " $want "
}

# writable reports whether a directory can actually be written. The permission
# bits are not enough: a read-only mount keeps them looking fine and fails with
# EROFS only on the write.
writable() {
    local dir="$1" probe
    mkdir -p "$dir" 2>/dev/null || return 1
    probe="$dir/.envsub-write-probe"
    touch "$probe" 2>/dev/null || return 1
    rm -f "$probe"
}

go_bin="$(go env GOBIN)"
[ -n "$go_bin" ] || go_bin="$(go env GOPATH)/bin"

# An already-installed copy wins: nothing to build, nothing to download.
for candidate in "$(command -v golangci-lint 2>/dev/null || true)" "$go_bin/golangci-lint"; do
    if matches "$candidate"; then
        exec "$candidate" "$@"
    fi
done

if [ -z "${LINT_HOME:-}" ] && writable "$go_bin" && writable "${XDG_CACHE_HOME:-$HOME/.cache}"; then
    echo "installing golangci-lint $version into $go_bin" >&2
    cd "$root"
    go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$version"
    exec "$go_bin/golangci-lint" "$@"
fi

# Scratch home for a runner whose standard locations are read-only. It is anchored
# to the main worktree (git's common dir), not to the current worktree, so every
# worktree shares one installed binary and one module cache.
common="$(git -C "$root" rev-parse --git-common-dir 2>/dev/null || true)"
case "$common" in
    /*) ;;
    "") common="$root/.git" ;;
    *) common="$root/$common" ;;
esac
shared="$(dirname "$common")"
home="${LINT_HOME:-$shared/.tmp/lint}"

export GOBIN="$home/bin"
export GOCACHE="${GOCACHE:-$shared/.tmp/gocache}"
export GOTMPDIR="${GOTMPDIR:-$shared/.tmp/gotmp}"
export GOLANGCI_LINT_CACHE="$home/lintcache"
# Reuse the default module cache whenever it is writable; only a read-only one is
# replaced. A fresh module cache per run is what made every build re-download.
if ! writable "$(go env GOMODCACHE)"; then
    export GOPATH="$home/gopath"
    export GOMODCACHE="$home/gopath/pkg/mod"
fi
mkdir -p "$GOBIN" "$GOCACHE" "$GOTMPDIR" "$GOLANGCI_LINT_CACHE" "$GOMODCACHE"

if ! matches "$home/bin/golangci-lint"; then
    echo "installing golangci-lint $version ($toolchain) into $home" >&2
    cd "$root"
    go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$version"
fi
exec "$home/bin/golangci-lint" "$@"
