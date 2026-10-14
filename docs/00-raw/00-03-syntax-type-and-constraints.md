---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 原始材料 · 03 语法、类型与约束决策

> 本文是 `00-00-index.md` 的拆分部分。

## 13. extended 语法确认项（2026-09-29）

- Helm-like pipeline formatter 支持：`${hosts | join ","}`、`${hosts | toJson}`、`${app | toYaml}`。
- 条件、循环、includes 后置 P1，不作为一期 P0。
- object 遍历默认按键排序，保证确定性输出。
- include 最大嵌套深度暂定 3 层，例如 A -> B -> C；同时需要循环 include 检测。
- 类型系统确认与 Go 基础类型对齐：`string`、`bool`、`int`、`float`、`array<T>`、`object`、`null`。
- boolean 只接受 `true` / `false`。
- 默认值/存在性操作语义为受限 POSIX shell 子集，而非 GNU envsubst；GNU envsubst 本身不支持这些语法。
- 路径未加引号标识符不允许 `-`，带特殊字符的 key 使用 quoted-string，例如 `${app["my-key"]}`。

以上已同步至 `docs/03-design/03-03-values-merge-and-schema.md`。

## 14. 设计细化确认（2026-09-29）

- `--unset` 的目标：控制“变量/路径不存在”时的行为，避免静默空值；候选为 `error`、`empty`、`keep`，默认建议 `error`，待确认。
- `.env` 允许 JSON 数组，但规则必须明确：只有 schema 声明为 array/object 时才做 JSON 解析，要求单行合法 JSON；未声明 schema
  时不自动推断数组。
- Helm-like pipeline 支持 `indent`；`indent N` 在每一行行首插入 N 个空格。
- 控制指令确认必须单独成行；插值仍可内联。
- include 文件在执行时通过命令行指定；include 细节（参数、白名单、循环检测、错误格式）在二期讨论。
- object 遍历按键排序；include 最大嵌套深度 3 层。

## 15. 零值、schema 快速声明与 include 待办（2026-09-29）

- 需求提出者提出零值概念：如 int 零值为 0。建议增加 `--unset=zero` 候选，按 schema 类型填充 Go 零值；required 字段是否允许自动填零待确认。
- `--unset=empty` 定义为替换为空字符串、不输出任何字符；`--unset=zero` 是独立的零值填充策略。
- 需求提出者询问 schema 如何声明、如何快速表示类型，例如是否提供 `| type` 关键字。设计草案增加内联 `| type` 候选：
  `${PORT | type int}`、`${HOSTS | type array<string> | toJson}`。
- include 功能暂不考虑，列为待办；未来再讨论执行时指定文件、白名单、循环检测与错误格式。
- object 遍历按键排序、条件/循环后置 P1、控制指令单独成行仍保持不变。

## 16. required 零值与内联 | type 决策（2026-09-29）

- required 字段不自动填充 zero 零值；缺失 required 直接报错。边界数据安全大于零值约定。
- 确认使用内联 `| type`，例如 `${PORT | type int}`、`${HOSTS | type array<string> | toJson}`。
- 建议优先级：schema 是全局契约，内联 `| type` 是局部断言/补类型；schema 优先，内联不得放宽 schema、default、required 和约束。
- 建议 `| type` 必须紧跟 path，作为 pipeline 第一个操作符；`${path | type T | toJson}` 合法，`${path | toJson | type T}`
  非法。
- `--unset=zero` 是否最终加入仍需确认；若加入，只适用于非 required 字段。

## 17. int/float 与冲突规则确认（2026-09-29）

- `int` 与 `float` 视为不同类型；`0` 与 `0.0` 也视为两种类型。
- 为了方便使用，`int` 与 `float` 均支持零值填充：`int` -> `0`，`float` -> `0.0`。
- 零值场景出现 `int`/`float` 冲突或歧义时，以 `0` 为准；普通类型不兼容仍按 schema 优先、直接报错处理。
- 冲突规则已同步至 `docs/03-design/03-03-values-merge-and-schema.md` 的 3.6.2 节。

## 18. schema 并集与约束能力确认（2026-09-29）

- schema 与内联 `| type` 做配置并集：值必须同时满足二者。
- schema 的 required/default/约束优先级高于内联 `| type`。
- 交集冲突时直接报错。
- schema 支持枚举、范围限制等校验；P0 建议 enum/minimum/maximum/minLength/maxLength/pattern/minItems/maxItems，P1 增强
  exclusiveMinimum/exclusiveMaximum/multipleOf/uniqueItems/minProperties/maxProperties/format。
- 已同步至 `docs/03-design/03-03-values-merge-and-schema.md` 的 3.6.2 与 3.6.3 节。

## 非目标（本部分）

- 不替代整体证据审阅与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的输入，完整索引见 `00-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
