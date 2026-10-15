# 外部插件协议与功能边界（设计记录，暂不实现）

> **状态：仅记录需求与设计，当前不实现。** 本阶段先做的是 **L0 仓内接口统一重构**（`05-04` TASK-207），
> 不含任何插件执行代码；L1–L3 的实现按 TASK-208/209/210 分阶段，需另行授权。
> 相关：`REQ-SRC-14`（值来源插件）、`REQ-SRC-15`（外部格式插件）、`REQ-TPL-10`（引擎插件）、
> `REQ-CLI-09`（子命令插件）；形态取舍见 [ADR-007](../04-decisions/04-07-adr-007-external-plugin-model.md)；
> 仓内扩展架构见 [`03-09`](03-09-extension-and-compat-architecture.md)。

## 1. 插件的意义（判据）

插件的意义只来自**表达力增益**：用户能写出以前写不出的东西。只是"换个地方实现同一语义"不叫插件，
那叫把代码统一到接口（本阶段 L0 做的正是后者，命名上也不该混用）。`[FACT]`

| 增益类型 | 例子 | 是否构成插件意义 |
|----------|------|------------------|
| 输入面 | 新格式（TOML/HCL/INI）、新来源（HTTP/Vault/SOPS） | ✅ 中：数据可达性 |
| 模板方言/函数 | 自定义语法、过滤器库、兼容模式引擎 | ✅ 高：语言面 |
| 输出面 | 写 S3/git/Vault、自定义 diff 呈现 | ⚠️ 弱：副作用风险高 |
| 语义权（合并、层序、物化、策略、退出码） | — | ❌ 让出去工具就不再确定（K4/K7） |

**结论**：插件 = 扩展**输入面 + 模板方言**，不扩展**语义权**。

## 2. 能力分层

| 层 | 内容 | 表达力增益 | 调用粒度 | 判定 |
|----|------|------------|----------|------|
| **L0** | 统一接口 + 注册表（值来源格式、输出、引擎），实现仍在仓内 | 0（纯重构） | — | ✅ 先做，与做不做插件无关 |
| **L1** | 输入面插件：格式 / 来源（子进程 + 清单 + NDJSON） | 中 | 1 次/文件、1 次/URL | ✅ 便宜且安全 |
| **L2** | 引擎 / 方言插件：请求＝模板字节 + 合并后的值树 + 策略；响应＝渲染字节 + 引用路径 + 来源帧 | 高（语言面） | **1 次/模板** | 🟡 有意义，但与 `REQ-TPL-08/09` 的既有结论冲突，待产品侧拍板 |
| **L3** | formatter / 函数插件（常驻 sidecar） | 高（函数面） | 每次值变换 | 🟡 可行但需性能契约（默认关、不进性能门禁） |

### 2.1 调用成本（本机实测，2026-10-14）

| 粒度 | 单次成本 | 400 次（200 占位符 × 2 管道步） | 10 MiB 模板（约 10⁵ 次） |
|------|----------|--------------------------------|--------------------------|
| 进程内（现状） | ns–µs | 与整次渲染同阶 | 秒级（`test/perf` 门禁内） |
| 常驻 sidecar（管道往返，含 JSON） | **24 µs** | ≈10 ms | ≈2.4 s（会吃掉门禁预算） |
| 每次 `fork+exec` 原生程序 | **0.6 ms** | 240 ms | ≈60 s |
| 每次解释器启动（`python3`） | **9 ms** | 3.6 s | ≈15 分钟 |
| 参照：整次真实渲染（examples/realworld） | — | — | 9 ms |

因此：**每值 `fork+exec` 与"同进程加载第三方代码"必须拒绝**；**引擎插件（1 次/模板）不该拒绝**。

## 3. 功能边界（Functional boundary）

**边界公式**：插件调用次数与**输入个数**同阶（O(插件数)~O(输入数)），绝不与**值数 × 管道步数**同阶。

