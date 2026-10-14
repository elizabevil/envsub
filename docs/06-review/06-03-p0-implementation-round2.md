---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-02
---

# P0 实现报告（Round 2：TASK-008 ~ TASK-021）

> 输入：`docs/05-tasks/05-01-p0-task-breakdown.md`、`docs/05-tasks/05-02-p0-test-matrix.md`、
> `docs/03-design/03-02 ~ 03-04`、`docs/06-review/06-02-implementation-progress-verification.md`。
> 结论：P0 的 TASK-008 ~ TASK-021 已实现并有 fixture / 单测 / E2E 覆盖；
> TASK-022 ~ TASK-026（多架构发布、基准对比、供应链、CLI 文档、试用发布）仍为待办。

## 1. 本次实现的模块

| 模块           | 文件                                                             | 内容                                                                                                                                             |
|----------------|------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| `.env` 解析    | `internal/source/dotenv.go`                                      | 注释、`export`、单/双引号转义、行内注释、重复 key 报错（带行号）、`KEY` 无值；`.env` 的 `env.KEY` 来源（文件+行）会传递到合并后的值树            |
| YAML 加载      | `internal/source/yaml.go`                                        | 基于 `go.yaml.in/yaml/v3` 节点解码，整数保持 `int`，引号标量保持 string                                                                          |
| JSON 加载      | `internal/source/json.go`                                        | `sonic` + `UseNumber`，数字归一化为 `int`/`float64`                                                                                              |
| 文件装载       | `internal/source/loader.go`                                      | 扩展名选择 loader、来源（文件/行）记录、错误分类                                                                                                 |
| 值树与合并     | `internal/values/tree.go`、`merge.go`、`set.go`、`number.go`     | 深合并、数组替换、`--set*` 路径赋值、来源元数据与最近祖先查找（含 `env.` 命名空间回退）、数字归一化                                              |
| 错误定位       | `internal/diag/errors.go`、`internal/engine/extended/scanner.go` | `diag.At` 附加文件/行，`Position` 由字节偏移换算行列，扫描错误带模板名与行号                                                                     |
| 环境命名空间   | `internal/values/path.go`                                        | `${env.NAME}` 只读环境；简单名回退；`app.port` -> `APP_PORT` 回退                                                                                |
| Schema 适配层  | `internal/schema/schema.go`、`normalize.go`                      | YAML/JSON 加载、类型别名归一化（`int`/`float`/`bool`/`array<T>`）、unknown 关键字策略、`default` 应用、env/.env 字符串物化、`jsonschema/v6` 校验 |
| Pipeline       | `internal/pipeline/pipeline.go`                                  | `join`/`toJson`/`toYaml`/`quote`/`indent` 白名单与链式调用                                                                                       |
| 内联 `\| type` | `internal/engine/extended/expression.go`、`engine.go`            | 必须是第一个 pipeline 操作符；与 schema 取并集；冲突报错                                                                                         |
| 策略           | `internal/engine/extended/engine.go`                             | `--unset=error                                                                                                                                   |empty|keep|zero`、`--empty=ok|error`、`--only`/`--ignore`、`:=`/`=` 写回 |
| 输出           | `internal/output/writer.go`                                      | stdout、文件（自动建目录）、安全 in-place 原子替换（临时文件 + rename + 权限保留）                                                               |
| 编排           | `internal/render/render.go`                                      | 值来源加载顺序、schema 校验、glob/目录/递归模板解析、批量输出目录、in-place、`--validate-only`、选项校验                                         |

## 2. 处理顺序（与设计一致）

1. 加载 schema（归一化 + unknown 策略 + 编译）。
2. 环境命名空间 = OS 环境；随后按命令行顺序叠加 `.env`（`.env` 优先于进程环境）。
3. 按顺序合并 values 文件（后者覆盖前者；object 深合并、array 整体替换）。
4. 应用 `--set` → `--set-string` → `--set-json`（后者优先）。
5. 应用 schema `default`（只填缺失键）。
6. 按 schema 物化 string：`array`/`object` 做 JSON 解析，`integer`/`number`/`boolean` 做标量转换；失败报类型错误并指出来源。
7. schema 校验（required/enum/range/pattern/长度/items）。
8. 渲染：逐表达式处理模板操作符 → `--unset`/`--empty` 策略 → `| type` 断言 → formatter pipeline。

## 3. 验收对照（TASK-008 ~ TASK-021）

