---
status: draft-ai
owner: [ 待指定 ]
version: 0.4
last-updated: 2026-10-08
---

# P0 任务拆分

> 输入：`docs/99-archive/02-product/02-03-prd.md`、`docs/99-archive/02-product/02-04-user-stories.md`、`docs/03-design/`。  
> 当前只实现 P0；P1/P2 保留为待办。  
> 进度（2026-10-22 更新）：P0 共 27 项，**27 项已完成**（TASK-026 试用发布已于 2026-10-21 之后完成）。
> 详见"状态总览"。
> 本轮另完成编解码层与体积优化（`internal/jsonx`、`internal/yamlx`、`test/jsonx` 对照），
> 见"编解码层与体积优化"一节；供应链基础（govulncheck / checksums / SBOM）见"供应链基础"一节。
> 核查与实现报告：`docs/99-archive/06-review/06-02-implementation-progress-verification.md`、
> `06-03-p0-implementation-round2.md`、`06-04-optimization-round3.md`（模板输入抽象与并行渲染修复）、
> `06-05-code-review-round4.md`（6 条评审整改）、`06-06-json-number-precision.md`（JSON 数字精度与解码方案对照）。

## 状态总览（2026-10-22 更新）

| 任务 | 状态 | 证据 |
|------|------|------|
| TASK-001 | 已完成 | module / justfile / CI / testdata 骨架；`just check` 通过 |
| TASK-002 | 已完成 | `envsub render --help` 可用；未知参数退出码 2（`internal/cli/root_test.go`） |
| TASK-027 | 已完成 | `engine.Engine` 及可选 `StreamEngine`/`StreamReaderEngine`/`PathEngine` 接口 + `extended` 实现；engine 单测 |
| TASK-003 | 已完成 | `diag.Kind` → 退出码 1..6；`diag` 单测覆盖分类与位置 |
| TASK-004 | 已完成 | stdin/stdout/file 均可渲染；`source.Input`（File/Stdin 实现）；E2E `basic_env`、`output_file` |
| TASK-005 | 已完成 | syntax 19 fixture；scanner/expression 单测 |
| TASK-006 | 已完成 | 路径 fixture（`app.port`、`hosts[0]`、quoted key、`env.NAME`）+ `values/path` 单测 |
| TASK-007 | 已完成 | defaults 26 fixture（`:- - :+ + :? ?` 与嵌套默认值） |
| TASK-008 | 已完成 | `.env` 规则 fixture + `source/dotenv` 单测（引号、转义、行内注释、重复 key 行号） |
| TASK-009 | 已完成 | YAML/JSON 加载单测与类型保留；sources fixture；编解码已集中到 `internal/jsonx` / `internal/yamlx`（19 + 6 个落盘 fixture），YAML 侧补上重复键查重与合并键 |
| TASK-010 | 已完成 | 多文件合并 fixture；E2E `set_precedence`；`values/merge` 单测 |
| TASK-011 | 已完成 | schema 类型别名与 unknown 策略 fixture（ignore/warn/error） |
| TASK-012 | 已完成 | `default` 应用 fixture（含嵌套与数组 items） |
| TASK-013 | 已完成 | env/.env 字符串物化 fixture + `types` 转换单测 |
| TASK-014 | 已完成 | jsonschema/v6 校验 fixture（required/enum/min/max/pattern/length/items） |
| TASK-015 | 已完成 | inline_type 19 fixture（首个操作符、与 schema 并集/冲突、管道后报错） |
| TASK-016 | 已完成 | E2E `template_error_location`（`input.tmpl:2`）、`dotenv_invalid_json`（`.env:1`） |
| TASK-017 | 已完成 | pipeline 21 fixture（join/toJson/toYaml/quote/indent 与链式、参数错误） |
| TASK-018 | 已完成 | `output` 原子替换单测（权限、取消、目录拒绝、无残留）+ E2E `in_place` |
| TASK-019 | 已完成 | E2E `output_dir_batch`（`--recursive`、`expect.dir`）+ `render_features_test` |
| TASK-020 | 已完成 | policies 16 fixture + E2E `unset_keep`/`unset_empty`/`empty_error`/`unset_zero_no_type` |
| TASK-021 | 已完成 | engine fixture 165 + E2E 64 + 346 个测试函数（8 个 fuzz 目标、17 个基准，2026-10-22 复点）；`just check` 与 `go test -race ./...`（28 包）通过 |
| TASK-022 | 已完成 | justfile 多架构配方 + release 6 目标矩阵（2026-10-21 补 windows/arm64）；交叉编译 linux/amd64 **5.02 MiB**、linux/arm64 4.75、darwin/arm64 4.82、windows/amd64 5.22（2026-10-22 用 go1.27.3 复测；linux/amd64 与 linux/arm64 实测 `statically linked`） |
| TASK-023 | 已完成 | 门禁 `test/perf`（`just test-perf`，CI job `perf`）：REF-04 同款 10,466,670 字节模板，本轮复测耗时 0.49×、峰值 RSS 1.29×、启动 4.8 ms，并断言与 GNU 输出逐字节一致 |
| TASK-024 | 已完成 | CI job `security` 跑 govulncheck（固定 v1.8.0，本机实测 `No vulnerabilities found.`）；release job `supply-chain` 产出 `SHA256SUMS`（并断言清单覆盖全部产物）与 SPDX JSON SBOM，tag 推送时创建 draft release；本地可用 `just vuln` / `just checksums` / `just checksums-verify` / `just sbom` |
| TASK-025 | 已完成 | README 的 CLI/语法/开发章节 + 示例（2026-10-13 重构为 `data/` + 逐模板 golden + 24 条配方）+ E2E `examples_tree` + ADR-002 迁移说明 |
| TASK-026 | 已完成 | 试用发布已进行（tag `v0.1.0`）；发布后的验证补齐见 [`05-06-verification-todo.md`](05-06-verification-todo.md) |

