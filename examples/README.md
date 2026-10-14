# Examples

Runnable, self-contained cases that show the whole product: a shared dataset, one
template per rendering view, and one recipe per question you might ask.

```bash
cd examples
just               # list the recipes
just data          # what the shared dataset contains
just scenarios-json
just demo          # run everything
```

## Layout

| Path | Holds | Why it is separate |
|------|-------|--------------------|
| `data/` | values only: `values-*.yaml`, `*.json`, `.env`, `schema.yaml` | **shared** by every recipe (the "same dataset, different command" requirement) |
| `secrets/` | inputs for `--set-file` | a value that must not appear on the command line |
| `templates/` | `*.tmpl` with placeholders, plus each template's `*.golden` | reused by more than one recipe |
| `cases/errors/` | one directory per failure path: `input.tmpl`, `expect.exit`, `expect.err` | self-contained cases |
| `out/` | the checked-in output tree | compared file by file by `just acceptance` |
| `tools/digest/` | the comparison/parse helper the recipes call | keeps the checks identical on every platform |

`data/` never contains `${...}`; `templates/` and `cases/*/input.tmpl` always do. That is the
whole difference: data is what the flags read, templates are what the command renders.

## Recipes

| Recipe | Answers |
|--------|---------|
| `data` | which keys the dataset has, and what the naming means |
| `scenarios-json` | all types/sources/positions/operators/pipelines, rendered as **valid JSON** |
| `scenarios-yaml` | the same dataset rendered as **valid YAML** |
| `scenarios-without-process` | what changes when no process environment is exposed |
| `precedence` | which layer wins (`.env` > `-f`, `--set` > `.env`, `--set-json` > `--set`, default never wins) |
| `precedence-two-dotenv` | a later `--dotenv-file` wins |
| `realworld` | one realistic service configuration |
| `unused` | which dataset keys no template referenced |
| `auxiliary-validate` | the same dataset with `--validate-only` (nothing written) |
| `auxiliary-check` | the same dataset with `--check` (exit 7 when outdated) |
| `auxiliary-explain` | where every value came from |
| `auxiliary-stdin` | stdin read exactly once with two reports active |
| `auxiliary-error-json` | the machine-readable error shape |
| `failures` | one case per exit code: 6, 3, 4, 4, 5, 2 |
| `acceptance` | the whole tree rendered and compared file by file |
| `digest-update` | regenerate `templates/*.golden` and `out/` after an intended change |

## Rule families and where they are exercised

Rule ids come from [`docs/07-serialization/00-index.md`](../docs/07-serialization/00-index.md).
`test/examples` fails when a family defined there is not named here.

