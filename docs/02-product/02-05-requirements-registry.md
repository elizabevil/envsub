---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-12
---

# 需求登记册（Requirements Registry）

> **用途**：本项目需求此前散落在原始需求、功能地图、ADR、任务清单与评审记录里，缺少一份"当前有哪些需求、
> 各自什么优先级、**能不能在本应用里实现**"的单一入口。本文件是那份入口；功能全景仍是 `02-01-feature-map.md`，
> 本文件负责**状态与可实现性判定**。
>
> 编号 `REQ-<域>-<序号>` 全局唯一、不重编号；已废弃的条目标注 `~~删除线~~` 与原因，不回收编号。
> 依据 `docs/prompts/_common.md` 的标注规范：`[FACT]` 有来源、`[INFERENCE]` 说明推断依据、`[ASSUMPTION]` 需验证、
> `[TODO]` 缺信息。

## 1. 需求从哪里来（采集来源）

| 来源 | 覆盖内容 | 备注 |
|------|----------|------|
| `00-raw/00-0[1-4]` | 线上痛点、需求提出者补充、语法/类型约束、优先级与 schema | 原始输入 |
| `02-product/02-01-feature-map.md` | 场景矩阵与功能全景（3.1–3.9） | 候选地图 |
| `02-product/02-03-prd.md`、`02-04-user-stories.md` | 验收口径与用户故事 | 验收来源 |
| `04-decisions/ADR-001..004` | 语言/渲染模型、兼容分期、schema 复用、依赖选型 | 决策约束 |
| `reference/REF-01..04` | 竞品能力差、兼容基线、schema 复用调研、性能基线 | **外部需求来源** |
| `05-tasks/05-03-pending-work.md` | 已登记待办与待决项 | 执行侧 |
| `07-serialization/99-non-goals.md`、`01-05-conflicts-and-non-goals.md` | 明确不做的事 | 排除依据 |
| 同类工具（gomplate / consul-template / Helm / Kustomize / ytt / confd 等） | 对照补缺 | 见 `REF-01` §4/§5 |

## 2. 硬约束（判定可实现性的前提）

判定任何需求前先过这七条；与约束冲突的需求**不可实现**（而非"难实现"）。

| # | 约束 | 来源 |
|---|------|------|
| K1 | 默认不执行 shell 命令、不访问网络、不自动读取远程密钥 | `[FACT]`（PRD §68/§183、`03-01` §34） |
| K2 | 不做图灵完备的通用编程语言；不做任意 shell 执行/递归函数/通用计算 | `[FACT]`（`02-01` §142） |
| K3 | 单静态二进制、`CGO_ENABLED=0`、多平台多架构、低运行时依赖 | `[FACT]`（`03-01`、`02-01` §1） |
| K4 | 输出确定性与幂等：键排序、行尾/缩进风格固定，便于版本控制 | `[FACT]`（`07-output` D1/D3） |
| K5 | 危险能力必须**显式启用**（默认关闭、可审计） | `[INFERENCE]`（依据 K1 与 `02-02` §104） |
| K6 | 不承诺与第三方实现（GNU/a8m）在数字/空值上一致；兼容模式是 P2 边界 | `[FACT]`（`99-non-goals`、ADR-002） |
| K7 | 核心模型是"层级值模型 + 受限渲染引擎"，模板不是编程语言 | `[FACT]`（ADR-001） |

## 3. 需求登记（状态与可实现性）

**可实现性**列的含义：
- ✅ 可实现：与 K1–K7 不冲突，且现有分层能容纳（多数只需新增 option 或 formatter）。
- 🟡 有条件可实现：需要新增隔离机制（显式开关、白名单、超时）或引入新依赖，须先做决策。
- ⛔ 不可实现：与 K1/K2/K7 冲突（记为 P3 永久不做）。