合计：已完成 27 / 部分完成 0 / 未开始 0。

## 性能实测（2026-10-08，本机）

方法：复现 REF-04 的 10,466,670 字节 / 200,000 行模板（每行一个 `${HOST_N}`）；
`hyperfine --warmup 3 --runs 10` 测总耗时；`RUSAGE_CHILDREN.ru_maxrss` 测峰值内存；
空输入调用测启动。GNU envsubst 与 `envsub render --on-missing=empty`（对齐 GNU 的空值语义）对比：

| 指标 | GNU 基线 | envsub 实测 | 目标 | 结论 |
|------|----------|-------------|------|------|
| 10.47MB / 200k 变量渲染 | 60.5 ms | 66.0 ms（1.09×） | ≤ 5× | 达标 |
| 峰值 RSS | 11,132 KB | 23,296 KB（2.09×） | ≤ 5× | 达标 |
| 空输入启动 | 0.81 ms | 16.3 ms | ≤ 100 ms | 达标 |

`go test -run XXX -bench=. -benchmem` 另有三个包级基准：
`BenchmarkRenderLargeTemplate`、`BenchmarkRenderSmallTemplate`、`BenchmarkParseDotenv`。

上表已自动化：`test/perf/`（`just test-perf`）用同一模板与同机器重复测量，
断言耗时 ≤5×、峰值 RSS ≤5×、启动 ≤100ms，并校验 GNU 与 envsub 输出逐字节一致；
CI 由 job `perf` 执行（先安装 `gettext-base` 提供 GNU envsubst），阈值可用
`PERF_MAX_TIME_RATIO`/`PERF_MAX_RSS_RATIO`/`PERF_MAX_STARTUP_MS`/`PERF_RUNS` 覆盖。
详见 `docs/reference/REF-04-gnu-envsubst-performance-baseline.md` 第 4 节。

**本轮复测（编解码层与体积优化后，同一门禁）**：耗时 **0.49×**、峰值 RSS **1.29×**、
启动 **4.8 ms**；上表数值为优化前首轮数据，保留作为对比。`[FACT]`

