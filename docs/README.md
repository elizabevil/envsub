---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 文档导航与编号规则

## 编号规则

- `00-raw`：原始材料，文件命名 `00-xx-<name>.md`。
- `01-context`：结构化上下文，文件命名 `01-xx-<name>.md`。
- `02-product`：产品定位与功能地图，文件命名 `02-xx-<name>.md`。
- `03-design`：设计文档，文件命名 `03-xx-<name>.md`。
- `04-decisions`：ADR，文件命名 `04-xx-adr-XXX-<name>.md`，阶段索引 `04-00-index.md`。
- `05-tasks`：任务拆分，文件命名 `05-xx-<name>.md`，阶段索引 `05-00-index.md`。
- `06-review`：评审报告，文件命名 `06-xx-<name>.md`，阶段索引 `06-00-index.md`。
- `reference`：参考调研，文件命名 `REF-xx-<name>.md`，阶段索引 `REF-00-index.md`。
- 每个阶段使用 `xx-00-index.md` 作为阶段索引。

## 阶段索引

- [00-raw 索引](00-raw/00-00-index.md)
- [01-context 索引](01-context/01-00-index.md)
- [02-product 索引](02-product/02-00-index.md)
- [03-design 索引](03-design/03-00-index.md)
- [04-decisions 索引](04-decisions/04-00-index.md)
- [05-tasks 索引](05-tasks/05-00-index.md)
- [06-review 索引](06-review/06-00-index.md)
- [reference 索引](reference/REF-00-index.md)
- ADR：
    - [ADR-001](04-decisions/04-01-adr-001-language-and-render-model.md)
    - [ADR-002](04-decisions/04-02-adr-002-compatibility-and-migration.md)
    - [ADR-003](04-decisions/04-03-adr-003-json-schema-reuse.md)
    - [ADR-004](04-decisions/04-04-adr-004-dependency-selection.md)
- 参考：
    - [竞品与市场空白](reference/REF-01-competitive-landscape.md)
    - [envsubst 兼容基线](reference/REF-02-envsubst-compatibility-baseline.md)
    - [JSON Schema 复用调研](reference/REF-03-json-schema-reuse-research.md)

## 非目标（整体）

- 本文只是索引，不承载需求或决策。`[ASSUMPTION]`
- 不替代各阶段文档的人工评审。`[INFERENCE]`
- 不保证覆盖所有历史版本。`[FACT]`