### 3.1 值来源（REQ-SRC）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-SRC-01 | 进程环境（`@process` 只读命名空间） | P0 | ✅ 已实现 | ✅ | — |
| REQ-SRC-02 | `.env` 配置层（`--dotenv-file`，进根、高于值文件） | P0 | ✅ 已实现 | ✅ | — |
| REQ-SRC-03 | YAML/JSON 值文件、多文件覆盖、object 深合并 | P0 | ✅ 已实现 | ✅ | — |
| REQ-SRC-04 | `--set` / `--set-json` | P0 | ✅ 已实现 | ✅ | `--set-string` 已删除 |
| REQ-SRC-05 | `--set-file KEY=FILE`（值来自文件，避免命令行泄密） | P1 | ❌ 未实现 | ✅ | `source` 已有文件读取与 `Input` 抽象；注意脱敏（REQ-SEC-02） |
| REQ-SRC-06 | TOML 值文件 | P1 | ❌ 未实现（`.toml` 当前被拒） | 🟡 | 需新增 loader + 依赖（`pelletier/go-toml`）；ADR-004 需补决策 |
| REQ-SRC-07 | INI / `.properties` 值文件 | P2 | ❌ | 🟡 | 与 REQ-SRC-06 同构，格式语义弱于 YAML，收益待评估 |
| REQ-SRC-08 | 值引用与插值（配置值内部引用其他值、检测循环） | P1 | ❌ | 🟡 | 与"确定性 + 无隐式逻辑"有张力：需定义求值顺序与循环诊断 |
| REQ-SRC-09 | profiles/environments 选择（dev/staging/prod） | P1 | ❌ | ✅ | 建议实现为"profile → 文件列表/覆盖层"的声明式映射，而非新语法 |
| REQ-SRC-10 | 工具自身配置文件（默认选项、profiles、搜索路径） | P2 | ❌ | ✅ | 需定义配置格式（可复用 YAML）与优先级（CLI 覆盖配置） |
| REQ-SRC-11 | 值来源于 HTTP(S) URL | P2 | ❌（占位类型已删） | 🟡 | 与 K1 冲突 → 必须 `--allow-remote` 显式开启 + 超时/大小/重定向限制 |
| REQ-SRC-12 | 值来源于 RPC/gRPC | P3 | ❌ | ⛔ | 无静态二进制内的合理实现路径，且违背"单次 CLI 渲染"定位 |
| REQ-SRC-13 | 远程密钥（Vault/SOPS/云 KMS） | P2 | ❌ | 🟡 | 只做 `SOPS` 这类**本地可解密**的形态（无网络）；网络型按 K1 排除 |

### 3.2 类型与校验（REQ-TYP）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-TYP-01 | 声明类型：`string/int/float/bool/array<T>/object/null` | P0 | ✅ | ✅ | — |
| REQ-TYP-02 | JSON Schema Draft 2020-12 校验（`required/enum/pattern/范围/长度`） | P0 | ✅ | ✅ | `santhosh-tekuri/jsonschema/v6` |
| REQ-TYP-03 | inline `\| type T` 与 schema 相容性 | P0 | ✅ | ✅ | SI11 |
| REQ-TYP-04 | `default` 兜底（不压过任何实际来源） | P0 | ✅ | ✅ | G21 |
| REQ-TYP-05 | schema 快速声明形式（内联 schema 片段） | P1 | ❌ | 🟡 | 需与 `include`/inline 语法一起设计，避免第二套 schema 方言 |
| REQ-TYP-06 | 严格/宽松类型模式开关 | P1 | 部分（严格已是默认） | ✅ | 主要是文档化边界，而非新机制 |
| REQ-TYP-07 | 敏感值标记与脱敏（`x-sensitive`） | P1 | ❌ | ✅ | 与 REQ-SEC-02 合并实现；`--explain` 已不输出值，是基础 |

