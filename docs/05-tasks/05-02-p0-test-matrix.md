---
status: draft-ai
owner: [ 待指定 ]
version: 0.4
last-updated: 2026-10-08
---

# P0 测试矩阵（Test-Driven）

> 目标：规则已定义清楚，先建立测试矩阵，再推动实现。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 1. 测试策略

- 采用测试先行：每个规则先写 fixture / golden，再实现功能。`[FACT]`
- 测试层次：
    - 单元测试：扫描器、路径、操作符、类型转换、formatter。
    - Golden 测试：模板 + values + schema -> 期望输出。
    - E2E 测试：CLI 命令、文件、退出码。
    - Fuzz：模板扫描器、表达式解析器、`.env` 解析器。
    - Benchmark：与 GNU envsubst 基线对比。
    - Conformance：P2 兼容模式预留。

## 2. 测试矩阵维度

| 维度          | 用例                                                                                                                    |
|---------------|-------------------------------------------------------------------------------------------------------------------------|
| 基础替换      | `$VAR`、`${VAR}`、混合文本、多变量                                                                                      |
| 转义          | `$$`、`$${`、`$1` 字面量、非法 `${`                                                                                     |
| 路径          | `app.port`、`hosts[0]`、`app["my-key"]`、`env.PORT`                                                                     |
| 默认值/存在性 | `:-`、`-`、`:+`、`+`、`:?`、`?`、嵌套默认值、`:` `-` 字面量                                                             |
| 类型          | string、bool、int、float、array<T>、object、null                                                                        |
| 值来源        | env、`.env`、YAML、JSON、多文件                                                                                         |
| 多文件覆盖    | 后覆盖前、对象深合并、数组替换、CLI `--set` 最高                                                                        |
| Schema        | required、default、enum、minimum、maximum、minLength、maxLength、pattern、minItems、maxItems、unknown ignore/warn/error |
| 内联 `        | type`                                                                                                                   | 缺省、第一个操作符、冲突报错、管道后 `| type` 报错 |
| Pipeline      | `join`、`toJson`、`toYaml`、`quote`、`indent`、链式                                                                     |
| `--on-missing`     | error、empty、keep、zero；required 不自动填零                                                                           |
| 输出          | stdout、file、in-place、批量、权限、最后一行无换行                                                                      |
| 错误          | 找不到变量、类型错误、Schema 错误、IO 错误、模板语法错误                                                                |

## 3. 已实现 Fixture 运行器

- `[FACT]` Engine fixture 位于 `internal/engine/extended/testdata/`。
- `[FACT]` 包级快捷测试入口为 `internal/engine/extended/justfile`。
- `[FACT]` 运行器位于 `internal/engine/extended/fixture_test.go`。
- `[FACT]` 测试不会在 Go 代码中实时生成模板，统一读取预先定义的 fixture 文件。
- `[FACT]` 当前共 160 个预定义 fixture。
- `[FACT]` 当前 fixture 覆盖：
    - syntax（19）：文本、`$NAME`、`${NAME}`、`env.NAME`、`$$`、`$1`、根路径、数组索引、quoted key。
    - defaults（26）：`:-`、`-`、`:+`、`+`、`:?`、`?`、`:=`、`=`、嵌套默认值、URL/负数字面量。
    - types（12）：string、int、float、bool、null、array 错误、object 错误。
    - errors（10）：未闭合 `${`、空 `${}`、缺失变量、非法路径、非法操作符。
    - pipeline（21）：`join`、`toJson`、`toYaml`、`quote`、`indent`、链式调用与参数错误。
    - inline_type（19）：`| type int|float|bool|string|array<T>|object`、string 解析、与 schema 的并集/冲突。
    - schema（19）：default、required、enum、minimum/maximum、pattern、minLength、minItems、类型别名、unknown 关键字策略。
    - sources（18）：`.env` 解析与引号规则、JSON 数组/对象物化、6 条 `.env` 规则、多文件合并与优先级、env 命名空间回退。
    - policies（16）：`--on-missing=error|empty|keep|zero`、`--on-empty=ok|error`、`--only-vars`/`--ignore-vars`、`:=`/`=` 写回。
