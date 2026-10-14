---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-12
---

# 06-17 架构评审：核心渲染路径 vs 服务于它的非核心功能

> 判据（用户 2026-10-12 指示）：**以核心功能为准**。核心 = `值来源装载 → 合并成值树 → schema 类型化 →
> 模板替换 → 写输出`；其余（`--explain`、`--validate-only`、批量输出、formatter 家族、未来的 HTTP/RPC 输入、
> 多引擎、`--dry-run` 等）都是**服务**于核心的能力，不得反过来驱动核心的结构。
> **状态（2026-10-12）**：D3→D4→D2→D1→D6→D5→D7 已全部执行，见 §2；`just check` 与 `just test-all` 全绿，
> 用户可见行为未变（语法、退出码、旗标、输出格式均不动）。

## 0. 核心路径现状（我读出来的数据流）

改造前（评审起点）：

```text
cmd/envsub/main.go
  └─ internal/app.Run                     26 行：建 Registry + 执行 CLI
       └─ internal/cli                    4 文件 228 行：cobra 树、flag → render.Request、**直接调用 render.Run**
            └─ internal/render.Run        670 行 + explain.go 267 行：
                 ├─ buildValues            载入 source 的树 → schema.ApplyDefaults/Materialize/Validate → values.Tree
                 ├─ resolveTemplates       把参数/glob/目录展开成 (输入, 输出) 对
                 ├─ newWriter/newStreamTargets  选 stdout/文件/output-dir/in-place
                 ├─ renderOne ──┬─ streamReader → engine.StreamReaderEngine.StreamTo
                 │              ├─ renderBuffered → engine.Render 或 StreamEngine.WriteTo
                 │              └─ emitExplain（读全文，走内存路径）
                 └─ explainRun{sources, tree} 贯穿每一条渲染分支
       └─ internal/engine/extended          738 行 engine.go + scanner/expression/stream/parallel
```

改造后：

```text
cmd/envsub/main.go
  └─ internal/app.Run                    装配：extended.New() + runRender（唯一执行点）→ 退出码
       └─ internal/cli                   cobra 树、flag → render.Request，执行交给注入的 cli.Runner
            └─ internal/render.Run      模板发现 → 输出目标 → （explain 扫描）→ 渲染循环 → 收尾
                 ├─ internal/load       值装载：source → 合并 → schema 默认值/物化/校验 → values.Tree
                 │                      （通过 load.Observer 上报来源，explain 只是其中一个观察者）
                 ├─ resolveTemplates / newWriter / newStreamTargets
                 └─ renderOne ──┬─ streamReader → engine.StreamReaderEngine.StreamTo
                                └─ renderBuffered → engine.Render 或 StreamEngine.WriteTo
       └─ internal/engine             Engine/StreamEngine/StreamReaderEngine/PathEngine 接口 + types.TypeResolver
            └─ extended               词法 → 表达式 → 求值 → pipeline/类型断言/输出（**不 import schema**）
```

规模（非测试行）：`engine` 1981（其中 extended 约 1900）、`values` 1322、`schema` 1102、`render` 1071、
`source` 592、`types` 582、`output` 426、其余 < 360。

## 1. 发现（按"是否损害核心"排序）

### A1｜`engine` 依赖 `schema`：核心求值器被外层契约反向依赖 ★★★

- 事实：`internal/engine`（接口层）import `internal/schema`；`engine.Input` 直接持有 `*schema.Schema`。
  使用点只有两处：`engine.go:293`（`--on-missing=zero`）与 `:315`（inline `| type` 校验），
  实现落在 4 个函数里（`applyType`/`schemaCompatibleMembers`/`normalizeBySchemaMembers`/`zeroValue`，约 180 行），
  全部只用到 schema 的**类型查询**能力。`[FACT]`
- 危害：文档 `03-07` §4 要求 `schema` 是"加载 + 薄适配层"、`engine` 是"接口 + 注册表"，两者应分层；
  现在 schema 是 engine 的编译期依赖，任何 schema 侧改动都会波及核心求值器，且"引擎中立"名不副实。
