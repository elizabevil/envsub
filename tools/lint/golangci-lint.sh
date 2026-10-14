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
# Where the tool lives: the user cache when it is writable, otherwise the
# repository's .tmp/lint. A restricted runner can mount $HOME read-only (the
# sandbox used while developing this does), and /tmp is a tmpfs that is emptied
# between invocations, so .tmp is the fallback that keeps the build cached.
# LINT_HOME overrides both. If LINT_VERSION changes, update lintPin in the
# justfile too.
set -euo pipefail

version="v2.14.0"
toolchain="go1.27.2"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# A writable location is picked once: the standard user cache first, then the
# repository's .tmp. Writability is tested by writing, because a read-only mount
# reports EROFS while the permission bits still look writable.
writable() {
    local dir="$1"
    mkdir -p "$dir" 2>/dev/null && [ -w "$dir" ] && touch "$dir/.probe" 2>/dev/null
}
pick_home() {
    local candidate
    # /tmp is deliberately not a candidate: it is a tmpfs that some harnesses
    # empty between invocations, so the tool would be rebuilt on every run.
    for candidate in "${XDG_CACHE_HOME:-$HOME/.cache}/envsub-lint" "$root/.tmp/lint"; do
        if writable "$candidate"; then
            rm -f "$candidate/.probe"
            printf '%s' "$candidate"
            return
        fi
    done
    printf '%s' "$root/.tmp/lint"
}

home="${LINT_HOME:-$(pick_home)}"
bin="$home/bin/golangci-lint"

export GOTOOLCHAIN="$toolchain"
export GOBIN="$home/bin"
export GOLANGCI_LINT_CACHE="$home/lintcache"

# A tool home inside the repository means the user caches are unusable (a
# read-only $HOME, an emptied /tmp), so Go's own caches move into the workspace
# too. With the standard user cache, leave GOPATH/GOMODCACHE/GOCACHE alone: the
# build then reuses the modules and packages the user already has.
case "$home" in
"$root"/.tmp/*)
    export GOPATH="$home/gopath"
    export GOMODCACHE="$home/gopath/pkg/mod"
    export GOCACHE="${GOCACHE:-$root/.tmp/gocache}"
    export GOTMPDIR="${GOTMPDIR:-$root/.tmp/gotmp}"
    ;;
esac
mkdir -p "$GOBIN" "$GOLANGCI_LINT_CACHE" "${GOCACHE:-}" "${GOTMPDIR:-}" 2>/dev/null || true

# `version` prints the bare number ("2.14.0") while the pin carries the v prefix.
want="${version#v}"
if [ ! -x "$bin" ] || ! "$bin" version 2>/dev/null | grep -q " $want "; then
    echo "installing golangci-lint $version ($toolchain) into $home" >&2
    cd "$root"
    go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$version"
fi

exec "$bin" "$@"