### 3.3 模板与渲染（REQ-TPL）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-TPL-01 | `$VAR` / `${path}` / `$$` 转义 / `$1` 保持字面量 | P0 | ✅ | ✅ | — |
| REQ-TPL-02 | 操作符 `:- - :+ + :? ? := =` 与嵌套默认值 | P0 | ✅ | ✅ | O1–O9 |
| REQ-TPL-03 | formatter `join/toJson/toYaml/quote/indent/type` | P0 | ✅ | ✅ | P1–P6 |
| REQ-TPL-04 | include/partials（含嵌套深度、循环检测、路径白名单） | P1 | ❌ | 🟡 | 与 K1/K5 交互：必须限制在允许目录内，且默认关闭或显式 `--include-dir` |
| REQ-TPL-05 | 条件与循环（`if`/`range`，控制指令单独成行、object 按键排序） | P1 | ❌ | 🟡 | 与 K7 有张力：须保持"受限"（无递归、无函数定义），并用 fixture 钉住确定性 |
| REQ-TPL-06 | 内置函数扩展（upper/lower/trim/base64/url/sha 等） | P1 | ❌ | ✅ | 纯函数、无副作用，逐个加 formatter 即可；注意与 `quote` 等既有语义一致 |
| REQ-TPL-07 | `--set-file` 之外的值注入：stdin 作为值文件 | P2 | ❌ | ✅ | 与模板 stdin 冲突，需 `--values-stdin` 之类显式旗标 |
| REQ-TPL-08 | 模板函数注册（插件/WASM） | P2 | ❌ | ⛔ | 与 K3（单静态二进制）和 K2 冲突；如需扩展，走"新增内置函数"而非运行时插件 |
| REQ-TPL-09 | 切换模板引擎（Jinja/Go template 风格） | P3 | ❌ | ⛔ | 与 K7 及项目定位（envsubst 超集）冲突 |

### 3.4 输出与写回（REQ-OUT）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-OUT-01 | stdout / `-o` / `--output-dir` / `--recursive` / `-i` 原子写回 | P0 | ✅ | ✅ | — |
| REQ-OUT-02 | `--validate-only` | P0 | ✅ | ✅ | — |
| REQ-OUT-03 | `--dry-run`（只预览不写） | P1 | ❌ | ✅ | 与 `--validate-only` + stdout 有重叠，需先明确差异（是否仍输出内容） |
| REQ-OUT-04 | `--diff`（输出 diff） | P1 | ❌ | ✅ | 需内置 unified diff（标准库无）；约 150 行或加轻依赖 |
| REQ-OUT-05 | `--check`（有差异即非零退出，CI 门禁） | P1 | ❌ | ✅ | 与 REQ-OUT-04 共用比较逻辑 |
| REQ-OUT-06 | 权限/行尾/编码保真（CRLF、mode 位） | P1 | ❌ | ✅ | 原子写回已保留文件，需显式测试 mode 与 CRLF |
| REQ-OUT-07 | 输出路径变量化（输出名由值决定） | P2 | ❌ | 🟡 | 需限制在输出根目录内（防路径穿越） |
| REQ-OUT-08 | 机器可读输出（错误/报告 JSON） | P1 | 部分（`--explain` 为 JSONL） | ✅ | 建议 `--error-format=json`，复用 `diag` 的分类 |
| REQ-OUT-09 | 渲染统计与未使用变量报告 | P1 | ❌ | ✅ | `--explain` 已枚举引用名，可直接做"未被引用的值文件键" |
| REQ-OUT-10 | 交互确认 / 覆盖保护 | P2 | ❌ | ✅ | 与 CI 非交互冲突：默认非交互，仅 `-i` 时加保护 |
| REQ-OUT-11 | 批量输出名处理（模板后缀） | P2 | ❌（现保留 `.tmpl`） | 🟡 | 真实配置仓库会得到 `nginx.conf.tmpl`；三种形态待决：文档化 / `--strip-suffix`（推荐）/ 自动剥离（隐式，不推荐）。需要时补 ADR 到 CLI 契约 |