- 候选：
  - **S1（推荐）** 在 `types` 定义求值器需要的最小接口（如 `TypeAt(path) (Declared, bool)` /
    `ConvertAt(path, text) (any, bool, error)`），`schema.Schema` 实现它，`engine.Input` 持有该接口。
    engine 从此不 import schema；schema 仍可 import values/types（同一方向）。
  - S2 把这 180 行搬到 `schema`（`schema.ApplyInline(...)`），engine 调用它——仍要 import schema，只是搬家。
  - S3 不动，在文档里承认"engine 依赖 schema"。

### A2｜`--explain` 塑造了核心渲染循环的形状 ★★★

- 事实：`renderOne` 的第一个分支是 `if req.Explain.Active()`，它读整份模板、放弃流式、并在渲染前后插入
  `emitExplain`；`explainRun{sources, tree}` 作为参数穿过 `renderOne`；`engine.PathEngine`
  （新增一个引擎能力接口）只为 explain 存在，`render.Run` 无条件构造 `explain` 值。`[FACT]`
- 危害：核心路径（流式、免缓冲）被一个诊断功能加了分叉；`explain` 关闭时这些参数仍然存在。
  读核心的人必须先理解 explain 才能读懂渲染循环。
- 候选：
  - **E1（推荐）** 把 explain 变成**渲染前后的独立一次扫描**：`Run` 在循环外先取模板文本 → 若是
    `--explain` 则用 `engine.PathEngine` 收集路径；随后渲染循环**不再感知 explain**（去掉 `explainRun`
    参数与 `renderOne` 里的分支），最后统一输出报告。stdin 只读一次的限制用"explain 时仍走内存路径"
    这一条固化在 `Run`（而不是散在 `renderOne`）。
  - E2 explain 做成 `engine.Input.Observer` 回调（求值时顺带上报），引擎不认识 explain 但也不认识 Observer 之外的东西。
  - E3 保持现状，只在文档里标注"explain 是渲染循环的一等分支"。

### A3｜未接线的扩展点：`Registry` / `Capabilities` / `ErrNotImplemented` / HTTP-RPC 输入 ★★

- 事实：
  - `Registry` 是"名字 → 引擎"的 map + `sync.RWMutex`，但全仓库只注册一个引擎，
    `render.Run` 硬编码 `engine.NameExtended`；没有 `--engine` 旗标；`app.Run` 只调用一次 `Register`。`[FACT]`
  - `Capabilities()` 在生产代码中**0 个调用点**（只有 `engine_test.go` 读 `TypeValidation`）；
    5 个字段里 `Defaults`/`Arrays`/`Escaping`/`ControlFlow` 无人读。`[FACT]`
  - `engine.ErrNotImplemented` **0 个引用**；真正在用的同类哨兵在 `source.ErrNotImplemented`。`[FACT]`
  - `source.HTTPInput`/`RPCInput` 的 `Open()` 恒返回 `ErrNotImplemented`，`TemplateInput(arg)` 也**从不构造它们**
    （P1 预留）。`[FACT]`
- 危害：为一个引擎维护了注册表与互斥锁（核心路径上每次渲染一次 RLock）；为未实现的能力维护了公开接口与
  死代码；新人无法分辨"已接线的能力"和"占位的形状"。
- 候选：
  - **X1（推荐，分两档）** ① 删除 `Capabilities` 整个类型与接口方法、删除 `engine.ErrNotImplemented`；
    ② 把 `Registry` 降为"包内单例 + `New()`"，或干脆 `render.Run` 直接持有 `engine.Engine`（去掉 map 与锁）。
    保留 HTTP/RPC 的**接口形状**（`source.Input` 有价值，已被 File/Stdin 使用），但把两个未实现的
    具体类型移到文档/注释里，或加 `//go:build` 占位标注。
  - X2 反过来：把扩展点补齐（`--engine` 旗标 + `Capabilities` 真正参与能力协商），代价是先做产品决策。
  - X3 不动，仅在 `03-07` 标注"预留未接线"。

### A4｜路径语法知识有两份：`values` 与 `engine` ★★

- 事实：`values` 拥有完整文法（`ParsePath`/`Segment`/`Namespace`/`Canonical`），
  但 `engine.Options.matchesPattern`（`engine.go:101-113`）自己用 `strings.Cut(path, ".")` +
  `IndexByte(root, '[')` 解析"根段"，并自带 `pathSeparator`/`indexOpen` 常量。`[FACT]`
