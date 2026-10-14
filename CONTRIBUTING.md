# Contributing

## Requirements

- Go 1.27.1
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
the pull request; the gate compares against GNU envsubst on the same machine
(see `docs/reference/REF-04-gnu-envsubst-performance-baseline.md`).

## Code style

Strictly follow Google Go Style Guide and the project-specific rules in:

- `docs/03-design/03-10-go-code-standards.md`

## Tests

Tests are fixture-first. New behavior must include a fixture or unit test.
See `docs/05-tasks/05-02-p0-test-matrix.md`.