- `[FACT]` E2E 数据位于 `test/e2e/testdata/`；执行逻辑为 `test/e2e/e2e_test.go`，入口为 `test/justfile`。
- `[FACT]` 根 `justfile` 提供统一测试入口。
- `[FACT]` `e2e/` 共 17 个 CLI 集成场景：env+stdout、output file、in-place、output-dir 批量、values+env+schema、`--set*`
  优先级、pipeline/内联类型、`--on-missing`/`--on-empty` 策略、`examples/` 快速开始、模板错误定位、`.env` 来源定位、exit 3/4/6。
- `[FACT]` fixture runner 支持 `values.yaml`、`values-2.yaml`、`env.yaml`、`env.txt`、`schema.yaml` 与 `meta.yaml`（`unset`/
  `empty`/`only`/`ignore`/`exit_code`/`schema_unknown`）。
- `[FACT]` E2E runner 支持 `args` 文件（相对路径、`@` 前缀绝对化）、`process.env`、`expect.dir` 目录比对与 `modified` 标记。
- `[FACT]` 命令：
    - `just test-fixtures`：运行 fixture 矩阵。
    - `just fixtures-list`：列出全部 fixture。
    - `just test-all`：运行单元测试与 E2E。
    - `just test-race`：并发检测。
    - `just test-e2e`：运行真实 CLI E2E。
    - `just e2e-list`：列出 E2E case。
    - `just test-decode`：解码入口契约的聚焦回归（BOM 统一剥离、常见标量/格式形式，
      含 YAML/JSON/`.env`/`--set-file` 与 sonic 后端）。全量 `test` 已覆盖同样用例，
      该 recipe 用于改动解码器时的快速复跑。

## 4. Fixture 约定

每层的**文件格式与运行方式以该层自己的 README 为准**（避免本文件与实现漂移）：

| 层 | 运行方式 | 格式与期望文件 |
|----|----------|----------------|
| 引擎用例 | `just --justfile test/justfile test-e2e` 之外的 `go test ./internal/engine/extended/` | `internal/engine/extended/testdata/README.md` |
| CLI 端到端 | `just --justfile test/justfile test-e2e`（单个：`test-e2e-case NAME`） | `test/e2e/README.md`（含 7 个退出码含义） |
| 可运行示例 | `go test ./test/examples/ -v`；配方：`just demo` | `examples/README.md` §"How to verify an example" |
| 编解码对拍 | `just --justfile test/justfile test` 或 `go test ./test/jsonx/ ./test/yamlx/ ./test/encoding/` | 各包内测试文件 |



建议目录结构：

```text
internal/engine/extended/testdata/<area>/<case>/
├── input.tmpl
├── values.yaml
├── values-2.yaml
├── schema.yaml
├── env.yaml
├── env.txt
├── expect.out
├── expect.err
└── meta.yaml

test/e2e/testdata/<case>/
├── input.tmpl          # 或 templates/ 目录
├── args                # 每行一个 CLI 参数
├── process.env         # 追加到子进程环境
├── expect.out          # stdout 期望
├── expect.dir/         # 输出文件/目录比对
├── modified            # 存在时按原地改写比对
├── exit
└── expect.err
```

- `meta.yaml` 声明期望退出码、`unset`/`empty` 策略与 `only`/`ignore` 过滤。`[FACT]`
- `schema.yaml`、`values-2.yaml`、`env.txt` 分别用于 schema 驱动、多文件合并与 `.env` 场景。`[FACT]`
- `args`、`process.env`、`expect.dir`、`modified` 用于 CLI 级 E2E 场景。`[FACT]`
- Golden 更新：`just test-update`（需人工 review diff）。`[INFERENCE]`
- 每个 fixture 必须有正常路径与至少一个异常路径。`[FACT]`

## 5. TDD 工作流

