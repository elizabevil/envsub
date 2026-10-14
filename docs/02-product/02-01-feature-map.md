---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# envsubst 升级版功能全景与场景覆盖

> 目标：在兼容 envsubst 的基础上，补齐现代化配置渲染工具需要的能力，并确保已知使用场景都有对应功能。  
> 本文为功能候选地图，优先级与取舍需人工确认。
> **各需求当前状态与"能否在本应用实现"的判定见** [`02-05-requirements-registry.md`](02-05-requirements-registry.md)。

## 1. 设计原则

- 尽量兼容 GNU envsubst / a8m envsubst 的简单替换场景，但不让兼容性限制扩展能力。`[INFERENCE]`（依据：需求提出者目标与线上证据）
- 单静态二进制、多平台多架构、低运行时依赖；采用常规 Go 二进制优化（`CGO_ENABLED=0`、`-ldflags="-s -w"`、strip/多架构交叉编译）。
  `[FACT]`（来源：需求提出者确认，2026-09-29）
- 核心模型采用层级值模型 + 渲染引擎；envsubst 兼容作为 P2 边界。`[INFERENCE]`（依据：ADR-001、ADR-002）
- 默认安全、可预测、确定性输出；危险能力必须显式启用。`[INFERENCE]`（依据：CI/CD、容器与密钥场景推断）
- 功能做“可组合的层次”，避免把简单替换工具变成不可控的通用编程语言。`[INFERENCE]`（依据：envsubst 简洁定位与维护成本）

## 2. 场景覆盖矩阵

| 场景               | 必须具备的能力                                       | 建议优先级 | 标注           |
|--------------------|------------------------------------------------------|------------|----------------|
| 单文件/管道替换    | stdin/stdout、`$VAR`/`${VAR}`、退出码                | P0         | `[INFERENCE]`  |
| 容器/NGINX 配置    | 可控替换范围、`$` 转义、白名单、多架构静态二进制     | P0         | `[INFERENCE]`  |
| K8s/GitOps 配置    | 多文件、目录批处理、dry-run、diff/check              | P1         | `[INFERENCE]`  |
| CI/CD 配置生成     | CLI `--set`、严格模式、错误码、JSON 错误、确定性输出 | P0/P1      | `[INFERENCE]`  |
| 多环境配置         | profiles/environments、多 values 文件、覆盖优先级    | P1         | `[INFERENCE]`  |
| 结构化配置         | 数组、map、类型校验、schema、默认值                  | P0/P1      | `[INFERENCE]`  |
| 本地开发           | `.env`、默认值、shell completion、dry-run            | P0/P1      | `[INFERENCE]`  |
| 密钥注入           | 文件/stdin/环境变量读取、日志脱敏、权限控制          | P1         | `[INFERENCE]`  |
| Windows/PowerShell | CRLF、路径、多架构 Windows 二进制                    | P1         | `[INFERENCE]`  |
| 大文件/大规模      | 流式处理、批量/并行、稳定内存占用                    | P1/P2      | `[INFERENCE]`  |
| 未来扩展           | 插件/data source、远程 secret、模板函数              | P2         | `[ASSUMPTION]` |

## 3. 功能全景

### 3.1 兼容与替换

