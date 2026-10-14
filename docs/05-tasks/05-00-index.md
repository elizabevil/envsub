---
status: draft-ai
owner: [ 待指定 ]
version: 0.5
last-updated: 2026-10-08
---

# 任务索引

- [05-01 P0 任务拆分](05-01-p0-task-breakdown.md)：含 2026-10-08 状态总览、性能复测与编解码层/体积优化
- [05-02 P0 测试矩阵](05-02-p0-test-matrix.md)：含 2026-10-08 收尾覆盖现状（规模、覆盖率、缺口）
- [05-03 待办工作交接](05-03-pending-work.md)：执行侧待办、待决项与 P1/P2 候选（冷启动接手入口）
- [05-04 P1/P2 任务拆分](05-04-p1-p2-task-breakdown.md)：P1/P2 的**可排期清单**（需求编号、目标、验收、依赖、风险）
- [05-05 示例与测试架构](05-05-examples-and-test-architecture.md)：`examples/` 重构方案（共享数据集 + 自洽案例 + 按问题组织的配方）与 e2e 命名对齐

> 需求本身与可实现性判定见 [`../02-product/02-05-requirements-registry.md`](../02-product/02-05-requirements-registry.md)；
> 本目录只负责"怎么落地"。

## 非目标（整体）

- 本目录不重新定义需求。`[ASSUMPTION]`
- 不替代开发评估。`[INFERENCE]`
- 不承诺具体交付日期。`[FACT]`
