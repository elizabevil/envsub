---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# Extended 设计 · 01 背景、目标与非目标

> 本文是 `03-00-index.md` 的拆分部分。

## 1. Context & Scope

- 当前已确认：一期只实现 `extended` 原生模式；不做 GNU/a8m/drone 完整兼容与自动转换。`[FACT]`（来源：ADR-002）
- 已确认类型校验为 P0；多文件覆盖规则为“后加载覆盖先加载、数组默认替换、CLI `--set` 最高”。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 目标用户优先级：平台/DevOps > CI 用户 > 容器维护者。`[FACT]`（来源：需求提出者确认）
- 本文范围：extended 模式的模板语法、值模型、类型系统、schema 与基础渲染规则。`[INFERENCE]`
- 本文不展开 Phase 2 的兼容模式与转换命令设计。`[FACT]`（来源：ADR-002）

## 2. Goals / Non-Goals

### Goals

- 提供类似 envsubst 的简单变量替换，同时支持默认值、转义、作用域控制。`[INFERENCE]`
- 支持数组/map/类型校验，并可用于 JSON/YAML 等结构化输出。`[INFERENCE]`
- 多文件 values 分层、覆盖优先级明确。`[FACT]`（来源：需求提出者确认）
- 渲染前完成类型解析与 schema 校验，错误信息可定位。`[INFERENCE]`
- 保持受限渲染模型：不做任意 shell/网络执行。`[FACT]`（来源：需求提出者确认）

### Non-Goals

- 不做图灵完备的通用编程语言或完整 Helm/Jinja 模板引擎。`[FACT]`（来源：需求提出者确认）
- 一期不实现 GNU/a8m/drone 完整兼容模式。`[FACT]`（来源：ADR-002）
- 默认不执行 shell、不访问网络、不自动读取远程 secret。`[FACT]`（来源：需求提出者确认）
- 不替代 Helmfile/Helm/Kustomize 的发布编排。`[FACT]`（来源：需求提出者确认）

## 非目标（本部分）

- 不替代整体设计评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `03-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
