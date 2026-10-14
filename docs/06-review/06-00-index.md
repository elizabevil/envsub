---
status: draft-ai
owner: [ 待指定 ]
version: 0.11
last-updated: 2026-10-10
---

# 评审索引

- [06-01 需求定义与方案评审 Round 1](06-01-review-needs-and-solution.md)
- [06-02 实现进度核查 Round 2](06-02-implementation-progress-verification.md)
- [06-03 P0 实现报告 Round 2](06-03-p0-implementation-round2.md)
- [06-04 模板输入抽象与并行渲染修复 Round 3](06-04-optimization-round3.md)
- [06-05 代码评审整改 Round 4](06-05-code-review-round4.md)
- [06-06 JSON 数字精度核查与 WithUnmarshalers 对照 Round 5](06-06-json-number-precision.md)
- [06-07 二进制体积归因与优化选项 Round 6](06-07-binary-size-attribution.md)
- [06-08 expression 解析优化（A/B 数据）](06-08-expression-parse-perf.md)
- [06-09 解析/处理路径优化 review（待办 D）](06-09-parsing-paths-review.md)
- [06-10 优化总结（体积 / 性能 / 代码质量）](06-10-optimization-summary.md)
- [06-11 结构变更与 `--explain` JSON 化](06-11-explain-json-and-options-split.md)
- [06-12 `--values-file` 的 `.env` 边界与 CLI 帮助参数名](06-12-values-dotenv-boundary.md)
- [06-13 配置覆盖优先级：`default` 降为兜底（显式 > 隐式）](06-13-config-precedence-default-fallback.md)
- [06-14 会话交接：改动状态与剩余任务](06-14-session-handoff.md)
- [06-15 命名空间与分层重构（方案 C 定案）](06-15-namespace-and-layering.md)
- [06-16 `go fix` 现代化建议全量记录（Go 1.27.2）](06-16-go-fix-modernization.md)
- [06-17 架构评审：核心渲染路径 vs 非核心功能（D3–D7 已执行）](06-17-architecture-review.md)

## 非目标（整体）

- 本目录不产生新需求或新方案。`[ASSUMPTION]`
- 不替代人工评审结论。`[INFERENCE]`
- 不覆盖实现代码评审。`[TODO]`