## 编解码层与体积优化（2026-10-08 收尾）

本阶段不计入 TASK 编号，属工程收尾（体积/性能/可维护性）；决策与数据见 ADR-004 第 8、9 节
与 `docs/99-archive/06-review/06-06-json-number-precision.md`。

| 项 | 结果 | 证据 |
|----|------|------|
| JSON 编解码 | 统一入口 `internal/jsonx`，build tag 双后端：默认标准库 `encoding/json/v2`（`Unmarshal` + 文档化 `WithUnmarshalers`，数字落 `json.Number` 保精度），`-tags=sonic` 切 sonic | `jsonv2.go`/`sonic.go`；契约测试与两后端差异测试 |
| YAML 编解码 | 统一入口 `internal/yamlx`，单一库 `go.yaml.in/yaml/v3`：解码走节点（查重、合并键），`toYaml` 用同库编码 | `yamlv3.go`/`marshal.go`；6 个落盘 fixture；对照在 `test/yamlx` |
| 默认二进制 | 8.55 MiB → **5.03 MiB（-41%）**；启动 9.5 ms → **4.6 ms**；REF-04 峰值 RSS 2.09× → **1.29×** | `just size`、`just test-perf` |
| JSON 吞吐取舍 | 默认后端 364 KB 解码 ~12.0 ms；`-tags=sonic` 2.45 ms；被拒的 token 遍历备选 3.55 ms 保留在 `test/jsonx` 可复现对比 | `go test -v/-bench ./test/jsonx/` |
| 版本信息 | Kubernetes `version.Info` 字段形状；`buildDate` 取可执行文件 mtime，彻底移除 `-X` 注入 | `internal/version`、`internal/cli/version_test.go` |
| 开发规则 | "优先标准库既有 API、不自行封装解析层；方案对比实验必须落盘可复现" 写入提示词 | `docs/prompts/_common.md` 第 7 节 |

顺带修复的两个 YAML 缺陷：重复映射键未查重（node 模式不校验）、合并键 `<<: *anchor`
未实现导致合并值丢失；详见 ADR-004 第 9 节。`[FACT]`

## 供应链基础（TASK-024）

| 环节 | 实现 | 本地复现 |
|------|------|----------|
| 漏洞扫描 | CI job `security`：`go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...`（固定版本），发现可达漏洞即失败 | `just vuln` |
| 校验和 | release job `supply-chain`：下载全部矩阵产物 → `sha256sum envsub-* > SHA256SUMS`，并断言清单行数等于产物数（防止漏项） | `just checksums`（生成） |
| 校验和验证 | 消费方视角：`sha256sum -c SHA256SUMS` | `just checksums-verify` |
| SBOM | release job 用 `anchore/sbom-action`（syft）扫描 `dist/` 生成 `envsub.spdx.json`（SPDX JSON） | `just sbom`（需安装 syft；本机已验证：SPDX-2.3、21 个包、15 个 Go 模块） |
| 发布产物 | tag 推送时创建 **draft** release，附全部二进制 + `SHA256SUMS` + SBOM；是否正式发布留给 TASK-026 | - |

验证记录（2026-10-08，本机）：

- govulncheck v1.8.0 对全模块运行结果 `No vulnerabilities found.`（exit 0）。`[FACT]`
- `just checksums` 生成 2 个产物的清单，`just checksums-verify` 通过；人为篡改二进制后
  `checksums-verify` 报 `1 computed checksum did NOT match`（exit 1），还原后恢复通过。`[FACT]`
- 两个 workflow 经 actionlint v1.7.12 校验无告警；YAML 解析确认 job 为
  `test/lint/security/perf` 与 `build/supply-chain`。`[FACT]`
- SBOM 本地已核对：安装 syft 后 `just sbom` 生成 `dist/envsub.spdx.json`，`spdxVersion` 为
  SPDX-2.3，含 21 个包（其中 15 个 Go 模块，如 cobra、jsonschema/v6）与 2 个文件条目。`[FACT]`