### 3.5 CLI 与可观测性（REQ-CLI）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-CLI-01 | 稳定退出码与错误分类 | P0 | ✅ | ✅ | 2/3/4/5/6 |
| REQ-CLI-02 | `--explain`（来源报告 JSONL） | P0 | ✅ | ✅ | — |
| REQ-CLI-03 | 结构化日志（`--log-level`） | P0 | ✅ | ✅ | slog |
| REQ-CLI-04 | shell completion | P1 | ❌ | ✅ | cobra 自带；只需暴露 `completion` 子命令并加 e2e 用例 |
| REQ-CLI-05 | man page | P2 | ❌ | ✅ | 由 cobra 生成，随发布资产 |
| REQ-CLI-06 | `--compat=gnu\|a8m\|drone` 兼容模式 | P2 | ❌ | 🟡 | 与 K6 一致（P2 边界）；需先冻结兼容矩阵（`REF-02`） |
| REQ-CLI-07 | `migrate`/`convert` 命令 | P2 | ❌ | 🟡 | ADR-002 二期项；依赖 REQ-CLI-06 的语法映射表 |
| REQ-CLI-08 | 静默模式 / 交互确认 | P3 | ❌ | ⛔ | 与"CI 非交互、确定性输出"冲突，无真实场景 |

### 3.6 安全与供应链（REQ-SEC）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-SEC-01 | 默认不执行命令、不访问网络 | P0 | ✅ | ✅ | K1；`@process` 与 `--dotenv-file` 均不触网 |
| REQ-SEC-02 | 敏感值脱敏（日志与诊断不泄露值） | P1 | 部分（`--explain` 不含值） | ✅ | 需覆盖错误信息（如 `.env` 值进入诊断的路径） |
| REQ-SEC-03 | 临时文件权限安全（`0600`、原子替换） | P1 | 部分（已原子写回） | ✅ | 补测试即可 |
| REQ-SEC-04 | 发布校验：checksums/SBOM/可复现构建 | P1 | 部分（checksums、sbom 脚本已存在） | ✅ | 补签名与可复现性验证 |
| REQ-SEC-05 | 解析器/扫描器 fuzz | P1 | ❌ | ✅ | `[INFERENCE]`（`03-10` §8 已列为要求） |
| REQ-SEC-06 | 远程 secret 自动读取 | P3 | ❌ | ⛔ | 与 K1 冲突 |

### 3.7 分发与平台（REQ-DIS）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-DIS-01 | Linux/macOS/Windows × amd64/arm64 静态二进制 | P0 | ✅（`build-release` 矩阵） | ✅ | — |
| REQ-DIS-02 | Docker scratch/distroless 多架构镜像 | P1 | 部分（`deploy/docker`） | ✅ | 需多架构构建与发布流水线 |
| REQ-DIS-03 | 包管理器渠道（Homebrew/Scoop/Winget/APT/RPM） | P2 | 部分（`deploy/package` 脚本） | 🟡 | 每个渠道都有外部评审周期，属发布工程而非代码 |
| REQ-DIS-04 | Windows/PowerShell 细节（CRLF、路径、编码） | P1 | ❌ | ✅ | 与 REQ-OUT-06 合并；需要 Windows CI 用例 |

### 3.8 性能与规模（REQ-PERF）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-PERF-01 | 大文件流式渲染、内存峰值有界 | P0 | ✅ | ✅ | 流式 + 窗口并行；perf 门禁在跑 |
| REQ-PERF-02 | 批量/并行渲染 | P0 | ✅ | ✅ | 赋值模板自动回退串行 |
| REQ-PERF-03 | 启动时间预算（<100ms） | P1 | ✅（约 5ms） | ✅ | 门禁已有 |
| REQ-PERF-04 | 与 GNU envsubst 的性能对照基线 | P1 | ✅ | ✅ | `REF-04` + `test/perf` |

## 4. 可实现性判定的理由（对 🟡/⛔ 逐条说明）

