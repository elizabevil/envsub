---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-12
---

# P1 / P2 任务拆分（可排期清单）

> **用途**：`05-01` 只把 P1/P2 列为"待办位置"（10 条，无目标与验收）；`02-05-requirements-registry.md` 给出了
> 每条需求的**可实现性判定**。本文把二者接起来：每个任务都有**需求编号、目标、验收口径、依赖与风险**，
> 可以直接排期或拒绝。
>
> 编号 `TASK-1xx`（P1）与 `TASK-2xx`（P2）接续 `05-01` 的 `TASK-0xx`，不复用旧编号。
> `05-01` 的 `TASK-B0x` 保留为历史位置记录，与本文的对应关系见 §6。

## 1. 分波（Wave）总览

波次按"依赖少、风险低、收益直接"排序；同一波内任务互相独立，可并行。

| 波 | 任务数 | 主题 | 是否可直接开工 |
|----|--------|------|----------------|
| W1 | 6 | 独立小项：completion、机器可读错误与统计、`--check`、安全债、`--set-file` | ✅ **已完成**（2026-10-12） |
| W2 | 3 | 平台与保真：Windows/CRLF、权限 mode、Docker 多架构 | ✅ 可直接开工 |
| W3 | 6 | 需要设计的特性：内置函数、profiles、TOML、include、条件/循环、值引用 | ⚠️ 部分需先出设计（见"开工前"列） |
| W4 | 5 | P2：兼容模式与转换、远程来源、`migrate`、包管理器、man page | ⚠️ 需先决策 |

**统计**：P1 已完成 6 项（W1）、待做 6 项（W2/W3）、需设计 3 项；P2 项 6 个（TASK-206 已完成，余 5 项）；
**被判定 ⛔ 不做**的需求见 `02-05` §4，不进本清单。

> **2026-10-12 范围修订**（产品侧论证结论，见本节末"`--dry-run` / `--diff` / `--check` 的取舍"）：
> 删除原 TASK-104（`--dry-run`：无独立语义，被 stdout 与 `--check` 完全覆盖）；
> `--diff` 移入"待需求"（呈现便利，外部 diff 已可替代，且需自研约 150 行或引入依赖）；
> TASK-105 收窄为 **`--check`**（唯一有不可替代点的功能：零写入、批量粒度、可移植、专用退出码）。
>
> **W1 落地记录（2026-10-12）**：TASK-101/102/103/105/106/107 全部完成，每项含实现、单测/e2e
> 与规则文档（`07-serialization/04-cli.md` §6.5.0–§6.5.4 的 C13–C17）。验证：`just check` 与
> `just test-all`（含 4 个 fuzz 目标与性能门禁）全绿。

## 2. W1：独立小项（建议下一轮落地）

### TASK-101 shell completion — ✅ 已完成（cobra 已提供，补防退化守护：命令挂载 + `__complete` 动态补全）

| 项 | 内容 |
|----|------|
| 需求 | REQ-CLI-04（P1，✅） |
| 目标 | 暴露 `envsub completion bash\|zsh\|fish\|powershell`；生成的补全能补出子命令与全部旗标 |
| 验收 | 新增 e2e fixture：`completion bash` 退出 0 且 stdout 含 `render` 与 `--dotenv-file`；`--help` 列出 `completion` |
| 依赖 | 无（cobra 自带） |
| 风险 | 低 |
| 参考 | `02-05` §5 第 1 项 |

### TASK-102 机器可读错误报告 — ✅ 已完成（`--error-format=json`；2 fixture + 单测）

| 项 | 内容 |
|----|------|
| 需求 | REQ-OUT-08（P1，✅） |
| 目标 | `--error-format=text\|json`；`json` 时错误以单个 JSON 对象写 stderr，含 `kind`/`exitCode`/`file`/`line`/`message` |
| 验收 | e2e：同一条错误在两种格式下的**退出码一致**；JSON 可被 `json.Unmarshal` 解析；`diag.Kind` 全覆盖 |
| 依赖 | `internal/diag` 已有分类（K1..K6） |
| 风险 | 低；需冻结 JSON 字段名（一次性契约） |