## Sprint 0：基础工程

| 任务     | 描述                                                          | 关联 US     | 关联设计     | 验收标准                                                 | 工时 | 依赖     | 风险                    |
|----------|---------------------------------------------------------------|-------------|--------------|----------------------------------------------------------|------|----------|-------------------------|
| TASK-001 | 初始化 Go module、目录结构、justfile、CI 与测试矩阵骨架       | -           | 03-07, 05-02 | `just build`/`go test ./...` 通过；testdata 目录符合设计 | 1.5  | -        | 低                      |
| TASK-002 | 实现 CLI 骨架与 `render` 命令                                 | US-001      | 03-04, 03-08 | `tool render --help` 可用；未知参数报错                  | 1    | TASK-001 | Cobra 命令树            |
| TASK-027 | 定义并实现 Engine 接口（含可选流式/路径能力），extended 实现 | -           | 03-09        | 接口可扩展；extended 可被 CLI/render 调用                | 1.5  | TASK-001 | P0 单实现，避免过度抽象 |
| TASK-003 | 定义错误码与退出码模型                                        | US-001..008 | 03-04        | 类型/语法/IO/校验错误映射稳定退出码                      | 1    | TASK-002 | 错误码表未定            |
| TASK-004 | 模板输入输出管线：stdin/stdout/file                           | US-001      | 03-04        | 文件/stdin 均可渲染到 stdout/file                        | 1    | TASK-002 | 大文件流式处理          |

## Sprint 1：模板语法与值来源

| 任务     | 描述                                           | 关联 US        | 关联设计 | 验收标准                                              | 工时 | 依赖               | 风险                  |
|----------|------------------------------------------------|----------------|----------|-------------------------------------------------------|------|--------------------|-----------------------|
| TASK-005 | 实现文本扫描器：`$NAME`、`${path}`、`$$`、`$1` | US-001, US-003 | 03-02    | fixture 覆盖基础替换、转义、`$1` 字面量               | 2    | TASK-004, TASK-027 | 嵌套默认值边界        |
| TASK-006 | 实现路径解析与 `env` / 根命名空间              | US-001         | 03-02    | `${app.port}`、`${env.PORT}`、`${app["my-key"]}` 可用 | 1.5  | TASK-005           | 特殊 key 规则         |
| TASK-007 | 实现默认值/存在性操作符                        | US-002         | 03-02    | `:-`、`-`、`:+`、`+`、`:?`、`?` fixture 通过          | 2    | TASK-006           | 默认词中 `:` `-` 解析 |
| TASK-008 | 实现 `.env` 解析器与 JSON 数组规则             | US-007         | 03-03    | 6 条 `.env` 数组规则 fixture 通过                     | 1.5  | TASK-006           | 行号与引号规则        |
| TASK-009 | 实现 YAML/JSON values 加载                     | US-007         | 03-03    | 单文件加载、类型保留、错误可定位                      | 1.5  | TASK-006           | YAML 归一化边界       |
| TASK-010 | 实现多文件合并与优先级                         | US-007         | 03-03    | 后覆盖前、数组替换、CLI `--set` 最高                  | 1.5  | TASK-008, TASK-009 | 深合并规则            |

## Sprint 2：类型系统与 Schema