1. 在 `internal/engine/extended/testdata/<area>/<case>` 新建 fixture。
2. 写 `expect.out` / `expect.err` / `meta.yaml`。
3. 运行 `just test`，确认失败（Red）。
4. 实现最小功能使其通过（Green）。
5. 重构并补充边界用例（Refactor）。
6. 在任务验收标准中引用 fixture ID。

## 6. 任务映射

- TASK-005：syntax fixture。
- TASK-006：path fixture。
- TASK-007：operators fixture。
- TASK-008：values/.env fixture。
- TASK-009/010：values merge fixture。
- TASK-011~016：schema/types fixture。
- TASK-017：pipeline fixture。
- TASK-018/019：output/e2e fixture。
- TASK-021：完整 P0 golden/E2E。
- TASK-023：performance benchmark。

## 7. 覆盖现状核实（2026-10-08，收尾复测）

> **更新（2026-10-10）**：本节数字是 P0 收尾时的快照，**不再逐项维护**。当前 e2e fixture 为
> **36 个**（P0 时 18 个 + `explain_*` 7 + `env_file_nonstandard` + `legacy_flag_*` 2 + `schema_default_is_fallback`
> 等），最新计数与逐组覆盖见 `05-03-pending-work.md` §10 #5 与 §11。`[FACT]`

- 规模：24 个包（22 个有测试通过），180 个测试函数，9 个基准。`[FACT]`
- Fixture 分领域数量与第 3 节记录一致，合计 160：defaults 26、errors 10、inline_type 19、
  pipeline 21、policies 16、schema 19、sources 18、syntax 19、types 12。`[FACT]`
- E2E 场景 18 个，`just test-e2e` 与 `just check` 通过。`[FACT]`
- 并发安全：`go test -race ./...` 19 个包全部通过（Round 3 修复了并行渲染的
  死锁、窗口别名与路径缓存竞争）。`[FACT]`
- 覆盖率（`go test -cover ./...`，语句）：utils 100、pipeline 95.5、cli 93.2、version 91.2、
  jsonx 90.0、diag 88.0、extended 85.8、source 84.2、logging 78.6、render 73.9、yamlx 77.3、
  types 71.5、schema 62.0、values 54.8、output 50.3、engine 38.2；`internal/app` 与
  `cmd/envsub` 为 0（仅 E2E 间接覆盖）。`[FACT]`
- Benchmark 7 个：`BenchmarkRenderLargeTemplate`、`BenchmarkRenderSmallTemplate`、
  `BenchmarkParseDotenv`、jsonx 的解码/编码 2 个，以及 `test/jsonx` 的生产/备选对照 2 个；
  REF-04 同款 10.47MB 模板的 GNU 对比已自动化为门禁 `test/perf`（`just test-perf`，
  CI job `perf`）：耗时 ≤5×、峰值 RSS ≤5×、启动 ≤100ms，并校验输出与 GNU 逐字节一致；
  收尾复测为 0.49×/1.29×/4.8 ms。`[FACT]`
- 编解码层同样是 fixture-first：`internal/jsonx/testdata/`（19 个文件）与
  `internal/yamlx/testdata/`（6 个文件）落盘输入，配 `jsonx_test.go` / `jsonv2_test.go` /
  `sonic_test.go` 与 `yamlv3_test.go` / `sigsyaml_test.go`（布局见 ADR-004 第 9 节）。`[FACT]`
- 方案对照实验落盘：`test/jsonx/`（被拒绝的 token 遍历备选 vs `encoding/json/v2`，数据见 06-06）
  与 `test/yamlx/`（被拒绝的 `sigs.k8s.io/yaml` 编码器 vs `go.yaml.in/yaml/v3`，功能等价 +
  已知差异 + 基准，数据见 ADR-004 第 10 节）；两者均只需 `go test -v ./test/<pkg>/` 复现。`[FACT]`
- 序列化规则一致性：`test/encoding/` 按 [序列化规则](../07-serialization/00-index.md)
  逐条测试（编码/解码/确定性/空值），未实现项以 `t.Skip` 标注为实施清单；
  `go test -v ./test/encoding/` 与 `go test -tags=sonic ./test/encoding/` 均可运行。`[FACT]`
