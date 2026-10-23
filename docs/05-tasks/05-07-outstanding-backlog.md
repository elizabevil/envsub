---
status: active
owner: 技术负责人
version: 0.1
last-updated: 2026-10-22
---

# 未完成清单（2026-10-22 盘点）

P0 全部完成、试用发布已做之后，剩下的开放项汇总在这里。**代码里没有未接线的扩展点**，剩下的都在文档里；
本清单是它们唯一的跟踪处。

已闭环的轮次记录在 [`05-06-verification-todo.md`](05-06-verification-todo.md)。

## 1. 缺陷：`--explain` 的来源报告

`--explain` 报告"每个被引用的变量从哪来"。下面 8 条都是实跑复现的，按严重度排序。共同的毛病是
**报告有时会编出来源**：它报的东西和值树里实际的东西对不上。

| # | 现象 | 根因 | 位置 | 建议修法 | 严重度 |
|---|---|---|---|---|---|
| E1 | 模板用 `${K:=v}` 赋值，且 `K` 还有值来源时，报告里**看不到这次赋值**——`origin` 指向值文件，`shadowed` 是空的 | 赋值帧只记在值树上（`Tree.Store` → `RecordMeta`），从不上报给 observer；而 `explainer.source` 只要 observer 有候选就直接返回，不再看值树 | `internal/render/explain.go:162-172`；`internal/engine/extended/engine.go:333-339` | observer 候选之后补上值树上的 `assign` 帧（赋值发生在装载之后，是链尾） | 高 |
| E2 | 报告说某个值来自 schema `default`，但那个 default **根本没被应用**（父对象不存在时不填） | `reportDefaults` 按 schema 递归收集全部 default，不看值树里父对象是否存在；`ApplyDefaults` 只在父对象已存在时填 | `internal/load/load.go:144-159`；`internal/schema/defaults.go:18-25` | 让 `ApplyDefaults` 回报它实际填了哪些，再据此报告 | 高 |
| E3 | 报告说某个**不存在的**子路径来自值文件（其实它没设置） | `Tree.MetaFor` 找不到精确路径时回退到**最近的祖先**，那是给错误文案用的"定义在…"提示，用在来源报告上就成了伪来源 | `internal/values/tree.go:273-293`；`internal/render/explain.go:169-172` | 来源报告用精确查找，祖先回退只留给错误文案 | 高 |
| E4 | 被 `--only-vars`/`--ignore-vars` 排除、根本没被替换的变量，仍被报告成"从 `--set` 来" | `explainTemplate` 只按 `Explain.matches` 过滤，没看 `Policies.replaces` | `internal/render/explain.go:200-206` | 同样按 `replaces` 过滤，与 `--report=unused` 的口径一致 | 中 |
| E5 | 只出现在默认值里的变量（`${A:-${B}}` 的 `B`）**完全不被报告** | 表达式解析只取操作符**之前**的路径，从不扫 `expr.Word` | `internal/engine/extended/explain.go:24-33` | 递归展开默认词里的路径 | 中 |
| E6 | 写报告失败时，真正的渲染错误被**吞掉**，换成 I/O 错误 | `reportSources` 的错误直接 `return`，丢掉了原来的 `err` | `internal/render/render.go:105-112` | 两个错误都报，或报渲染错误、把报告失败记日志 | 中 |
| E7 | `kind=set` 配 `method=assign`，违反"method 属于该 kind 的允许集合" | ADR-005 I2 说 `set` 只有 flag/json/file 三种，但赋值用的是第四种；`values.SourceKind` 里根本没有 `assign` 这个 kind | `internal/engine/extended/engine.go:334-335`；`internal/values/set.go:16-29` | 要么加 `SourceAssign` kind，要么把 ADR 与 kind 表改齐 | 低 |
| E8 | stderr 不是纯 JSONL：渲染失败时错误文案和报告混在一条流里 | 报告写 stderr，错误也写 stderr | `internal/app/app.go:32-43` | 要么放宽文档里的"纯 JSONL"说法，要么把错误写到别处 | 低 |

已核对无问题的部分：`source`/`file` 的一致性、`--explain=PATH` 的精确匹配、重复变量、JSON 转义
（含非 ASCII）、报告在渲染失败时仍会写出、层序与 shadowed 顺序。

