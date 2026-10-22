---
status: active
owner: 技术负责人
version: 0.1
last-updated: 2026-10-22
---

# 归档目录

本目录存放**已被后续文档取代的历史材料**。它们保留下来是为了交代"当时为什么这么定"，不是为了描述工具现在是什么样。

## 规则

- **现行规范在别处。** 工具当前的行为以 [`../07-serialization/`](../07-serialization/00-index.md) 为准；需求状态看 [`../02-product/02-05-requirements-registry.md`](../02-product/02-05-requirements-registry.md)；已生效的决策看 [`../04-decisions/`](../04-decisions/04-00-index.md)。
- **归档文档不更新。** 里面的旗标名、依赖版本、目录结构、退出码都可能与今天的代码不符，这是它们被归档的原因。读它们时按"某年某月的记录"来读。
- **新工作不进这里。** 只有被明确取代的文档才移动过来，移动时同时更新指向它的引用。
- **链接仍然有效。** 归档保留原有文件名，指向归档的链接一律写全 `docs/99-archive/...` 路径。

## 目录内容

| 目录 | 是什么 | 为什么归档 |
|---|---|---|
| [`00-raw/`](00-raw/00-00-index.md) | 项目启动前的原始材料：线上痛点证据、需求提出者补充、当时的语法与依赖确认（2026-09-29） | 描述的是**立项时**的世界。里面的 `sonic`/`sigs.k8s.io/yaml` 依赖选型、`--unset`/`--schema-unknown` 旗标名、`env` 命名空间都已不成立 |
| [`01-context/`](01-context/01-00-index.md) | 结构化上下文：已确认事实、推断、假设与缺口（2026-09-29） | 同上，是立项时的快照。`01-02-facts.md` 里的"已确认事实"是本项目最完整的一份历史底稿，值得留档 |
| [`02-product/`](02-product/02-03-prd.md) | 旧版产品文档：功能全景、产品定位、PRD、用户故事 | 已被 [`../02-product/02-05-requirements-registry.md`](../02-product/02-05-requirements-registry.md) 取代。旧稿里的 `--set-string`、`--dry-run`、`--compat` 都没实现 |
| [`03-design/`](03-design/03-05-api-data-tradeoffs.md) | 旧版设计稿：背景与目标、API 与数据模型、开放问题清单 | 被后续设计与实际代码取代 |
| [`03-design/03-09-extension-and-compat-architecture.md`](03-design/03-09-extension-and-compat-architecture.md) | P2 兼容模式架构草案 | 里面的 `engine.Registry`、`engine.Capabilities`、`ValueSource`、`Validator` 已在架构评审里整体删除，按原文实现不了 |
| [`03-design/03-10-external-plugin-protocol.md`](03-design/03-10-external-plugin-protocol.md) | 外部插件协议设计 | 文档自己写明"仅记录需求与设计，当前不实现"；对应需求在需求登记册里是 ❌ |
| [`06-review/`](06-review/06-00-index.md) | 按轮次的实施与评审记录（17 篇） | 记录的是**每一轮当时**的状态，数字与旗标名大多过时。结论若已生效，已经写进 `07-serialization/` 与 ADR |
| [`reference/REF-01-competitive-landscape.md`](reference/REF-01-competitive-landscape.md) | 竞品与市场空白调研（2026-09-29） | 一次性市场快照 |
| [`reference/REF-03-json-schema-reuse-research.md`](reference/REF-03-json-schema-reuse-research.md) | JSON Schema 复用可行性调研（2026-09-29） | 结论已固化进 ADR-003；文中"日期标量待定"的遗留问题后来已解决 |

## 仍留在原处的历史材料

- [`../reference/REF-02-envsubst-compatibility-baseline.md`](../reference/REF-02-envsubst-compatibility-baseline.md) —— P2 兼容模式的输入依据，仍在用。
- [`../reference/REF-04-gnu-envsubst-performance-baseline.md`](../reference/REF-04-gnu-envsubst-performance-baseline.md) —— `test/perf` 门禁的规格，仍在用。
- [`../04-decisions/04-07-adr-007-external-plugin-model.md`](../04-decisions/04-07-adr-007-external-plugin-model.md) —— ADR 状态是 Proposed，只记录方向，但它是决策记录，留在 ADR 目录。
- [`../05-tasks/`](../05-tasks/05-00-index.md) —— 任务拆分与进度看板，是持续更新的工作文档。