- **缺口 1（已闭环）**：Fuzz 已落地——`test/fuzz/fuzz_test.go` 有 6 个 `func Fuzz` 目标
  （扫描器/流式 span/路径文法/表达式路径/数值/`.env`），另外 2 个在
  `internal/engine/extended/`（表达式解析器与窗口读取器，受测函数未导出），共 8 个；
  `just test-fuzz` 与 CI job `fuzz` 覆盖。`[FACT]`
- **缺口 2（已解决）**：供应链检查已接入——CI job `security` 跑 govulncheck（本机实测
  `No vulnerabilities found.`），release job `supply-chain` 产出 SHA256SUMS 与 SPDX SBOM
  （见 05-01"供应链基础"）。`[FACT]`
- **缺口 3（已闭环）**：`internal/app/app_test.go` 与 `cmd/envsub/main_test.go` 已补最小装配测试
  （参数装配、输入/输出形态、`--validate-only`、`--explain`、退出码与诊断文案；覆盖率见 `05-03` §8）。`[FACT]`

## 8. 测试评审与去重（2026-10-13）

对全部测试做了一次系统评审（按区域盘点覆盖、过时与重复），随后去重、加固弱断言、补高价值缺口。

### 8.1 规模变化

| 指标 | 评审前 | 评审后 |
|------|--------|--------|
| Test/Fuzz/Benchmark 函数 | 323 | 308 |
| e2e fixture | 54 | 56 |
| 引擎 fixture | 158 | 156 |

净减约 360 行测试代码（+300/-660），删掉的多是"逐例与 fixture 重复"的 Go 表。

### 8.2 去重原则（评审的判定口径）

只有当**保留者覆盖了被删者守护的全部行为**时才删；断言若为被删者独有，则并入保留者。
以下曾被列为重复但**判定为非等价而保留**，并记录了理由：

- `TestOnlyAndIgnore`：唯一守护"裸 `$NAME` 在 Only/Ignore 下不走快路径"
- `TestSchemaAndInlineTypeUnion`：5 例中 3 例无 fixture 对应
- `types/array_empty_error`、`errors/missing_index`、`inline_type/int_rejects_blank`、
  `pipeline/indent_two` 与 `chain_yaml_indent`：各覆盖互不蕴含的边界
- `TestProcessNamespaceIsNotAMaterializationSource`、`TestNoPathDerivationForNestedPaths`、
  `TestTreeResolve` 的 no-env-fallback 子例：不同 API 路径
- `TestLiteralNumberRoundTrip`（唯一覆盖 `1e309`）
- `TestLoadLayerOrder` 与 `TestRunLayerOrder`：前者独有 `--set-json` 高于 `--set` 与
  `default` 兜底，后者独有"经渲染管线落到输出文件"

### 8.3 加固的弱断言（原为"只判 err != nil"）

`schema` 的 UnknownWarn 与 union 拒绝、`values` 的 `TestParsePathRejects`（改为逐例断言
错误片段与字节偏移）、`engine` 的 scanner 与并行判定、`render` 的选项校验/错误定位/validate-only、
`load` 的 `--set-file` 失败分类、`encoding` 的 `TestTypeMatrixBoundaries`（原为丢弃结果的空调用）。

### 8.4 新增守护

- `test/examples`：示例必须覆盖 `internal/cli` 注册的每个旗标；数据集键名必须符合
  `<类型>_<来源>_<效果>` 且每种类型/来源都有键；`docs` 定义的每个规则族都要在示例 README 里出现；
  `internal/pipeline` 注册的每个 formatter 都要被某个模板使用；两份 scenarios 模板的解析标记必须成立
- `examples/justfile` 的 `acceptance` 配方：把整棵输出树渲染到 `.tmp` 并用
  `examples/tools/digest` 与 `examples/out/` 逐文件比对（跨平台，不依赖 diff/cmp）