| 任务     | 描述                                       | 关联 US | 关联设计     | 验收标准                                             | 工时                                                   | 依赖               | 风险               |
|----------|--------------------------------------------|---------|--------------|------------------------------------------------------|--------------------------------------------------------|--------------------|--------------------|
| TASK-011 | 实现 JSON Schema 加载、别名与 unknown 策略 | US-004  | 03-03        | `int`/`float`/`bool` 别名归一化；unknown 默认 ignore | 2                                                      | TASK-009           | 远程 `$ref` 禁用   |
| TASK-012 | 实现 `default` 应用引擎                    | US-004  | 03-03        | 缺失键按 schema 填默认值                             | 1                                                      | TASK-011           | 嵌套默认值         |
| TASK-013 | 实现 env/.env 字符串物化与类型转换         | US-004  | 03-03        | int/float/bool/array JSON 规则 fixture 通过          | 2                                                      | TASK-010           | 宽松/严格策略      |
| TASK-014 | 集成 `santhosh-tekuri/jsonschema/v6` 校验  | US-004  | 03-03        | valid/invalid schema fixture 通过                    | 1.5                                                    | TASK-012, TASK-013 | 未知关键字默认忽略 |
| TASK-015 | 实现内联 `                                 | type`   | US-005       | 03-03                                                | 缺省可用；提供时必须为 pipeline 第一个操作符；冲突报错 | 1.5                | TASK-006, TASK-014 | 与 formatter 顺序 |
| TASK-016 | 实现错误信息与来源路径                     | US-004  | 03-03, 03-04 | 错误包含变量路径、来源文件、行列                     | 1                                                      | TASK-014           | 行列信息可获取性   |

## Sprint 3：渲染、写回与 E2E

| 任务     | 描述                                                               | 关联 US     | 关联设计 | 验收标准                                              | 工时 | 依赖               | 风险             |
|----------|--------------------------------------------------------------------|-------------|----------|-------------------------------------------------------|------|--------------------|------------------|
| TASK-017 | 实现 pipeline formatter：`join`/`toJson`/`toYaml`/`quote`/`indent` | US-006      | 03-02    | formatter fixture 与链式调用通过                      | 2.5  | TASK-006           | 转义与缩进语义   |
| TASK-018 | 实现安全 in-place 原子写回                                         | US-008      | 03-04    | 临时文件替换、权限保留、失败不损坏原文件              | 1.5  | TASK-004           | 跨平台 rename    |
| TASK-019 | 实现批量/多文件渲染编排                                            | US-008      | 03-04    | 多文件、目录/glob、部分失败报告                       | 2    | TASK-004, TASK-018 | 并发与顺序       |
| TASK-020 | 实现 `--on-missing` / `--on-empty` 策略                                    | US-002      | 03-04    | `error/empty/keep/zero` fixture 通过；required 不填零 | 1    | TASK-007, TASK-014 | empty 与类型冲突 |
| TASK-021 | 建立 P0 golden fixture 与端到端测试                                | US-001..008 | 03-06    | 所有 P0 fixture 通过                                  | 2    | TASK-005..020      | 覆盖面           |

## Sprint 4：发布与验证

| 任务     | 描述                         | 关联 US | 关联设计 | 验收标准                          | 工时 | 依赖          | 风险         |
|----------|------------------------------|---------|----------|-----------------------------------|------|---------------|--------------|
| TASK-022 | 多架构静态构建与发布流水线   | -       | 03-05    | Linux amd64/arm64 静态二进制产出  | 2    | TASK-002      | 跨平台构建   |
| TASK-023 | 性能基准与 GNU envsubst 对比 | -       | REF-04   | 10MB 模板 ≤5x 基线；启动 ≤100ms   | 1    | TASK-004      | 基准环境差异 |
| TASK-024 | 安全与供应链基础             | -       | ADR-003  | govulncheck 通过、checksums、SBOM | 1    | TASK-022      | 工具链依赖   |
| TASK-025 | CLI 文档、迁移指南与示例     | -       | 02-03    | README/CLI docs/示例可运行        | 1.5  | TASK-002      | 内容维护     |
| TASK-026 | P0 试用发布与反馈收集        | -       | 02-03    | 发布候选、反馈记录、问题清单      | 1    | TASK-022..025 | 用户接受度   |

## Sprint 划分建议

- Sprint 0：TASK-001 ~ TASK-004、TASK-027。
- Sprint 1：TASK-005 ~ TASK-010。
- Sprint 2：TASK-011 ~ TASK-016。
- Sprint 3：TASK-017 ~ TASK-021。
- Sprint 4：TASK-022 ~ TASK-026。

## 关键路径

