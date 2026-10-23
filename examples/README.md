# Examples

Runnable, self-contained cases that show the whole product: a shared dataset, one
template per rendering view, and one recipe per question you might ask.

Requires `just` **1.58.0 or newer** (CI installs at least this version; the root
justfile's `test-examples` refuses to run on an older one).

```bash
cd examples
just               # list the recipes
just data          # what the shared dataset contains
just scenarios-json
just demo          # run everything
```

## How to verify an example

Three layers, cheapest first. Run them after any change to `data/`, `templates/` or `cases/`.

```bash
# 1. Guards: structure + render every template against its golden (needs no just)
go test ./test/examples/ -v

# 2. Recipes: render one scenario and show the trees/diffs a human can read
just --justfile examples/justfile scenarios-json
just --justfile examples/justfile scenarios-yaml
just --justfile examples/justfile precedence
just --justfile examples/justfile realworld
just --justfile examples/justfile failures      # one case per exit code
just --justfile examples/justfile unused        # --report=unused
just --justfile examples/justfile jsonl         # --explain --jsonl, one typed JSON per stderr line
just --justfile examples/justfile auxiliary-check   # compare out/ with --check (exit 7 = outdated)
just --justfile examples/justfile acceptance        # whole out/ tree, file by file
just --justfile examples/justfile demo              # every recipe in sequence

# 3. Only after an intended change: rewrite the expectations, then review the diff
just --justfile examples/justfile digest-update
git diff examples/
go test ./test/examples/ && just --justfile examples/justfile demo
```

What each guard checks (they fail with a readable diff, not a silent pass):

| Guard | Checks |
|-------|--------|
| `TestDatasetNamesFollowTheConvention` | every key is `<type>_<source>_<effect>` with a known word |
| `TestDatasetCoversEveryDimension` | each type × each source has at least one key |
| `TestRuleFamiliesAreMapped` | every rule family (O1–O9, P1–P8, …) names where it is exercised |
| `TestExamplesCoverEveryRegisteredFlag` | every registered CLI flag is used somewhere |
| `TestTemplateFormattersAreExercised` | all six formatters appear in a template |
| `TestScenariosParseAsTheirDeclaredFormat` | `<name>.parse` agrees with the rendered output |
| `TestExampleTemplatesMatchTheirGolden` | renders every template and compares byte-for-byte with `<name>.golden`, then parses it |
| `TestExampleUnusedReportMatchesItsExpectation` | `--report=unused` stdout and stderr |
| `TestExampleFailureCasesMatchTheirExitCodes` | `cases/errors/*`: expected exit code, stderr fragment, empty stdout |

The `scenarios-*`, `precedence` and `realworld` recipes do all three steps themselves:
render → parse the output with `tools/digest` (using that template's `.parse` marker) →
`diff` against the golden.

To see where a golden's values actually came from, without re-deriving them by hand:

```bash
just --justfile examples/justfile explain-sources
```

It renders with `--explain` and prints the winning layer per key:

```text
30 values referenced

[envfile]
  from examples/data/app.env
    string_prec_multi_layer   <- envfile/file examples/data/app.env:4   (covered: values/file examples/data/values-prod.yaml | values/file examples/data/values-base.yaml | schema/default #/properties/string_prec_multi_layer/default)

[values]
  from examples/data/values-prod.yaml
    string_type_root   <- values/file examples/data/values-prod.yaml   (covered: values/file examples/data/values-base.yaml)
```

Each line reads `variable <- kind/method address (covered: the layers it shadowed)`.
The chain is the model in
[ADR-005](../docs/04-decisions/04-05-adr-005-explain-origin-model.md): `kind` is the layering
stage, `method` is how the value entered it, and the address is a file, a schema pointer, an
environment variable name or a key.

`examples/justfile` is **not** part of `just check` (it builds the binary first); the guards in
step 1 are, so a broken example is caught by `just check` even without running the recipes.

## Glossary: what each kind of file means

| Term | Meaning |
|------|---------|
| **golden** | The **final expected output** of a template, checked in and compared **byte for byte**. A golden is not a sample: it is the contract. `digest-update` rewrites it, and `git diff` is how you review that rewrite. |
| `.parse` | The format the golden claims to be (`json` or `yaml`). **The single source of truth for that claim**: `test/examples` and the recipes read this same file, so a real parser proves it on both sides. |
| `.unused` | The expected typed JSONL `--report=unused` output for that template (the report goes to stderr). |
| `expect.out` | Expected stdout of a **failure case** (`cases/errors/`), byte for byte. |
| `expect.err` | A fragment that **must appear** in stderr (substring, not the whole text: messages may carry paths). |
| `expect.exit` | The exit code that case must produce. |
| `expect.dir` | Expected tree of an output directory, compared file by file. |
| `args` | In `test/e2e`, the argv of one case (e2e only; `examples/` uses recipes instead). |
| `.tmpl` | A template containing `${...}` placeholders; it is the **input**. |
| dataset | `data/`, which holds values only and never a placeholder; it is what the flags read. |

Rule of thumb: **`data/` is input, `templates/*.tmpl` is what runs, `*.golden` is what must come out.**

## Every file, and who reads it

| Path | What it holds | Read by |
|------|---------------|---------|
| `data/values-*.{yaml,json}` | layered values (`-f`, in order) | every recipe via `{{ sources }}` |
| `data/app.env`, `data/extra.env` | `.env` layer, and a second one that must win | `--dotenv-file` |
| `data/schema.yaml` | JSON Schema: defaults, materialization, validation | `--schema` |
| `data/schema-invalid-data.yaml`, `data/invalid-syntax.yaml` | inputs that must fail | failure cases |
| `secrets/tls.key` | the value for `--set-file` | the `sets` group |
| `templates/*.tmpl` | templates with placeholders | the matching recipe |
| `templates/*.golden` | exact expected stdout for that template | `diff -u` in the recipe, and `test/examples` |
| `templates/*.parse` | `json`/`yaml` the golden must parse as (**single source**: recipes read it, `test/examples` enforces it) | `tools/digest --parse` and the guards |
| `templates/unused.unused` | expected `--report=unused` report | `test/examples` |
| `templates/batch/*.tmpl` | inputs for `--output-dir` | `auxiliary-batch`（保留文件名）、`auxiliary-strip-suffix`（`--strip-suffix .tmpl` 去掉后缀） |
| `templates/stdin.tmpl` | template delivered on stdin | `auxiliary-stdin` |

`--strip-suffix EXT`（可重复、默认关）只在配合 `--output-dir` 时生效：它从每个输出的文件名去掉匹配的后缀，最长匹配优先；两个模板剥成同名时报用法错误而不是互相覆盖。见 `auxiliary-strip-suffix` 与 `docs/07-serialization/04-cli.md` C18。
| `cases/errors/<name>/input.tmpl` | the failing template | `failures` recipe, `test/examples` |
| `cases/errors/<name>/expect.exit` | required exit code | same |
| `cases/errors/<name>/expect.err` | required stderr fragment | same |
| `cases/errors/<name>/args` | optional extra flags for that case, one per line (`${CASE_DIR}` expands to the case directory) | same |
| `cases/errors/<name>/values.yaml`, `current.txt` | extra inputs that case references | same |
| `out/**` | the checked-in output tree | `acceptance` (`--expect out --got .tmp/...`) |
| `tools/digest/` | tree compare + format parse, cross-platform | the recipes (never `diff`/`cmp`/`wc` for trees) |
| `justfile` | the recipes; flags are defined once at the top | you, and `just check` |

Per-directory READMEs carry the details: [`data/`](data/README.md), [`templates/`](templates/README.md),
[`cases/errors/`](cases/errors/README.md).

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
| `jsonl` | the typed JSONL stderr stream (`type=explain/error/log/unused`) |
| `auxiliary-validate` | the same dataset with `--validate-only` (nothing written) |
| `auxiliary-check` | the same dataset with `--check` (exit 7 when outdated) |
| `auxiliary-explain` | where every value came from |
| `auxiliary-stdin` | stdin read exactly once with two reports active |
| `auxiliary-error-json` | the machine-readable error shape |
| `failures` | one case per exit code: 6, 3, 4, 4, 5, 2, 7 (outdated) and 7 (missing target) |
| `acceptance` | the whole tree rendered and compared file by file |
| `digest-update` | regenerate `templates/*.golden` and `out/` after an intended change |
| `explain-sources` | where every rendered value came from (winning layer, file and line) |

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
| I1–I7 `--explain` 帧不变式（kind/method 闭集、地址、链序、被覆盖的 default） | `just explain-sources`（`string_prec_multi_layer` 的四层链）；`test/e2e` 的 `TestExplainFramesFollowADR005`；`schema_default_is_fallback` |
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
