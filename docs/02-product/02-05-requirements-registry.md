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
| `99-archive/00-raw/00-0[1-4]` | 线上痛点、需求提出者补充、语法/类型约束、优先级与 schema | 原始输入（已归档） |
| `99-archive/02-product/02-01-feature-map.md` | 场景矩阵与功能全景（3.1–3.9） | 候选地图（已归档） |
| `99-archive/02-product/02-03-prd.md`、`02-04-user-stories.md` | 验收口径与用户故事 | 验收来源（已归档） |
| `04-decisions/ADR-001..004` | 语言/渲染模型、兼容分期、schema 复用、依赖选型 | 决策约束 |
| `reference/REF-02`、`REF-04`；`99-archive/reference/REF-01`、`REF-03` | 兼容基线、性能基线、竞品能力差、schema 复用调研 | **外部需求来源** |
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
| REQ-SRC-05 | `--set-file KEY=FILE`（值来自文件，避免命令行泄密） | P1 | ✅ 已实现 | ✅ | W1（2026-10-12，TASK-107）：4 单测 + 2 fixture；脱敏见 REQ-SEC-02 |
| REQ-SRC-06 | TOML 值文件 | P1 | ❌ 未实现（`.toml` 当前被拒） | 🟡 | 需新增 loader + 依赖（`pelletier/go-toml`）；ADR-004 需补决策 替代路径：**外部格式插件**（`REQ-SRC-15`）——核心不引依赖，用户装插件即可（设计见 `03-10`、ADR-007）；本条（核心内置 loader）保持未实现。 |
| REQ-SRC-07 | INI / `.properties` 值文件 | P2 | ❌ | 🟡 | 与 REQ-SRC-06 同构，格式语义弱于 YAML，收益待评估 同 REQ-SRC-06：优先考虑外部格式插件路线（`REQ-SRC-15`）。 |
| REQ-SRC-08 | 值引用与插值（配置值内部引用其他值、检测循环） | P1 | ❌ | 🟡 | 与"确定性 + 无隐式逻辑"有张力：需定义求值顺序与循环诊断 |
| REQ-SRC-09 | profiles/environments 选择（dev/staging/prod） | P1 | ❌ | ✅ | 建议实现为"profile → 文件列表/覆盖层"的声明式映射，而非新语法 |
| REQ-SRC-10 | 工具自身配置文件（默认选项、profiles、搜索路径） | P2 | ❌ | ✅ | 需定义配置格式（可复用 YAML）与优先级（CLI 覆盖配置） |
| REQ-SRC-11 | 值来源于 HTTP(S) URL | P2 | ❌（占位类型已删） | 🟡 | 与 K1 冲突 → 必须 `--allow-remote` 显式开启 + 超时/大小/重定向限制 替代形态：**来源插件**（`REQ-SRC-14`）——核心不触网（K1 完整保留）；本条（核心内置 `--allow-remote`）与插件形态共用同一套策略语义，二选一或并存待定。 |
| REQ-SRC-12 | 值来源于 RPC/gRPC | P3 | ❌ | ⛔ | 无静态二进制内的合理实现路径，且违背"单次 CLI 渲染"定位 |
| REQ-SRC-13 | 远程密钥（Vault/SOPS/云 KMS） | P2 | ❌ | 🟡 | 只做 `SOPS` 这类**本地可解密**的形态（无网络）；网络型按 K1 排除 |
| REQ-SRC-14 | 值来源插件（格式/来源，独立进程 + YAML 清单 + NDJSON） | P2 | ❌ 未实现（**仅记录设计**） | 🟡 | 调用次数与输入个数同阶（1 次/文件、1 次/URL）；核心不触网、不引依赖；能力门禁 `network`→`--allow-remote`。设计见 `03-10`，任务见 `05-04` TASK-208 |
| REQ-SRC-15 | 外部格式插件（TOML/INI/HCL… 由用户实现） | P2 | ❌ 未实现（**仅记录设计**） | 🟡 | 让 `REQ-SRC-06/07` 不必进核心依赖；`-f app.toml` 派发到 `envsub-format-toml`；发现机制与清单格式见 `03-10` |

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
| REQ-TPL-08 | 模板函数注册（插件/WASM） | P2 | ❌ | ⛔ | 与 K3（单静态二进制）和 K2 冲突；如需扩展，走"新增内置函数"而非运行时插件 边界澄清：本条的 ⛔ 针对**同进程**加载（Go `plugin`/WASM）；**子进程**形态另立 `REQ-TPL-10`（引擎插件）与 L3（formatter sidecar），见 `03-10`、ADR-007——是否放行待产品侧拍板。 |
| REQ-TPL-09 | 切换模板引擎（Jinja/Go template 风格） | P3 | ❌ | ⛔ | 与 K7 及项目定位（envsubst 超集）冲突 张力：`REQ-TPL-10`（子进程引擎插件）在效果上接近"用户可换引擎"，若放行需修订本条与 K7 措辞；当前**未拍板**。 |
| REQ-TPL-10 | 引擎/方言插件（独立进程引擎，1 次/模板） | P3 | ❌ 未实现（**仅记录设计**） | 🟡 | 语言面扩展（自定义语法/过滤器/兼容方言）；与 `REQ-TPL-08/09` 及 K7 张力需产品侧拍板。见 `03-10` §5、ADR-007 §6 |