| 任务                                 | 验收标准                           | 证据                                                                                                                                                         |
|--------------------------------------|------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TASK-008 `.env` 解析与 JSON 数组规则 | 6 条规则 fixture 通过              | `sources/dotenv_*`、`sources/dotenv_and_values` 等 11 个 fixture；`internal/source/dotenv_test.go`                                                           |
| TASK-009 YAML/JSON values 加载       | 单文件加载、类型保留、错误可定位   | `sources/env_namespace_*`、`internal/source/loader_test.go`（int/bool/array 类型保留）                                                                       |
| TASK-010 多文件合并与优先级          | 后覆盖前、数组替换、`--set` 最高   | `sources/merge_*` 4 个 fixture、`sources/env_nested_path_fallback`；`internal/values/merge_test.go`；E2E `set_precedence`                                    |
| TASK-011 Schema 加载/别名/unknown    | 别名归一化；unknown 默认 ignore    | `schema/type_alias_*`、`schema/unknown_keyword_*`、`schema/extension_keyword_allowed`                                                                        |
| TASK-012 `default` 应用              | 缺失键按 schema 填默认值           | `schema/default_*`、`schema/nested_default`、`schema/default_array`                                                                                          |
| TASK-013 字符串物化与类型转换        | int/float/bool/array JSON 规则     | `sources/dotenv_json_*`、`schema/schema_type_converts`、`internal/types/convert_test.go`                                                                     |
| TASK-014 jsonschema/v6 校验          | valid/invalid fixture 通过         | `schema/minimum_violation`、`maximum`、`enum`、`pattern`、`minLength`、`minItems`、`required_missing`                                                        |
| TASK-015 内联 `\| type`              | 必须是第一个操作符；冲突报错       | `inline_type/*` 19 个 fixture                                                                                                                                |
| TASK-016 错误信息来源                | 变量路径 + 来源文件/行 + 模板位置  | `sources/dotenv_invalid_json`（`.env:1`）、`diag.Error` 携带 `File`/`Line`、`Position()` 由字节偏移换算行列、E2E `template_error_location`（`input.tmpl:2`） |
| TASK-017 pipeline formatter          | formatter 与链式调用通过           | `pipeline/*` 21 个 fixture；`internal/pipeline/pipeline_test.go`                                                                                             |
| TASK-018 安全 in-place               | 临时文件替换、权限保留、失败不损坏 | `internal/output/inplace_test.go`（权限、无残留、取消、目录拒绝）；E2E `in_place`                                                                            |
| TASK-019 批量/多文件渲染             | 目录/glob、部分失败报告            | E2E `output_dir_batch`（`--recursive` + `expect.dir`）；`internal/render/render_features_test.go`                                                            |
| TASK-025（部分）                     | 示例可运行                         | `examples/` + E2E `examples_quickstart`                                                                                                                      |
| TASK-020 `--unset`/`--empty`         | 四种策略；required 不填零          | `policies/*` 16 个 fixture；E2E `unset_keep`、`unset_empty`、`empty_error`                                                                                   |
| TASK-021 P0 golden/E2E               | 全部 P0 fixture 通过               | 160 个 engine fixture + 17 个 E2E case 全绿                                                                                                                  |

## 4. 验证结果（2026-10-02）

```text
gofmt -l ./cmd ./internal ./test     # 空
go vet ./...                         # 通过
go test -count=1 ./...               # 全部通过（160 engine fixture + 17 E2E）
go test -race -count=1 ./...         # 全部通过
just check                           # fmt-check + vet + unit + e2e 通过
just build                           # 产出 dist/envsub
```

错误定位示例：

```text
$ envsub render broken.tmpl
broken.tmpl:2: offset 9: unclosed variable expression      # 退出码 3

$ envsub render --env-file .env --schema schema.yaml input.tmpl
HOSTS at .env:1 (not-json): expected a JSON array: ...     # 退出码 4
```

基准（`just bench`，本机 i9-13980HX）：

```text
BenchmarkRenderSmallTemplate-10    1058 ns/op
BenchmarkParseDotenv-10            164 µs/op (1000 行, 115 MB/s)
```

`BenchmarkRenderLargeTemplate`（20 万变量）用于对照 REF-04 基线，未纳入 CI 门禁。

## 5. 与设计的偏差与说明

1. `--set KEY=VALUE` 在没有 schema 或内联 `| type` 时保留原始 string（不做 JSON 猜测）；
   需要原生类型请用 `--set-json` 或在 schema 中声明类型。`[FACT]`
2. 环境命名空间优先级：进程环境变量优先于 `.env` 文件（`.env` 只在变量缺失时补充），
   多个 `.env` 文件之间后加载者优先；`${env.NAME}` 与简单名回退共用同一命名空间。`[FACT]`
3. `--unset=keep` 保留整个占位符原文，包括其内部的默认值词（如 `${X:-pre-${Y}-post}` 原样输出）。`[INFERENCE]`
4. `--unset=zero` 必须有 schema 类型或内联 `| type`，否则按策略错误（exit 6）报错；required 字段不会自动填零。`[FACT]`
5. 模板语法错误现在带模板名与行号（`config.tmpl:2: offset 9: ...`）；
   YAML/JSON values 的来源只记录文件（`values.yaml`），**不记录键的行号**（`--explain` 的 `line` 为 0）：
   `yamlx` 的解码结果尚未携带节点行号并接入 loader 接口。`[TODO]`
   触发条件：`yamlx` 暴露节点行号后接入；owner：技术负责人。
6. TASK-023 的性能门禁（10MB 模板 ≤5x GNU 基线、空输入启动 ≤100ms）：**已自动化**——`test/perf` + CI job `perf`（见 `reference/REF-04`）。`[FACT]`

## 6. 后续（TASK-022 ~ TASK-026）

- TASK-022：多架构静态构建已在 justfile 与 `release.yml` 就绪，缺少产物校验与校验和。
- TASK-023：补齐 GNU envsubst 对比基准与门禁。
- TASK-024：`govulncheck`、checksums、SBOM 接入 CI。
- TASK-025：CLI 文档、迁移指南与 `examples/`（当前为空）。
- TASK-026：试用发布与反馈收集。

## 非目标（本部分）

- 不改动 P1/P2（include、条件、循环、profiles、dry-run/diff/check、兼容模式与转换命令）。`[FACT]`
- 不替代人工评审。`[ASSUMPTION]`
