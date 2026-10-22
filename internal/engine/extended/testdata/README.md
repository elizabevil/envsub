# Engine test fixtures

Every directory under this tree that holds an `input.tmpl` is one fixture,
discovered and run by `fixture_test.go` (`TestFixtures`). A directory without
one is not a fixture: the group directories (`syntax/`, `defaults/`, ...)
only organise the cases, and `fuzz/` holds the corpus Go's fuzzing engine
owns.

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
| `repeat` | render the template N times; the expected output is the expected output of one copy repeated N times |

Group directories (`syntax/`, `defaults/`, `policies/`, `pipeline/`, `types/`,
`inline_type/`, `schema/`, `sources/`, `errors/`, `parallel/`) only organise the
fixtures; the runner does not treat them specially.

Every fixture runs twice: once buffered and once streamed from a file. The two
paths must agree byte for byte, and a failure fixture must fail with the same
exit code and message on both.

Write the template in `input.tmpl`; do not generate templates in Go test code.
The one exception is `repeat`: the parallel render path needs a template of
several 256 KiB windows, and `parallel/parallel_window_split/` reaches it by
repeating a small template instead of committing a megabyte of text. Only a
success fixture may repeat, because a repeated failure would fail at many lines
and one error cannot describe it. `TestParallelFixtureSpansEnoughWindows` pins
that the fixture still spans enough windows to take the parallel path.
