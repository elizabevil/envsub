---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-08
---

# 模板输入抽象与并行渲染修复（Round 3：步骤 1 + 步骤 2 预扫描）

> 输入：本轮对话中确认的优化方案（Input 接口 / `:=`/`=` 语义 / 并发边界 / 无锁有序并行 / 实施步骤）。
> 结论：**步骤 1（Input 抽象）与步骤 2（顺序预扫描）已完成**；
> 过程中发现并修复了并行渲染路径的 3 个 P0 缺陷（死锁、窗口别名、路径缓存数据竞争），
> 以及 6 处测试内共享值树导致的既有 `-race` 竞争。
> 步骤 3 的"有界缓冲"与步骤 4 的基准调参仍待办。

## 1. 本轮改动

| 文件                                          | 内容                                                                                                                                                         |
|-----------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `internal/source/input.go`（新增）            | `Input` 接口（`Name`/`Open`/`Size`）+ `FileInput`、`StdinInput` 真实现 + `HTTPInput`、`RPCInput` 空实现（`Open` 返回 `ErrNotImplemented`）+ `TemplateInput(arg)` 分发 |
| `internal/source/input_test.go`（新增）       | 分发选择、文件大小/重复 `Open` 从头开始、stdin 关闭不关进程 stdin、未知大小、远程实现未实现                                                                  |
| `internal/engine/engine.go`                   | `Input` 增加 `SourceSize`、`SourceSizeKnown`，把"能否并行"从 `io.Seeker` 猜测改成输入的显式能力                                                              |
| `internal/render/render.go`                  | 文件/stdin 统一走 `source.TemplateInput`；设置大小元数据；错误文案保持原样（`read template %s` / `read stdin`）                                               |
| `internal/engine/extended/stream.go`          | 并行闸门改用 `SourceSizeKnown`；`countWindows` 替换为 `measureWindows`（一次预扫描同时数块 + 编译路径 + 赋值检测）                                            |
| `internal/engine/extended/parallel.go`        | 修复死锁；窗口切片改为拷贝；删除启发式 `containsAssignment`；新增强制顺序的选项闸门                                                                          |
| `internal/engine/extended/parallel_test.go`（新增） | 并行/顺序逐字节一致、点号路径（无锁回归）、赋值回退、`inspectText` 判定表、闸门判定表                                                                   |
| `internal/engine/extended/engine_features_test.go` | 6 个测试的子测试不再共享值树（既有 `-race` 竞争）                                                                                                        |

不 commit。

## 2. Input 抽象（步骤 1）

```go
type Input interface {
	Name() string                  // 诊断名：路径 / "-" / URL
	Open() (io.ReadCloser, error)  // 每次调用返回从头开始的新流
	Size() (int64, bool)           // false = 未知（stdin / HTTP / RPC）
}
```

- `FileInput`：`os.Open`，`Size()` 用 `os.Stat`（目录或缺失文件报 unknown，错误由 `Open` 给出）。
- `StdinInput`：`Size()` 恒为 unknown，`Open` 返回 `io.NopCloser`，**关闭不会关掉进程 stdin**。
- `HTTPInput` / `RPCInput`：只固定形状，`Open` → `ErrNotImplemented`，未接入 `resolveTemplates`，因此 CLI 行为不变。
- 流式输入（长连接不停输入/不停输出）未实现，仅在接口注释中说明形状：`Open` 返回长期存活的流 + `Size()==false`。

`engine.Input` 新增 `SourceSize`/`SourceSizeKnown`；`StreamTo` 签名保持不变（测试与调用点零改动），
渲染层把 `source.Input.Size()` 的结果填进去，引擎不再靠 `io.Seeker` 猜来源。

## 3. 发现的 P0 缺陷（并行渲染此前从未被真正执行过）

并行分支此前没有任何测试覆盖，第一次触发即暴露 4 个 P0 缺陷。**全部已修复**，且每个都有可复现的失败证据。