### 3.4 输出与写回（REQ-OUT）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-OUT-01 | stdout / `-o` / `--output-dir` / `--recursive` / `-i` 原子写回 | P0 | ✅ | ✅ | — |
| REQ-OUT-02 | `--validate-only` | P0 | ✅ | ✅ | — |
| REQ-OUT-03 | `--dry-run`（只预览不写） | P1 | ❌ **已删除任务** | ✅ | 无独立语义：stdout 即"不写"，`--check` 天然不写（2026-10-12 产品结论，见 `05-04` 取舍表） |
| REQ-OUT-04 | `--diff`（输出 diff） | P1 | ⏸ **待需求** | ✅ | 呈现便利，外部 `diff`/`git diff` 可替代；出现需求时作为 `--check` 的输出模式实现（见 `05-04` 取舍表） |
| REQ-OUT-05 | `--check`（有差异即非零退出，CI 门禁） | P1 | ✅ 已实现 | ✅ | TASK-105：`--check` + 专用退出码 7；5 fixture + 5 单测 |
| REQ-OUT-06 | 权限/行尾/编码保真（CRLF、mode 位） | P1 | ❌ | ✅ | 原子写回已保留文件，需显式测试 mode 与 CRLF |
| REQ-OUT-07 | 输出路径变量化（输出名由值决定） | P2 | ❌ | 🟡 | 需限制在输出根目录内（防路径穿越） |
| REQ-OUT-08 | 机器可读输出（错误/报告 JSON） | P1 | ✅ 已实现 | ✅ | `--error-format=json`（TASK-102，2 fixture + 单测）；`--explain` 为 JSONL |
| REQ-OUT-09 | 渲染统计与未使用变量报告 | P1 | ✅ 已实现 | ✅ | `--report=unused`（TASK-103：6 单测 + 2 fixture）；口径见 `04-cli` C15 |
| REQ-OUT-10 | 交互确认 / 覆盖保护 | P2 | ❌ | ✅ | 与 CI 非交互冲突：默认非交互，仅 `-i` 时加保护 |
| REQ-OUT-11 | 批量输出名处理（模板后缀） | P2 | ✅ 已实现 | ✅ | `--strip-suffix EXT`（可重复、默认关；仅 `--output-dir`；最长匹配；撞名 exit 2）：ADR-006、`04-cli` C18、fixture `strip_suffix`/`strip_suffix_collision` |

### 3.5 CLI 与可观测性（REQ-CLI）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-CLI-01 | 稳定退出码与错误分类 | P0 | ✅ | ✅ | 2/3/4/5/6 |
| REQ-CLI-02 | `--explain`（来源报告 JSONL） | P0 | ✅ | ✅ | — |
| REQ-CLI-03 | 结构化日志（`--log-level`） | P0 | ✅ | ✅ | slog |
| REQ-CLI-04 | shell completion | P1 | ✅ 已实现 | ✅ | cobra 自带 `completion`（bash/zsh/fish/powershell，实测可用）；防退化守护见 TASK-101 |
| REQ-CLI-05 | man page | P2 | ❌ | ✅ | 由 cobra 生成，随发布资产 |
| REQ-CLI-06 | `--compat=gnu\|a8m\|drone` 兼容模式 | P2 | ❌ | 🟡 | 与 K6 一致（P2 边界）；需先冻结兼容矩阵（`REF-02`） |
| REQ-CLI-07 | `migrate`/`convert` 命令 | P2 | ❌ | 🟡 | ADR-002 二期项；依赖 REQ-CLI-06 的语法映射表 |
| REQ-CLI-08 | 静默模式 / 交互确认 | P3 | ❌ | ⛔ | 与"CI 非交互、确定性输出"冲突，无真实场景 |
| REQ-CLI-09 | 子命令插件（`envsub <name>` 派发到外部可执行文件） | P3 | ❌ 未实现（**仅记录设计**） | 🟡 | Helm 式扩展点；每命令 1 次执行；需显式授权；不参与值树与渲染结果。见 `03-10` §7 |

### 3.6 安全与供应链（REQ-SEC）

