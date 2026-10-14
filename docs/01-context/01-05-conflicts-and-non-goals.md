---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 上下文摘要 · 05 冲突与非目标

> 本文是 `01-00-index.md` 的拆分部分。

## 7. 冲突与矛盾

- 用户补充项之间未发现直接矛盾。`[FACT]`（来源：需求提出者补充）
- “升级版/复合能力”与“保持原 envsubst 兼容”可能存在冲突。`[INFERENCE]`（依据：a8m 已有行为差异与回归 issue）
- “数组/类型校验”与 envsubst 传统纯字符串替换模型存在语义扩展张力。`[INFERENCE]`（依据：envsubst 原本只处理字符串环境变量）

## 非目标（Non-Goals）

- 暂不以 GUI/Web 界面作为首版范围。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 暂不承诺覆盖 POSIX shell 的全部参数展开与命令替换语义。`[ASSUMPTION]` 验证：确认兼容边界；owner：待指定。
- 暂不承诺与所有 envsubst 实现完全向后兼容。`[ASSUMPTION]` 验证：确认兼容矩阵；owner：待指定。
- 不做 Helmfile/Helm/Kustomize 的发布编排替代品。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 不做图灵完备的通用编程语言；不实现任意 shell 执行、递归函数、网络访问等通用计算能力。`[ASSUMPTION]`
  验证：确认“受限模板能力”边界；owner：待指定。
- 默认不执行 shell 命令、不访问网络、不自动读取远程密钥。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 非目标（本部分）

- 不替代整体上下文评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `01-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