- 兼容模式（P2）：`--compat=gnu|a8m|drone|extended`；一期只实现 extended 原生模式。`[FACT]`（来源：ADR-002）
- shell 风格参数展开：`${VAR:-default}`、`${VAR:=}`、`${VAR:+}`、`${VAR:?}`、嵌套变量。`[INFERENCE]`
- 转义格式：`$$`、`\` 或可配置转义符；兼容模式与扩展模式行为分离。`[INFERENCE]`
- 变量范围控制：allowlist/denylist、只匹配大写变量、忽略 `$1` 等非环境变量。`[INFERENCE]`
- 可配置分隔符、注释语法、保留原始占位符策略。`[INFERENCE]`
- 未设置/空值策略：空字符串、保留原样、报错、警告。`[INFERENCE]`

### 3.2 值与配置来源

- 环境变量、`.env` 文件、YAML/JSON/TOML values 文件。`[INFERENCE]`
- 多个 values 文件按顺序合并，支持覆盖优先级。`[INFERENCE]`
- CLI 覆盖：`--set`、`--set-string`、`--set-file`、stdin。`[INFERENCE]`
- profiles/environments/selectors：dev/staging/prod 等环境选择。`[INFERENCE]`
- 值引用与插值：配置值内部引用其他值，检测循环引用。`[INFERENCE]`
- 工具自身配置文件：默认选项、profiles、文件搜索路径。`[INFERENCE]`

### 3.3 类型、数组与校验

- 一等值类型：`string`、`bool`、`int`、`float`、`array<T>`、`object`、`null`；内部映射 JSON Schema 标准类型。`[FACT]`（来源：需求提出者确认与
  ADR-003）
- 数组表达：YAML/JSON 原生数组；`.env` 等文本格式提供明确编码规则。`[INFERENCE]`
- schema 校验：直接复用 JSON Schema Draft 2020-12，采用 santhosh-tekuri/jsonschema/v6 +
  薄适配层；required、type、enum、pattern、range、数组长度、嵌套结构均走标准关键字。`[FACT]`（来源：ADR-003，2026-09-29）
- 快速类型声明：内联 `| type T` + schema 文件；schema 优先，内联只做局部断言/补类型；required 不自动填零。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- 默认值与必填校验在渲染前完成。`[INFERENCE]`
- 类型转换策略：严格模式 / 宽松模式，错误必须带文件、行、列、变量名。`[INFERENCE]`
- 敏感值标记与输出脱敏。`[INFERENCE]`
- **类型检查边界**：规划支持值级/schema 级运行时类型检查；不承诺对任意纯文本模板做静态类型检查。`[INFERENCE]`
  （依据：纯文本模板缺少类型信息；YAML/JSON values 可保留类型）
- 示例：`port: int`、`hosts: array<string>`、`debug: bool`；来自 env/.env 的字符串可按 schema 解析并校验。`[INFERENCE]`

### 3.4 渲染与模板能力

- 基础变量替换与结构值插值。`[INFERENCE]`
- 条件与循环 P1，仅在扩展模式启用；object 遍历按键排序，控制指令单独成行。`[FACT]`（来源：需求提出者确认，2026-09-29）
- includes/partials P1；执行时指定文件，细节在 P1 设计和实现时确定。`[FACT]`（来源：需求提出者确认 P1 实现
  include，2026-09-29）
- 内置函数：upper/lower/trim/quote/base64/json/yaml/url/sha 等。`[INFERENCE]`
- 默认禁止任意 shell 执行与远程代码加载。`[INFERENCE]`
- 可解释渲染：记录变量来源、覆盖链与未使用变量。`[INFERENCE]`

### 3.5 批量、写回与输出

- 单文件、多文件、目录递归、glob 批量渲染。`[INFERENCE]`
- 安全 in-place：临时文件 + 原子替换，避免 `< file > file` 截断。`[INFERENCE]`
- 输出目录/镜像路径、输出路径变量化。`[INFERENCE]`
- 权限、行尾、编码和换行保真。`[INFERENCE]`
- `--dry-run`、`--diff`、`--check`，适用于 GitOps/CI。`[INFERENCE]`
- 幂等、确定性输出，便于版本控制与缓存。`[INFERENCE]`
- 大文件流式处理，减少内存峰值。`[INFERENCE]`

### 3.6 CLI、CI 与可观测性

- 机器可读输出：JSON 错误/报告、稳定退出码。`[INFERENCE]`
- verbose/debug/trace 日志与渲染统计。`[INFERENCE]`
- `--explain`：说明每个值来自哪个文件/环境变量/CLI。`[INFERENCE]`
- 静默模式、交互确认开关、禁止覆盖保护。`[INFERENCE]`
- shell completion、man page、迁移指南与兼容矩阵。`[INFERENCE]`

### 3.7 安全与供应链

- 默认不执行命令、不访问网络。`[INFERENCE]`
- 敏感值脱敏、日志不泄露、临时文件权限安全。`[INFERENCE]`
- 定期升级 Go 版本与依赖，修复 CVE。`[INFERENCE]`
- 发布校验：checksums、SBOM、签名、可复现构建。`[INFERENCE]`
- 解析器 fuzz 测试、兼容性 conformance 测试。`[INFERENCE]`

### 3.8 分发与平台

- Linux/macOS/Windows 的 amd64/arm64 静态二进制。`[FACT]`（来源：需求提出者目标描述）
- Docker scratch/distroless 多架构镜像。`[INFERENCE]`
- Homebrew/Scoop/Winget/APT/RPM等包管理器渠道。`[INFERENCE]`
- 离线安装、版本固定与 latest 下载路径。`[INFERENCE]`
- Go 多平台构建链与二进制优化：发布矩阵、checksums 与 SBOM 见 `.github/workflows/release.yml`，体积归因见 `04-04` §8 与 `06-07`。`[FACT]`

### 3.9 扩展性

- 自定义 provider/function 机制；优先考虑独立进程/WASM等隔离方案。`[ASSUMPTION]` 验证：确认是否进入 v1/v2 范围；owner：待指定。
- 远程 secret/datasource：Vault、SOPS、云密钥服务等，P2 预留设计，默认关闭、显式启用，不得违背默认不访问网络的 Non-Goal。
  `[ASSUMPTION]` 验证：P2 前确认安全与依赖约束；owner：待指定。
- 插件 API：首版不做插件市场（§5 非目标）；扩展与兼容边界见 `03-09-extension-and-compat-architecture.md`。`[FACT]`

## 4. 建议优先级

> 当前只实现 P0；P1/P2 保留为待办。

- **P0：覆盖高频核心场景**  
  默认值、转义、数组/基础类型、env/.env/YAML/JSON、多文件合并、Helm-like pipeline（`join`/`toJson`/`toYaml`/`quote`/`indent`
  ）、安全 in-place、批量单文件、静态多架构发布、基础错误码。`[INFERENCE]`
- **P1（待办，当前不实现）：include + 生产增强**  
  include/partials、受限条件/循环；profiles/environments、CLI `--set`、dry-run/diff/check、JSON
  报告、密钥脱敏、确定性输出、容器镜像、conformance 测试、迁移文档。`[FACT]`（来源：需求提出者确认 P1 实现 include，2026-09-29）
- **P2（待办，当前不实现）：兼容其它实现与生态扩展**  
  GNU/a8m/drone 兼容模式与 `migrate`/`convert`；更多内置函数、变量来源解释、watch、插件/datasource、远程 secret 集成。`[FACT]`
  （来源：需求提出者确认 P2 实现兼容其它，2026-09-29）

## 5. 非目标（Non-Goals）

- 不做 Helmfile/Helm/Kustomize 的发布编排替代品。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 不做图灵完备的通用编程语言；不实现任意 shell 执行、递归函数、网络访问等通用计算能力。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 不做完整 Helm/Jinja 模板引擎；条件、循环、include 仅作为 P1 受限能力。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 首版不做 GUI/Web 界面、远程配置中心、插件市场。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 默认不执行 shell 命令、不访问网络、不自动读取远程密钥。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 6. 待确认信息

- P0/P1/P2 范围：以 `02-05` 需求台账为准（逐条含优先级与实现状态），任务拆分见 `05-01`/`05-04`。`[FACT]`
- 条件/循环/include 的验收标准见 `05-04` TASK-115/TASK-114 的"验收"列；需求状态见 `02-05` REQ-TPL-04/05。`[FACT]`
- 远程密钥：`02-05` REQ-SRC-13 定为 P2，且只做**本地可解密**形态（SOPS 类），网络型按 K1 排除；HTTP 值来源见 REQ-SRC-11（须 `--allow-remote` 显式开启）。`[FACT]`
- 平台/格式覆盖：Windows 见 `02-05` REQ-DIS-04、TOML 见 REQ-SRC-06（P1，待决策）、包管理器渠道见 REQ-DIS-03；任务见 `05-04` TASK-204/TASK-205。`[FACT]`
- 非功能指标已定：10MB/200k 变量模板耗时与峰值内存 ≤ GNU envsubst 基线 5 倍、空输入启动 ≤100ms（`reference/REF-04`，由 `test/perf` 门禁执行）。`[FACT]`
