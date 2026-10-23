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

> **Coming from `envsubst`?** A bare `${NAME}` reads envsub's **value tree** —
> values files, `.env` files and `--set` — not your shell environment. The
> process environment is a separate, read-only namespace you reach with
> `${@process.NAME}`. That is the one thing to know before you start; see
> [Where values come from](#where-values-come-from).

## Why not just `envsubst`?

`envsubst` substitutes strings. Real configuration needs more:

| | GNU `envsubst` | envsub |
|---|---|---|
| Values | strings only | typed: int, float, bool, list, map, null |
| Sources | process environment | env, `.env` files, YAML/JSON files, `--set`, JSON Schema defaults |
| Layering | one source | deterministic precedence, later files win |
| Validation | none | JSON Schema Draft 2020-12, with a friendly error location |
| Writing back | none | atomic in-place replacement, batch output, staleness check |
| Formatting | none | `toJson`, `toYaml`, `join`, `quote`, `indent` |
| Missing value | empty string | error, empty, keep, or zero — you choose |

## Install

```bash
# From a release (linux/darwin/windows × amd64/arm64)
go install github.com/elizabevil/envsub/cmd/envsub@latest

# Or from source
git clone https://github.com/elizabevil/envsub && cd envsub
just build        # → dist/envsub
```

Requires Go 1.27 to build. The released binaries are static and need nothing else.

## Quick start

Create a template and some values:

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

Render it:

```console
$ envsub render -f values.yaml app.tmpl
server:
  host: 10.0.0.1
  port: 8080
  tags: web, prod
```

`PORT` stays `8080` as an integer — no quoting, no type coercion. To make it a
string, say so: `${PORT | type string}`.

## Use cases

### Substitute environment variables

The process environment is reached explicitly, through the `@process`
namespace:

```bash
cat > config.tmpl <<'EOF'
host: ${@process.HOST}
EOF

HOST=10.0.0.1 envsub render config.tmpl
# host: 10.0.0.1
```

Or expose a whole prefix at once — the variables keep their original names,
the prefix is only a gate:

```bash
cat > prefixed.tmpl <<'EOF'
host: ${@process.APP_HOST}
EOF

APP_HOST=10.0.0.1 envsub render --process-env-prefix APP_ prefixed.tmpl
# host: 10.0.0.1
```

Templates and their output go wherever you point them:

```bash
envsub render config.tmpl                 # template on stdout
envsub render -o out.yaml config.tmpl     # …or to a file
cat config.tmpl | envsub render -         # …or from stdin
```

### Read typed values from a file

```bash
cat > deploy-values.yaml <<'EOF'
replicas: 3
enabled: true
limits: {cpu: 500m, memory: 1Gi}
EOF

cat > deploy.tmpl <<'EOF'
replicas: ${replicas}
enabled: ${enabled}
limits: ${limits | toJson}
EOF

$ envsub render -f deploy-values.yaml deploy.tmpl
replicas: 3
enabled: true
limits: {"cpu":"500m","memory":"1Gi"}
```

### Layer several sources

Later sources win. The full order is
`--set` > `.env` files > value files > schema `default`.

```bash
envsub render \
  -f values.yaml \
  --dotenv-file .env \
  --set PORT=9090 \
  app.tmpl
```

### Validate against a JSON Schema

```bash
cat > schema.yaml <<'EOF'
type: object
properties:
  port: {type: integer, minimum: 1}
EOF

cat > port.tmpl <<'EOF'
port: ${port}
EOF

envsub render --schema schema.yaml -f good-values.yaml port.tmpl
```

A value that violates the schema fails with the path that broke it and exit
code 4:

```bash
cat > bad-values.yaml <<'EOF'
port: 0
EOF
```

```console
$ envsub render --schema schema.yaml -f bad-values.yaml port.tmpl
schema validation failed: at /port: minimum: got 0, want 1
$ echo $?
4
```

### Decide what a missing value does

```bash
envsub render --on-missing=error   app.tmpl   # fail (default)
envsub render --on-missing=empty   app.tmpl   # render as empty
envsub render --on-missing=keep    app.tmpl   # leave ${VAR} as written
envsub render --on-missing=zero    app.tmpl   # the type's zero value
```

`keep` is what you want when a template mixes required and optional values.
`zero` needs to know the type — give it one with a schema or an inline
`| type`, otherwise it cannot tell what "zero" means:

```bash
envsub render --on-missing=zero app.tmpl                  # error: no type
echo 'port: ${PORT | type int}' | envsub render --on-missing=zero -
# port: 0
```

### Render a whole directory

```bash
envsub render -f values.yaml --output-dir rendered/ --recursive templates/
```

The directory layout is mirrored. Add `--strip-suffix .tmpl` to drop the
template suffix from the output names.

### Edit a file in place, safely

```bash
envsub render --in-place config.yaml
```

The file is replaced atomically: the new content is written to a temporary file
and renamed over the original, so an interrupted run never leaves a half-written
file. Permissions are preserved.

### Format a value

```bash
envsub render -f values.yaml -o - <<'EOF'
${config | toYaml}
${config | toJson}
${hosts | join ", "}
${name | quote}
${config | toYaml 4}
EOF
```

### Check in CI whether a generated file is stale

`--check` compares the target against what the current values would produce
and writes nothing. It needs to know the target, so pair it with the output
option that names it:

```bash
cat > check.tmpl <<'EOF'
port: ${port}
EOF

cat > check-values.yaml <<'EOF'
port: 8080
EOF

envsub render -f check-values.yaml -o config.yaml check.tmpl   # generate it
envsub render --check -f check-values.yaml -o config.yaml check.tmpl
```

The command exits 7 when the target differs, which is what you want in a
pipeline:

```bash
envsub render --check -f check-values.yaml -o config.yaml check.tmpl || exit 1
```

```bash
cat > other-values.yaml <<'EOF'
port: 9090
EOF
```

```console
$ envsub render --check -f other-values.yaml -o config.yaml check.tmpl
1 of 1 outputs are outdated: config.yaml
$ echo $?
7
```

### See where a value came from

```bash
envsub render --explain --no-process-env \
  -f values.yaml --dotenv-file .env --set PORT=9090 app.tmpl
```

Each referenced variable gets a JSON line on stderr naming the layer that won
and the layers it shadowed — never the value itself, so it is safe to paste into
a ticket.

```console
{"template":"app.tmpl","variable":"PORT","origin":{"kind":"set","method":"flag","source":"PORT","file":"","line":0},"shadowed":[{"kind":"envfile","method":"file","source":".env","file":".env","line":1}]}
```

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

## Where values come from

```text
--set / --set-json / --set-file
  >  .env files (--dotenv-file)
  >  YAML/JSON value files (-f/--values-file)
  >  schema default
```

Later files win within each kind, and a schema `default` only fills a key no
source provides.

The process environment is a **separate, read-only namespace**. It never fills
a bare name — reaching it is always explicit:

```bash
envsub render --no-process-env app.tmpl          # ignore it entirely
envsub render --process-env-prefix APP_ app.tmpl # expose only APP_* variables
```

`.env` files are always loaded as dotenv, whatever the file is called, and
their keys land in the root namespace.

## CLI reference

```text
envsub render [flags] [template...]
envsub version                                      build information as JSON
envsub completion bash|zsh|fish|powershell         shell completion script

value sources
  -f, --values-file FILE      YAML/JSON value files, repeatable
      --dotenv-file FILE      dotenv files (parsed as dotenv, any name), repeatable
      --schema FILE           JSON Schema (Draft 2020-12), JSON or YAML
      --set KEY=VALUE         raw string value (typed by the schema or an inline | type)
      --set-json KEY=JSON      native JSON value
      --set-file KEY=FILE      value read from a file, kept as a string
      --no-process-env         ignore the process environment
      --process-env-prefix P   expose only process variables with this prefix

rendering
  -o, --output FILE           single output file
  -i, --in-place              atomic replace of each template
      --output-dir DIR        batch output directory (mirrors the input layout)
      --strip-suffix EXT      drop this suffix from batch output names, repeatable
      --recursive             walk template directories recursively
      --validate-only         load, materialize and validate; write nothing
      --check                 report targets that are out of date; write nothing
      --explain[=PATH]        JSON source report on stderr (PATH limits it, repeatable)
      --report=unused         list value-file entries no template referenced, on stderr

policies
      --on-missing=error|empty|keep|zero    default: error
      --on-empty=ok|error                   default: ok
      --unknown-keywords=ignore|warn|error  default: ignore
      --only-vars VAR...                    only substitute these variables
      --ignore-vars VAR...                  never substitute these variables

global
      --log-level=debug|info|warn|error     default: warn
      --error-format=text|json              default: text
```

`--error-format=json` makes stderr a single JSON object per failure, with
`kind`, `exitCode` and `message` — handy for tooling.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | internal or unclassified error |
| 2 | usage error |
| 3 | template syntax error |
| 4 | type or schema validation error |
| 5 | file or I/O error |
| 6 | missing value or value policy error |
| 7 | target is out of date (`--check`) |

Every usage problem is a 2, including a mistyped subcommand:

```console
$ envsub rendr
unknown command "rendr" for "envsub"

Did you mean this?
	render
$ echo $?
2
```

A 1 is reserved for the tool's own defects: an internal error, or an error that
carries no classification at all.

## Development

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

### JSON backend

JSON goes through `internal/jsonx`, which has two build-time implementations:

| | default (stdlib `encoding/json/v2`) | `-tags=sonic` |
|---|---|---|
| linux/amd64 binary | 5.03 MiB | 8.55 MiB |
| startup | 4.6 ms | 9.5 ms |
| REF-04 peak RSS | 1.30× GNU | 1.80–2.25× GNU |
| JSON decode (364 KB) | 12.04 ms | 2.45 ms |

The default keeps the binary 41% smaller with better startup and memory; use
`-tags=sonic` when JSON throughput matters more than size. The binary sizes
are measured with the Go release named in `go.mod`; a newer Go release shifts
them by a few hundred KiB, so compare within one toolchain.

YAML goes through `internal/yamlx` and uses one library,
`go.yaml.in/yaml/v3`: decoding walks nodes (integers stay integers, duplicate
keys and merge keys are handled) and `toYaml` encodes the same tree.

## Documentation

Coming from GNU `envsubst`? Read the
[migration guide](docs/migration.md) first — the one thing to know is that a
bare `${NAME}` reads envsub's value tree, not your shell environment.

The normative behaviour lives in
[`docs/07-serialization/`](docs/07-serialization/00-index.md) — every rule has
an ID, a worked example and a test. Start at
[`docs/README.md`](docs/README.md) for the full map; superseded material is in
[`docs/99-archive/`](docs/99-archive/README.md).

`examples/` holds runnable cases for every rule family:

```bash
cd examples
just            # list the cases
just demo       # run all of them
just full       # every type, source, operator and formatter
```

## License

MIT — see [LICENSE](LICENSE).
