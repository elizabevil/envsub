# Repository tasks for envsub.
#
#   just                  list the recipes
#   just build            local binary at dist/envsub
#   just check            formatting, vet, unit/fixture and end-to-end tests
#   just build-release    cross-compiled artifacts (then checksums / sbom)

set shell := ["bash", "-uc"]

# --- settings ---------------------------------------------------------------

# Go's caches are left at their defaults ($GOCACHE, $GOTMPDIR, the module cache);
# recipes use whatever the environment provides. Only the linter is special.
# A restricted runner whose default caches are read-only must point them somewhere
# writable, for example: GOCACHE=/tmp/gocache GOTMPDIR=/tmp/gotmp just check

# The linter is built from source because a prebuilt release is compiled with an
# older Go than this repository uses (see the script for where it lives).
lintPin := "v2.14.0"
lintTool := "bash " + (justfile_directory() / "tools" / "lint" / "golangci-lint.sh")

# Local builds and release artifacts share one output directory.
exe := "dist/envsub"
ldflags := "-s -w"

# Scratch directory for recipes that write intermediate files (.tmp is ignored).
[private]
caches:
    @mkdir -p {{ justfile_directory() }}/.tmp

# --- build ------------------------------------------------------------------

# Version, commit, tree state, compiler and platform come from the build info the
# toolchain embeds, and the build date from the binary's modification time
# (internal/version), so no -X injection is needed.
#
# The default build uses the standard-library JSON backend (encoding/json/v2) and
# is ~3.5 MiB smaller; build-sonic swaps in sonic via a build tag, which decodes
# JSON faster for a much larger binary (see internal/jsonx).

build: (build-with "")

build-sonic: (build-with "sonic")

# build-with builds the local binary, optionally with a build tag.
build-with tag="": caches
    go build -trimpath -ldflags "{{ ldflags }}" {{ if tag != "" { "-tags=" + tag } else { "" } }} -o {{ exe }}{{ if tag != "" { "-" + tag } else { "" } }} ./cmd/envsub

# size reports the total size and the biggest sections of the local binary;
# `just size sonic` reports the sonic backend instead of the default.
size tag="": (build-with tag)
    @target={{ exe }}{{ if tag != "" { "-" + tag } else { "" } }}; bytes=$(wc -c < $target); awk -v b="$bytes" 'BEGIN {printf "%-16s %8.2f MiB (%s bytes)\n", "total", b/1048576, b}'
    @target={{ exe }}{{ if tag != "" { "-" + tag } else { "" } }}; size -A -d $target 2>/dev/null | awk '$2 ~ /^[0-9]+$/ && $2 > 0 && $1 !~ /bss|Total/ {printf "%-16s %8.2f KiB\n", $1, $2/1024}' | sort -k2 -rn | head -8 || true

# size-deps attributes the local binary's size to packages. The shipped binary
# has no symbols, so it builds an unstripped copy and sums the symbol sizes nm
# reports per package; the difference between the named symbols and the file size
# is metadata (pclntab, type descriptors, rodata). See docs/06-review/06-07.
size-deps: caches
    @go build -trimpath -o {{ justfile_directory() }}/.tmp/envsub-symbols ./cmd/envsub
    @nm --size-sort --print-size -t d {{ justfile_directory() }}/.tmp/envsub-symbols | awk '{s=$2+0; n=$4; if (n ~ /^github\.com\// || n ~ /^golang\.org\// || n ~ /^go\.yaml\.in\// || n ~ /^sigs\.k8s\.io\//) {split(n,a,"/"); k=a[1]"/"a[2]"/"a[3]} else if (n ~ /^internal\//) {split(n,a,"/"); k="internal/"a[2]} else if (n ~ /^encoding\/json/) {k="encoding/json*"} else if (n ~ /^runtime/) {k="runtime"} else {split(n,a,"."); k=a[1]} t[k]+=s} END {for (k in t) printf "%9.1f KB  %s\n", t[k]/1024, k}' | sort -rn | head -20
    @echo "--- feature stacks ---"
    @nm --size-sort --print-size -t d {{ justfile_directory() }}/.tmp/envsub-symbols | awk '{s=$2+0; n=$4; if (n ~ /^go\.yaml\.in\// || n ~ /^sigs\.k8s\.io\/yaml/ || n ~ /^gopkg\.in\/yaml/) y+=s; else if (n ~ /^github\.com\/spf13\//) c+=s; else if (n ~ /^github\.com\/santhosh-tekuri\/jsonschema/ || n ~ /^golang\.org\/x\/text/ || n ~ /^regexp/ || n ~ /^math\/big/ || n ~ /^text\/template/ || n ~ /^net\/url/) sc+=s; else if (n ~ /^encoding\/json/) j+=s; else if (n ~ /^github\.com\/elizabevil\/envsub/ || n ~ /^internal\//) o+=s} END {printf "%-22s %9.1f KB\n", "schema stack", sc/1024; printf "%-22s %9.1f KB\n", "yaml stack", y/1024; printf "%-22s %9.1f KB\n", "json stack", j/1024; printf "%-22s %9.1f KB\n", "cobra + pflag", c/1024; printf "%-22s %9.1f KB\n", "our code", o/1024}'
    @printf "%-22s %s\n" "file size" "$(wc -c < {{ exe }} | awk '{printf "%.2f MB", $1/1000000}')"

# --- release artifacts ------------------------------------------------------