- `test/fuzz`：四个 fuzz 目标（扫描器、流式 span、路径文法、`.env`）
- 本轮补：错误类别目录（含 `--check` 的 exit 7）、`Report` 零值、
  `--explain`/`--report` 在引擎不支持时的失败、报告的 stderr 默认通道、
  `WalkMeta` 遍历与 `RecordMeta` 索引、`FileWriter` 空路径、`CheckWriter` 排序与不可读目标

### 8.6 示例层与 e2e 层的用例清点（2026-10-13）

| 层 | 用例 | 说明 |
|----|------|------|
| `examples/justfile` | 24 条配方 | 渲染场景、辅助命令、失败路径、完整验收；旗标集内联在配方顶部 |
| `examples/templates/*.golden` | 6 个 | 每个模板的期望产物，由 `test/examples` 逐个渲染比对 |
| `examples/cases/errors/` | 6 个 | 失败路径，`test/examples` 断言退出码 + stderr 片段 + stdout 为空 |
| `test/examples` | 10 个测试 | 6 条结构守护（规则族、旗标、命名、维度、formatter、解析标记）+ 4 条渲染守护（自带旗标集，不经 justfile） |
| `test/e2e/testdata` | 60 个 fixture | CLI 精确契约；本轮补 `filters_only_ignore`、`filters_ignore_root`、`unknown_keywords_error`、`log_level_debug`、`in_place_long_flag`，并把 `toYaml N/align` 并入 `pipeline_inline_type` |

**两层互不引用**：`examples/justfile` 的旗标集内联在配方顶部；`test/examples` 自己声明一份等价的旗标与环境。
两边各自完整、都不读对方的文件——这是有意的重复（示例是标准答案，Go 守护只在自己那层成立）。

**本轮发现的两个真实问题**（均已修）：
1. 示例的 golden 此前只在手工 `just demo` 里比对，`go test` 跑不到——`just check` 发现不了示例被改坏。
   现在 `test/examples/render_test.go` 真正渲染每个模板并与 golden 比对，并校验解析标记。
2. `examples/cases/errors/missing` 被共享的 `--on-missing=empty` 掩盖，从未真正触发 exit 6；
   现在该案例显式覆盖为 `--on-missing=error`（`failures` 配方与 Go 测试各自声明）。

### 8.5 仍然存在的缺口（未解决，如实记录）

> 2026-10-22 更新：下列两条已闭环，过程与判据见
> [`05-06-verification-todo.md`](05-06-verification-todo.md)。

- ~~`--explain` 报告里 `shadowed` 链的**顺序**没有独立断言（只有内容断言）~~ → **已闭环**：
  `internal/render/explain_test.go` 的 `TestExplainTemplateShadowedOrder` 断言 4 帧 shadowed
  的完整 JSONL，e2e 的 `TestExplainShadowedOrder` 用真实 CLI 解码后逐帧比对
- `internal/app` 与 `cmd/envsub` 的装配仍主要靠 e2e 覆盖（缺口 3）
- ~~引擎 fixture 只经缓冲 `Render` 路径~~ → **已闭环**：每个 fixture 现在都跑缓冲与流式两条路径
  并逐字节比对；`parallel/parallel_window_split` 用 `meta.yaml` 的 `repeat` 触发并行路径
- `test/perf` 只与 GNU envsubst 在同一台机器上对比，没有跨平台基线

## 9. 示例层重构（已实施，2026-10-13）

示例层（`examples/`）已重构为共享 `data/` + 逐模板 golden + 按"回答什么问题"组织的 24 条 justfile 配方；
`test/e2e` 的期望文件命名已对齐（`expect.exit`）。方案、实施小结与两条待定规则见
[05-05 示例与测试架构](05-05-examples-and-test-architecture.md) §10。

## 10. 非目标（本部分）

- 本矩阵不覆盖 P1/P2 功能。`[FACT]`
- 不替代代码覆盖率与人工评审。`[INFERENCE]`
- 不承诺所有 fixture 都能自动生成；复杂 case 需人工维护。`[FACT]`