| 编号 | 需求 | 优先级 | 现状 | 可实现性 | 说明 |
|------|------|--------|------|----------|------|
| REQ-SEC-01 | 默认不执行命令、不访问网络 | P0 | ✅ | ✅ | K1；`@process` 与 `--dotenv-file` 均不触网 |
| REQ-SEC-02 | 敏感值脱敏（日志与诊断不泄露值） | P1 | ✅ 已实现 | ✅ | 脱敏修复（TASK-106）；`--explain` 只输出来源不输出值；测试 `internal/schema/redaction_test.go`；`x-sensitive` 标记见 REQ-TYP-07 |
| REQ-SEC-03 | 临时文件权限安全（`0600`、原子替换） | P1 | ✅ 已实现 | ✅ | `internal/output` 原子替换 + 0600 权限守护（TASK-106；`internal/output/writer_test.go`）；mode 保真见 REQ-OUT-06 |
| REQ-SEC-04 | 发布校验：checksums/SBOM/可复现构建 | P1 | 部分（checksums、sbom 脚本已存在） | ✅ | 补签名与可复现性验证 |
| REQ-SEC-05 | 解析器/扫描器 fuzz | P1 | ✅ 已实现 | ✅ | 8 个 `func Fuzz` 目标（扫描器/流式 span/路径文法/表达式路径/数值/`.env`/窗口读取器）：6 个在 `test/fuzz/`，另外 2 个在 `internal/engine/extended/`（`parseExpression`、`streamWindow` 未导出）；`just test-fuzz` 与 CI job `fuzz` 覆盖 |
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
| REQ-SRC-08 值引用 | 🟡 | 与"无隐式逻辑、确定性"张力最大：需要定义求值顺序、循环诊断、与 schema 物化的先后。**建议先出设计再决定是否做**（设计材料：现状实测、三种求值模型对照、业界案例、最小边界与 8 项决策清单见 `05-tasks/05-03-pending-work.md` §14） |
| REQ-SRC-11 HTTP 值来源 | 🟡 | K1 要求默认不触网。可实现形态：`--allow-remote` + 只读单次 GET + 超时/大小/重定向上限 + 明确不进日志。须用户确认后再设计 |
| REQ-SRC-13 远程密钥 | 🟡 | 仅接受**本地可解密**（SOPS/age）形态；Vault/云的联网读取与 K1 冲突 |
| REQ-TPL-04 include | 🟡 | 与 K5 交互：需要 `--include-dir` 白名单、深度上限（候选 3 层）、循环检测与稳定错误格式；不默认开启 |
| REQ-TPL-05 条件/循环 | 🟡 | 与 K7 张力：可以做成受限形式（无递归、无函数定义、无变量捕获），但必须用 fixture 钉住"按键排序 + 单独成行"的确定性 |
| REQ-TPL-08 插件/WASM | ⛔ | 与 K3（单静态二进制、低依赖）和 K2 直接冲突；扩展走内置函数 |
| REQ-TPL-09 换模板引擎 | ⛔ | 与项目定位（envsubst 超集）和 K7 冲突 |
| REQ-SRC-14 值来源插件 | 🟡 | 技术上可行（独立进程 + 清单 + NDJSON，零新依赖），但核心无法强制插件的网络策略（只能强制 exec 门禁、超时、输出上限、环境最小集）——是防误用与可审计，不是沙箱 |
| REQ-SRC-15 外部格式插件 | 🟡 | 取决于 REQ-SRC-14 的协议落地；收益是 `REQ-SRC-06/07` 不必进核心依赖 |
| REQ-TPL-10 引擎插件 | 🟡 | 1 次/模板的成本可控，但效果上接近"用户可换引擎"，与 `REQ-TPL-08/09` 及 K7 的措辞冲突，需产品侧修订措辞后才行 |
| REQ-CLI-09 子命令插件 | 🟡 | 纯 exec 扩展，不参与核心语义；风险仅在"执行外部命令"需显式授权 |
| REQ-SRC-12 RPC 值来源 | ⛔ | 无静态二进制内可接受的实现路径，且"单次 CLI 渲染"无长连接语义 |
| REQ-CLI-08 静默/交互确认 | ⛔ | CI 场景要求非交互与确定性输出，交互确认只在 `-i` 下有边际价值 |
| REQ-SEC-06 远程 secret | ⛔ | K1 |

## 5. 推荐落地顺序（P1 内部）

按"依赖少、风险低、收益直接"排序；**状态截至 2026-10-14**（✅ 已完成、❌ 待做）：

1. **REQ-CLI-04 shell completion** — ✅ 已实现（TASK-101，含防退化守护）
2. **REQ-OUT-05 `--check`** — ✅ 已实现（TASK-105）；同组：REQ-OUT-03 `--dry-run` **已删除任务**、
   REQ-OUT-04 `--diff` **⏸ 待需求**（见 `05-04` 的取舍表）
3. **REQ-OUT-08 机器可读错误 + REQ-OUT-09 未使用变量报告** — ✅ 已实现（TASK-102/103）
4. **REQ-SEC-02/03 脱敏与临时文件权限 + REQ-SEC-05 fuzz** — ✅ 已实现（TASK-106）
5. **REQ-SRC-05 `--set-file`** — ✅ 已实现（TASK-107）
6. **REQ-DIS-04 + REQ-OUT-06 Windows/CRLF/权限保真** — ❌ 待做（需要 Windows CI）
7. **REQ-TPL-06 内置函数扩展** — ❌ 待做（注意与既有 formatter 语义一致）
8. **REQ-SRC-09 profiles** — ❌ 待做（设计成"声明式覆盖层"，不引入新语法）
9. **REQ-TPL-04 include**、**REQ-TPL-05 条件/循环** — ❌ 待做（两项都需先出设计文档与安全边界，见 §4）
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
| `99-archive/06-review/06-15`、`06-17` | 命名空间/分层与架构收敛的决策记录（已归档） |