### TASK-103 未使用值报告 — ✅ 已完成（`--report=unused`；6 单测 + 2 fixture）

| 项 | 内容 |
|----|------|
| 需求 | REQ-OUT-09（P1，✅） |
| 目标 | `--explain=unused`（或 `--report=unused`）：列出值文件里**从未被模板引用**的键，按路径排序 |
| 验收 | e2e fixture：值文件含 3 个键、模板只用 1 个 → 报告列出另 2 个；与 `--explain` 一致地只写 stderr、不影响 stdout |
| 依赖 | `engine.PathEngine` 已能枚举引用名；`--explain` 已有 JSONL 通道 |
| 风险 | 低；注意与 `--only-vars` 过滤的交互（被过滤掉的引用是否算"未使用"需在任务内定） |

### TASK-105 `--check`（检测生成物是否过期） — ✅ 已完成（`--check` + exit 7；5 fixture + 5 单测）

| 项 | 内容 |
|----|------|
| 需求 | REQ-OUT-05（P1，✅） |
| 目标 | 渲染结果与现有目标**逐字节比较**，有差异则报告并退出非零；**一个字节都不写** |
| 作用范围 | 仅在有输出目标时有效：`-o`（单文件）、`-i`（回写目标）、`--output-dir`（批量/递归，一次给出全部过期目标） |
| 边界（必须先定，见下） | ① 与 `--validate-only` 互斥（usage，exit 2）；② 无任何输出目标 → usage，exit 2；③ 目标不存在计为"需要生成"（差异），非 usage；④ 只报告哪些目标过期，不打印内容（要看变化用 stdout 或外部 diff）；⑤ 差异退出码 **7**（1–6 已占用，且不与 `cmp` 的 1 混淆） |
| 验收 | e2e：目标一致 → exit 0 且无输出；目标有漂移 → exit 7 且列出过期目标路径；目标不存在 → exit 7；`--check --validate-only` → exit 2；`--check` 不写任何文件（运行前后目标 mtime/内容不变）；`--check --output-dir --recursive` 多目标场景列出全部过期项 |
| 依赖 | `render` 的渲染结果与目标路径解析（`-o`/`-i`/`--output-dir` 已有） |
| 风险 | 低-中：仅新增比较逻辑 + 一个退出码契约 |
| 参考 | `02-05` REQ-OUT-05；本文 §1 末的范围修订说明 |

#### `--dry-run` / `--diff` / `--check` 的取舍（2026-10-12，产品侧论证）

| 功能 | 现有等价物 | 不可替代点 | 结论 |
|------|-----------|------------|------|
| `--dry-run` | 完全可替代（stdout 即"不写"；`--check` 天然不写） | 无 | ❌ 删除任务 |
| `--diff` | `render … \| diff - target`、`git diff`（配合 `-i`） | 仅呈现便利；需自研约 150 行或引入依赖 | ⏸ 待需求出现（实现时是 `--check` 的一个输出模式） |
| `--check` | 部分（`-i` + `git diff`；单文件 `cmp`，但 `cmp` 用 1 与内部错误码撞车） | 零写入、批量粒度、跨平台、专用退出码 | ✅ 保留 |

### TASK-106 安全债：脱敏、临时文件权限、fuzz — ✅ 已完成（脱敏修复、临时文件权限守护、4 个 fuzz 目标；接入 test-fuzz）

| 项 | 内容 |
|----|------|
| 需求 | REQ-SEC-02/03/05（P1，✅） |
| 目标 | ① 诊断/日志不出现敏感值（含 `.env` 值进入错误信息的路径）；② 临时文件 `0600` 且原子替换后权限与源文件一致；③ scanner / expression / dotenv 三个解析器加 fuzz target |
| 验收 | 单测：构造含敏感值的失败用例，断言错误串不含该值；`output` 包断言 mode 位；`go test -fuzz=FuzzX -fuzztime=10s` 无 crash（CI 里跑短时版本） |
| 依赖 | 无 |
| 风险 | 低；fuzz 需要 CI 配短时运行策略，避免时长失控 |