**另有一项不是缺陷而是限制**：值文件（YAML/JSON）的行号恒为 0，schema 帧的行号也恒为 0。
ADR-005 说"需加载器记录位置，成本较高"——但 **YAML 侧成本并不高**：解码走 `yaml.Node`，行号本来就在
解析树里（`internal/yamlx/yamlv3.go:98` 的重复键诊断就用了它），只是 `nodeToValue` 那一步丢掉了。
JSON 侧才是真限制（`encoding/json/v2` 不给位置）。所以值文件的行号应当能补上，只是要动加载器。

## 2. 文档承诺了但没兑现

| 事项 | 承诺在哪 | 实际 |
|---|---|---|
| `internal/source` 拆包：模板输入与值来源加载器分开 | `99-archive/06-review/06-17` D8（"先注释分层，拆包留到 P1"） | 未做；`internal/source/input.go` 与 `loader.go` 仍混放 |
| 目录结构文档漏了模板输入这一族 | `03-design/03-07-code-and-repo-structure.md:38` 只说 `source/` 是"值来源加载器" | `FileInput`/`StdinInput`/`TemplateInput` 从未在目录说明里出现 |
| `09-gaps.md` G20 说 explain 的 `line` 用 `*int` + `omitempty` | `07-serialization/09-gaps.md:33` | 代码是对的（`Line int` 普通字段，两后端输出一致），**是这条说明过期了** |
| "stderr 是纯 JSONL 通道" | `07-serialization/13-materialization.md:394` | 与 E8 冲突 |

## 3. P1/P2 功能

逐项的状态、依赖与"开工前要定的事"在 [`05-04-p1-p2-task-breakdown.md`](05-04-p1-p2-task-breakdown.md)，
这里只记当前判断：

| 波 | 任务 | 判断 |
|---|---|---|
| W2 | TASK-108 Windows/CRLF/编码保真、TASK-109 输出 mode 保真、TASK-110 Docker 多架构镜像 | **可直接开工**；108 需要 Windows CI runner，109 与原子写回共用实现，110 需要 buildx 与推送凭据 |
| W3 | TASK-111 内置函数、TASK-112 profiles、TASK-113 TOML、TASK-114 include、TASK-115 条件循环、TASK-116 值插值 | 需先设计；114/115/116 风险最高（分别与安全约束、确定性、流式架构、隐式逻辑相撞） |
| W4 | TASK-201 兼容模式、TASK-202 migrate/convert、TASK-203 HTTP 来源、TASK-204 包管理渠道、TASK-205 man page | 需先决策；201 是 202 的前置 |
| W5 | TASK-208/209/210 插件 | 仅记录，暂不执行；TASK-211 插件安装器明确不做 |

已删除的任务：`--dry-run`（stdout 即"不写"，`--check` 天然不写）。`--diff` 是 ⏸ 待需求，
实现时作为 `--check` 的输出模式。

## 4. 需要产品拍板的

| 事项 | 现状 | 归谁 |
|---|---|---|
| **试用反馈** | 试用发布已做（tag `v0.1.0`），但**反馈记录是空的**。`02-02` 的三个产品问题（用户接不接受手工迁移/并行运行、多文件合并语义够不够好懂、与轻量工具的差异值不值得切换）与 ADR-002 的"一期发布后收集反馈"全都在等这个 | 产品负责人 |
| `sonic` 后端实机运行 | Windows/macOS/arm64 从未实机跑过（ADR-004 §7 的 `[TODO]`）；触发条件是有实机或 CI 矩阵 runner | 技术负责人 |
| 插件是否放行 | REQ-TPL-08（同进程插件）与 REQ-TPL-09（换模板引擎）的 ⛔ 结论，ADR-007 只在子进程形态上给了方向，"是否成立待产品侧明确" | 产品负责人 |

## 5. 本轮已关闭

- **`+dirty` 剥离**：`gitVersion` 现在是"git tag 或分支 + 短 commit id"（`v0.1.0-a215676` /
  `main-a215676`），脏状态由 `gitTreeState` 单独报。tag/分支由构建配方注入（工具链只嵌模块版本，
  不嵌分支名），没有注入时回落到模块版本并剥掉 `+dirty`。
- **未知子命令的退出码**：定为 2（用法错误），规则 C19。
- **G22 的迁移说明**：[`../migration.md`](../migration.md) 已交付。