# build-target cross-compiles one artifact: `just build-target linux arm64`.
build-target os arch:
    CGO_ENABLED=0 GOOS={{ os }} GOARCH={{ arch }} go build -trimpath -ldflags "{{ ldflags }}" -o dist/envsub-{{ os }}-{{ arch }}{{ if os == "windows" { ".exe" } else { "" } }} ./cmd/envsub

# build-release cross-compiles the matrix the release workflow uses.
build-release:
    @for target in "linux amd64" "linux arm64" "darwin amd64" "darwin arm64" "windows amd64"; do set -- $target; just --justfile {{ justfile() }} --quiet build-target "$1" "$2"; done
    @ls -1 dist/envsub-linux-* dist/envsub-darwin-* dist/envsub-windows-*

# --- checks -----------------------------------------------------------------

fmt:
    go fmt ./...

fmt-check:
    test -z "$(gofmt -l ./cmd ./internal ./test)"

# fmt-just checks that every justfile is formatted; this also parses them, so a
# syntax error anywhere fails `just check`.
fmt-just:
    @for f in justfile docs/justfile examples/justfile internal/engine/extended/justfile test/justfile; do just --fmt --check --justfile "$f"; done

vet: caches
    go vet ./...

lint: caches
    {{ lintTool }} run ./...

tidy: caches
    go mod tidy

# check is the local gate: formatting (Go and just), vet, unit/fixture tests,
# the end-to-end suite and the runnable examples.
check: caches fmt-check fmt-just vet lint test test-e2e test-examples

# --- tests ------------------------------------------------------------------

test: caches
    go test -count=1 ./...

test-race: caches
    go test -race -count=1 ./...
    go test -race -count=1 -tags=sonic ./internal/jsonx/

test-e2e: caches
    cd test && just test-e2e

# Every example recipe in sequence: renders each template against its golden,
# walks the failure cases by exit code and compares the checked-in output tree.
# The recipes live in examples/justfile so the guard and the human-facing demo
# are the same code.
# Requires the just version below or newer: the examples recipes use this
# justfile's syntax, and CI installs at least this version.
minJust := "1.58.0"

test-examples:
    @current=$(just --version | awk '{print $2}'); \
        awk -v min='{{ minJust }}' -v cur="$current" 'BEGIN { n=split(min,a,"."); split(cur,b,"."); for (i=1;i<=n;i++) { if ((b[i]+0)>(a[i]+0)) exit 0; if ((b[i]+0)<(a[i]+0)) exit 1 } exit 0 }' \
        || { echo "just $current is older than the supported minimum {{ minJust }}"; exit 1; }
    just --justfile examples/justfile demo

# Performance gate (TASK-023): renders the REF-04 baseline template with GNU
# envsubst and with envsub and enforces the ratios from docs/reference/REF-04.
# Needs GNU envsubst in PATH (Debian/Ubuntu: gettext-base). Tune with
# PERF_RUNS / PERF_MAX_TIME_RATIO / PERF_MAX_RSS_RATIO / PERF_MAX_STARTUP_MS.
test-perf: caches
    go test -count=1 -v ./test/perf

test-all: test test-e2e test-perf test-examples

test-fixtures: caches
    cd internal/engine/extended && just test-fixtures

test-engine: test-fixtures

test-update: caches
    UPDATE_GOLDEN=1 go test -count=1 ./internal/engine/extended

fixtures-list:
    cd internal/engine/extended && just fixtures-list

fixtures-tree:
    cd internal/engine/extended && just fixtures-tree

e2e-list:
    cd test && just e2e-list

bench: caches
    go test -run XXX -bench=. -benchmem ./...

# --- supply chain (TASK-024) ------------------------------------------------

# vuln needs network: the tool module and the vulnerability database are fetched
# on demand. CI runs the same scan in the `security` job.
vuln: caches
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# checksums writes dist/SHA256SUMS for the release artifacts. The patterns are
# explicit so the local dev builds (dist/envsub, dist/envsub-sonic) are not
# included.
checksums:
    @cd dist && shopt -s nullglob && files=(envsub-linux-* envsub-darwin-* envsub-windows-*); sha256sum "${files[@]}" > SHA256SUMS && cat SHA256SUMS

# checksums-verify checks dist/ against an existing dist/SHA256SUMS, which is
# what a consumer does after downloading a release.
checksums-verify:
    cd dist && sha256sum -c SHA256SUMS

# sbom needs syft (https://github.com/anchore/syft); the release workflow runs
# the same scan through anchore/sbom-action and publishes the SPDX JSON.
sbom: caches
    syft dir:dist --source-name envsub --source-version $(git describe --tags --always --dirty 2>/dev/null || echo dev) -o spdx-json > dist/envsub.spdx.json
    @echo "wrote dist/envsub.spdx.json"

# --- development ------------------------------------------------------------

run *args: caches
    go run ./cmd/envsub {{ args }}

# Remove the scratch directory (intermediate files, cached lint tool). Go's own
# caches live in the environment's default locations and are left alone.
clean-cache:
    rm -rf {{ justfile_directory() }}/.tmp
    @echo "removed .tmp"

# Remove every scratch artifact (.tmp plus dist and coverage output).
clean: clean-cache
    rm -rf {{ justfile_directory() }}/.tmp {{ justfile_directory() }}/dist {{ justfile_directory() }}/coverage.out {{ justfile_directory() }}/coverage.txt
    @echo "removed .tmp and dist"