```text
TASK-001 → TASK-027 → TASK-005 → TASK-006 → TASK-007
TASK-001 → TASK-002 → TASK-004 → TASK-005
                                              ↓
TASK-009 → TASK-010 → TASK-011 → TASK-013 → TASK-014 → TASK-015
                                              ↓
TASK-017 → TASK-019 → TASK-020 → TASK-021 → TASK-022 → TASK-024 → TASK-026
```

## P1 / P2 待办清单

> **已升级为可排期任务清单**：[05-04 P1/P2 任务拆分](05-04-p1-p2-task-breakdown.md)
> （含需求编号、目标、验收口径、依赖与风险；本文下方 TASK-B0x 仅作历史位置记录）。

- ~~TASK-B01~~ → TASK-114；~~TASK-B02/B03~~ → TASK-115；~~TASK-B04~~ → TASK-112；
  ~~TASK-B05~~ → TASK-105（仅 `--check`）；~~TASK-B06~~ → TASK-110；~~TASK-B07/08/09~~ → TASK-201；
  ~~TASK-B10~~ → TASK-202。

> 优化与规则差异的待办清单见 [05-03 待办工作交接](05-03-pending-work.md)（自包含，可冷启动接手）。

## 本轮核实发现的缺口（状态：1 项待办、5 项已闭环）

**仍待办**（各有 owner 与触发条件）：

1. **`sonic` 后端实机运行未验证**：交叉编译已覆盖 linux/amd64、linux/arm64、darwin/arm64、windows/amd64，
   但 Windows/macOS 实机运行仍未验证（ADR-004 §7）。触发条件：有实机或 CI 矩阵 runner；owner：技术负责人。

**已闭环**（保留结论，避免重复排查）：

1. **Fuzz 已落地**（2026-10-22 补齐）：`test/fuzz/fuzz_test.go` 有 6 个 `func Fuzz` 目标
   （扫描器/流式 span/路径文法/表达式路径/数值/`.env`），表达式解析器与窗口读取器那 2 个在
   `internal/engine/extended/`（`parseExpression` 与 `streamWindow` 未导出），共 8 个；
   `test/justfile` 的 `test` 目标含 `test-fuzz`，CI job `fuzz` 跑短时版本。
2. **TASK-026 试用发布已完成**：tag `v0.1.0` 已打，发布后的验证补齐见
   [`05-06-verification-todo.md`](05-06-verification-todo.md)。
3. **应用层单测已补**：`internal/app/app_test.go`、`cmd/envsub/main_test.go` 覆盖参数装配、输入/输出形态、
   `--validate-only`、`--explain`、退出码 1/2/3/4/5/6 与诊断文案（覆盖率见 `05-03` §8）。
4. **序列化规则收尾完成**：第 7 步 `NaN`/`±Inf` 拒绝已实现（`09-gaps` G5：解码拒绝 + `codec.RejectNonFinite`
   的编码侧拒绝）；第 8 步 §3 类型表逐行测试——`01-types.md` 的 8 条规则行全部带证据列与 ✅；
   第 9 步 `uint` 类型名已定为**不提供**并从落地步骤中删除（`00-index` 变更记录）。
5. **`gitVersion` 不再带 `+dirty`**（2026-10-22）：版本号改为"git tag 或分支 + 短 commit id"
   （`v0.1.0-a215676` / `main-a215676`），脏状态由 `gitTreeState` 单独报；tag/分支由构建
   配方注入，工具链只嵌模块版本、不嵌分支名。

（原"TASK-023 缺门禁"已解决：`test/perf/` + CI job `perf`，见 TASK-023 行与"性能实测"。
原"TASK-024 供应链空白"已解决：CI job `security` + release job `supply-chain`，见"供应链基础"。）

## 非目标（本部分）

- 本文不是排期承诺，工时仅参考。`[ASSUMPTION]`
- P1/P2 不在当前实现范围。`[FACT]`
- 不替代开发评估与资源确认。`[INFERENCE]`