### TASK-107 `--set-file KEY=FILE` — ✅ 已完成（`--set-file`；4 单测 + 2 fixture）

| 项 | 内容 |
|----|------|
| 需求 | REQ-SRC-05（P1，✅） |
| 目标 | 从文件读取值写入根（与 `--set` 同层），支持与 `--set-json` 一致的键路径文法 |
| 验收 | e2e：`--set-file K=file` 后 `${K}` 得文件内容（去尾换行规则需在任务内定）；文件不存在 → exit 5；与 `--set` 同键时的先后顺序有 fixture 钉住 |
| 依赖 | `values.SetPath`、`source` 文件读取 |
| 风险 | 低；需定"是否去尾换行"（建议不去，保持字节保真并文档化） |

## 3. W2：平台与保真

### TASK-108 Windows / CRLF / 编码保真

| 项 | 内容 |
|----|------|
| 需求 | REQ-DIS-04、REQ-OUT-06（P1，✅） |
| 目标 | `-i` 与 `-o` 保持原文件的行尾（CRLF 不转 LF）、编码（UTF-8 原样）、BOM 策略明确；Windows 路径与 `--output-dir` 行为一致 |
| 验收 | 单测：CRLF 模板渲染后仍 CRLF；e2e 或 Windows CI job 跑通 `in_place`/`output_dir_batch` |
| 依赖 | 需要 Windows 运行环境（当前 CI 只在 linux 上跑测试） |
| 风险 | 中：CI 需新增 Windows runner；BOM 策略需先定 |

### TASK-109 输出权限与 mode 保真

| 项 | 内容 |
|----|------|
| 需求 | REQ-OUT-06（P1，✅） |
| 目标 | `-i` 后 mode 与源文件一致（含可执行位）；`-o` 新建文件用 `0644`（受 umask 影响需明确） |
| 验收 | 单测断言 `os.Stat().Mode()`；含 0755 源文件的 fixture |
| 依赖 | 与 TASK-106 的临时文件项共用实现 |
| 风险 | 低 |

### TASK-110 Docker 多架构镜像

| 项 | 内容 |
|----|------|
| 需求 | REQ-DIS-02（P1，部分已有） |
| 目标 | scratch/distroless 镜像支持 amd64+arm64，发布时推 manifest list；镜像内 `envsub --version` 可用 |
| 验收 | 本地 `docker buildx build --platform linux/amd64,linux/arm64` 成功；镜像大小与二进制一致（无多余层） |
| 依赖 | TASK-022 已有的发布矩阵 |
| 风险 | 中：需要 buildx 与推送凭据；属发布工程 |

## 4. W3：需要设计的特性（先设计、后实现）

> 这三项的**开工前**必须先落一份设计（含语法/语义/安全边界与 fixture 计划），否则实现会与既有规则冲突。

### TASK-111 内置函数扩展

| 项 | 内容 |
|----|------|
| 需求 | REQ-TPL-06（P1，✅） |
| 目标 | 增补纯函数 formatter：`upper` `lower` `trim` `base64`/`base64d` `urlEncode`/`urlDecode` `sha256`（清单以 `02-01` §3.4 为准，实现前裁剪） |
| 验收 | 每个函数：单测 + engine fixture（含非法输入报错文案与退出码）；`pipeline.Names()` 与文档同步 |
| 开工前 | 定函数命名与参数形式（与既有 `join "sep"`、`indent 4` 一致）；定错误分类（多数应是 KindType=4） |
| 风险 | 低；但函数清单最容易膨胀，需一次冻结 |

### TASK-112 profiles / environments

| 项 | 内容 |
|----|------|
| 需求 | REQ-SRC-09（P1，✅） |
| 目标 | `--profile dev`（可重复）把"profile → 值文件/覆盖层"的映射声明化；与 `-f`/`--dotenv-file` 的合并点固定为**层内追加** |
| 验收 | e2e：同键在三层（base / profile / `--set`）下的取值；未定义 profile 报 usage（exit 2） |
| 开工前 | 定映射来源（CLI 多次 `--profile-file`？配置文件？）；**不要引入新语法**，优先复用 `-f` 语义 |
| 风险 | 中：配置来源与优先级必须一次定清，否则与 REQ-SRC-10（工具配置文件）纠缠 |