| # | 严重度 | 缺陷 | 触发条件 | 现象（修复前） | 修复 | 验证（修复后） |
|---|--------|------|----------|----------------|------|----------------|
| P0-1 | 阻塞 | `streamParallel` 没有任何地方 `close(results)` | 任意 ≥1MB 的文件模板（渲染层传 `*os.File`，`io.Seeker` 成立） | 主 goroutine `for range results` 永不结束 → **进程挂死**，`go test` 一直卡住 | `workers.Wait(); close(results)` 收尾 | 2.0MB/12 万行 CLI 0.024s 完成；`TestParallelOutputMatchesSequential` 通过 |
| P0-2 | 输出错误 | 生产者把 `streamWindow` 的**可复用缓冲区切片**直接交给 worker，随后原地移位覆写 | 同上，多块并行 | worker 读到被覆写的字节 → 输出与顺序渲染不一致（实测 1560000 vs 1548890 字节） | 发送前 `bytes.Clone(window)` | 并行输出与 `Render` 逐字节一致 |
| P0-3 | 崩溃 | worker 调用 `Tree.CompilePath` 会写 `tree.paths` 缓存 | 并行 + 点号/带 pipeline 路径（裸名走快路径不触发） | `fatal error: concurrent map read and map write` | 预扫描（单 goroutine）先编译模板内全部路径；worker 只读 | 去掉预编译后 `TestParallelDottedPathsMatchSequential` 立刻崩溃，加回后通过 |
| P0-4 | 报错/挂死 | 旧 `containsAssignment`/`isOperatorContext` 把"窗口内 4096 字节内出现过 `$` 且出现 `=`"判为赋值 | 最常见的 `KEY=${VAR}`（字面量 `=`）+ ≥1MB 文件 | 每块都返回 `assignment operator requires sequential rendering` → **K8s 风格配置直接报错**；配合 P0-1 亦表现为挂死 | 改由扫描器判定：只有运算符为 `:=` / `=`（含嵌套 `${A:-${B:=x}}`）才算赋值，命中即整份回退顺序 | `TestInspectText` 判定表 + `TestAssignmentTemplateFallsBackToSequential`（2.4MB 含嵌套赋值）通过 |

P0-1 ~ P0-3 表明该分支自写入起从未被真正跑过；P0-4 表明它的"能否并行"判定也是错的。
四者叠加后，**任何 ≥1MB 的文件模板要么挂死、要么报错、要么输出错误**，因此本轮不存在"保持行为不变"的选项（见第 6 节 D4）。

## 4. 预扫描（步骤 2）与并发边界

`measureWindows` 单遍顺序读取，同时完成：

1. 块边界计数（按行切分，256KB/块）；
2. 逐块 `inspectText`：扫描 + 解析每个 `${...}`；
3. 赋值检测（`:=` / `=`，含嵌套）；
4. 路径预编译（`Tree.CompilePath`），**并行阶段 worker 只读**。

决策表（`shouldRenderParallel`）：

| 条件                                                    | 决策                     |
|---------------------------------------------------------|--------------------------|
| 值树已被 `:=`/`=` 修改（`tree.Mutated()`）               | 顺序                     |
| `--only` / `--ignore` / `--unset=keep`                   | 顺序（裸名会走通用路径并编译路径） |
| `Size()` 未知（stdin / HTTP / RPC）                      | 顺序流式，不预读         |
| 已知大小 ≤ 256KB（必然只有 1 块）                        | 顺序（顺带省掉一次重复读） |
| 预扫描发现赋值 / 模板语法错误 / 读错误                   | 顺序（带模板名与行号报错） |
| 其余（文件、≥4 块、无赋值）                              | 并行                     |

预扫描失败一律回退顺序：顺序路径本来就会读一遍，且能给出精确的 `文件:行` 定位。

## 5. 验证结果（2026-10-08，本机）

```text
gofmt -l ./cmd ./internal ./test     # 空
go vet ./...                         # 通过
go test -count=1 ./...               # 通过（160 engine fixture + 17 E2E）
go test -race -count=1 ./...         # 通过（修复前 12 个 DATA RACE）
just check                           # fmt-check + vet + unit + e2e 通过
```

并行路径回归测试（新增）：

- `TestParallelOutputMatchesSequential`：1.9MB / 6 万变量，并行输出与 `Render` 逐字节一致；
- `TestParallelDottedPathsMatchSequential`：4MB / 点号路径 + pipeline + 嵌套默认值，
  **渲染用值树不做预热**，缺预热时该测试会以 `fatal error: concurrent map read and map write` 失败（已实测）；
