> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §9。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 9. schema 语义规则

**本章解决**：支持的 schema 关键字、`default` 物化、`Materialize` 字符串处理、`Validate` 输入
构造、未知关键字策略与错误，并给出每规则案例。

### 9.1 支持的关键字

_列：ID | 类别 | 关键字 | 说明。_

| 编号 | 类别 | 关键字 | 说明 |
|------|------|--------|------|
| SC1 | core/结构 | `$schema` `$id` `$ref` `$defs` `$anchor` `$dynamicRef` `$dynamicAnchor` `$vocabulary` `$comment` `definitions` `title` `description` `examples` `default` `deprecated` `readOnly` `writeOnly` | `x-*` 前缀永远允许 |
| SC2 | 校验 | `type` `enum` `const` `required` `properties` `patternProperties` `additionalProperties` `propertyNames` `items` `prefixItems` `contains` `minContains` `maxContains` `unevaluatedItems` `unevaluatedProperties` `dependentRequired` `dependentSchemas` `minItems` `maxItems` `uniqueItems` `minLength` `maxLength` `pattern` `minimum` `maximum` `exclusiveMinimum` `exclusiveMaximum` `multipleOf` `minProperties` `maxProperties` `allOf` `anyOf` `oneOf` `not` `if` `then` `else` | 由 `jsonschema/v6` 执行 |
| SC3 | 注解/format | `format` `contentEncoding` `contentMediaType` `contentSchema` | 同上 |
| SC4 | 类型别名 | `int→integer` `float→number` `bool→boolean`；`array<T>` → `type:array` + `items`（仅当 `items` 缺失） | normalize.go:18-29,169-186 |
| SC5 | 名字容器 | `properties` `patternProperties` `$defs` `definitions` `dependentSchemas` `dependentRequired` 的子键不判未知 | normalize.go:36-43 |

### 9.2 `default` 物化规则

_列：ID | 规则 | 证据。_

| 编号 | 规则 | 证据 |
|------|------|------|
| SC6 | 只在 `properties` 中缺失的键写入 `default`；已存在不覆盖 | defaults.go:34-39；schema_test.go:120-136 |
| SC7 | `default` 原样克隆、**不做类型转换**；字符串由后续 `Materialize` 转换 | defaults.go:36；materialize.go:58-77 |
| SC8 | 嵌套对象递归；数组只对已存在元素补默认 | defaults.go:44-49,54-72 |
| SC9 | 走展开后的 schema：本地 `$ref` 按 JSON Pointer 展开、`allOf` 分支合并，`$ref`/`allOf` 处的 default 生效；`patternProperties`/`additionalProperties` 仍不生效 | refs.go:17-21；load.go:40 |

### 9.3 Materialize 对 `.env` / `--set` 字符串的处理

_列：声明类型 | 处理 | 证据。_

| 声明类型 | 处理 | 证据 |
|----------|------|------|
| `array` / `object` | JSON 解析 + `NormalizeNumbers` | materialize.go:60-66；normalize.go:232-257 |
| `integer` / `number` / `boolean` | `convertScalar` | materialize.go:67-76；normalize.go:198-229 |
| 其它 | 保持字符串 | materialize.go:90；sources/dotenv_string_not_parsed |
| 顶层 `.env` key | root 缺失时复制进 root 再物化 | materialize.go:35-46 |
| 嵌套路径的环境读取 | **不提供**：无"路径 → 环境变量名"推导；环境只经 `${@process.NAME}` 读取（见 `13-materialization.md` §13/§14） | — |

### 9.4 Validate 输入构造

_列：ID | 规则 | 证据。_

| 编号 | 规则 | 证据 |
|------|------|------|
| SC10 | 校验对象为 `codec.ToJSON(tree.Root)`；只覆盖 root 命名空间 | jsonschema.go:25-35 |
| SC11 | `ExactInt` 在校验前转 `json.Number`，仅含该类型时才复制树 | exact.go:83-88；schema_test.go:271-298 |
| SC12 | 错误扁平化为至多 8 条 `at /ptr: reason`，包 `KindSchema`=4 | jsonschema.go:22,53-74 |

### 9.5 未知关键字策略

_列：取值 | 行为 | 退出码 | 证据。_

| 取值 | 行为 | 退出码 | 证据 |
|------|------|--------|------|
| `ignore`（默认） | 静默接受 | 0 | schema.go:37-39；cli/render.go:36 |
| `warn` | 逐条日志，仍通过 | 0 | load.go:49-54 |
| `error` | `unknown schema keywords: ...` | 4 | load.go:55-57 |
| 非法取值 | `invalid --unknown-keywords value` | 2 | schema.go:45-46 |

**案例（schema 规则）**

| 规则 | 命令 / schema | 输出 / exit | 标注 |
|------|---------------|-------------|------|
| SC1 | schema 含 `x-sensitive: false` + `${a}` | 正常输出，不报未知关键字 | 现状 |
| SC2 | schema `mode: {enum: [dev, prod]}` + `--set mode=other` | `schema validation failed: at /mode: ...`，exit 4 | 现状 |
| SC3 | schema `format: date` + 合法值 | 通过（format 由库处理） | 现状 |
| SC4 | schema `port: {type: int}` + `port="8080"` | 物化为 `8080` | 现状 |
| SC4 | schema `hosts: {type: array<string>}` + `${hosts \| join}` | 逐元素断言为 string | 现状 |
| SC5 | schema `properties.customKeyword`（属性名） | 不判为未知关键字 | 现状 |
| SC6 | schema `port: {type: integer, default: 8080}` + 无 `port` | 输出 `8080` | 现状 |
| SC7 | schema `hosts: {type: array, default: []}` + `${hosts \| toJson}` | `[]` | 现状 |
| SC8 | schema 嵌套 `app.port` default + `app: {}` | 补出 `app.port` | 现状 |
| SC9 | schema `$ref` 指向的定义含 default | default 生效（`$ref` 在装载时展开） | 现状 |
| SC10 | 校验只覆盖 root | `.env` 未复制进 root 的嵌套 key 不参与校验（G15） | 现状 |
| SC11 | `--set-json 'id=2^64（18446744073709551616）'` + schema `integer` | 校验通过，值不被改动 | 现状 |
| SC12 | 多个校验错误 | 至多 8 条 `at /ptr: reason` | 现状 |
| §9.5 | `--unknown-keywords=warn` + 未知关键字 | 日志 warn，exit 0 | 现状 |
| §9.5 | `--unknown-keywords=error` + 未知关键字 | `unknown schema keywords: ...`，exit 4 | 现状 |