| 层 | 单元 | 一次渲染的典型次数 | 归谁 |
|----|------|--------------------|------|
| 取一份数据（解码一个值文件） | 文件 | 1–5 | ✅ 格式插件 |
| 从外部系统取一份值 | URL / 密钥路径 | 1–3 | ✅ 来源插件 |
| 值树合并/覆盖/schema 物化/策略/退出码 | — | — | ❌ **核心** |
| 值的使用：`${x}` 展开、`\| formatter` | 占位符 × 管道步 | 10¹–10⁵ | ❌ **核心** |
| 模板解析与渲染 | 每次渲染 | 1 | ❌ 核心（引擎插件为 1 次/模板，见 L2） |
| 写一个输出目标 | 目标 | 1–500（批量） | ⚠️ 输出插件（P2，可接受） |
| CLI 子命令（`envsub <name>`） | 每次命令 | 1 | ✅ 子命令插件 |

### 3.1 协议**故意不提供**的帧（故越界设计不可表达）

| 不存在的帧 | 因此无法做 |
|------------|------------|
| `eval` / `format` / `render` | formatter 插件、模板片段插件、每值调用（协议层排除） |
| `tree`（把合并中的值树交给插件） | 插件无法篡改其他层、无法自行决定优先级 |
| `write` / `commit` | 格式/来源插件不能改变核心的写出与 `--check` 语义 |

### 3.2 每类插件的正向/负向清单

| 类别 | 必须做 | 可以做 | 绝不可以 | 能力门禁 | 次数上限（默认） |
|------|--------|--------|----------|----------|------------------|
| `ValueFormat` | 字节 → 值树（JSON 兼容类型 + 可选 `origin.line`） | 纯解析、格式校验、报错定位 | 触网、读其他值文件、写文件、决定合并 | `filesystem`（仅在需要读被包含文件时） | 1/文件 |
| `ValueSource` | 目标（URL/密钥路径）→ 值树 | 网络、本地解密（SOPS/age）、用 `$ENVSUB_PLUGIN_CACHE_DIR` 缓存 | 读核心合并后的树、写用户目录、把凭据写进日志 | `network`（需 `--allow-remote`）、`secrets` | 1/目标 |
| `Engine`（L2） | 模板字节 + 值树 → 渲染字节 + 引用路径 | 自定义语法/过滤器；只读值树 | 改写值树、自定退出码策略、写文件、触网 | — | 1/模板 |
| `Subcommand` | 提供 `envsub <name>` 行为 | 任意（它就是用户自己的命令） | 参与值树/渲染结果 | exec 需显式授权 | 1/命令 |
| `Output`（仅设计，P2） | 把已渲染字节写到目标 | 分块写、校验和、重试 | 改变字节内容、影响 `--check` 判定 | `network`/`secrets` | 1/目标 |

### 3.3 会话模式与护栏

- **会话模式**：一次渲染中同一插件只启动**一个进程**，多个输入用 `{"frame":"request","id":N}` 复用
  （插件在 `hello` 中声明 `features:["session"]`）；不可用时退化为"一请求一进程"。
- `--plugin-max-calls`（默认 64，按**不同输入**计数）：误设计成 O(值) 时立刻报错，而不是慢慢跑。
- `--explain` 记录每次插件调用（`plugin`、`requests`、`bytes`、`duration`）→ 满足 K5"可审计"。
- `envsub plugin verify --deterministic`：同输入调两次并比对 → K4 的可验证手段。

## 4. 协议草案（v1）

### 4.1 清单 `plugin.yaml`（用现有 `yamlx` 解析，零新依赖）

```yaml
apiVersion: envsub.dev/v1
kind: ValueFormat          # ValueFormat | ValueSource | Engine | Subcommand
name: toml
version: 0.1.0
description: TOML value files
usage: envsub render -f app.toml
runtime:
  command: ./envsub-format-toml     # 相对插件目录；必须可执行
  args: []
  env: ["FOO=bar"]                  # 显式追加环境
capabilities: [filesystem]          # network 需要 --allow-remote 才放行
limits:
  timeoutMs: 5000
  maxOutputBytes: 1048576
```

发现顺序：`--plugin-dir DIR`（可重复）→ `$ENVSUB_PLUGIN_DIR`（`:` 分隔）→
`$XDG_DATA_HOME/envsub/plugins`（默认 `~/.local/share/envsub/plugins`）；每个插件一个目录 + 一份清单；
**默认不扫 `PATH`**（K1"默认不执行外部命令"），仅在 `--allow-plugins` 下允许自动发现。

