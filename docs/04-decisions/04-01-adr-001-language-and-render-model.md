---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# ADR-001：采用 Go 的层级渲染引擎，一期实现 extended 原生模式

## 1. 标题

ADR-001：采用 Go 的层级渲染引擎，一期实现 extended 原生模式

## 2. 状态

Accepted（已确认），来源：需求提出者确认继续使用 Go 作为 v0/v1 技术选型，2026-09-29。`[FACT]`

## 3. 背景

- 目标是基于 `a8m/envsubst` 构建升级版工具，支持 amd64/arm64、多平台单文件、尽量静态编译。`[FACT]`（来源：需求提出者目标描述）
- 需求提出者补充了默认值、数组表示、转义格式、类型校验、文件配置、多文件配置。`[FACT]`（来源：需求提出者补充）
- 线上证据显示：安装分发、转义、默认值、参数展开、in-place、多文件和错误处理是主要痛点。`[FACT]`（来源：
  `docs/00-raw/00-00-index.md`）
- 需求提出者要求：尽量兼容 envsubst；若 envsubst 方案不合适，可直接换方案；并参考 helmfile 类“渲染”工具。`[FACT]`
  （来源：需求提出者本轮补充）
- 需求提出者确认继续使用 Go；非功能采用常规 Go 二进制优化即可，不额外追求非常规指标。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 4. 决策

建议采用以下具体决策：

- **语言：v0.x/v1 继续使用 Go。** 原因是原工具 `a8m/envsubst` 是 Go，兼容性验证与行为对照成本最低；`CGO_ENABLED=0`
  下多平台静态交叉编译路径成熟；CI、Docker buildx、Go 生态足够支撑 CLI、JSON/YAML、schema 校验。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- **核心模型：层级值模型 + 渲染引擎，而不是纯 envsubst 字符串替换器。** 渲染流程定义为：变量来源加载 → 多文件合并 →
  类型校验/默认值填充 → 转义与模板替换 → 输出/写回。`[INFERENCE]`（依据：六项补充诉求无法由纯 envsubst 语义自然覆盖）
- **兼容模式分期：一期只实现 extended 原生模式，不实现完整兼容模式；二期再按需求加入 `--compat=gnu|a8m|drone` 与转换命令。**
  extended 模式启用默认值、数组、类型校验、文件/多文件配置，不要求与 GNU envsubst 完全一致。`[FACT]`（来源：需求提出者确认，2026-09-29；详见
  ADR-002）
- **参考 helmfile/Helm/gomplate/ytt，而不是复刻它们。** 借鉴 Helmfile/Helm 的多 values 文件、环境覆盖、默认值与 schema 校验；借鉴
  gomplate 的 datasource/函数模型；借鉴 ytt 的结构化数组/YAML 处理。`[INFERENCE]`（依据：需求提出者指定参考方向 + 六项结构化渲染诉求）
- **缺口补齐为默认设计假设：**
    - 默认值：支持值层默认值与模板层默认值。`[ASSUMPTION]` 验证：需求提出者确认语法；owner：待指定。
    - 数组表示：数组作为一等值类型，来源支持 YAML/JSON，输出由模板决定。`[ASSUMPTION]` 验证：确认 `.env` 中数组如何表达；owner：待指定。
    - 转义格式：扩展模式建议 `$$` 表示字面量 `$`，兼容模式按对应实现处理。`[ASSUMPTION]` 验证：确认默认转义符与是否可配置；owner：待指定。
    - 类型校验：已确认 P0；采用 JSON Schema 声明，校验失败默认报错退出；具体失败策略待细化。`[FACT]`（来源：需求提出者确认，2026-09-29）
    - 文件配置：支持 `.env`、YAML、JSON values 文件；TOML 范围待确认。`[FACT]`（来源：需求提出者确认，2026-09-29）
    - 多文件配置：后者覆盖前者，数组默认替换而不是追加；CLI `--set` 优先级最高。`[FACT]`（来源：需求提出者确认，2026-09-29）
- **Rust 切换条件：** 当前不做切换；如未来出现明确的性能/体积/类型正确性硬指标且 Go 无法满足，可另开 ADR 重新评估。
  `[INFERENCE]`（依据：当前无此类硬指标，需求提出者确认继续 Go）

## 5. 被拒绝的方案

- **立即切换 Rust：拒绝。** 理由是当前没有证据表明 Go 无法满足静态多架构 CLI 目标；重写会丢失与 `a8m/envsubst`
  的代码/语义对照优势；Rust 静态 musl 交叉编译链路和发布复杂度通常更高；团队 Rust 能力尚未确认。若触发条件满足，可另开 ADR。
  `[INFERENCE]`（依据：现有代码库、目标约束、缺失基准数据）
- **坚持纯 envsubst 语义，不引入结构化渲染模型：拒绝。** 理由是默认值、数组、类型校验、文件/多文件配置无法由 GNU envsubst
  的字符串替换模型自然表达；强行兼容会导致大量特殊分支和不可预测行为。envsubst 应作为兼容模式，而不是核心数据模型。
  `[INFERENCE]`（依据：六项补充诉求 + GNU envsubst 已知限制）
- **直接基于 Helmfile/gomplate/ytt 二次开发：拒绝。** 理由是这些工具定位分别是编排器、通用模板渲染器、YAML
  结构化模板器，不一定满足“单静态二进制、envsubst 升级、可控兼容模式”的产品边界；可作为设计参考。`[INFERENCE]`
  （依据：需求提出者目标与工具定位差异）

## 6. 后果

- 正面：复用 Go 生态与 a8m 兼容基础，迭代和交叉编译成本低；结构化值模型能支撑六项补充诉求；兼容/扩展模式隔离可减少升级冲突。
  `[INFERENCE]`（依据：决策内容）
- 负面：Go 二进制体积可能大于 Rust 优化结果；双模式/多文件合并会增加测试矩阵；结构化渲染若设计过度会偏离 envsubst 简洁性。
  `[INFERENCE]`（依据：语言与功能复杂度）
- 风险：团队 Go 能力、schema 表达力、数组/深合并语义、默认值语法、兼容测试覆盖不足。`[TODO]` 均需在 Design Doc
  中定义并验证，owner：技术负责人。

## 7. 相关文档

- `docs/00-raw/00-00-index.md`
- `docs/01-context/01-00-index.md`
- `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`
- 待创建：Design Doc、PRD、任务拆分。`[TODO]` owner：技术负责人/产品负责人。

## 非目标（Non-Goals）

- 不做 Helmfile/Helm/Kustomize 的发布编排替代品。`[ASSUMPTION]` 验证：确认工具边界；owner：待指定。
- 不承诺完整 POSIX shell、Helm 模板语言或全部 envsubst 实现 100% 兼容。`[ASSUMPTION]` 验证：确认兼容矩阵；owner：待指定。
- 首版不做 GUI/Web 界面、插件市场或远程配置中心。`[ASSUMPTION]` 验证：确认首版交付形态；owner：待指定。
