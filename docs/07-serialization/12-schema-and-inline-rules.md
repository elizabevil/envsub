---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-09
---

# schema 与 inline `| type` 规则（规范性，SI 系列）

> 目的：把"两处类型声明如何交互"固定成**无歧义**的规则，并为每条规则写明**为什么**。
> 相关：`docs/03-design/03-00-index.md`（核心决策）、本文 §5 的 §6（CLI 交互）、
> 05-formatters-and-types（`| type` 类型名）。

> 物化的背景、来源差异、类型阶梯与联合处理见 [§17 物化模型](13-materialization.md)。

## 1. 术语

| 术语 | 含义 |
|------|------|
| schema 类型 | 值文件对应 JSON Schema 的 `type`，是**整份来源**的契约 |
| inline 类型 | 模板表达式里的 `| type xxx`，是**单点**断言 |
| 物化 | 把字符串按声明类型转成值树类型（`.env`/`--set`/赋值字面量） |
| 相容 | 两条声明可以同时成立，值域不冲突 |

## 2. 规则

- **SI1 单一文法**：类型声明（含 `array<T>`）只有**一个**解析实现，schema 与 inline 共用，
  诊断在同一处产生。
  *为什么*：此前两处各写一份，已经分叉——inline 拒绝 `array<array<int>>`，schema 却静默降级；
  同一份用户意图得到两种结果。共用后新增类型只需改一处，也不可能再漂移。
- **SI2 并集语义**：两者同时出现时必须**相容**；inline 只能**收窄**值域，不能改变值域类别。
  *为什么*：schema 描述来源，inline 描述使用点；收窄是安全的信息增加，换类别是信息冲突。
- **SI3 顺序固定**：**先按 schema 物化 → 再相容性检查 → 最后按 inline 断言**。
  *为什么*：值文件路径与渲染路径必须得到同一种类型；断言作用在已物化的值上，保证重复渲染幂等。
- **SI4 冲突即错误**：不相容时报错，**绝不静默取一方**（exit 4，
  `inline type %s conflicts with schema type %s`）。
  *为什么*：两条声明都是用户的显式意图；静默取舍会造成难以定位的数据损坏（例如把整型当字符串）。
- **SI5 约束归属 schema**：`required`/`default`/`enum`/`minimum`/`nullable` 等约束一律以 schema 为准，
  inline **只声明类型**。
  *为什么*：约束的是"这份来源的全部值"；把约束分散到模板里会破坏来源级一致性，且默认值无法在渲染前参与校验。
- **SI6 数值族互通**：两者都属于数值族（`int`/`number`/`float`）时视为相容。
  *为什么*：JSON/YAML 只有一个数字族；把 `number` + `int` 判为冲突会逼出无意义的重复声明。
  数值边界由浮点规则（F1–F8：float 仅 float64 有限值、整数不得静默降级）约束，而非由冲突规则承担。
- **SI7 位置与次数**：`| type` 必须是 pipeline 的**第一个** step，且每条表达式**只能声明一次**。
  *为什么*：类型断言必须作用在**原始值**上；若排在 formatter 之后，断言对象已变成字符串，
  语义不可预测、也无法与 schema 物化对齐。
- **SI8 缺省行为**：两者都无 → 未声明数字按 F6 保留字面量；只有 schema → 按 schema 物化；
  只有 inline → 按 inline 断言（原始字符串按 `| type` 物化）。
  *为什么*：三种情形分别对应"没有类型信息"“来源给类型”“使用点给类型”，规则互不重叠。
- **SI9 错误分类固定**（用户可见，含退出码）：

  | 场景 | 文案 | 退出码 |
  |------|------|--------|
  | 声明冲突 | `inline type %s conflicts with schema type %s` | 4（type） |
  | 未知类型名 | `unsupported type %q` | 2（usage） |
  | 缺类型名 | `type requires a type name` | 3（template） |
  | 位置错误 | `inline type must be the first pipeline step` | 3（template） |
  | 重复声明 | `declares more than one type` | 3（template） |
  | 嵌套数组 | `nested array types are not supported in %q` | 2（usage）※ 见 §5 G13 |

- **SI11 联合物化不得否定合法值**：`type` 是数组（联合）时，按**成员集合**处理——
  可物化成员（`integer`/`number`/`boolean`/`array`/`object`）按书写顺序依次尝试，第一个成功即用；
  全部失败且联合含 `"string"` 时**保留字符串**（让校验裁决），不含 `"string"` 才报错；
  `"string"` 只作兜底、不参与正向偏好（因此它的书写位置不影响结果）；
  兼容性判定同样看成员集合（schema `["integer","string"]` 同时接受 `\| type int` 与 `\| type string`）。
  *为什么*：联合是析取，"选第一个成员"会**误拒**其他合法成员（实测：`["integer","string"]` + `"hello"`
  原先报 `expected an integer`）；物化的职责只是"选方向"，合法性必须由校验裁决，且不得依赖书写顺序。
- **SI10 每条规则必须有落盘测试**：见 §6 的对应关系；新增类型名或改变相容性判定时，
  必须同时更新本文件、§6 表与测试。

## 3. 相容性判定表（现状 = 规范）

