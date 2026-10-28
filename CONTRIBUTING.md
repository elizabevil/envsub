# Contributing

## Requirements

- Go 1.27.3
- `just`
- `golangci-lint` v2 (validated with v2.13.2)
- `govulncheck`

## Before submitting

```bash
just fmt
just vet
just test
just test-fixtures
just test-all
just test-perf   # REF-04 performance gate; needs GNU envsubst (gettext-base)
just lint
```

Changes that can affect rendering speed must report `just test-perf` numbers in
the pull request; the gate compares against GNU envsubst on the same machine (see
`docs/reference/REF-04-gnu-envsubst-performance-baseline.md`).

## Code style

Strictly follow Google Go Style Guide and the project-specific rules in:

- `docs/03-design/03-10-go-code-standards.md`

## Tests

Tests are fixture-first. New behavior must include a fixture or unit test.
See `docs/05-tasks/05-02-p0-test-matrix.md`.

## Build and test commands

```bash
just check      # gofmt + vet + lint + unit/fixture tests + e2e + examples
just fmt
just lint       # golangci-lint v2, see .golangci.yml
just test
just test-race
just test-fuzz  # fuzz every parser (8 targets, 30s each)
just test-perf  # performance gate, needs GNU envsubst (gettext-base)
just bench
just size       # binary size report
just build-sonic  # same binary with the sonic JSON backend (faster, larger)
```

`just` points `GOCACHE`/`GOTMPDIR` at a workspace-local `.tmp/` directory
because some shared toolchains mount the default caches read-only.

## JSON backend

JSON encoding and decoding are implemented in `internal/jsonx`, which provides
two build-time variants:

|                      | default (stdlib `encoding/json/v2`) | `-tags=sonic`  |
|----------------------|-------------------------------------|----------------|
| linux/amd64 binary   | 5.03 MiB                            | 8.55 MiB       |
| startup              | 4.6 ms                              | 9.5 ms         |
| REF-04 peak RSS      | 1.30× GNU                           | 1.80–2.25× GNU |
| JSON decode (364 KB) | 12.04 ms                            | 2.45 ms        |

The default variant reduces binary size by 41% and has lower startup time and
memory use. Use `-tags=sonic` when JSON throughput is more important than size.
Binary sizes are measured with the Go release named in `go.mod`; a newer Go
release shifts them by a few hundred KiB, so compare within one toolchain.

YAML encoding and decoding use `internal/yamlx` with one library,
`go.yaml.in/yaml/v3`: decoding walks nodes (integers remain integers, duplicate
keys and merge keys are handled), and `toYaml` encodes the same tree.