| 需求 | 判定 | 理由 |
|------|------|------|
| REQ-SRC-06 TOML | 🟡 | 技术上简单（新增 loader），但会引入新依赖并扩大"值文件格式"契约面（ADR-004 需补决策）；建议先确认是否真有 TOML 用户 |
| REQ-SRC-08 值引用 | 🟡 | 与"无隐式逻辑、确定性"张力最大：需要定义求值顺序、循环诊断、与 schema 物化的先后。**建议先出设计再决定是否做** |
| REQ-SRC-11 HTTP 值来源 | 🟡 | K1 要求默认不触网。可实现形态：`--allow-remote` + 只读单次 GET + 超时/大小/重定向上限 + 明确不进日志。须用户确认后再设计 |
| REQ-SRC-13 远程密钥 | 🟡 | 仅接受**本地可解密**（SOPS/age）形态；Vault/云的联网读取与 K1 冲突 |
| REQ-TPL-04 include | 🟡 | 与 K5 交互：需要 `--include-dir` 白名单、深度上限（候选 3 层）、循环检测与稳定错误格式；不默认开启 |
| REQ-TPL-05 条件/循环 | 🟡 | 与 K7 张力：可以做成受限形式（无递归、无函数定义、无变量捕获），但必须用 fixture 钉住"按键排序 + 单独成行"的确定性 |
| REQ-TPL-08 插件/WASM | ⛔ | 与 K3（单静态二进制、低依赖）和 K2 直接冲突；扩展走内置函数 |
| REQ-TPL-09 换模板引擎 | ⛔ | 与项目定位（envsubst 超集）和 K7 冲突 |
| REQ-SRC-12 RPC 值来源 | ⛔ | 无静态二进制内可接受的实现路径，且"单次 CLI 渲染"无长连接语义 |
| REQ-CLI-08 静默/交互确认 | ⛔ | CI 场景要求非交互与确定性输出，交互确认只在 `-i` 下有边际价值 |
| REQ-SEC-06 远程 secret | ⛔ | K1 |

## 5. 推荐落地顺序（P1 内部）

按"依赖少、风险低、收益直接"排序，前四项互相独立：

1. **REQ-CLI-04 shell completion**（最小、cobra 自带、可立刻 e2e 覆盖）
2. **REQ-OUT-03/04/05 `--dry-run`/`--diff`/`--check`**（CI/GitOps 直接受益，共用一套比较逻辑）
3. **REQ-OUT-08 机器可读错误 + REQ-OUT-09 未使用变量报告**（复用 `diag` 与 `--explain` 已有枚举）
4. **REQ-SEC-02/03 脱敏与临时文件权限 + REQ-SEC-05 fuzz**（安全债，成本低）
5. **REQ-SRC-05 `--set-file`**（与 secret 场景配套）
6. **REQ-DIS-04 + REQ-OUT-06 Windows/CRLF/权限保真**（需要 Windows CI，独立推进）
7. **REQ-TPL-06 内置函数扩展**（逐个加，注意与既有 formatter 语义一致）
8. **REQ-SRC-09 profiles**（设计成"声明式覆盖层"，不引入新语法）
9. **REQ-TPL-04 include**、**REQ-TPL-05 条件/循环**（两项都需先出设计文档与安全边界，见 §4）
10. **REQ-CLI-06/07 兼容模式与转换**（P2，需先冻结 `REF-02` 的兼容矩阵）

## 6. 非目标（Non-Goals）

- 本文件不做优先级裁决：`P1/P2/P3` 是**建议**，最终由需求提出者确认。`[FACT]`
- 不重复 `02-01-feature-map.md` 的功能描述，只登记**状态与可实现性**。`[INFERENCE]`
- 不承诺 P1/P2 的排期；上表"现状"列以仓库当前代码与测试为准。`[FACT]`
- 不把 K1/K2/K7 当作可协商项：与之冲突的需求一律 ⛔，不进入待办。`[FACT]`

## 7. 与其它文档的关系

| 文档 | 关系 |
|------|------|
| `02-01-feature-map.md` | 功能全景与场景矩阵（本文件的上游） |
| `04-decisions/ADR-00x` | 决策约束；REQ-SRC-06（TOML）等 🟡 项需新增 ADR |
| `05-tasks/05-01/05-03` | P0 任务与执行侧待办 |
| `07-serialization/*` | 规则细则；本文件只登记"是否需要" |
| `06-review/06-15`、`06-17` | 命名空间/分层与架构收敛的决策记录 |