- 危害：`${@process.NAME}` 的根段被算成 `@process`；`${["a.b"].k}` 的根段被算成 `["a`。
  这是**同一个语法的第二份实现**，随语法演进必然分叉（本轮刚给语法加了 `@` 与引导索引）。
- 候选：
  - **P1（推荐）** `values` 导出 `RootKey(path)`（或 `Selector(path)`），`engine` 改调它，
    删除本地常量与手写解析。
  - P2 把 `Only/Ignore` 匹配整体下移到 `values`（连同语义测试），engine 只做布尔判断。

### A5｜`render` 包已经长成"什么都有"的编排层 ★★

- 事实：670 行的 `render.go` 里同时住着 6 类关注点：
  ① 值装载编排（`buildValues`/`loadSchema`/`applySets`/`setOne`）
  ② 输入展开（`resolveTemplates`/`expandTemplateArg`/`listDirectory`/`commonDir`）
  ③ 输出目标选择（`newWriter`/`newStreamTargets`/`newStreamTarget`/`templateOutput`）
  ④ 渲染循环（`Run`/`renderOne`/`renderBuffered`/`streamReader`）
  ⑤ 模板读取与 stdin（`readTemplate`/`templateReadError`）
  ⑥ explain（`emitExplain`）
  import 了 8 个本项目包（`diag engine jsonx logging output schema source utils values`）。`[FACT]`
- 危害：核心流水线的每一步都在这里，但没有边界；单测只能整包起，改动 `output` 语义会牵动 render 的测试。
- 候选：
  - **R1（推荐）** 按关注点拆包，但**只拆值装载**这一个核心步骤（它最有价值、最可测）：
    `internal/values` 增加 `values.Loader` 编排（或新包 `internal/pipeline/load`），
    `render.Run` 退化为"准备 → 逐模板渲染 → 收尾"三段薄编排。
  - R2 全拆：`internal/render` 保留循环；`internal/input`（模板发现）、`internal/target`（输出选择）独立。
  - R3 不动，只把 `render.go` 内部按关注点分组（文件内分区 + 注释）。

### A6｜`internal/app` 名不副实：装配逻辑其实在 `cli`/`render` ★

- 事实：`app.Run` 26 行（建注册表 → 执行 CLI）；而"装配"真正发生在 `cli.renderFlags.request()`
  （把 flag 映射成 `render.Request`）与 `render.buildValues`。`[FACT]`
- 危害：`03-07` §4 写"`app`：应用装配、配置与生命周期"，文档与代码不一致；新人找装配点会去 app 扑空。
- 候选：**A1'** 把 `cli → render.Request` 的映射移到 `app`（CLI 只解析 flag，不再认识 render.Request）；
  **A2'** 改文档，承认 `app` 只是进程入口。

### A7｜同一件事有两套"数字归一"与两套类型表示 ★

- 事实：`values.NormalizeNumbers`（JSON 侧：`json.Number` → int/uint/float/精确整数）与
  `types.Normalize`/`normalizeScalar`（类型侧）都在做"解码后的数字 → 统一 Go 类型"；
  `values` 另有 `codec.ExactInt`/`LiteralNumber` 的消费逻辑，`types.Normalize` 也认这两种类型。`[FACT]`
- 危害：`values.Tree` 里的值类型契约需要两处保持一致，新类型加入时容易只改一处（G20 就是同类问题的历史）。
- 候选：**N1** 把"解码数字 → 统一类型"收敛到 `codec`（它已拥有 `ExactInt`/`LiteralNumber` 与 `ToJSON`），
  `values`/`types` 都调用它；**N2** 明确切分边界（"解码归一"归 values、"类型断言归一"归 types）
  并各加一个对拍测试；**N3** 不动。

### A8｜`source` 同时装"模板输入"与"值来源" ★

- 事实：`source` 里既有 `FileInput`/`StdinInput`/`HTTPInput`/`RPCInput`（模板输入）与 `Loader`/`Environ`/`YAML`/`JSON`
  （值来源）两族类型，名字都叫 source；`render` 两处都要 import 它。`[FACT]`
- 候选：**I1** 拆成 `internal/input`（模板输入）+ `internal/source`（值来源）；**I2** 保持现状，
  在 `source` 包注释里显式分两族。

### A9｜测试基建的重复与体量 ★

