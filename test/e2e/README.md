# End-to-end contracts

One directory per case under `testdata/`. The runner executes the real binary with the
directory's `args`, then asserts the exit code, stdout, stderr and (optionally) a directory
tree. Everything a case needs lives inside its own directory, so cases cannot interfere.

## Run

```bash
just --justfile test/justfile test-e2e              # all cases, verbose
just --justfile test/justfile test-e2e-case NAME    # one case, e.g. NAME=check_outdated
go test -run 'TestE2E/check_outdated' ./test/e2e    # the plain go form
just --justfile test/justfile e2e-list              # list the case names
```

## Case files

| File | Required | Meaning |
|------|----------|---------|
| `args` | yes | one argument per line; the first line is the subcommand (`render`) |
| `input.tmpl` | usually | the template; its name comes from `args`, so any name works |
| `values.yaml` / other value files | no | any file the `args` reference |
| `env` | no | `KEY=VALUE` lines added to the process environment for this case only |
| `expect.out` | no | exact stdout |
| `expect.err` | no | substring that stderr must contain |
| `expect.exit` | no | expected exit code (default 0) |
| `expect.dir` | no | expected tree of the `result/` output directory, compared file by file |

A missing `expect.*` means "do not check that part". The process environment is not inherited:
only `env` plus a fixed PATH reaches the binary, so `@process` results are reproducible.

## Exit codes

| Code | Meaning |
|------|---------|
| 1 | internal error |
| 2 | usage (bad flag, bad flag value, unreadable schema path) |
| 3 | template (syntax, unknown formatter, bad formatter argument) |
| 4 | type or schema (inline `\| type` conflict, schema validation, unknown keyword) |
| 5 | IO (missing or unreadable file, failed write) |
| 6 | missing value or a policy set to `error` |
| 7 | `--check`: the target exists but differs |

## Adding a case

1. `mkdir test/e2e/testdata/<name>` and write `args`, `input.tmpl` and the value files it references;
2. run the case once to capture the real output, then decide what the contract should be —
   copy it into `expect.out` / `expect.err` / `expect.exit` / `expect.dir`;
3. run it again: it must pass;
4. `just check` runs the whole directory, so no registration step is needed.

Structural expectations that outlive one case (import layering, CLI/runner split) belong in
`test/architecture` instead.
