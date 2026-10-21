# envsub

Extended configuration renderer.

> P0 (TASK-001 ~ TASK-021) is implemented and covered by unit tests, a 160-case
> fixture matrix and 18 CLI end-to-end cases (180 test functions, 9 benchmarks,
> 19 packages green under `-race`). See
> `docs/06-review/06-03-p0-implementation-round2.md`; the large-template
> streaming/parallel path and its fixes are in
> `docs/06-review/06-04-optimization-round3.md`; the JSON/YAML codec layer is in
> ADR-004 §8-9 and `docs/06-review/06-06-json-number-precision.md`.
> Module path: `github.com/elizabevil/envsub`.

## Requirements

- Go 1.27.1
- `just` for shortcut commands
- `golangci-lint` v2 (validated with v2.13.2) and `govulncheck` for checks

## Quick start

```bash
just build
./dist/envsub version

# render a template from the environment (template on stdin)
echo 'service: ${NAME}' | NAME=world ./dist/envsub render -

# the complete example: values files, .env and a JSON Schema
./dist/envsub render \
  -f examples/full/values.yaml -f examples/full/values-prod.yaml \
  -f examples/full/values-extra.json --dotenv-file examples/full/.env \
  --schema examples/full/schema.yaml examples/full/config.tmpl

# safe in-place rewrite (atomic replace, permissions preserved)
./dist/envsub render -i --on-missing=error config.tmpl
```

`examples/justfile` turns the examples in that directory into runnable cases:

```bash
cd examples
just            # list the cases
just demo       # run all of them
just full       # the complete example: all types, sources, operators, formatters
```

`examples/full/` is the complete example; `examples/README.md` maps every value
type, source, operator, formatter and render setting to a file or a case.

## CLI

```text
envsub render [flags] [template...]

value sources
  -f, --values-file FILE...   YAML/JSON values files, repeatable
      --dotenv-file FILE...   dotenv files (parsed as dotenv, any name), repeatable
      --schema FILE           JSON Schema (Draft 2020-12), JSON or YAML
      --set KEY=VALUE         raw string override (typed by schema or | type)
      --set-json KEY=JSON     native JSON value
      --no-process-env        ignore the process environment (dotenv files still load)
      --process-env-prefix P  expose only process variables with this prefix (dotenv files unaffected)

rendering
  -o, --output FILE           single output file
  -i, --in-place              atomic replace of each template
      --output-dir DIR        batch output directory (mirrors input layout)
      --recursive             walk template directories recursively
      --validate-only         load, materialize and validate, write nothing
      --explain[=PATH]        JSON source report on stderr (PATH limits it, repeatable)

policies
      --on-missing=error|empty|keep|zero    default: error
      --on-empty=ok|error                   default: ok
      --unknown-keywords=ignore|warn|error  default: ignore
      --only-vars VAR... / --ignore-vars VAR...
```

Exit codes: `0` success, `1` internal, `2` usage, `3` template syntax,
`4` type/schema, `5` IO, `6` missing value or value policy.

Value precedence (first match wins), all of it inside the root namespace:

```text
--set / --set-json  >  .env files (--dotenv-file)  >  YAML/JSON values files  >  schema default
```

Later files win within each kind, and a schema `default` only fills a key no source provides.
The process environment is a separate, read-only namespace: it never fills a bare name, and
reaching it is always explicit (`${@process.NAME}`). See
[docs/06-review/06-15](docs/06-review/06-15-namespace-and-layering.md).

## Template syntax

```text
$NAME ${app.port} ${app["my-key"]} ${hosts[0]} ${["a.b"].k} ${@process.PORT}
${path:-default} ${path-default} ${path:+alt} ${path+alt}
${path:?message} ${path?message} ${path:=default} ${path=default}
$$ -> $        $1 stays literal
${hosts | join ","} ${PORT | type int} ${name | quote}
${app | toJson} ${app | toYaml} ${app | toYaml 4} ${app | toYaml align}
${app | toYaml 4 align} ${app | toYaml | indent 2}
```

`toYaml` takes an optional nesting width and/or `align`; `toJson` takes `align`.
`align` makes every line of the result continue at the column the expression
starts on, which is what lets a block sit under a parent key. Without it a
multi-line value is inserted verbatim, so its later lines keep their own position.

## Development

