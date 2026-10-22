---
status: draft-ai
owner: [ 待指定 ]
version: 0.2
last-updated: 2026-10-08
---

# Extended 设计索引

> 本文原为单个大文件，现已按编号拆分。所有子文档共享同一组约束与决策；完整索引如下。

## 子文档

- [03-02 值模型与模板语法](03-02-value-model-and-template-syntax.md)
- [03-03 值来源、多文件与 Schema](03-03-values-merge-and-schema.md)
- [03-04 Unset、CLI 与控制结构](03-04-unset-cli-and-control-structures.md)
- [03-07 代码与仓库目录结构](03-07-code-and-repo-structure.md)
- [03-08 CLI 设计](03-08-cli-design.md)
- [03-10 Go 代码开发规范](03-10-go-code-standards.md)
- [序列化规则（多篇，见 docs/07-serialization/00-index.md）](../07-serialization/00-index.md)

已归档（被后续设计与实际代码取代，仅供追溯）：

- [03-01 背景、目标与非目标](../99-archive/03-design/03-01-overview-and-goals.md)
- [03-05 API、数据、替代方案与权衡](../99-archive/03-design/03-05-api-data-tradeoffs.md)
- [03-06 开放问题与评审清单](../99-archive/03-design/03-06-open-questions-and-review.md)
- [03-09 扩展与兼容架构](../99-archive/03-design/03-09-extension-and-compat-architecture.md)
- [03-10 外部插件协议](../99-archive/03-design/03-10-external-plugin-protocol.md)

## 已确认的核心决策

- 技术选型：v0/v1 继续使用 Go，`go.mod` 使用 Go 1.27.3。`[FACT]`
- 代码规范：严格遵循 Google Go Style Guide。`[FACT]`
- 依赖选型：JSON `encoding/json/v2`（`-tags=sonic` 可切 sonic）、YAML `go.yaml.in/yaml/v3`、JSON Schema `santhosh v6`。`[FACT]`（2026-10-08 更新）
- 发布：GitHub Actions + `justfile` 快捷编译。`[FACT]`
- 测试：测试先行，先建立 P0 测试矩阵与 golden fixtures。`[FACT]`
- 非功能：常规二进制优化，性能参考 GNU envsubst。`[FACT]`
- P0 核心功能、P1 include、P2 GNU/a8m/drone 兼容与转换。`[FACT]`
- Schema 直接复用 JSON Schema Draft 2020-12 + `santhosh-tekuri/jsonschema/v6` + 薄适配层。`[FACT]`
- 未知非 `x-*` 关键字默认不报错；可选 `--unknown-keywords=warn|error`。`[FACT]`
- 内联 `| type` 与 schema 取并集；schema 的 required/default/约束优先；交集冲突报错；如果提供 `| type`，必须为 pipeline
  第一个操作符，可缺省。`[FACT]`
- Helm-like pipeline：`join` / `toJson` / `toYaml` / `quote` / `indent`。`[FACT]`
- 条件/循环/include 为 P1；include 执行时指定文件，细节 P1 细化；object 遍历按键排序。`[FACT]`
- 序列化规则（已确认）：整数按 int64 → uint64 → 精确文本逐级表示（类型名只提供 `int`，
  含负数、无内建上限；不提供 `uint` 与宽度类型名）、
  任何路径都不得静默降级为 float64；浮点为 IEEE-754 binary64 有限值、最短往返编码，
  `NaN`/`±Inf` 一律拒绝；`nil`（含 typed nil map/slice）与空集合一律输出 `{}`/`[]`（遵循库默认，
  不再归一化为 `null`）；`null`/缺失/空字符串语义各自独立；映射键在所有编码器排序输出；
  每种类型的取值范围见 [§3](../07-serialization/01-types.md)；方案选型记录见其附录 A。`[FACT]`

## 非目标（整体）

- 不做 Helmfile/Helm/Kustomize 发布编排替代品。`[FACT]`
- 不做图灵完备通用编程语言或完整 Helm/Jinja 模板引擎。`[FACT]`
- 默认不执行 shell、不访问网络、不自动读取远程密钥。`[FACT]`
- 一期不实现 GNU/a8m/drone 完整兼容与自动转换。`[FACT]`
