---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 上下文摘要 · 01 背景与角色

> 本文是 `01-00-index.md` 的拆分部分。

## 1. 背景与问题

- 目标是基于 `github.com/a8m/envsubst` 构建升级版复合工具，支持 amd64/arm64、多平台单文件交付，并尽量静态编译、少依赖特殊库。
  `[FACT]`（来源：需求提出者目标描述）
- 线上证据显示：现有 envsubst 在安装分发、转义、默认值、参数展开、in-place、多文件、错误处理上都有大量痛点。`[FACT]`（来源：
  `docs/99-archive/00-raw/00-00-index.md` 中 GitHub/StackOverflow 证据）
- 需求提出者补充六项诉求：默认值、数组表示、转义格式、类型校验、文件配置、多文件配置。`[FACT]`（来源：需求提出者本轮补充）
- 需求提出者要求尽量兼容 envsubst，但允许在 envsubst 方案不合适时直接换方案；并希望参考 helmfile 类渲染工具。`[FACT]`
  （来源：需求提出者本轮补充）
- ADR-002 已确认：一期只实现 extended 原生模式；GNU/a8m/drone 兼容与转换后置 P2。`[FACT]`（来源：
  `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`）
- ADR-001 已确认 v0/v1 继续使用 Go，并采用层级值模型 + 渲染引擎。`[FACT]`（来源：
  `docs/04-decisions/04-01-adr-001-language-and-render-model.md`）
- PRD 与用户故事已生成；量化成功指标目标值仍待补充。`[TODO]` 口径与 owner 见 `02-03-prd.md` §2 表后说明；目标值待产品/技术确认。owner：产品/技术负责人。

## 2. 涉及角色

- 需求提出者/工具作者：负责提出升级目标与痛点。`[FACT]`（来源：需求提出者目标描述与补充）
- 线上证据涉及 macOS 用户、GitLab CI、Alpine 镜像维护者、Docker/NGINX/K8s 配置工程师。`[FACT]`（来源：线上调研文档）
- 其共同诉求可归纳为：免安装可移植、替换范围可控、支持默认值与配置来源、错误可观察。`[INFERENCE]`（依据：上述线上证据）

## 非目标（本部分）

- 不替代整体上下文评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `01-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