- 事实：`internal/engine/extended` 1947 行测试（其中 `fixture_test.go` 255 行自建 harness + `testdata/` 数十个
  目录），`test/e2e` 另有 42 个 fixture；两套 fixture 机制（包内 YAML/目录 vs e2e args/expect）语义相近。`[FACT]`
- 候选：**T1** 让包内 fixture 只覆盖"引擎语义"，跨包行为统一由 e2e 覆盖，删除重叠目录；
  **T2** 抽一个共享的 fixture 读取器（`internal/testutil`）；**T3** 不动（当前 `just check` < 20s，无压力）。

## 2. 执行结果（2026-10-12，按 D3 → D4 → D2 → D1 → D6 → D5 → D7 顺序）

| # | 决策 | 结果 | 证据 |
|---|------|------|------|
| D3 | 未接线扩展点 | ✅ **已执行**：删除 `engine.Capabilities` 与 `engine.ErrNotImplemented`；`Registry`/`NewRegistry`/`Register` 整体删除，装配改为 `cli` 接收一个引擎；`source.HTTPInput`/`RPCInput` 占位类型与 `hasScheme` 删除（`TemplateInput` 只认 file/stdin）；`internal/engine` 175→174 行但删掉整个注册表与能力面 | `TestExtendedEngineImplementsPipelineInterfaces`；`internal/source/input_test.go` 改为"非 stdio 一律当路径"；`03-07` §4、`03-09` §2 同步 |
| D4 | 路径语法单一来源 | ✅ **已执行**：新增 `values.RootKey`，`engine.matchesPattern` 改调它并删除本地 `pathSeparator`/`indexOpen` 手写拆分；带 `.` 的 pattern 用 `filterKey`（根路径不带引号、命名空间保留 `@`）比较 | `TestRootKey`（9 例）、`TestReplacesMatchesGrammarRoots`（8 例）；`04-cli` §6.4 新增匹配规则表 |
| D2 | explain 退出核心循环 | ✅ **已执行**：`--explain` 变为循环**之前**的 `reportSources` 一次扫描，模板文本只读一次并存入 `template.Text`；`renderOne` 不含 explain 分支与 `explainRun` 参数 | e2e 42 个 fixture 全绿（stdout/stderr 断言均不改）；`13-materialization` §15 记录先后顺序调整 |
| D1 | engine 去 schema 依赖 | ✅ **已执行**：`types.TypeResolver` + `types.TypeResolverFuncs` + `types.SchemaTypeToDeclared`；`engine.Input.Schema` 改为该接口；`extended` 的 `applyType/zeroValue/schemaCompatibleMembers/normalizeBySchemaMembers` 改收接口。**`internal/engine` 与 `internal/engine/extended` 的依赖里已无 `internal/schema`** | `go list` 依赖核对；`TestSchemaTypeToDeclared`、`TestTypeResolverFuncs` |
| D6 | app 装配职责 | ✅ **已执行（方式调整，见 §2.1）**：`cli` 不再调用 `render.Run`，改为接收注入的 `cli.Runner`；`app.runRender` 是唯一的执行点 | `TestRunnerReceivesAssembledRequest` 钉住 flag→Request 装配；`03-07` §4 同步 |
| D5 | render 拆包 | ✅ **已执行**：新增 `internal/load`（`Spec`/`Result`/`Observer`/`Origin` + `Load`），承接值装载、schema 默认值/物化/校验与 `--set` 应用；`render` 只留模板发现、输出目标、渲染循环与 explain 报告。`render.go` 691→559 行，新增 `load.go` 242 行 | `internal/load/load_test.go`（层序、dotenv 拒绝、@process 隔离、来源上报 4 组） |
| D7 | 数字归一 | ✅ **已执行（先文档+对拍，不搬代码）**：在 `values.normalizeNumbers` 与 `types.Normalize` 上写明各自边界；新增 `test/encoding/number_paths_test.go` 对拍 12 种字面量（含 2^63、2^64、超 uint64、高精度小数、`-0.0`） | `go test ./test/encoding/` |

### 2.1 对推荐做的两处调整（需知悉）