### TASK-113 TOML 值文件

| 项 | 内容 |
|----|------|
| 需求 | REQ-SRC-06（P1，🟡） |
| 目标 | `-f values.toml` 可用；类型映射与 YAML 一致（整数/浮点/布尔/数组/表） |
| 验收 | 与 YAML 同结构文件的渲染结果**逐字节一致**（对拍性测试）；`.toml` 不再是"无法判定格式"错误 |
| 开工前 | **ADR-004 需补依赖决策**（`pelletier/go-toml` 之类）；确认存在真实 TOML 用户 |
| 风险 | 中：新依赖影响二进制体积（`just size` 复测并写入评审） |

### TASK-114 include / partials

| 项 | 内容 |
|----|------|
| 需求 | REQ-TPL-04（P1，🟡） |
| 目标 | 受限 include：`--include-dir DIR`（可重复）建立白名单；深度上限（候选 3 层）；循环检测；被 include 的文件同样受限替换 |
| 验收 | 正例：两层 include 渲染成功；反例：越出白名单 / 超过深度 / 循环 → 各自稳定错误文案与退出码；`--include-dir` 未给时 include 语法直接报错 |
| 开工前 | 设计文档必须给出：语法（单独成行？）、白名单语义（是否允许 `..`）、错误分类、与流式渲染的关系（include 会破坏单遍流式？） |
| 风险 | 高：与 K5 安全约束、K4 确定性、流式架构三者都有交互，是本清单最需要先设计的一项 |

### TASK-115 受限条件 / 循环

| 项 | 内容 |
|----|------|
| 需求 | REQ-TPL-05（P1，🟡） |
| 目标 | 控制指令单独成行；无递归、无函数定义、无变量捕获；object 遍历按键排序；条件只做真值判断（复用 OV1 的空值定义） |
| 验收 | fixture：`if`/`else`、`range` 数组与 object（排序断言）；嵌套只允许有限层；与 `--on-missing=error` 的交互 |
| 开工前 | 设计文档必须钉住"受限"边界与确定性，否则会滑向 K7 禁止的编程语言 |
| 风险 | 高：与 K7 直接张力，是产品定位问题，不只是实现问题 |

### TASK-116 值引用 / 插值

| 项 | 内容 |
|----|------|
| 需求 | REQ-SRC-08（P1，🟡） |
| 目标 | 值文件内部引用其他值（语法与范围待定），循环引用必须报错而非挂起 |
| 验收 | 正例：A 引用 B；反例：A↔B 循环 → 稳定错误；与 schema 物化的先后有 fixture 钉住 |
| 开工前 | **建议先出设计再决定是否做**：与"无隐式逻辑 + 确定性"张力最大，且与 schema 物化顺序耦合。现有设计记录（现状实测、三模型对照、业界案例、最小边界、8 项决策清单）见 `05-03` §14 |
| 风险 | 高：不做也不影响其它任务，属可选增强 |

## 5. W4：P2（需先决策，缓排）

| 任务 | 需求 | 目标 | 开工前必须先定 |
|------|------|------|----------------|
| TASK-201 GNU/a8m/drone 兼容模式 | REQ-CLI-06 | `--compat=NAME` 切换替换语义；`--compat=extended` 为默认 | 冻结 `REF-02` 的兼容矩阵（哪些行为必须一致、哪些允许不同）；用一个 conformance 套件钉住 |
| TASK-202 `migrate` / `convert` | REQ-CLI-07 | 把 GNU/a8m 语法的模板转换成本项目语法并给出报告 | 依赖 TASK-201 的映射表；ADR-002 二期范围确认 |
| TASK-203 HTTP(S) 值来源 | REQ-SRC-11 | `--allow-remote` 显式开启后，从 URL 读取 values/template；默认关闭 | K1 相容的安全边界：开关名、超时、最大字节、重定向上限、是否禁止进日志、是否允许非 https |
| TASK-204 包管理器渠道 | REQ-DIS-03 | Homebrew/Scoop/Winget/APT/RPM 至少一条通路跑通 | 每个渠道的外部评审周期与签名要求 |
| TASK-205 man page | REQ-CLI-05 | 发布资产含 man page | 生成方式（cobra doc）与版本号来源 |

