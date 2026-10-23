---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# ADR-002：一期仅提供 extended 原生模式，兼容与转换后置二期

## 1. 标题

ADR-002：一期仅提供 extended 原生模式，兼容与转换后置二期

## 2. 状态

Accepted（已确认），来源：需求提出者确认采用方案 A，并明确一期/二期范围，2026-09-29。`[FACT]`

## 3. 背景

- GNU、a8m、drone 的 envsubst 兼容基线存在明显语义冲突，无法用一个默认行为同时完全兼容。`[FACT]`（来源：
  `docs/reference/REF-02-envsubst-compatibility-baseline.md`）
- 产品差异化核心是类型校验、数组/map、多文件分层、安全批量渲染；兼容层主要是迁移与获客桥。`[INFERENCE]`（来源：ADR-001、产品定位文档）
- 需求提出者确认采用“窄兼容 + 转换 + 原生扩展”的方案 A，但要求分期：一期先做 extended 本应用，二期再考虑兼容与转换。`[FACT]`
  （来源：需求提出者确认，2026-09-29）

## 4. 决策

- **一期只实现 extended 原生模式**，即本应用自有语法与行为。`[FACT]`（来源：需求提出者确认）
- extended 模式可支持 `$VAR`、`${VAR}` 等基础写法，但 **不承诺**与 GNU/a8m/drone 逐字节兼容。`[INFERENCE]`（依据：一期只支持
  extended 的确认）
- 一期不实现 `--compat=gnu|a8m|drone` 等完整兼容模式。`[FACT]`（来源：需求提出者确认）
- 一期不实现 `migrate` / `convert` 命令；但可以先提供手工迁移指南与语法对照。`[INFERENCE]`（依据：一期范围）
- **功能优先级补充**：P0 实现核心功能；P1 实现 include/partials；P2 实现 GNU/a8m/drone 兼容模式与自动转换命令。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- **二期再评估并实现**兼容模式与自动转换命令，优先级由一期用户反馈决定。`[FACT]`（来源：需求提出者确认）
- 二期触发条件建议：出现明确的 GNU/a8m/drone 迁移需求；现有用户强烈要求 drop-in；不兼容导致阻碍采用。`[ASSUMPTION]`
  验证：一期后收集用户反馈；owner：产品负责人。

## 5. 被拒绝的方案

- **一期直接实现 GNU/a8m/drone 多模式兼容：拒绝。** 理由是兼容差异大、测试矩阵大，会拖慢 typed 核心能力上线。`[INFERENCE]`
- **永远不做兼容与转换：拒绝。** 理由是平台/DevOps 与 CI 用户的迁移门槛会显著提高，违背方案 A 的分期迁移策略。`[INFERENCE]`
- **用转换命令完全替代兼容：拒绝。** 理由是运行时模板、shell 引号、环境变量语义难以自动转换，仍需兼容桥或人工复核。
  `[INFERENCE]`

## 6. 后果

- 正面：一期范围更聚焦，核心 typed 渲染能力可更快形成差异化；少维护多套语义。`[INFERENCE]`
- 负面：现有 envsubst/a8m 用户不能直接替换；一期采用需要手工迁移或并行运行。`[INFERENCE]`
- 风险：若目标用户把 drop-in 作为首要决策因素，一期可能 adoption 不足。
  `[TODO]` 验证方式：一期发布后收集试用反馈。**阻塞于 TASK-026（试用发布）**——仓库当前无 tag、
  无反馈记录（`05-01`）。owner：产品负责人。
- 缓解：一期提供清晰的迁移指南、语法对照、示例和错误提示（见 [`../migration.md`](../migration.md)）；二期补自动转换与兼容模式。`[INFERENCE]`

## 7. 相关文档

- `docs/reference/REF-02-envsubst-compatibility-baseline.md`
- `docs/99-archive/02-product/02-02-product-positioning.md`
- `docs/99-archive/01-context/01-00-index.md`
- `docs/04-decisions/04-01-adr-001-language-and-render-model.md`
