> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §14。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 14. 落地清单与状态

**本章解决**：给出实现顺序、影响文件、验证方式与状态。

_列：序号 | 内容 | 影响文件 | 验证方式 | 状态。_

| 序号 | 内容 | 影响文件 | 验证方式 | 状态 |
|------|------|----------|----------|------|
| 1 | 移除 `FormatNilMapAsNull`/`FormatNilSliceAsNull` 与 yamlx nil 归一化：typed nil → `{}`/`[]` | jsonx/jsonv2.go、yamlx/marshal.go | `test/encoding`（V1） | ✅ |
| 2 | 新增 `codec.ExactInt`（`MarshalJSON` 裸数字），YAML 侧隐式 `!!int` | codec/exact.go、yamlx/marshal.go | `test/encoding`、`internal/codec` | ✅ |
| 3 | 三个入口同一把尺子：YAML 大整数节点保留精确文本；`.env`/`--set`/`${A:=}` 与 `NormalizeNumbers` 按 int64 → uint64 → ExactInt | yamlx/yamlv3.go、types/convert.go、values/number.go、schema/normalize.go | `test/encoding`、`internal/schema` | ✅ |
| 4 | 送 jsonschema 前把 `ExactInt` 转 `json.Number` | schema/jsonschema.go | `TestValidateExactInteger`/`TestMaterializeExactInteger` | ✅ |
| 5 | 更新受影响的断言与 golden（typed nil 不再钉成 `null`） | jsonx/yamlx 测试、test/jsonx、test/yamlx | 全部 | ✅ |
| 6 | `sonic` 归一化：typed nil → `{}`/`[]`；非法 UTF-8 编解码双向后置校验 | jsonx/sonic.go | `test/encoding`、`test/jsonx -tags=sonic` | ✅ |
| 7 | `NaN`/`±Inf` 在解码、物化、编码三路径无条件报错；`\| type float` 超范围报错（F7/F8） | yamlx/yamlv3.go:149-150、types/convert.go:130,202-211、schema/normalize.go:218、codec/exact.go:63、jsonx/*、yamlx/marshal.go:24 | `test/encoding`（`both encoders reject NaN/+Inf`、`YAML .inf is rejected`） | ✅ |
| 8 | 按 §3 类型表逐行补测试（T3） | — | `test/encoding/types_test.go`（10 类值 × 解码/通道/物化/判空/schema/边界） | ✅ |
| 9 | **未声明小数字面量保存与渲染**（v0.8 完成）：新增 `codec.LiteralNumber`（internal/codec/literal.go）、`values.normalizeJSONNumber`、`types`（isNumeric/Stringify/Describe/normalizeScalar/checkCompatible）、`yamlx`（scalarValue/marshal 标量 Node）、`codec.ToJSON` | values/number.go、types/*、codec/*、yamlx/*、jsonx | `test/encoding`（`TestLiteralNumberRules`、`TestLiteralNumberRoundTrip`、`TestTypeMatrix*`） | ✅ |
| 10 | 删除 `--set-string`，统一到 `--set`（v0.6 完成：flag、`Options.SetStrings`、`values.SetTyped` 全部移除；e2e 夹具与示例改用 `--set`） | cli/*、render/*、values/set.go、schema/materialize.go | e2e、`internal/schema` | ✅ |
| 11 | 删除 `\| type null` 的 `""` 隐式转换（G18，v0.7 完成）：`""`/`"null"` 报 `value ... cannot be converted to null`，exit 4 | types/convert.go:175 | `internal/types`、`test/encoding` | ✅ |

