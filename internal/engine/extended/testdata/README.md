# Engine test fixtures

Every directory under this tree is one fixture, discovered and run by
`fixture_test.go` (`TestFixtures`). A directory without `input.tmpl` fails the
suite, so a typo cannot silently drop a case.

| File | Meaning |
|------|---------|
| `input.tmpl` | template under test (required) |
| `values.yaml` | root values, YAML |
| `values-2.yaml` | second values file, for merge order |
| `env.yaml` | process environment, read through `@process` |
| `env.txt` | `--dotenv-file` content |
| `schema.yaml` | JSON Schema for defaults, materialization and validation |
| `meta.yaml` | the options below |
| `expect.out` | exact expected stdout for a success case |
| `expect.err` | substring expected on stderr for a failure case |

`meta.yaml` keys:

| Key | Effect |
|-----|--------|
| `unset` | `--on-missing` policy (`error`/`empty`/`keep`/`zero`) |
| `empty` | `--on-empty` policy (`ok`/`error`) |
| `only` / `ignore` | `--only-vars` / `--ignore-vars` roots or path prefixes |
| `schema_unknown` | `--unknown-keywords` policy, default `ignore` |
| `exit_code` | expected exit code; without it the case must succeed |

Group directories (`syntax/`, `defaults/`, `policies/`, `pipeline/`, `types/`,
`inline_type/`, `schema/`, `sources/`, `errors/`) only organise the fixtures; the
runner does not treat them specially.

Write the template in `input.tmpl`; do not generate templates in Go test code.
