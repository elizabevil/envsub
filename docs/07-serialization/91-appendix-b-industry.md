> 本文件是[序列化规则](00-index.md)的一部分，覆盖 附录 B。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 附录 B：业界惯例核查

**本章解决**：用业界惯例支撑整数类型命名与范围的取舍。

_列：标准/生态 | 整数表达方式 | 说明/链接。_

| 标准/生态 | 整数表达方式 | 说明 / 链接 |
|-----------|--------------|-------------|
| JSON Schema | `type: integer` 为数学整数，精度不限；实现如何处理 int64 长期有讨论 | [json-schema-spec#574](https://github.com/json-schema-org/json-schema-spec/issues/574) |
| OpenAPI | `type: integer` + `format: int32\|int64`，用 `format` 表达宽度 | — |
| JDDF（IETF draft） | 显式位宽+符号：`int8…int64`、`uint8…uint64`、`float32/float64` | [draft-ucarion-jddf-04](https://datatracker.ietf.org/doc/html/draft-ucarion-jddf-04.txt#5) |
| TOML | 只有 64 位有符号整数，规范不提供无符号 | [toml#688](https://github.com/toml-lang/toml/issues/688) |
| Protobuf | `int32/int64`、`uint32/uint64`、`sint32/sint64`、`fixed32/fixed64` | — |
| YAML 1.2 core | `!!int` 概念上不限精度，实现多用 64 位/`uint64` | — |

结论：业界表达整数范围有两条路——`format`/宽度后缀（OpenAPI、JDDF、Protobuf），或单一整数
类型 + 约束校验（TOML）。本项目采用后者：只提供 `int`（精确整数，含负数，无内建上限）；
需要非负约束时用 schema `minimum: 0`，不提供 `uint`，也不提供宽度类型名（T4/T8）。`[推断]`