- `TestAssignmentTemplateFallsBackToSequential`：含顶层与嵌套赋值的 2.4MB 模板回退顺序且输出一致；
- `TestInspectText`：`${V:=d}`/`${V=d}`/`${V:-d}`/`${A:-${B:=x}}`/`${A:-a=b}`/`a=b` 判定表。

CLI 冒烟（`dist/envsub`）：

```text
2.0MB, 12 万行 KEY=${VAR}（字面量 =）          0.024s   120000 行输出正确（修复前：挂死）
2.0MB, 同上 + tail=${NEW:=written}             0.023s   tail=written（顺序回退）
4.0MB, 点号路径 + join + 嵌套默认值             0.439s   输出正确
stdin 渲染                                     正常
```

## 6. 与方案的偏差（供裁决）

方案指本轮对话中确认的优化方案（Input 接口 / `:=`/`=` 语义 / 并发边界 / 无锁有序并行 / 实施步骤）。
下表列出每一处"实际做法 ≠ 方案原文"，含理由、影响与回退成本，供逐条判断。

| # | 方案原文 | 实际做法 | 理由与证据 | 影响 | 回退成本 |
|---|----------|----------|------------|------|----------|
| D1 | 渲染层统一走 `source.Input`；接口注释"不假设可 Seek"，由引擎消费 | `StreamTo` 签名**保持** `src io.Reader` 不变；改为在 `engine.Input` 增加 `SourceSize`/`SourceSizeKnown`，渲染层填入 `source.Input.Size()`。引擎仍用 `io.Seeker` 定位并回退复读做预扫描 | ① 引擎层保持 IO 无关，不引入 `engine → source` 依赖；② `StreamTo` 有测试直接调用，签名不动即"调用点零改动"，符合步骤 1 的零行为变化验收；③ **方案的实质目的已达成**——并行与否不再靠 `io.Seeker` 猜测，而是输入的显式能力。仅"复读做预扫描"这一步仍要求流可回退 | `source.Input.Open()` 的"可重复打开"能力目前只有渲染层用（打开一次），引擎侧未用；将来接 HTTP/RPC 时需二选一：引擎改收 `Input`，或渲染层把流落到临时缓冲 | 小：改签名 + `streamReaderTemplate` 打开两次 + 2 个测试 |
| D2 | 决策表：**已知大小 < 4MB → 顺序**（阈值待基准标定） | 保留既有边界：`≤1 个窗口(256KB)` 直接顺序，`块数 < 4` 顺序；即约 **≥1MB** 就并行，4MB 阈值未生效 | ① 方案自己标注"阈值待基准标定"，属步骤 4；步骤 1 的验收是行为零变化，此时改阈值会让决策边界漂移且无基准依据；② 实测 2MB 并行 0.024s，1–4MB 区间并行有收益；③ 先固定"与旧代码相同的边界"，把阈值变化留给有基准的步骤 4，便于二分定位性能回归 | 1–4MB 模板走并行（旧代码同样如此）；若步骤 4 基准显示小文件并行反而慢，再上调阈值 | 极小：改一个判断 |
| D3 | 决策表：`--validate-only / --unset=keep / --only / --ignore` → 顺序 | `--unset=keep`、`--only`、`--ignore` 已加顺序闸门；**`--validate-only` 未加**，≥1MB 文件模板仍走并行、输出丢弃 | 并行阶段的共享写已消除（无赋值、路径预编译、worker 只读），校验语义与顺序一致；模板类错误在预扫描即被拦下并回退顺序，错误定位不退化；并行更快 | 无功能差异，仅"实现路径与方案表格不同" | 极小：加条件（需把 `opts` 传入闸门） |
| D4 | 步骤 1 验收：**行为零变化** | 提前执行步骤 2（预扫描），并把赋值检测由启发式改为扫描器判定 → **行为确有变化**：≥1MB 且含 `=` 字面量的模板由"报错/挂死"变为"正确渲染"；含 `:=` 的模板整份顺序；语法错误模板回退顺序并带 `文件:行` | 旧启发式（`$` 附近 4096 字节内有 `=` 即算赋值）无法区分字面量与运算符，若不修，步骤 1 的闸门一接通就必然把大文件打挂（实测正是它挡住新增回归测试）；而精确判定恰是步骤 2 的内容，故提前做 | 原先"挂死/报错/错误输出"被"正确渲染"取代；这是修 P0，不可能同时保行为。P0-1~P0-4 见第 3 节 | 不建议回退（会恢复挂死/报错） |
| D5 | 方案未提及改测试 | 修改 6 个**既有**测试：并行子测试不再共享值树（新增 `newTree` 助手） | `Tree.CompilePath` 会写 `tree.paths`，共享值树 + `t.Parallel()` 本身就是数据竞争；方案第 4 点要求"并行阶段不写 `Tree.paths`"，测试侧同类竞争不修则 `go test -race` 必红（修复前 12 个 `DATA RACE`） | 仅改测试构造方式，断言语义不变 | 小；但 `-race` 会重新变红 |
| D6 | 无锁有序并行："`pending` 只缓存乱序到达的块（最多约 2×worker 块，超出即阻塞在 `results` 上）" | **未实现该上界**：writer 持续接收乱序结果进 `pending`，最坏接近整个模板；已列为步骤 3 待办 | ① 方案这句话描述的"有界"在既有代码里并不存在，属于要**新写**的机制，不在本轮"修 4 个 P0 + 步骤 1/2"范围；② 先修正确性（死锁/别名/竞争），把有界缓冲与内存上限测试放步骤 3，可单独验证、单独回退 | 最坏峰值内存可能接近模板大小（典型乱序度低；实测 4MB 点号模板正常）；方案"内存 ≤30MB"目标尚未验证 | 中：需引入 slots 信号量或 baton 传递 |
| D7 | — | 命名与死代码清理：`windowUsesAssignment`→`prepareWindow`、`textUsesAssignment`→`inspectText`；删除 `countWindows`、`chunkJob.baseLine`（原本无人读取） | 预扫描现同时做三件事（赋值检测 + 路径预编译 + 块计数），旧名不再准确；`baseLine` 是死字段，留着会误导 | 无行为影响 | 无 |

