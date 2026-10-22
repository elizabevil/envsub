---
status: active
owner: 技术负责人
version: 0.1
last-updated: 2026-10-22
---

# 验证强化代办清单

> 本文件既是本轮的实施清单，也是执行时的进度看板。每完成一项就把 `[ ]` 改成 `[x]`。
> 状态图例：`[ ]` 未开始 · `[x]` 已完成 · `[!]` 发现问题，详见「待决项」
> 关联：[`05-02 P0 测试矩阵`](05-02-p0-test-matrix.md)（缺口登记）、[`06-17 架构评审`](../99-archive/06-review/06-17-architecture-review.md)（最近一次架构结论）

## 0. 背景

`envsub` 已到 tag `v0.1.0`，试用发布已完成（本轮不碰发布）。基线实测为绿：`go build ./...`、`go vet ./...`、`go test -count=1 ./...` 全部通过，27 个包 PASS。

本轮定位是**给已交付的功能补验证**，不加新功能、不做发布。仓库文档自己登记的验证缺口，逐条核实后确认全部属实：

| 缺口 | 文档登记位置 | 核实结果 |
|---|---|---|
| 表达式解析器无 fuzz target | `05-02:19`（测试策略要求「模板扫描器、**表达式解析器**、`.env` 解析器」） | 属实。`parseExpression` 位于 `internal/engine/extended/expression.go:71`，未被任何 fuzz 目标触达 |
| 数值解析器无 fuzz target | `05-02:172`、`05-01:212`、`02-05:137` 均声称有「扫描器/表达式/`.env`/**数值**」4 个目标 | 属实。实际 4 个是 扫描器/流式span/路径文法/`.env`，**表达式**与**数值**两格为空 |
| `--explain` shadowed 链顺序无独立断言 | `05-02:248` | 属实。所有 e2e fixture 的 `shadowed` 数组只有 0 或 1 个元素，顺序是空集；唯一断言顺序的 `TestExplainerSource` 走内部 API 且只比 `frame.Source` |
| 引擎 fixture 只走缓冲 `Render` 路径 | `05-02:249` | 属实。`fixture_test.go` 只调 `Render`；`StreamTo` 仅被 `stream_test.go` 里的一个模板覆盖过 |
| 退出码 1 无 e2e 覆盖 | `05-03` §11（记为「已知缺口」） | 属实。`TestRunExitCodes` 注释明写「1 is an internal error that has no stable trigger」 |
| CI 不跑 fuzz | TASK-106 验收「CI 里跑短时版本」 | 属实。`ci.yml` 的 test job 只有 `go test ./...`，那只执行种子 |

顺带发现三处：README 严重过时（漏 6 个旗标、漏退出码 7、计数为 P0 期快照）、`diag.NotImplemented` 是 0 调用点的死代码、`envsub bogus`（拼错子命令）实际退出 1 而非文档约定的 2。

## 1. 总体约束

- 不推 tag、不建 release、不改 `release.yml`、不做任何 GitHub 发布动作
- 不实现 P1 功能（include / 条件循环 / profiles / `--diff`），不做 P2 兼容模式
- 不新增第三方依赖；不引入 `internal/testutil` 之类公共测试辅助包（仓库现状是每个包自带局部 helper）
- 预存的 `go.mod` / `go.sum` 依赖升级保持原样不提交
- 改动用户可见行为前先讨论（[`../prompts/_common.md`](../prompts/_common.md) §7.10）

---

## 2. 代办清单

### V-1 补齐缺失的两个解析器 fuzz 目标

> [!] **本项发现并修复了一个真实的文法分歧**：`FuzzParseExpression` 上线即命中——`isBareIdentifier` 用 `isIdentifierByte` 判断**首字节**，而该函数的语义是「可以*继续*一个标识符」，于是 `${0}`（数字开头）被当成合法引用，直到解析阶段才被值路径文法拒绝。扫描器的 `$NAME` 规则（`scanner.go:169`）与 `simpleSpanName`（`engine.go:774`）用的都是 `isIdentifierStart` 判断首字节——**只有这一行是例外**。已改为 `isIdentifierStart`，两个文法就此一致，`${0}` 现在与文档记载的 `${!A}`（`03-operators.md:46`）报同一条错（`expected identifier`，退出码 3，均为模板类错误，退出码未变）。同时补了 `expression_test.go` 用例与 `testdata/errors/digit_leading_name/` fixture。

- [x] **V-1.1 `FuzzParseExpression`**
  - 文件：新建 `internal/engine/extended/expression_fuzz_test.go`，`package extended`（包内测试，与同目录 `expression_parse_test.go:1`、`expression_bench_test.go:1` 一致）
  - 为何放包内而非 `test/fuzz/`：`parseExpression` 未导出，而仓库认可的桥接模式（`internal/render/export_test.go`）只在同包内有效；仓库已明确拒绝为测试导出生产符号（`06-17` D3 删掉 `engine.ErrNotImplemented`）。已有 4 个目标能留在 `test/fuzz/`，是因为其主体 `Scan`/`ScanSpans`/`ParsePath`/`ParseDotenv` 本就导出
  - 主体：`parseExpression(raw)`（`expression.go:71`）
  - 断言（仅成功解析时）：
    - `expr.Path` 非空，且 `values.ParsePath(expr.Path)` 成功
    - `expr.Op` ∈ {"", ":-", "-", ":+", "+", ":?", "?", ":=", "="}（`expression.go:15-24`）
    - `expr.Op == ""` ⟹ `expr.Word == ""`
    - `expr.HasType` ⟹ `expr.Declared.Name != ""`
    - `expr.Steps` 中不出现 `type` 步骤（类型存于 `Declared` 而非 `Steps`），步骤名均在 pipeline 白名单内（`internal/pipeline/pipeline.go:15-24`）
    - 不 panic（fuzz 引擎默认覆盖）
  - 种子：取自 `expression_parse_test.go` 的「操作符 × formatter」矩阵
  - 完成判据：`go test -fuzz=FuzzParseExpression -fuzztime=30s ./internal/engine/extended/` 无 crash

- [x] **V-1.2 `FuzzExpressionPaths`**
  - 文件：`test/fuzz/fuzz_test.go` 追加（黑盒，`package fuzz_test`）
  - 主体：`extended.New().Paths(data)`（`internal/engine/extended/explain.go:8`）→ 打通「扫描 → 表达式解析 → 路径规范化」
  - 断言：每个返回路径非空，且能 `values.ParsePath` → `values.CanonicalPath` → 再 `ParsePath` 往返成功
  - 与 `FuzzPathParser` 的分工：后者只测 `values.ParsePath` 的路径文法，本目标测的是表达式解析器
  - 完成判据：`go test -fuzz=FuzzExpressionPaths -fuzztime=30s ./test/fuzz/` 无 crash

- [x] **V-1.3 `FuzzNumberText`**
  - 文件：`test/fuzz/fuzz_test.go` 追加（全部走导出符号，无需包内测试）
  - 覆盖：
    - `types.Assert(text, types.Declared{Name: types.NameInt})`（`internal/types/convert.go:102`）成功时结果类型必须是 `int`/`uint64`/`codec.ExactInt`——**绝不能是 `float64`**（不静默舍入）
    - `types.Assert(text, types.Declared{Name: types.NameFloat})` 成功时结果**绝不能是 NaN/±Inf**
    - `values.NormalizeNumbers(json.Number(text))`（`internal/values/number.go:79`）与 `types.Normalize(json.Number(text))`（`convert.go:407`）不 panic、结果非 NaN/Inf
  - 完成判据：`go test -fuzz=FuzzNumberText -fuzztime=30s ./test/fuzz/` 无 crash

### V-2 把 fuzz 接进 runner 与 CI

- [x] **V-2.1 `test/justfile`**：`test-fuzz` 配方（当前 `:28-33`）增加三条 `-fuzz` 行——`FuzzExpressionPaths`、`FuzzNumberText`（`./fuzz`）与 `FuzzParseExpression`（`../internal/engine/extended`）
- [x] **V-2.2 `internal/engine/extended/justfile`**：新增 `test-fuzz` 配方，供本地单跑（当前该 justfile 无 fuzz 配方）
- [x] **V-2.3 根 `justfile`**：新增 `test-fuzz` 配方转调 `test/justfile`；**不**加入 `check`（7×30s 会让 check 变慢），加入 `test-all`
- [x] **V-2.4 `.github/workflows/ci.yml`**：新增 `fuzz` job，每个目标 `-fuzztime=20s`。这补上 TASK-106 验收里「CI 里跑短时版本」这条一直没兑现的要求
- [x] 完成判据：`just --justfile test/justfile test-fuzz` 全绿；`just --justfile test/justfile fuzz-list` 列出 7 个目标（`test/fuzz` 6 个 + `internal/engine/extended` 1 个）

### V-3 让 fixture 矩阵覆盖流式与并行路径

- [x] **V-3.1 每个 fixture 多跑一遍流式路径**
  - 文件：`internal/engine/extended/fixture_test.go` 的 `runFixture`
  - 缓冲 `Render` 断言通过后：把模板写到临时文件、以 `*os.File`（可 seek）打开，调 `extended.New().StreamTo(ctx, &buf, file, engine.Input{Name:…, Values: 新树, SourceSize: len, SourceSizeKnown: true})`，断言输出与 `expect.out` **逐字节相同**
  - 必须用**新树**：赋值操作符会把 `values.Tree` 标记为 mutated（`internal/values/tree.go`），会影响并行判定
  - 失败用例断言退出码一致；**错误文本**是否一致**先实测再定**——若 `StreamTo` 与 `Render` 的位置信息不同（例如文件名或行号未挂上），则只断言 kind + 退出码，并把该差异记为一条发现，不掩盖
  - 覆盖价值：163 个模板（含语法错误、缺值、schema 失败等异常路径）第一次全量过流式渲染；目前该路径只被 `stream_test.go` 的一个模板覆盖过
  - 完成判据：`go test -run TestFixtures ./internal/engine/extended/` 全绿

- [x] **V-3.2 并行路径的 fixture**
  - `meta.yaml` 新增可选键 `repeat: N`（默认 1）：模板被重复 N 份、期望输出是 `expect.out` 重复 N 份
  - 约束：`repeat > 1` 仅允许用于成功用例（`exit_code == 0`）；失败用例重复 N 次会产生 N 个不同行号的错误，语义混乱，runner 应直接报错拒绝
  - 新增 `internal/engine/extended/testdata/parallel/parallel_window_split/`：小模板 + `repeat: 20000`（约 2 MB → 8 个 256 KiB 窗口）触发并行路径。前置条件（`internal/engine/extended/parallel.go:15-24`、`:74-126`）：已知长度、可 seek、无 `:=`/`=` 赋值、≥4 窗口
  - 更新 `internal/engine/extended/testdata/README.md`：写明 `repeat` 语义，以及这是「模板必须写在 `input.tmpl`」规则的**显式例外**（仅此一处）
  - 完成判据：新 fixture 通过，且 `expect.out` 的重复语义与实际输出一致

### V-4 钉住 `--explain` 的 shadowed 顺序

- [x] **V-4.1 `internal/render/explain_test.go`**：新增 `TestExplainTemplateShadowedOrder`
  - 构造 5 层叠加：schema default → values 文件 → dotenv base → dotenv debug → `--set` 胜出
  - 断言**完整 JSONL**：证明 `shadowed[0]` 是最近的败者（debug.env）、最后一项是最低层（schema default），即「Shadowed 后接 Origin 就是整条血缘，最低优先级在最后」（`internal/render/explain.go:44-46`）
- [x] **V-4.2 `test/e2e/testdata/explain_shadowed_order/`**：用真实 CLI 叠同样的层（`--schema` + `--values-file` + 两个 `--dotenv-file` + `--set`），`expect.err` 写完整报告行
- [x] **V-4.3 `test/e2e/e2e_test.go`**：再加一个 Go 测试 `TestExplainShadowedOrder`，JSON 解码后逐项断言 `shadowed` 数组顺序
  - 必要性：e2e 的 `expect.err` 是**子串**匹配，单独靠它钉不住顺序（该缺口已登记于 `05-02:248`）；参照既有 `TestExplainFramesFollowADR005`（`test/e2e/e2e_test.go:521-619`）的写法
- [x] 完成判据：`go test ./internal/render/ ./test/e2e/` 全绿

### V-5 补上退出码 1 的覆盖

- [x] **V-5.1 `internal/app/app_test.go`**：`TestRunExitCodes` 增加一条 `want: 1` 用例（拼错的子命令），并加注释说明 cobra 的 unknown-command error 不带 `diag` kind，故 `diag.ExitCode` 回落到 internal（`internal/diag/errors.go:35-43`）
- [x] **V-5.2 `internal/diag/errors_test.go`**：~~补 `diag.ExitCode(errors.New(...)) == 1`~~ —— **原判断有误**：`TestExitCode` 的 `plain` 用例（`errors.New("boom")` → 1）**已经**覆盖了「未分类错误回落 internal」这条分支，无需再加。本项因此关闭但不改代码。
- [x] **V-5.3 `test/e2e/testdata/unknown_command/`**：`expect.exit: 1`
- [x] 完成判据：退出码 1 在 app 层、e2e 层有覆盖，diag 层原本已覆盖

### V-6 让文档与代码一致

- [x] **V-6.1 `README.md`**
  - 更正计数（`:5-7`）：340 测试函数 + 7 fuzz 目标、165 引擎 fixture、62 e2e、17 基准、27 个包
  - 补 CLI 段（`:51-78`）漏掉的 6 个旗标：`--set-file`、`--strip-suffix`、`--check`、`--report=unused`、`--log-level`、`--error-format`，以及 `completion` / `help` 子命令
  - 补退出码 7（`:80-81` 现只列 0-6）
  - 改掉 `:176` 的「outstanding P0 work is TASK-026 plus the Fuzz targets」（TASK-026 已完成、fuzz 已覆盖）
  - 修 P1/P2 段（`:194-198`）：`--check` 已发布、`--dry-run` 已删、`--diff` 是 ⏸ 待需求
- [x] **V-6.2 fuzz 描述对齐**：[`../02-product/02-05-requirements-registry.md`](../02-product/02-05-requirements-registry.md)`:137`、[`05-01`](05-01-p0-task-breakdown.md)`:212`、[`05-02`](05-02-p0-test-matrix.md)`:172`——把「4 个 fuzz 目标（扫描器/表达式/`.env`/数值）」改成与代码一致的清单
- [x] **V-6.3 缺口状态更新**：[`05-02`](05-02-p0-test-matrix.md)`:246-251` 四条缺口改为新状态（顺序、流式/并行、退出码 1 已闭环；app/cmd 装配仍靠 e2e 这条不变）
- [x] **V-6.4 退出码矩阵**：[`05-03`](05-03-pending-work.md) §11 补上 1 的覆盖状态
- [x] 完成判据：README 与 `dist/envsub render --help` 的旗标逐条对齐；文档中的 fuzz 目标数与 `fuzz-list` 输出一致

> 附带修正：README 改口「P0 已完成」后，[`05-01`](05-01-p0-task-breakdown.md) 的 TASK-026 状态行、进度抬头、缺口清单与合计数也一并更新，否则两份文档会互相矛盾。

### V-7 清理死代码

- [x] **V-7.1** 删除 `internal/diag/errors.go:71-74` 的 `NotImplemented`（0 调用点）。同类清理先例：`engine.ErrNotImplemented` 已在 `06-17` D3 以同样理由删除
- [x] **V-7.2** 确认 `internal/diag/errors_test.go` 无对应用例（已核实无）
- [x] 完成判据：`go build ./...` 与 `go vet ./...` 通过，lint 无新增告警

### V-8 流式渲染的长行死循环（本轮实测发现并修复）

> [!] **这是评估「stream 是否满足流式使用」时实测出来的真实缺陷**：模板里只要有一行**超过窗口大小**，流式渲染器就会 **99.5% CPU 无限空转**，永不返回（RSS 平在 13 MiB，是纯自旋）。缓冲路径（`--validate-only`）渲染同一模板正常退出 0，所以这是两条路径的**分歧**，不是模板本身的问题。

- [x] **V-8.1 根因**（`internal/engine/extended/stream.go` 的 `streamWindow`）：

  ```go
  for !eof && len(buffer) < windowSize { ...读... }   // ① 只在没满窗口时读
  ...
  cut := bytes.LastIndexByte(buffer, '\n')
  if cut < 0 {
      if len(buffer) >= streamMaxCarry { return 报错 }  // ② 4 MiB 上限
      continue                                        // ③ 回到循环顶部
  }
  ```

  ③ 回到顶部时 ① 的条件已经不成立 → **再也不读一个字节** → buffer 不再增长 → ② 那个 4 MiB 上限**永远够不到** → 死循环。阈值就是窗口大小：顺序流式 1 MiB（`streamWindowSize`），并行的测量阶段 256 KiB（`parallelChunkSize`）。

- [x] **V-8.2 用测试证明**（`internal/engine/extended/stream_longline_test.go`）：4 个测试，每个都给渲染设 10s 期限，让死循环变成失败而不是挂死测试套件。
  - `TestStreamToLongLineTerminates`：单行 1.5 MiB（超过顺序窗口）→ 必须终止且输出与缓冲渲染逐字节相同
  - `TestStreamToLongLineInLargeTemplateTerminates`：单行 300 KiB + 整个模板超过 256 KiB（走并行测量路径）→ 同上
  - `TestStreamToRejectsLineLongerThanCarryLimit`：单行 5 MiB → 必须报「single line longer than」而不是自旋（证明 carry 上限从不可达变为可达）
  - `TestRenderAcceptsLongLine`：缓冲路径渲染同一模板正常（钉住两条路径的分歧边界）

  **未修复代码上的实测结果**：前 3 个测试全部以 `StreamTo() did not finish: context deadline exceeded` 失败（各耗时 10s），第 4 个也失败——它先抓出了**我自己测试断言里的一个错误**（`${NAME}` 替换后输出比模板短 200 字节，我却断言等长）。

- [x] **V-8.3 修复**：`streamWindow` 增加 `carry` 标志——buffer 里还没有行尾时就继续读（允许超过窗口大小增长），并把 carry 上限的判断挪进读循环，使 `streamMaxCarry` 真正可达。修复后：5 MiB 单行 → 退出 3，报 `template contains a single line longer than 4194304 bytes`；2 MiB 单行 → 正常渲染。

- [x] **V-8.4 fuzz 目标**（`internal/engine/extended/stream_fuzz_test.go`，`FuzzStreamWindow`）：任意字节喂给窗口读取器，必须终止。
  - 教训：fuzz 参数必须小。最初把 64 KiB / 256 KiB 的长行直接作为**种子**，Go 的 fuzzing 引擎会花全部时间去最小化这些种子（执行速率从 3,880/s 掉到 0/s）。改成 fuzz `(data, repeat)`、在测试体内用 `bytes.Repeat` 拼出长行后，语料保持小、执行速率恢复正常（25s 内 128,036 次执行、新增 26 个 interesting 输入）。

- [x] **V-8.5 内存没有回归**（修复后复测，模板 50/200 MB）：

  | 模板 | 并行（文件路径） | 顺序流式（stdin） | 缓冲路径 |
  |---|---|---|---|
  | 50 MB | 43.6 MiB | 13.0 MiB | 123.9 MiB |
  | 200 MB | 52.7–67.1 MiB（连测 5 次） | 8.8–12.8 MiB | 423.6 MiB |

  顺序流式的峰值内存与模板大小**无关**（50 MB 与 200 MB 模板都是 9–13 MiB），这才是真正的流式；并行路径收敛到约 55 MiB 的平台；缓冲路径线性增长。**结论：流式的内存模型成立，V-8 修掉的是它唯一的正确性缺陷。**

---

## 3. 待决项（本轮不擅自改，交产品负责人定）

- [!] **未识别的子命令该退出 1 还是 2（文档没有裁定）**：`envsub bogus`（拼错子命令）当前退出 **1**。逐条核对三处文档后，**当前行为并没有违反契约**：

  - [`../03-design/03-08-cli-design.md`](../03-design/03-08-cli-design.md) §5 的退出码表把 1 写成「**通用/未分类错误**」、2 写成「CLI 用法错误」，并注明「退出码 1–6 由 `diag.Kind` 映射」；
  - [`../07-serialization/08-errors.md`](../07-serialization/08-errors.md) 的 kind 表里，`usage`(2) 列举的触发条件全是 flag 值、赋值语法、类型名、扩展名，**没有**「未知子命令」；`internal`(1) 的触发条件写的是「未分类内部错误」；
  - cobra 的未知子命令错误是裸 error，**不带 `diag.Kind`**，正好落进「未分类」→ 1。

  所以这不是「实现偏离文档」，而是「**文档之间措辞不一致**（README 写「1 internal」，设计文档写「通用/未分类错误」）+ 文档没裁定未知子命令属于哪一类」。README 的措辞已按设计文档改齐。

  真正要产品定的是：拼错子命令对脚本来说算「用法错」还是「内部错」，两种都说得通：(a) 把 unknown-command 归类为 `diag.KindUsage` → 退出 2；(b) 保留 1。改退出码属用户可见行为变更，按 [`../prompts/_common.md`](../prompts/_common.md) §7.10 需先讨论，故本轮只补覆盖（V-5）不动行为。
- [!] **公共测试辅助包**：仓库现状是每个包自带局部 helper（`writeFile` 在 3 个包里各写一份、`metaSegments` 在 2 个包里各写一份）。本轮沿用现状不合并，若要顺手收敛可另开一项。

## 4. 验收门禁

执行完成后逐条跑，全绿才算收尾：

- `just check`（gofmt + goimports + vet + golangci-lint v2 + 单测/fixture/e2e/examples）
- `go test -race -count=1 ./...`
- `go test -tags=sonic -count=1 ./...`
- `just --justfile test/justfile test-fuzz`（7 个目标各 30s）
- `just test-perf`（性能门禁不受影响）
- `just size`（不新增依赖，二进制体积不变）
- `git status` 确认只有预期文件被改

### 实测记录（2026-10-22）

| 门禁 | 结果 |
|---|---|
| `just check` | exit 0（golangci-lint v2.14.0：**0 issues**）|
| `go test -race -count=1 ./...` | exit 0，28 个包全过 |
| `go test -tags=sonic -count=1 ./...` | exit 0 |
| `just --justfile test/justfile test-fuzz` | 7 个目标各 30s 全过，无 crash |
| `just test-perf` | render 0.39× GNU、峰值 RSS 1.11× GNU、启动 4.15 ms（预算 100 ms），与 GNU 输出逐字节一致 |
| `just size` | linux/amd64 5.03 MiB、sonic 8.55 MiB |

**二进制体积未变**：用 `git worktree` 检出 HEAD 单独构建，字节数与当前工作区**完全相同**（5,275,911 字节），说明本轮改动对二进制的贡献为 0。README 里 4.92 MiB 的旧数字来自 go1.27.2；go1.27.3 下复测为 5.03 MiB，已按新数字更新（并注明体积随 Go 版本浮动，跨工具链不可直接比较）。

### 顺带修掉的两处

- **`tools/lint/golangci-lint.sh` 的工具链 pin 漂移**：脚本硬编码 `GOTOOLCHAIN=go1.27.2`，而 `go.mod` 已是 1.27.3，`just lint` 直接失败（`go.mod requires go >= 1.27.3`）。改为从 `go.mod` 的 `go` 指令推导，两者不会再漂。
- **`testdata/README.md` 与 walker 行为不符**：README 写「没有 `input.tmpl` 的目录会让套件失败」，实际代码是跳过（分组目录 `syntax/`、`defaults/` 等本就没有 `input.tmpl`）。按实际行为改写 README，并说明 `fuzz/` 是 Go fuzz 引擎自己的语料目录。

## 5. 本轮涉及文件清单

**新增**

- `internal/engine/extended/expression_fuzz_test.go`
- `internal/engine/extended/stream_fuzz_test.go`（`FuzzStreamWindow`）
- `internal/engine/extended/stream_longline_test.go`（V-8 的 4 个回归测试）
- `internal/engine/extended/testdata/fuzz/FuzzParseExpression/771e938e4458e983`（Go fuzz 引擎写下的失败语料，作为回归种子长期保留）
- `internal/engine/extended/testdata/errors/digit_leading_name/`（`input.tmpl`、`meta.yaml`、`expect.err`）
- `internal/engine/extended/testdata/parallel/parallel_window_split/`（`input.tmpl`、`values.yaml`、`expect.out`、`meta.yaml`）
- `test/e2e/testdata/unknown_command/`（`args`、`expect.exit`、`expect.err`）
- `test/e2e/testdata/explain_shadowed_order/`（`args`、`input.tmpl`、各值文件、`expect.err`、`expect.out`）
- `docs/05-tasks/05-06-verification-todo.md`（本文件）

**修改**

- `internal/engine/extended/stream.go`（V-8：窗口读取器的 carry 修复）
- `test/fuzz/fuzz_test.go`、`internal/engine/extended/fixture_test.go`、`internal/render/explain_test.go`、`internal/app/app_test.go`、`internal/diag/errors.go`
- `test/e2e/e2e_test.go`
- `test/justfile`、`internal/engine/extended/justfile`、根 `justfile`
- `.github/workflows/ci.yml`（新增 `fuzz` job）
- `tools/lint/golangci-lint.sh`、`.gitignore`
- `README.md`、`docs/02-product/02-05-requirements-registry.md`、`docs/05-tasks/05-01-p0-task-breakdown.md`、`docs/05-tasks/05-02-p0-test-matrix.md`、`docs/05-tasks/05-03-pending-work.md`、`docs/05-tasks/05-00-index.md`
- `internal/engine/extended/testdata/README.md`

> `internal/diag/errors_test.go` 未改：退出码 1 的未分类回落分支本来就由 `TestExitCode` 的 `plain` 用例覆盖。

## 6. 预期风险

- **流式 fixture（V-3.1）可能真的翻出 bug**：165 个模板第一次全量过流式/并行路径，历史上没有这个覆盖。若发现 `Render` 与 `StreamTo` 的输出或错误定位不一致，**报告**而不是把断言改松去掩盖。
- **fuzz 目标（V-1）可能立刻抓到 panic**。抓到了就是真 bug，同样先报告再修。
