English | [简体中文](README.zh-CN.md)

# envsub

**Extended configuration renderer.** A superset of GNU `envsubst` that adds typed
values, multi-file layering, JSON Schema validation, safe in-place editing and
CI-friendly staleness checks — while keeping the familiar `${VAR}` template
syntax.

```console
$ envsub render -f values.yaml app.tmpl
server:
  host: 10.0.0.1
  port: 8080
  tags: web, prod
```

> **Migrating from `envsubst`?** A bare `${NAME}` resolves against envsub's
> **value tree** (values files, `.env` files, and `--set`), not the shell
> environment. The process environment is a separate, read-only namespace
> accessed through `${@process.NAME}`. See
> [Process environment](#process-environment).

## Why we recommend envsub

`envsubst` is appropriate for single-layer `${VAR}` substitution. envsub is
intended for configurations that require typed values, multiple sources, schema
validation, and atomic write-back:

| | Classic `${VAR}` substitution | envsub |
|---|---|---|
| Values | strings | typed: int, float, bool, list, map, null |
| Sources | the process environment | env, `.env` files, YAML/JSON files, `--set`, JSON Schema defaults |
| Layering | one source | deterministic precedence, later files win |
| Validation | — | JSON Schema Draft 2020-12, with a friendly error location |
| Writing back | — | atomic in-place replacement, batch output, staleness check |
| Formatting | — | `toJson`, `toYaml`, `join`, `quote`, `indent` |
| Missing value | empty string | configurable: error, empty, keep, or zero |

## Install

```bash
# From a release (linux/darwin/windows × amd64/arm64)
go install github.com/elizabevil/envsub/cmd/envsub@latest

# Or from source
git clone https://github.com/elizabevil/envsub && cd envsub
just build        # → dist/envsub
```

Building requires Go 1.27. Released binaries are statically linked and have no runtime dependencies.

## Quick start

Create a template and a values file:

```bash
cat > app.tmpl <<'EOF'
server:
  host: ${HOST}
  port: ${PORT}
  tags: ${TAGS | join ", "}
EOF

cat > values.yaml <<'EOF'
HOST: 10.0.0.1
PORT: 8080
TAGS: [web, prod]
EOF
```

Render the template:

```console
$ envsub render -f values.yaml app.tmpl
server:
  host: 10.0.0.1
  port: 8080
  tags: web, prod
```

`PORT` is rendered as the integer `8080` without quoting or explicit coercion.
To force a string, use `${PORT | type string}`.

## Complete example

The repository includes a runnable dataset and templates:

```text
examples/
├── data/
│   ├── values-base.yaml
│   ├── values-prod.yaml
│   ├── values-extra.json
│   ├── app.env
│   └── schema.yaml
├── templates/
│   ├── realworld.tmpl
│   └── scenarios-*.tmpl
├── out/                 # checked-in expected output
└── justfile             # one recipe per scenario
```

```bash
ENVSUB_STRING=from-process envsub render \
  -f examples/data/values-base.yaml \
  -f examples/data/values-prod.yaml \
  -f examples/data/values-extra.json \
  --dotenv-file examples/data/app.env \
  --schema examples/data/schema.yaml \
  --on-missing=empty \
  examples/templates/realworld.tmpl
```

`>` means the left side has higher priority than the right side:

```text
--dotenv-file examples/data/app.env > -f examples/data/values-extra.json > -f examples/data/values-prod.yaml > -f examples/data/values-base.yaml > schema default
```

`ENVSUB_STRING` belongs to the process environment (`@process`) and is not part
of this precedence order.

Output:

```yaml
service:
  name: prod-string
  replicas: 7
  enabled: true
  region: prod
  endpoints: ["prod","a","c"]
  labels: {"a":2,"m":3,"z":9}
  started_by: from-process
```

[`examples/README.md`](examples/README.md) maps every recipe, golden and failure
case.

## Template syntax

```text
$NAME ${NAME}                     variable reference
${app.port}                       nested path
${app["my-key"]}                  key with a dash
${hosts[0]}                       array element
${["a.b"].k}                      top-level key with a dot
${@process.PORT}                  process environment (read-only)

${path:-default}                  default if unset or empty
${path-default}                   default if unset
${path:+alt}                      alternate if set and non-empty
${path+alt}                       alternate if set
${path:?message}                  fail with a message if unset or empty
${path?message}                   fail with a message if unset
${path:=default}                  assign the default into the value tree
${path=default}                   assign the default if unset

$$                                a literal $
$1                                stays literal

${hosts | join ", "}              formatter pipeline
${PORT | type int}                inline type assertion
${name | quote}                   quote a string
${app | toJson}                   JSON
${app | toYaml}                   YAML
${app | toYaml 4}                 YAML with a nesting width
${app | toYaml align}             YAML aligned to the expression's column
${app | toYaml 4 align}
${app | toJson | indent 2}        chained formatters
```

`toYaml` takes an optional nesting width and/or `align`; `toJson` takes
`align`. `align` makes every line of the result continue at the column the
expression starts on, which is what lets a block sit under a parent key.
Without it a multi-line value is inserted verbatim, so its later lines keep
their own position.

An inline `| type` assertion is combined with the schema: a value that
satisfies either is accepted, a value that satisfies neither is an error.
When a type is given it must be the first step of the pipeline.

## Usage

### Render command template

```text
envsub render [flags] [template...]
```

`[template...]` accepts files, directories, or `-` for stdin. When it is empty,
stdin is read.

A single-template example:

```bash
envsub render -f values/base.yaml app.yaml.tmpl
```

Write to a file:

```bash
envsub render -f values/base.yaml -o rendered/app.yaml app.yaml.tmpl
```

### Parameters

Parameters are grouped by purpose; value sources and precedence are defined
below.

| Group | Flags |
|---|---|
| Value sources | `-f/--values-file`, `--dotenv-file`, `--schema`, `--set`, `--set-json`, `--set-file` |
| Output | `-o/--output`, `--output-dir`, `--strip-suffix`, `--recursive` |
| Write-back and check | `-i/--in-place`, `--check` |
| Policies | `--on-missing`, `--on-empty`, `--only-vars`, `--ignore-vars`, `--unknown-keywords` |
| Reports | `--explain`, `--report`, `--jsonl`, `--error-format` |

Run `envsub render --help` for the complete flag list.

### Value sources

| Flag | Input | Processing |
|---|---|---|
| `-f/--values-file FILE` | YAML or JSON value file | Merged into the root; repeatable, later > earlier |
| `--dotenv-file FILE` | dotenv file | Loaded into the root; repeatable, later > earlier; > `-f/--values-file` |
| `--schema FILE` | JSON Schema | Fills defaults, materializes declared types, validates the final tree |
| `--set KEY=VALUE` | Raw string | Stored as a string; schema or `\| type` may convert it |
| `--set-json KEY=JSON` | JSON literal | Parsed as JSON: numbers, booleans, arrays, objects, null |
| `--set-file KEY=FILE` | File content | Stored as a string; BOM and one trailing newline are removed |

`--set`, `--set-json` and `--set-file` all write to the root namespace. A dotted
key creates nested objects. A repeated key within the same flag uses the later value, and the
three flags apply in this fixed order regardless of their order on the command
line:

```text
--set  →  --set-json  →  --set-file
```

If the same key is supplied through all three, the `--set-file` value takes
precedence.

Examples:

```bash
# A plain string override
envsub render --set image.tag=2026.10 app.yaml.tmpl

# A typed JSON value: array
envsub render --set-json 'app.hosts=["a","b"]' app.yaml.tmpl

# A multi-line secret from a file
envsub render --set-file tls.crt=secrets/tls.crt app.yaml.tmpl
```

`--set-json` rejects invalid JSON as a type error (exit 4). `--set-file` reports
an unreadable file as an IO error (exit 5); it preserves inner newlines and only
drops one trailing newline and a leading BOM.

### Value priority

`>` means the left side has higher priority than the right side:

```text
--set-file > --set-json > --set > --dotenv-file (later > earlier) > -f/--values-file (later > earlier) > schema default
```

Priority example: `--set image.tag=2026.10` > `values/prod.yaml (2026.09)` > `values/base.yaml (canary)` > `schema default`.

`@process.*` is a separate read-only namespace and does not participate in this
precedence order. `--only-vars` and `--ignore-vars` decide whether a reference
is substituted; they do not change source priority.

### Process environment

`${@process.NAME}` reads one process environment variable. A bare `${NAME}`
never reads the process environment; it reads the value tree.

| Flag | Effect |
|---|---|
| `--no-process-env` | Leaves the `@process` namespace empty |
| `--process-env-prefix PREFIX` | Exposes only process variables whose names start with `PREFIX`; names are kept as-is and the prefix is not stripped |

`--process-env-prefix` only filters the `@process` namespace. It does not read,
rename, or filter `.env` files or the value tree. An empty prefix keeps every
process variable. `--no-process-env` and `--process-env-prefix` are mutually
exclusive.

### Single template

Without output options, the rendered template is written to stdout:

```bash
envsub render -f values/base.yaml app.yaml.tmpl
```

`-o/--output FILE` writes one rendered file:

```bash
envsub render -f values/base.yaml -o rendered/app.yaml app.yaml.tmpl
```

Read the template from stdin with `-`:

```bash
envsub render -f values/base.yaml - < app.yaml.tmpl
```

### Batch rendering

Batch output is a separate mode. It writes a directory tree and does not use
`-o` or `-i`:

```bash
envsub render \
  -f values/base.yaml \
  --output-dir rendered \
  --strip-suffix .tmpl \
  --recursive templates/
```

| Flag | Meaning |
|---|---|
| `--output-dir DIR` | Write one output file per template under `DIR` |
| `--recursive` | Walk template directories recursively |
| `--strip-suffix EXT` | Remove `EXT` from output names; requires `--output-dir` |

The input layout is mirrored under `--output-dir`. Batch rendering and
`-o/--output` are mutually exclusive, as are batch rendering and `-i/--in-place`.

### Write-back and check

`-i/--in-place` replaces each input file atomically: the new content is written
to a temporary file and renamed over the target, so an interrupted run never
leaves a half-written file. Permissions are preserved.

```bash
envsub render -i -f values/base.yaml app.yaml
```

`--check` renders in memory and compares the result with the existing output; it
writes nothing. It requires an output target and must be combined with `-o`, `--output-dir`, or
`-i`. It exits 7 when at least one output is stale.

```bash
envsub render --check -f values/base.yaml -o rendered/app.yaml app.yaml.tmpl
```

### Validation, missing values and exit codes

`--schema` fills defaults, materializes declared types, and validates the final
tree. `--validate-only` performs the same load and validation without rendering.

`--on-missing` controls a missing value:

| Value | Effect |
|---|---|
| `error` | Fail (default) |
| `empty` | Render an empty string |
| `keep` | Leave the reference as written |
| `zero` | Render the type's zero value; needs a schema or inline type |

`--on-empty=error` fails when a plain reference resolves to an empty value;
`0` and `false` are not empty. The default is `ok`.

| Exit | Meaning |
|---|---|
| 0 | Success |
| 2 | Usage or syntax error |
| 3 | Template error |
| 4 | Type or schema validation error |
| 5 | IO error |
| 6 | Missing or empty value policy error |
| 7 | Stale output with `--check` |

### Reports

`--explain[=PATH]` writes one JSON line per referenced variable to stderr,
naming the winning origin and the layers it shadowed. The value itself is not emitted.

```bash
envsub render --explain -f values/base.yaml app.yaml.tmpl
```

`--report=unused` reports variables defined in the value tree but not referenced
by the template. `--jsonl` emits the unified event stream for CI; it is enabled
automatically for `--explain` and `--report`. `--error-format=json` renders
errors as JSON.

See [`docs/07-serialization/04-cli.md`](docs/07-serialization/04-cli.md) for the
event envelope and the complete flag reference.

## Documentation

- [`examples/README.md`](examples/README.md) — runnable dataset, recipes, goldens
- [`docs/migration.md`](docs/migration.md) — migration from GNU `envsubst`
- [`docs/07-serialization/04-cli.md`](docs/07-serialization/04-cli.md) — CLI and protocol reference
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — build, test, lint and development notes

## License

MIT. See [`LICENSE`](LICENSE).
