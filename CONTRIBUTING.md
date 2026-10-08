# Contributing

## Requirements

- Go 1.27.1
- `just`
- `golangci-lint`
- `govulncheck`

## Before submitting

```bash
just fmt
just vet
just test
just test-fixtures
just test-all
just lint
```

## Code style

Strictly follow Google Go Style Guide and the project-specific rules in:

- `docs/03-design/03-10-go-code-standards.md`

## Tests

Tests are fixture-first. New behavior must include a fixture or unit test.
See `docs/05-tasks/05-02-p0-test-matrix.md`.