| Family | Exercised by |
|--------|--------------|
| C1–C12 CLI sources, precedence, policies | `precedence`, `precedence-two-dotenv`, `values-long-flag`, `filters`, `on-empty`, `unknown-keywords`, `failures` |
| N1–N5 integers (narrowest exact type, exact literals) | `data/values-extra.json` (`big_int_exact`), `scenarios-json` (`int_type_materialized`) |
| F1–F8 floats (IEEE, shortest round-trip, literal spelling, range, NaN/Inf) | `scenarios-json` (`float_type_literal_kept`), `data/values-extra.json` (`float_json_literal`) |
| FB1–FB3 float boundary fallbacks | `data/values-extra.json` (`float_json_literal`), `scenarios-json` |
| V1–V5 empty/null encoding, backend parity | `scenarios-yaml` (`null_type_preserved`), `scenarios-json` |
| D1–D3 deterministic key order, no trailing newline, YAML style | `scenarios-json` (`object_type_yaml_sorted`), `scenarios-yaml` (`object_type_yaml_sorted`) |
| Q1–Q7 quoting, HTML, empty string, literal spelling | `scenarios-json` (`quoted_scalar`, `escape_dollar`) |
| L1 nesting depth | library-level; `go test ./test/encoding/` |
| T1–T9 type and backend internals | library-level; `go test ./test/jsonx/`, `go test ./test/yamlx/` |
| O1–O9 operators (`:-` `-` `:+` `+` `:?` `?` `:=` `=`) | `scenarios-yaml` / `scenarios-json` (`operators` region) |
| OV1–OV9 existence vs emptiness (`0`/`false` not empty, write-back) | same region (`alternate_plus`, `assign_visible`, `default_no_write*`) |
| P1–P6 formatters (`join`, `toJson`, `toYaml`, `quote`, `indent`, `type`) | both scenarios templates (`pipeline` region) |
| P7–P8 serializer parameters (`toYaml N`, `toYaml align`, `toJson align`) | `scenarios-yaml` (`types.object_nested_block`, `object_nested_block_width`), `just failures` |
| TY1–TY8 inline `\| type` (`array<T>`, `null`, unsupported names) | both scenarios templates (`types`, `pipeline` regions) |
| SC1–SC9 schema keywords, aliases, defaults, `$ref` limits | `data/schema.yaml`, `unknown-keywords`, `failures` (`schema`) |
| R1–R5 round-trip and consistency rules | `scenarios-json` + `scenarios-yaml` (same dataset, two encodings), `go test ./test/encoding/` |
| G1–G22 known gaps | each gap names its own test in `docs/07-serialization/09-gaps.md`; `failures` covers the exit codes |
| `@process` namespace and layering | `scenarios-with-prefix`, `scenarios-no-process`, `scenarios-without-process` |
| Top-level special keys (`["a.b"]`) | library-level; `go test ./internal/values/` |
| `--only-vars` / `--ignore-vars` | `filters` |

## Multi-line values and indentation

| Written | Result |
|---------|--------|
| `${x \| toYaml}` | nesting width 2, inserted **verbatim** |
| `${x \| toYaml 4}` | nesting width 4 |
| `${x \| toYaml align}` | every line continues at the expression's column |
| `${x \| toYaml 4 align}` | both, in any order |
| `${x \| toYaml \| indent 2}` | the other way to place a block: `indent` prefixes every line |

`align` and `indent` are mutually exclusive (both define a column) → exit 2.

## Variable naming

Keys are `<type>_<source>_<effect>`, so a key name says what it proves:

| Segment | Values |
|---------|--------|
| type | `string` `int` `float` `bool` `array` `object` `null` |
| source | `type` (type behaviour), `src` (which source provides it), `prec` (which layer wins), `process` (`@process`), `schema` (materialization/constraints), `fmt` (formatter) |
| effect | `root` `default` `materialized` `kept` `sorted` `joined` `explicit` `miss` `wins` `over_values` `over_dotenv` `from_string` `enum_ok` `min_items` … |

Examples: `string_type_root`, `string_prec_dotenv_over_values`, `int_process_materialized`,
`bool_schema_from_string`, `float_type_literal_kept`, `array_type_json_joined`.

`test/examples` fails when a key does not follow this pattern or when a type or source is missing
from the dataset.

## Golden files and parsing

Each template has its expectation next to it:

| File | Meaning |
|------|---------|
| `templates/<name>.golden` | exact expected stdout |
| `templates/<name>.parse` | `json` or `yaml`: the output must parse as that format |
| `templates/unused.unused` | expected `--report=unused` output |

`just acceptance` renders the tree into `.tmp/examples/out` and diffs it against `out/` with
`tools/digest`, so the check is byte-exact and works on any platform (no `diff`/`cmp`/`wc`).

## Two notes about the current behaviour

- **Multi-line values nest by column**: a substituted value that spans several lines continues at
  the indentation of the line the expression is on, so `toYaml` output can sit under a parent key
  (see `templates/scenarios-yaml.tmpl`, `types.object_nested_block`). At column 0 nothing is added.
  `toJson` is single-line and needs no alignment.
- **`--report=unused` lists many keys**: the dataset is deliberately larger than any single
  template (it has to feed every recipe), so an unused report of a dozen keys is expected, not a
  defect.
