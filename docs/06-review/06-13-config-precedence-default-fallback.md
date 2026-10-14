---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-10
---

# 06-13 配置覆盖优先级：`default` 降为兜底（显式 > 隐式）

> 用户提出"优先级顺序感觉不符合使用习惯"。本文记录诊断、决策（方案 A 必须）、被拒方案与落盘证据。
> 依据 `docs/prompts/_common.md` §7.10：行为契约变更先讨论确认，再实施。

## 1. 问题

早期实现里 schema `default` 写入**根命名空间**，而 `ResolveName` 先查根再查环境，于是 `default`
压过 `.env` 与进程环境。实测：schema 声明 `K: {default: from_default}`，`.env` 写 `K=from_env_file`、
进程写 `K=from_process`，渲染得 `from_default`。`[FACT]`（来源：原 fixture `schema_default_precedence`）

这既不符合"default 是兜底"的直觉，也与本项目自己的设计文档相反：`03-03` §3.5 的"建议默认加载顺序"
把 `schema default` 列为**最低**。`[FACT]`

## 2. 决策（用户确认，2026-10-10）

两条不变量：

1. **显式指定 > 隐式包含**：`--set`/值文件/`.env` 是手动指定的配置，进程环境是隐式带入。
2. **schema `default` 只是兜底**：任何来源都没有该 key 时才生效。

由此得到**最终优先级（高→低）**：

| 优先级 | 来源 | 性质 |
|--------|------|------|
| 1 | `--set` / `--set-json` | 显式（CLI） |
| 2 | YAML/JSON 值文件（后者覆盖前者） | 显式（文件） |
| 3 | `.env` 文件（后者覆盖前者） | 显式（文件，环境命名空间） |
| 4 | 进程环境 | 隐式 |
| 5 | schema `default` | **兜底，非来源** |

相对旧实现**只动一条**：`default` 从第 3 位降到第 5 位。`[FACT]`

### 2.1 值文件 vs `.env`：同一显式档，固定 tie-break

第 2、3 行都属"命令行**显式指定**的文件来源"，可视为同一档；同 key 冲突时的破平规则定为
**结构化值文件优先**，**不**采用"按命令行出现顺序后者胜"。理由：`[FACT]`（来源：需求提出者确认，
2026-10-10；代价分析见下）

1. 冲突面只有**单段同名**：点号/索引路径在环境侧没有拼写（§13 已移除路径推导），`.env` 键也只能是单段。
2. pflag 只记录每个 flag**首次出现**的顺序（`orderedActual`），两个 `StringArray` 的交错顺序在解析后丢失；
   要真序需自定义 `pflag.Value` + `Request` 有序字段 + 解析期按序比较，成本明显。
3. 固定规则与"确定性输出"一致，不会因参数书写顺序把 YAML 的 `int` 换成 `.env` 的 `string`。
4. 已有逃生通道：`${env.NAME}` 始终显式读环境命名空间。

本轮只**更新文档描述**，代码未改（实现本就是这个顺序）。`[FACT]`

## 3. 候选方案与权衡（Alternatives & Trade-offs）

| 方案 | 做法 | 结论 |
|------|------|------|
| **A `default` 降为兜底（采用）** | `ApplyDefaults` 先查环境命名空间；key 有任何来源都不填 | 同时符合"兜底"直觉与 `03-03` §3.5；改动面最小（1 处判定 + fixture/文档） |
| B 环境可覆盖值文件（12-factor） | 根/环境顺序反转 | **未采用**：会重新引入 §13 论证过的**宿主环境污染**（无关键如 `HOME` 偷袭配置）；`--process-env-prefix`/`--no-process-env` 会从可选变必需，属定位级变更 |
| C 进程环境压过 `.env`（Compose/dotenv 习惯） | 反转 §14 的环境内顺序 | **未采用**：推翻已评审决定（`TestMergeEnvPrefersOverlay`、fixture `env_file_wins`），且同样带来宿主环境偷袭项目配置的风险 |
| D 保持现状，只改文档 | 把 `03-03` §3.5 改成"default 高于 env" | **未采用**：与"default 是兜底"的产品共识相反，且把最小意外原则让位给历史实现 |

`[INFERENCE]`（依据：A 与用户确认的两条不变量直接一致；B/C 与项目的防污染目标冲突，需单独立项评估。）

## 4. 实现与影响面

- `internal/schema/defaults.go`：`defaultsVisitor.missing` 先调用 `envFallback(tree, path)`，命中则不填
  default、不记 schema 来源 meta；未命中时行为不变（`CloneValue` + `RecordEncodedMeta`）。`[FACT]`
- 连带效果（有意）：环境提供了值但不符合 schema 类型时，现在会走**类型错误**（exit 4），而不是被
  default 静默顶掉；需要忽略宿主环境时用 `--no-process-env`。`[FACT]`
- fixture：`schema_default_precedence` → 重做并更名 `schema_default_is_fallback`，同时钉住两件事：
  `.env` 压过 `default`，以及"无任何来源时 default 生效"。`[FACT]`
- 文档：`04-cli.md` §6.2、`02-sources.md` §4.2、`03-03` §3.5、`13-materialization.md` §15、`09-gaps.md` G21。`[FACT]`

## 5. 证据与复现命令

```bash
go test -count=1 -run TestApplyDefaultsDefersToEnvironment ./internal/schema/
go test -count=1 -run TestApplyDefaults ./internal/schema/
go test -count=1 ./test/e2e/            # fixture schema_default_is_fallback
just check

# 手工确认（.env 应压过 default）
printf 'K=from_env_file\n' > .env
printf '{"type":"object","properties":{"K":{"type":"string","default":"from_default"}}}\n' > schema.json
printf 'K=${K}\n' > input.tmpl
envsub render --explain --env-file .env --schema schema.json input.tmpl
#   {"template":"input.tmpl","variable":"K","source":".env:1","kind":"dotenv","file":".env","line":1,"shadowed":["process"]}
```

`[FACT]`（`just check` 本轮退出码 0）

## 6. 非目标（Non-Goals）

1. **不反转根/环境命名空间**：值文件与 `.env` 仍高于进程环境，防止宿主环境偷袭配置（§13）。
2. **不反转 `.env` 与进程环境**（§14 的评审结论保持不变）。
3. **不改变 `--set` 的最高优先级**，也不改变组顺序（`--set` → `--set-json`）。
4. **不为 `default` 引入"按来源类型分别兜底"的复杂规则**：它只有一个语义——无来源时填入。
5. **不把该优先级做成配置项**（本轮不引入 `--env-overrides-files` 之类的开关）。
6. **不把值文件与 `.env` 改为按命令行交错顺序覆盖**：同一显式档内固定"值文件优先"，理由与代价见 §2.1。

## 7. 待确认（TODO）

- `[ASSUMPTION]` "环境值不符合 schema 类型时报错"是可接受的行为（不再被 default 顶掉）。验证方式：若
  试用反馈认为宿主环境不应导致失败，再评估"仅显式来源触发类型校验"。owner：技术负责人。
- `[已决]` 已列出：schema `default` 作为最低层**先播种**，被覆盖时出现在 `shadowed` 链尾，
  以 JSON 指针标识（`#/properties/K/default`），见 ADR-005 与 `schema_default_is_fallback` fixture。
  owner：产品/技术负责人。