```bash
just check      # gofmt check + vet + unit/fixture tests + e2e
just fmt
just lint       # golangci-lint v2, see .golangci.yml
just test
just test-race
just test-perf  # REF-04 performance gate, needs GNU envsubst (gettext-base)
just bench
just size       # binary size report (total + biggest sections)
just build-sonic  # same binary with the sonic JSON backend (faster, larger)
```

`just` points `GOCACHE`/`GOTMPDIR`/`GOLANGCI_LINT_CACHE` at a workspace-local
`.tmp/` directory because some shared toolchains mount the default caches
read-only. Override the variables in the environment to use different locations.

### JSON backend

JSON goes through `internal/jsonx`, which has two build-time implementations
(see `docs/04-decisions/04-04-adr-004-dependency-selection.md` §8):

| | default (stdlib `encoding/json/v2`) | `-tags=sonic` |
|---|---|---|
| linux/amd64 binary | 4.92 MiB | 8.43 MiB |
| startup | 4.6 ms | 9.5 ms |
| REF-04 peak RSS | 1.30x GNU | 1.80-2.25x GNU |
| JSON decode (364 KB) | 12.04 ms | 2.45 ms |

The default keeps the binary 41% smaller with better startup and memory; use
`-tags=sonic` when JSON throughput matters more than size. The default backend
decodes through `encoding/json/v2`'s `Unmarshal` plus its documented
`WithUnmarshalers` hook (numbers become `json.Number`, so integers above 2^53
stay exact); the rejected single-pass token walker is kept runnable in
`test/jsonx` for comparison — `go test -v ./test/jsonx/` prints the trade-off.

YAML goes through `internal/yamlx` as well and uses one library,
`go.yaml.in/yaml/v3`: decoding walks nodes (integers stay integers, duplicate
keys and merge keys are handled) and `toYaml` encodes the same tree. The
rejected `sigs.k8s.io/yaml` encoder stays runnable as a comparison in
`test/yamlx` (`go test -v ./test/yamlx/`). See ADR-004 §9-10 for the layout and
the decision.

See `CONTRIBUTING.md` and `docs/03-design/03-10-go-code-standards.md`.

## P0 status

- template substitution, escaping, default/existence/assignment operators
- JSON Schema validation and type checking (aliases, defaults, unknown keywords)
- `.env`, YAML and JSON value sources with multi-file precedence
- pipeline formatters: `join`, `toJson` (`align`), `toYaml` (`N`, `align`), `quote`, `indent`
- inline `| type` union with the schema
- safe in-place and batch rendering
- windowed streaming for small/streamed input and marked parallel rendering for
  large file templates (>=4 windows, no assignment operators, deterministic order)
- multi-arch static builds (linux/darwin/windows amd64+arm64); the default
  linux/amd64 binary is 4.92 MiB (8.43 MiB with `-tags=sonic`)
- JSON/YAML codecs centralized in `internal/jsonx` (standard-library
  `encoding/json/v2` by default, sonic behind a build tag) and `internal/yamlx`

Measured against the REF-04 baseline (10.47 MB / 200k variables, GNU envsubst
0.23.1) and enforced by `just test-perf` plus the CI `perf` job: 33 ms vs 67 ms
(0.49x), 15.6 MB vs 10.5 MB peak RSS (1.29x), 4.8 ms startup (target 100 ms).
TASK-022, TASK-023, TASK-024 and TASK-025 are done; outstanding P0 work is
TASK-026 (trial release), plus the Fuzz targets listed in the test matrix. See
`docs/05-tasks/05-01-p0-task-breakdown.md` for the per-task status snapshot.

## Supply chain

TASK-024 wires vulnerability scanning and release integrity:

- CI job `security` runs `govulncheck` (pinned version); reproduce with `just vuln`
- the release workflow's `supply-chain` job writes `SHA256SUMS` for every artifact,
  checks that the manifest covers all of them, and generates an SPDX JSON SBOM
  (`envsub.spdx.json`) with syft; a tag push gets a draft release with the
  binaries, the checksums and the SBOM attached
- locally: `just checksums` writes `dist/SHA256SUMS`, `just checksums-verify`
  validates a downloaded release, `just sbom` generates the SBOM (needs syft)

TASK-025 (CLI docs, migration notes and runnable examples) is covered by the CLI
section above, `examples/` and `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`.

## P1/P2

- P1: include, condition, loop, profiles, dry-run/diff/check
- P2: GNU/a8m/drone compatibility and conversion commands
