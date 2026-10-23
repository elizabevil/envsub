---
status: active
owner: 技术负责人
version: 0.2
last-updated: 2026-10-22
---

# 文档导航与编号规则

## 先看哪里

| 你想知道 | 去哪 |
|---|---|
| 工具现在是什么行为、每个规则的具体口径 | [`07-serialization/`](07-serialization/00-index.md) —— **现行规范** |
| 某个需求实现了没有、还能不能做 | [`02-product/02-05-requirements-registry.md`](02-product/02-05-requirements-registry.md) |
| 为什么这么定（已生效的决策） | [`04-decisions/`](04-decisions/04-00-index.md) —— ADR |
| 本轮在做什么、做到哪一步 | [`05-tasks/`](05-tasks/05-00-index.md) |
| 从 `envsubst` 迁过来 | [`migration.md`](migration.md) —— 迁移指南、语法对照与错误提示 |
| 设计是怎么展开的 | [`03-design/`](03-design/03-00-index.md) |
| 性能门禁的规格与基线 | [`reference/REF-04-gnu-envsubst-performance-baseline.md`](reference/REF-04-gnu-envsubst-performance-baseline.md) |
| 这个仓库怎么干活、写文档写代码的规矩 | [`prompts/_common.md`](prompts/_common.md) |
| 当初为什么这么定（已被取代的方案） | [`99-archive/`](99-archive/README.md) —— **历史归档** |

`99-archive/` 里的文档不描述工具现在的样子，旗标名、依赖版本、目录结构都可能过时。

## 编号规则

- `00-raw`：原始材料，文件命名 `00-xx-<name>.md`。
- `01-context`：结构化上下文，文件命名 `01-xx-<name>.md`。
- `02-product`：产品定位与功能地图，文件命名 `02-xx-<name>.md`。
- `03-design`：设计文档，文件命名 `03-xx-<name>.md`。
- `04-decisions`：ADR，文件命名 `04-xx-adr-XXX-<name>.md`，阶段索引 `04-00-index.md`。
- `05-tasks`：任务拆分，文件命名 `05-xx-<name>.md`，阶段索引 `05-00-index.md`。
- `06-review`：评审报告，文件命名 `06-xx-<name>.md`，阶段索引 `06-00-index.md`。
- `07-serialization`：序列化与行为规则，按 § 编号（`NN-<name>.md`），索引 `00-index.md`。
- `reference`：参考调研，文件命名 `REF-xx-<name>.md`，阶段索引 `REF-00-index.md`。
- `prompts`：AI 协作提示词与开发规则。
- `99-archive`：已被取代的历史材料，见其 [README](99-archive/README.md)。
- 每个阶段使用 `xx-00-index.md` 作为阶段索引。

## 阶段索引

- [02-product 索引](02-product/02-00-index.md)：需求登记册
- [03-design 索引](03-design/03-00-index.md)：设计文档
- [04-decisions 索引](04-decisions/04-00-index.md)：ADR
- [05-tasks 索引](05-tasks/05-00-index.md)：任务拆分与进度
- [06-review 索引](06-review/06-00-index.md)：按轮次的评审记录
- [07-serialization 索引](07-serialization/00-index.md)：现行行为规则
- [reference 索引](reference/REF-00-index.md)：参考调研
- [99-archive 索引](99-archive/README.md)：历史归档

### ADR

- [ADR-001 采用 Go 的层级渲染引擎，一期实现 extended 原生模式](04-decisions/04-01-adr-001-language-and-render-model.md) —— Accepted
- [ADR-002 一期仅提供 extended 原生模式，兼容与转换后置二期](04-decisions/04-02-adr-002-compatibility-and-migration.md) —— Accepted
- [ADR-003 复用 JSON Schema Draft 2020-12 + santhosh v6 + 薄适配层](04-decisions/04-03-adr-003-json-schema-reuse.md) —— Accepted
- [ADR-004 项目依赖库选型](04-decisions/04-04-adr-004-dependency-selection.md) —— Accepted
- [ADR-005 `--explain` 的来源模型](04-decisions/04-05-adr-005-explain-origin-model.md) —— Accepted
- [ADR-006 批量输出名去掉模板后缀（`--strip-suffix`）](04-decisions/04-06-adr-006-batch-output-name.md) —— Accepted
- [ADR-007 外部插件模型](04-decisions/04-07-adr-007-external-plugin-model.md) —— Proposed，仅记录，不实现

### 参考

- [envsubst 兼容基线](reference/REF-02-envsubst-compatibility-baseline.md)
- [GNU envsubst 性能基线](reference/REF-04-gnu-envsubst-performance-baseline.md)

## 维护约定

- 文档头部带 `status` / `owner` / `version` / `last-updated`；`status` 取 `active`（现行）或 `draft-ai`（草稿）。
- 一份文档被后来者取代时，移入 `99-archive/` 并在归档 README 里说明原因，同时更新指向它的引用。
- 文档里的命令要能直接复制执行；旗标名与退出码以 `07-serialization/` 为准。
- 改完文档跑一次链接体检：`python3 tools/docs/checklinks.py`（相对链接必须能解析）。
- 改了 README 里的示例再跑一次输出核对（需要先 `just build`）：
  `python3 tools/docs/checkreadme.py README.md README.zh-CN.md`。

## 非目标（整体）

- 本文只是索引，不承载需求或决策。`[ASSUMPTION]`
- 不替代各阶段文档的人工评审。`[INFERENCE]`
- 不保证覆盖所有历史版本；历史材料在 `99-archive/`。`[FACT]`
