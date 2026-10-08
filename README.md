# envsub

Extended configuration renderer.

> P0 (TASK-001 ~ TASK-021) is implemented and covered by unit tests, a 160-case
> fixture matrix and 17 CLI end-to-end cases. See
> `docs/06-review/06-03-p0-implementation-round2.md`.
> Module path: `github.com/elizabevil/envsub`.

## Requirements

- Go 1.27.1
- `just` for shortcut commands
- `golangci-lint` and `govulncheck` for checks

## Quick start

```bash
just build
./bin/envsub version

# render a template from the environment
NAME=world ./bin/envsub render examples/template.tmpl

# values files, .env and a JSON Schema
./bin/envsub render -f base.yaml -f prod.yaml --env-file .env \
  --schema schema.yaml -o out.yaml template.tmpl

# safe in-place rewrite (atomic replace, permissions preserved)
./bin/envsub render -i --unset=error config.tmpl
```

## CLI

```text
envsub render [flags] [template...]

value sources
  -f, --values FILE...        YAML/JSON values files, repeatable
      --env-file FILE...      .env files, repeatable
      --schema FILE           JSON Schema (Draft 2020-12), JSON or YAML
      --set KEY=VALUE         raw string override (typed by schema or | type)
      --set-string KEY=VALUE  force a string
      --set-json KEY=JSON     native JSON value

rendering
  -o, --output FILE           single output file
  -i, --in-place              atomic replace of each template
      --output-dir DIR        batch output directory (mirrors input layout)
      --recursive             walk template directories recursively
      --validate-only         load, materialize and validate, write nothing

policies
      --unset=error|empty|keep|zero   default: error
      --empty=ok|error                default: ok
      --schema-unknown=ignore|warn|error  default: ignore
      --only VAR... / --ignore VAR...
```

Exit codes: `0` success, `1` internal, `2` usage, `3` template syntax,
`4` type/schema, `5` IO, `6` missing value or value policy.

## Template syntax

```text
$NAME ${app.port} ${app["my-key"]} ${hosts[0]} ${env.PORT}
${path:-default} ${path-default} ${path:+alt} ${path+alt}
${path:?message} ${path?message} ${path:=default} ${path=default}
$$ -> $        $1 stays literal
${hosts | join ","} ${app | toYaml | indent 2} ${PORT | type int} ${name | quote}
```

## Development

```bash
just check      # gofmt check + vet + unit/fixture tests + e2e
just fmt
just test
just test-race
just bench
```

`just` points `GOCACHE`/`GOTMPDIR` at a workspace-local `.tmp/` directory because
some shared toolchains mount the default Go cache read-only. Override the
variables in the environment to use different locations.

See `CONTRIBUTING.md` and `docs/03-design/03-10-go-code-standards.md`.

## P0 status

- template substitution, escaping, default/existence/assignment operators
- JSON Schema validation and type checking (aliases, defaults, unknown keywords)
- `.env`, YAML and JSON value sources with multi-file precedence
- pipeline formatters: `join`, `toJson`, `toYaml`, `quote`, `indent`
- inline `| type` union with the schema
- safe in-place and batch rendering

Outstanding P0 work: TASK-022 产物校验与校验和, TASK-023 GNU 基线门禁,
TASK-024 govulncheck/SBOM, TASK-025 CLI 文档与 `examples/`, TASK-026 试用发布.

## P1/P2

- P1: include, condition, loop, profiles, dry-run/diff/check
- P2: GNU/a8m/drone compatibility and conversion commands