### 4.2 帧（NDJSON，逐行 JSON）

```
► {"frame":"hello","api":"envsub.dev/v1","features":["session","log"],"capabilities":["filesystem"]}
◄ {"frame":"request","id":1,"kind":"ValueFormat","name":"toml","source":{"file":"app.toml"},
   "content":{"encoding":"base64","data":"…"}}
► {"frame":"log","level":"info","message":"3 tables"}
► {"frame":"origin","path":["app","port"],"line":12,"kind":"values","method":"plugin"}
► {"frame":"values","value":{"app":{"port":8080}}}
► {"frame":"result","source":"app.toml"}          # 结束帧
► {"frame":"error","kind":"type","message":"…"}   # 失败结束帧
```

核心在**每帧**上强制：单 JSON 对象；总输出 ≤ `maxOutputBytes`；`api` 主版本匹配；`capabilities` 与
开关一致；值类型落在值文法白名单内（`time.Time` 这类越界在此被拦，同 G24）；`kind` 落在闭集
（插件一律 `values`），新能力只加 **`method: plugin`**；键按**字面键**处理（同 G23，点号不构成路径）。
超过 `--plugin-inline-limit`（默认 256 KiB）时内容落临时文件（0600，用后即删），帧里给路径。

### 4.3 环境契约

最小集 `PATH`/`HOME`/`TMPDIR`/`TZ` + `ENVSUB_PLUGIN_DIR`/`_NAME`/`_VERSION`/`_API`/`_TARGET`/`_CACHE_DIR`
＋清单 `runtime.env`；**值永不进环境**；凭据用 `--plugin-env KEY=VALUE` 或 `@env:VAR` 显式传入。

### 4.4 失败矩阵

| 情形 | 退出码 | 诊断 |
|------|--------|------|
| 清单缺失/字段非法/命令不可执行 | 2 | `plugin <name>: …` |
| `api` 主版本不匹配 | 2 | 期望版本与实际版本 |
| 能力未获授权（`network` 无 `--allow-remote`） | 2 | 指出缺失开关 |
| 非 0 退出（无 `error` 帧） | 5 | stderr 截断文本 |
| 坏帧 / 非法 JSON / 未知帧类型 | 5 | 帧号与原因 |
| 输出超限 | 5 | 上限值与实际值 |
| 超时 | 5 | 超时值 |
| 值类型越界 / `kind` 越界 | 4 | 路径与类型 |
| 请求数超过 `--plugin-max-calls` | 2 | 指出疑似 O(值) 设计 |

## 5. L2 引擎插件（最有意义的一层）

- 请求：`{template, values(合并后的树，含类型与字面键), policy(on-missing/on-empty), limits}`
- 响应：`{rendered, referenced[](支撑 --explain/--report=unused/--only-vars), origin[](ADR-005 闭集)}`
- 核心保留：合并、层序/`shadowed`、schema 物化、策略、退出码、写出、脱敏；引擎插件只做
  "模板字节 → 输出字节"的**纯函数**（无副作用；确定性由契约要求）。
- `--compat=gnu/a8m` 这类**产品承诺**仍在仓内实现（TASK-201）。
- **张力（必须产品侧拍板）**：引擎插件允许用户替换模板语义，与本册 `REQ-TPL-09 切换模板引擎`（⛔）、
  `REQ-TPL-08 模板函数注册`（⛔）以及 K7"受限渲染引擎"的表述冲突。可能的措辞修订：
  **内置引擎受限期约束；用户显式启用的第三方引擎，风险由用户自担**——本例为记录项，**未拍板、未实现**。

## 6. 与 Helm 插件对照