| schema ↓ \ inline → | `int` | `float` | `string` | `bool` | `object` | `array` | `array<T>` | `null` |
|---|---|---|---|---|---|---|---|---|
| `integer` | ✅ 同名 | ✅ 数值族 | ❌ 冲突 | ❌ | ❌ | ❌ | ❌ | ❌ |
| `number` | ✅ 数值族 | ✅ 同名 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `string` | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `boolean` | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `object` | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |
| `array` | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅（元素任一为空即放行） | ❌ |
| `array<T>` | ❌ | ❌ | ❌ | ❌ | ❌ | ✅（同上） | ✅（元素必须相同） | ❌ |
| `null` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

判定实现：`internal/engine/extended/engine.go` 的 `schemaCompatible`（同名 → 相容；array 比对
`Element`，任一为空放行；数值族互通；其余不相容）。

## 4. 设计理由汇总（一句话版）

1. **契约 vs 断言**：schema 管来源、inline 管使用点——所以 schema 定约束、inline 只能收窄（SI2/SI5）。
2. **幂等与一致**：先物化再断言，值文件路径与渲染路径得到同一类型（SI3/SI8）。
3. **失败要显式**：两条显式声明冲突时宁可报错，不可静默取舍（SI4）。
4. **单一文法**：文法与诊断只有一份实现，才可能长期保持一致（SI1/SI9）。
5. **数值是一族**：数字类型间的差异交给浮点/精度规则，不交给冲突规则（SI6）。
6. **断言位置**：类型断言必须在格式化之前，否则断言对象已变成字符串（SI7）。

## 5. 例外与待统一

| 项 | 内容 |
|----|------|
| **G13（已实现）** | **拒绝数组嵌套** `array<array<…>>`：共用文法 `types.ArrayElement` 由 inline `ParseType` 与 schema `normalizeTypeName` 共同调用，两侧同一句 `nested array types are not supported`；`array<object>` 放行；需要嵌套数组时用 JSON Schema 原生 `items`。对拍用例 `TestArrayAliasGrammarIsShared`。 |
| **G13 边界** | object 内含数组（任意深度）、`array<object>`、对象套对象**均已可用且不受 G13 影响**：类型模型与 formatter 都不遍历对象内部（SI5 分工）。缺口严格限定为 `array<array<…>>`。 |
| **G11** | 联合类型 `type: ["integer","null"]` 取第一个非 `null` 成员作为物化目标（待实现，未改变 SI 规则）。 |
| G14 / G12 | 属物化与默认值遍历，不改变本文规则；见 `09-gaps.md`。 |

## 6. 案例（Examples）

| 场景 | 写法 | 结果 |
|------|------|------|
| 仅 schema | `properties.x.type = "integer"`，`.env` `x=8080` | `x` 物化为 `int 8080` |
| 仅 inline | 模板 `${x \| type int}`，x 为 `"8080"` | 按 inline 断言物化 |
| 两者相容 | schema `integer` + inline `int` | 通过；先按 schema 物化，再按 inline 断言 |
| 两者相容（数值族） | schema `number` + inline `float` | 通过（SI6） |
| 冲突 | schema `string` + inline `int` | exit 4：`inline type int conflicts with schema type string`（SI4） |
| 联合 + inline | schema `["integer","string"]` + inline `string` | 相容（SI11：看成员集合，而非折叠后的 `integer`） |
| 联合 + 字符串值 | schema `["integer","string"]`，`.env` `x=hello` | 保留 `"hello"`（SI11，不再误拒） |
| 位置错误 | `${x \| toJson \| type int}` | exit 3：`inline type must be the first pipeline step`（SI7） |
| 重复声明 | `${x \| type int \| type float}` | exit 3：`declares more than one type`（SI7） |
| 嵌套数组 | `\| type array<array<int>>` 或 schema 同名写法 | exit 2：两侧同一句 `nested array types are not supported`（SI1/G13） |

## 7. 非目标（Non-Goals）

1. **不引入 schema/inline 之外的第三处类型声明**（例如配置文件级类型映射）。
2. **不允许 inline 声明联合**：`\| type` 仍是单一类型且必须在 pipeline 首位（SI7）。
3. **不允许某一方静默覆盖另一方**：冲突一律报错，不提供"以 schema 为准/以 inline 为准"的开关（SI4）。
4. **不处理外部 `$ref`**：本地 `$ref`/`allOf` 的默认值见 `09-gaps.md` G12。

## 8. 规则与测试对应（复现）

| 规则 | 测试位置 | 复现命令 |
|------|----------|----------|
| SI1（单一文法，待 G13 落地） | 待补：两侧共用解析器后加对拍用例 | — |
| SI2/SI3/SI4（并集、顺序、冲突） | `internal/engine/extended`（`schema_and_inline_conflict`、`inline_type` 系列 fixture） | `just test-fixtures`、`go test ./internal/engine/extended/` |
| SI5（约束归属 schema） | `internal/schema`（`TestApplyDefaults*`、`TestMaterialize*`） | `go test ./internal/schema/` |
| SI6（数值族互通） | `internal/engine/extended`（`float_from_string`、inline type 用例） | `go test -run Test.*Type ./internal/engine/extended/` |
| SI7（位置与次数） | `test/e2e/testdata/{type_after_formatter,duplicate_type}` | `go test ./test/e2e/` |
| SI8（缺省行为） | `test/encoding`（规则矩阵与类型表）、`internal/types` | `go test ./test/encoding/ ./internal/types/` |
| SI9（错误分类） | 上表各场景的 e2e fixture + `docs/07-serialization/08-errors.md` | `go test ./test/e2e/` |