> TASK-206 已于 2026-10-14 完成：形态定为 `--strip-suffix EXT`（可重复、默认关、最长匹配、撞名报错），
> 见 ADR-006 与 `04-cli` C18；W4 余 5 项。

> TASK-203 对应我此前删除的 `HTTPInput` 占位：**需求保留在册，代码按需重新引入**（连同 `--allow-remote` 与安全边界），
> 不恢复占位类型（依据 `06-review/06-17` D3）。

## 5.5 W5：插件化（**仅记录，暂不执行**）

> 需求登记：`02-05` 的 `REQ-SRC-14`、`REQ-SRC-15`、`REQ-TPL-10`、`REQ-CLI-09`；
> 协议与功能边界见 [`03-10`](../03-design/03-10-external-plugin-protocol.md)，形态取舍见 ADR-007。
> **本阶段只做 TASK-207（L0 重构），其余仅登记；W5 不计入上文 §1 的 P1/P2 统计。** `[FACT]`

| 任务 | 层 | 目标 | 开工前必须先定 | 状态 |
|------|----|------|----------------|------|
| TASK-207 接口统一重构 | L0 | 值来源格式与引擎改为**注册表 + 统一接口**；对外行为零变化，为 `--compat`（TASK-201）与外部插件铺路 | 目标接口形状（见 `03-09` §4、`03-10` §2） | **当前工作项** |
| TASK-208 输入面插件 | L1 | `plugin.yaml` + NDJSON 帧：格式/来源插件；核心零依赖、零网络代码 | 协议 v1 冻结；清单发现顺序；同名冲突策略；是否允许扫 `PATH` | 仅记录 |
| TASK-209 引擎插件 | L2 | 子进程引擎，1 次/模板（含 `referenced`/`origin` 帧） | K7 措辞与 `REQ-TPL-08/09` 的取舍（产品侧拍板） | 仅记录 |
| TASK-210 formatter sidecar | L3 | 常驻进程 + 性能契约（默认关、不进性能门禁） | 是否允许用户自定义函数；门禁策略 | 仅记录 |
| TASK-211 插件安装器 | — | `plugin install` / `plugin verify` | 下载/构建脚本属 K1/K5 危险能力，需单独评审 | **不做**（明确排除） |

## 6. 与 `05-01` 旧编号的对应

| 旧编号（`05-01`） | 新编号 | 说明 |
|------------------|--------|------|
| TASK-B01 include/partials | TASK-114 | 补需求编号与安全边界 |
| TASK-B02 条件指令 | TASK-115 | 与循环合并为一项受限控制流 |
| TASK-B03 循环指令 | TASK-115 | 同上 |
| TASK-B04 profiles/environments | TASK-112 | — |
| TASK-B05 dry-run / diff / check | TASK-105（仅 `--check`） | `--dry-run` 删除、`--diff` 待需求；论证见本文 §2 |
| TASK-B06 容器镜像与 conformance 测试 | TASK-110 | conformance 部分已由 `test/architecture`/`test/examples` 承担 |
| TASK-B07/08/09 兼容模式 | TASK-201 | 三个引擎合并为一个兼容开关 |
| TASK-B10 migrate/convert | TASK-202 | — |

## 7. 非目标（本部分）

- 本文件不排期、不承诺日期与人力。`[ASSUMPTION]`
- 不重新定义需求：需求与可实现性以 `02-05-requirements-registry.md` 为准。`[FACT]`
- 被判定 ⛔ 的需求（插件/WASM、RPC 值来源、换模板引擎、远程 secret 自动读取、静默/交互确认）**不进入本清单**。`[FACT]`
- 不改变 P0 范围；P0 的收尾缺口见 `05-01` 末节与 `05-03`。`[FACT]`