| 项 | 推荐原文 | 实际做法 | 原因 |
|----|----------|----------|------|
| D6 | "把 flag→Request 的映射移到 app" | 只把**执行**移给 app：`cli` 仍负责 flag→`render.Request` 的解析，但渲染由注入的 `cli.Runner` 执行 | 把映射搬进 `app` 需要把 `renderFlags` 一族（input/value/output/policy/explain 五个结构）导出成 `cli` 的公开 API，边界反而更差；`app` 现在的职责（选引擎、注入执行器、错误→退出码）与 `03-07` 的"应用装配"一致 |
| D2 | 报告"渲染后写出"保持不变 | 报告改为**渲染前**写出 | 模板文本只能读一次（stdin 不可回绕），先读才能让渲染复用同一份文本；e2e 对 stdout 与 stderr 分别断言，顺序变化不改变任何断言结果，且 `--validate-only` 下顺序与文档一致 |

## 3. 原始决策清单（推荐已标注）

| # | 决策 | 推荐 | 影响面 | 预估 |
|---|------|------|--------|------|
| D1 | `engine` 是否去掉 `schema` 依赖 | **S1**：`types` 定义最小类型解析接口 | `engine`、`schema`、`extended`、约 180 行搬迁 + 接口 | 中 |
| D2 | `--explain` 是否退出核心渲染循环 | **E1**：独立扫描 + 循环不感知 explain | `render.Run/renderOne`、`engine.PathEngine` 保留 | 小-中 |
| D3 | 未接线扩展点如何处置 | **X1**：删 `Capabilities`/`engine.ErrNotImplemented`，`Registry` 降级为直接持有引擎 | `engine`、`app`、`cli`、`extended` 签名 | 小 |
| D4 | 路径语法是否只有一份 | **P1**：`values.RootKey`，`engine` 改调 | `engine.Options` + 其单测 | 小 |
| D5 | `render` 是否拆包 | **R1**：只拆值装载 | `render` → `values`（或新包） | 中 |
| D6 | `app` 职责是否回归 | **A1'**：装配移到 `app`（或改文档 A2'） | `cli`、`app`、`render` | 小-中 |
| D7 | 数字归一是否收敛 | **N2**：明确边界 + 对拍测试（先不搬代码） | `values`、`types` 测试 | 小 |
| D8 | `source` 是否拆分 | **I2**：先注释分层，拆包留到 P1 | 无代码改动 | 极小 |
| D9 | 测试基建是否收敛 | **T3**：暂不动（`just check` < 20s） | 无 | 极小 |

## 4. 非目标（Non-Goals）

- 本轮不改任何**用户可见行为**（语法、退出码、旗标、输出）；纯结构重构，`just check` / `just test-all` 必须保持全绿。`[FACT]`
- 不引入新依赖、不改 `go.mod`。`[FACT]`
- 不为"将来可能需要"保留无人调用的抽象（A3 若选 X1 即执行此原则）。`[FACT]`
- 不在本文件决定 P1/P2 功能（include、条件循环、profiles、watch、远程 secret）。`[FACT]`

## 5. 候选方案与被拒理由（摘要）

| 方案 | 内容 | 结论 |
|------|------|------|
| 全拆包（R2） | 一次性把 render 拆成 input/target/render/load 四个包 | **暂拒**：收益真实但一次改动跨 20+ 文件，风险与收益不成比例；先做 R1 验证边界 |
| 扩展点补齐（X2） | 现在就做 `--engine` + 能力协商 | **拒**：先有第二个引擎/第二种模板语法的真实需求，再付出这个成本 |
| 把 render 值装载留在原处（D5 不做） | 只加注释分组 | **备选**：若不想动包边界，这是可接受的最小方案 |

## 6. 复现与证据命令

```bash
export GOCACHE=/tmp/gocache GOTMPDIR=$PWD/.tmp/gotmp
# 依赖图
go list -f '{{.ImportPath}} -> {{join .Imports " "}}' ./...
# 死路径与调用点
grep -rn "Capabilities()" internal/ --include=*.go | grep -v _test      # 0 个生产调用点
grep -rn "engine.ErrNotImplemented" internal/ --include=*.go            # 0 个引用
grep -rn "reg.Get" internal/ --include=*.go | grep -v _test             # 只用 NameExtended
# 规模与测试
for d in internal/*/; do find "$d" -name '*.go' ! -name '*_test.go' -exec cat {} + | wc -l | tr '\n' ' '; echo "$d"; done | sort -rn
go test -count=1 ./...                                                  # 各包耗时
```