未偏离项（与方案一致，故不列入上表）：接口放在 `internal/source/input.go`、HTTP/RPC 空实现与
`Name()/Size()` 取值、`:=`/`=` 语义、预扫描单遍同时得到块边界 + 赋值检测 + 路径预编译、
取消沿用 `context.WithCancel`（首个错误取消生产与其余 worker）、in-place 仍走临时文件 + rename。
`pending` 有界化与基准调参未做，见 D6 与第 7 节。

需注意（非偏差）：`internal/source` 原本是"值来源加载器"（dotenv/yaml/json），
现在同时容纳"模板输入来源"。方案原文即如此命名，本轮照做；若将来要区分职责，
可把 Input 迁到 `internal/template` 或 `internal/input`。`[INFERENCE]`

## 7. 下一步

1. 步骤 3 收尾：`pending` 有界化（`slots` 信号量或 baton 传递），补取消与内存上限测试。
2. 步骤 4：按路径密集 / 纯文本 / 大文件三组基准调 `parallelChunkSize`、worker 数、并行阈值。
3. 步骤 5：把上述结论并入本文档与本索引。

## 8. 待裁决清单

| 裁决项 | 当前实现（A） | 按方案原文（B） | 建议 |
|--------|---------------|-----------------|------|
| D1 引擎消费方式 | `StreamTo` 收 `io.Reader` + 大小元数据 | `StreamTo` 收 `source.Input`，复读改为重新 `Open()` | 若近期要接 HTTP/RPC，选 B；否则 A 成本更低 |
| D2 并行阈值 | 约 ≥1MB 即并行（与旧代码一致） | < 4MB 一律顺序 | 先做步骤 4 基准再定，避免无依据改边界 |
| D3 `--validate-only` | 大文件走并行、输出丢弃 | 一律顺序 | A 语义一致且更快；B 只是更保守 |
| D4 P0 修复 vs 零行为变化 | 修 P0 优先（行为确有变化） | 保行为不变（保留挂死/报错） | 选 A；P0-1~P0-4 不可保留 |
| D5 既有测试 | 已改为子测试独立值树 | 不动测试、接受 `-race` 红 | 选 A；否则方案第 4 点的"无锁"无法验证 |
| D6 `pending` 上界 | 未实现（步骤 3 待办） | 立即实现"最多约 2×worker 块" | 若内存 ≤30MB 是硬指标，提前到下一步第一项 |

裁决结果确认后，再继续步骤 3 / 步骤 4；未获确认前不再改代码。
