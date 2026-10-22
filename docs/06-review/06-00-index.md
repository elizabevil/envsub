---
status: active
owner: 技术负责人
version: 0.12
last-updated: 2026-10-22
---

# 评审记录索引

本目录存放**按轮次的实施与评审记录**。每篇记录的是那一轮当时的状态：结论若已生效，已经写进
[`../07-serialization/`](../07-serialization/00-index.md) 与
[`../04-decisions/`](../04-decisions/04-00-index.md)；记录本身不再更新。

## 新记录写在这里

评审的结论（无论"改"还是"不改"）写入本目录，必须附实测数字与复现命令。见
[`../prompts/_common.md`](../prompts/_common.md) §7。

## 历史记录

2026-10-22 之前的 17 篇已归档，因为其中的旗标名、依赖版本、目录结构与计数都已被后续改动取代，
按原样读会得到与今天的代码相反的结论。归档清单见
[`../99-archive/06-review/`](../99-archive/06-review/06-00-index.md)。

## 非目标（整体）

- 本目录不产生新需求或新方案。`[ASSUMPTION]`
- 不替代人工评审结论。`[INFERENCE]`
- 不覆盖实现代码评审。`[FACT]`
