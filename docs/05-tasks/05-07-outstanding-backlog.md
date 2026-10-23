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

## 1. 缺陷：`--explain` 的来源报告（2026-10-22 已关闭）

8 条已全部修复，并补了单测与 e2e。**通俗说明与最小案例见 [`05-08`](05-08-explain-fixes-plain.md)。** 最终口径：

| # | 修法 | 回归 |
|---|---|---|
| E1 | observer 候选链尾补值树上的 `assign` 帧；只有 `:=`/`=` 真正写回时才出现 | `TestExplainerSourceAssignmentClosesTheChain`、`TestExplainReportsAssignmentOverAnEmptySource` |
| E2 | 不再报“所有声明”，改为 **精确 meta 可达性**：只有值树能到达的 default 进入链；被覆盖但仍可达的仍在 `shadowed`（I6） | `TestLoadReportsOnlyReachableDefaults`、`TestLoadDefaultOriginsUseTheValueThatSuppliedThePath`、`TestExplainOmitsUnreachableSchemaDefault` |
| E3 | 来源报告先精确查找；精确 meta 不存在但值存在时回退最近祖先（物化子路径）；值不存在才报 `missing` | `TestExplainerSourceFallsBackToAncestorOnlyForExistingValues`、`TestExplainKeepsMaterializedChildOrigin`、`TestExplainDoesNotInheritAncestorForMissingChild` |
| E4 | `explainTemplate` 同时按 `Policies.replaces` 过滤，与 `--report=unused` 同口径；可选 `PathsFor` 让被排除外层词内的嵌套引用也不上报 | `TestExplainTemplateSkipsFilteredVariables`、`TestExplainSkipsFilteredVariables`、`TestPathsForHonorsPolicy`、`TestExplainSkipsNestedReferenceUnderExcludedParent` |
| E5 | `Paths` 递归扫描操作符的默认词，`${A:-${B}}` 同时枚举 A、B | `TestPaths` 嵌套用例、`TestExplainReportsNestedDefaultWord` |
| E6 | 报告失败与渲染错误 `errors.Join`，渲染错误在前，kind/exit code 不变 | `TestExplainReportFailureKeepsRenderError` |
| E7 | 新增 `SourceAssign`，帧为 `kind=assign, method=assign`；`frameFor` 补 method 推断 | `TestExplainFramesFollowADR005`、两个 assignment e2e |
| E8 | 新增 `--jsonl` 统一事件流：所有事件共用 `schema`/`type`/`time`/`seq`/`data` 信封，载荷按类型特化；时间统一走 `internal/timefmt`；`--explain`/`--report` 自动启用，`--jsonl=false` 可关闭 | `TestWriterEmitsEnvelope`、`TestEventLoggerWritesEnvelope`、`TestJSONL*`；`13-materialization.md` §15、`04-cli.md` C16 |

E2 没有采用早先的“只报 `ApplyDefaults` 实际填了哪些”：被高层覆盖的 default 同样没被填充，
但按 I6 仍必须在链尾；只报实际填充会丢掉这些帧。

已核对无问题的部分：`source`/`file` 的一致性、`--explain=PATH` 的精确匹配、重复变量、JSON 转义
（含非 ASCII）、报告在渲染失败时仍会写出、层序与 shadowed 顺序。

**另有一项不是缺陷而是限制**：值文件（YAML/JSON）的行号恒为 0，schema 帧的行号也恒为 0。
YAML 侧行号本来就在 `yaml.Node` 里，只是 `nodeToValue` 丢掉了；JSON 侧才是真限制（`encoding/json/v2`
不给位置）。需要时再动加载器。

## 2. 文档承诺了但没兑现

| 事项 | 承诺在哪 | 实际 |
|---|---|---|
| `internal/source` 拆包：模板输入与值来源加载器分开 | `99-archive/06-review/06-17` D8（"先注释分层，拆包留到 P1"） | 未做；`internal/source/input.go` 与 `loader.go` 仍混放 |
| 目录结构文档漏了模板输入这一族 | `03-design/03-07-code-and-repo-structure.md:38` 只说 `source/` 是"值来源加载器" | `FileInput`/`StdinInput`/`TemplateInput` 从未在目录说明里出现 |

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

- **`--explain` 的 E1–E8**：见 §1；代码、单测、e2e 与文档同步更新。E8 最终采用 `--jsonl` 统一事件流，而不是仅改文档。
- **G20 的过期说明**：`--explain` 的 `line` 是普通 `int`（`0` 也输出），已改掉 `09-gaps.md` 与 `13-materialization.md` 里的 `*int` + `omitempty` 说法。
- **`+dirty` 剥离**：`gitVersion` 现在是"git tag 或分支 + 短 commit id"（`v0.1.0-a215676` /
  `main-a215676`），脏状态由 `gitTreeState` 单独报。tag/分支由构建配方注入（工具链只嵌模块版本，
  不嵌分支名），没有注入时回落到模块版本并剥掉 `+dirty`。
- **未知子命令的退出码**：定为 2（用法错误），规则 C19。
- **G22 的迁移说明**：[`../migration.md`](../migration.md) 已交付。