| 维度 | Helm | 本设计 | 为什么不同 |
|------|------|--------|------------|
| 扩展形态 | `helm-<name>` 可执行；`helm foo` 派发 | 同样有子命令插件，另有数据插件（格式/来源/引擎） | Helm 插件不参与 Helm 数据模型；我们要把值树与来源元数据交回核心 |
| 清单 | `plugin.yaml`（name/version/usage/command/hooks） | `plugin.yaml`（apiVersion/kind/name/runtime/capabilities/limits） | 用 `kind` 声明扩展点、`capabilities` 做门禁、`limits` 由核心强制 |
| 安装 | `helm plugin install <git|url>` + 下载器/hooks | **一期不做安装器**（用户自行放目录 + `plugin verify`） | 安装＝执行下载/构建脚本，属 K1/K5 危险能力，需单独评审 |
| 数据通道 | 无（插件自己读写文件） | NDJSON 帧 | 核心需要结构化结果与来源 |
| 发现 | `$HELM_PLUGINS`/`$HELM_PLUGIN_DIR`，PATH 中 `helm-*` | 显式目录 + XDG，默认不扫 PATH | K1 |
| 环境 | 全套 `HELM_*` | 最小集 + 清单追加 | 降低凭据泄漏与不确定性 |

## 7. 主题盘点：可插件化与明确不插件化

| 主题 | 现有接缝 | 类别 | 收益 | 结论 |
|------|----------|------|------|------|
| 值来源格式（TOML/INI/HCL…） | `values.Loader` + `source.ForFile` 扩展名分派 | `ValueFormat` | 格式生态不进核心 | ✅ L1 |
| 值来源获取（HTTP/Vault/S3） | `values.Loader` 变体 + `load.Spec` | `ValueSource` | 核心永不触网 | ✅ L1 |
| 子命令（`envsub diff/lint/fmt`） | `cli/root.go` 的 `AddCommand` | `Subcommand` | 扩展 CLI 而不改核心 | ✅ L1 |
| 渲染引擎（`--compat`、自定义方言） | `engine.Engine`（当前无注册表，`engine.go:130`） | `Engine` | 语言面扩展 | 🟡 L2，待拍板 |
| formatter / 管道函数 | `internal/pipeline` 的 `Apply`/`Names()` | `Formatter` | 用户自定义函数 | 🟡 L3（sidecar + 性能契约） |
| 输出写入器（S3/git/Vault） | `internal/output` 的 Writer/StreamWriter | `Output` | 集成面 | ⏸ P2 设计 |
| schema 校验器（CEL/OPA） | `schema.Load` | `Validator` | 自定义规则 | ❌ 暂不做（校验结果影响退出码，契约需先冻结） |
| 报告/诊断消费 | `render` + `diag` 的稳定 JSON/JSONL | `Reporter` | 低（外部脚本已可消费） | ❌ 不做 |
| `\| type` 类型转换 | `internal/types` | — | 无 | ❌ 不做（属值文法核心语义） |

## 8. 非目标

- 不实现安装器（下载/构建脚本）——属 K1/K5 危险能力，需单独评审。`[FACT]`
- 不提供 `eval`/`format`/`render` 之外的"每值"通道；不接受同进程加载第三方代码（Go `plugin` 需 cgo，破 K3；
  WASM 需嵌入运行时，破 K3 低依赖）。`[FACT]`
- 不让插件获得语义权（合并/层序/物化/策略/退出码）。`[FACT]`
- **本文件不含任何 `internal/` 实现**；测试桩 `test/pluginstub`（仅测试用）是否入库**待确认**。`[TODO]` owner：技术负责人。

## 9. 验证计划（未来实现时）

| 层 | 证据 |
|----|------|
| 清单与发现 | `plugin list` / `plugin verify` 单测：字段校验、目录优先级、同名冲突处理、不扫 PATH |
| 帧协议 | 桩插件（`test/pluginstub`）：握手、会话复用、未知帧、坏 JSON、超时、超限、非 0 退出 |
| 契约校验 | 值类型越界（复用值文法白名单测试）、`kind` 越界、`method=plugin`、字面点号键不构成路径 |
| 能力门禁 | 声明 `network` 而无 `--allow-remote` → exit 2；`--plugin-max-calls` 触发 → exit 2 |
| 确定性 | `plugin verify --deterministic`（同输入两次比对） |
| E2E | fixture：格式插件成功/失败；来源插件成功/失败；`--explain` 显示 `method=plugin` |
