> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §7–§8。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 7. formatter 与管道规则

**本章解决**：管道 formatter 的接受类型、参数、输出与错误，并给出每 formatter 案例。

管道按书写顺序执行（pipeline.go:44-54）；formatter 白名单固定，禁止用户自定义函数/代码执行/网络。
`type` 是解析器识别的管道步骤但不是 `pipeline.Names()` 成员（pipeline.go:74-78）。

_列：ID | formatter | 接受类型 | 参数 | 输出 | 错误/退出码 | 案例（现状）。_

| 编号 | formatter | 接受类型 | 参数 | 输出 | 错误 / 退出码 | 案例（现状） |
|------|-----------|----------|------|------|---------------|--------------|
| P1 | `join` | 仅 `[]any`；拒绝嵌套数组/对象 | 可选，**必须加引号**；默认 `","` | 连接字符串 | KindType=4 | `--set-json 'x=[1,2]'` + `${x \| join "-"}` → `1-2` |
| P2 | `toJson` | 任意 | 不允许 | 紧凑 JSON，键排序，无尾换行 | KindType=4 | `--set-json 'x={"b":1,"a":2}'` + `${x \| toJson}` → `{"a":2,"b":1}` |
| P3 | `toYaml` | 任意 | 不允许 | yaml.v3 块格式，2 空格缩进，去尾换行 | KindType=4 | `--set-json 'x={"a":1}'` + `${x \| toYaml}` → `a: 1` |
| P4 | `quote` | 标量（`Stringify`） | 不允许 | `strconv.Quote`（Go 转义，非 JSON） | 结构化→KindType=4 | `--set-json 'x="a b"'` + `${x \| quote}` → `"a b"` |
| P5 | `indent` | 标量（`Stringify`） | 必需，`Atoi`，≥0 | 每行加 N 空格；0 或空串原样 | 缺参/非整数/负→3；结构化→4 | `--set-json 'x="a\nb"'` + `${x \| indent 2}` → 每行前两空格 |
| P6 | `type` | 见 §8 | 必需类型名 | 断言/转换后的值 | 见 §8.4 | `--set 'x=8080'` + `${x \| type int}` → `8080` |
| P7 | `toYaml` 的 `N`（嵌套宽度） | 任意 | 可选，正整数 | 文档**内部**每深一层缩 N 格；缺省 2 | 0/负数/非整数 → 3 | `${obj \| toYaml 3}` → 层宽 3 |
| P8 | `toYaml`/`toJson` 的 `align` | 任意 | 可选关键字 | 结果标记为"请对齐"：整段每行对齐到表达式所在列；缺省**原样插入** | 与 `indent` 同时出现 → 2 | `${obj \| toYaml align}`；`${cert}` → 原样 |
| — | 未知 formatter | — | — | — | KindTemplate=3 | `${x \| nope}` → `unknown formatter "nope"`，exit 3 |

```bash
# formatter 可复制案例
./dist/envsub render --set-json 'x=[1,2]'         <(printf '${x | join "-"}\n')       # 1-2
./dist/envsub render --set-json 'x={"b":1,"a":2}' <(printf '${x | toJson}\n')        # {"a":2,"b":1}
./dist/envsub render --set-json 'x={"a":1}'       <(printf '${x | toYaml}\n')        # a: 1
./dist/envsub render --set-json 'x="a b"'         <(printf '${x | quote}\n')         # "a b"
./dist/envsub render --set-json 'x="a\nb"'        <(printf '${x | indent 2}\n')      # 缩进两格
./dist/envsub render --set-json 'x=[1]'           <(printf '${x | indent 2}\n')      # exit 4（结构化）
```

## 8. `| type` 声明类型规则

**本章解决**：内联类型断言的类型名、字符串物化、结构化兼容性与错误，并给出每类型名案例。

### 8.1 类型名清单

_列：ID | 类型名 | 语义 | 状态。_

| 编号 | 类型名 | 语义 | 状态 |
|------|--------|------|------|
| TY1 | `string` | 原样 | ✅ |
| TY2 | `int` | 精确整数，含负数，无内建上限 | ✅ |
| TY3 | `float` | IEEE-754 binary64 有限值（F7：超范围/非有限值报错） | ✅ |
| TY4 | `bool` | 仅 `true`/`false` | ✅ |
| TY5 | `object` / `array` / `array<T>` | JSON 结构；`array<T>` 逐元素断言；嵌套 `array` 拒绝 | ✅ |
| TY6 | `null` | **保留**：断言"该值必须为 null"；与 JSON Schema `type: null` 对齐；**禁止**从字符串隐式转换（G18，已实现） | ✅ |
| TY7 | `uint` | **不提供**（无需求；需要非负约束用 schema `minimum: 0`）；`\| type uint` → `unsupported type "uint"`，exit 2 | 不提供 |
| TY8 | `int32`/`int64`/`uint32`/`uint64` | **禁止**新增宽度类型名；宽度约束用 schema `minimum`/`maximum` | 已决（T8） |

### 8.2 字符串 → 类型物化规则

_列：类型 | 规则 | 证据。_

| 类型 | 规则 | 证据 |
|------|------|------|
| `string` | 原样 | convert.go:109-110 |
| `int` | `Atoi` → `ParseUint(10,64)` → `codec.New` | convert.go:111-121 |
| `float` | `ParseFloat`；拒绝空串/含空白/非数字；`nan`/`inf`/`Infinity` 拒绝（`value "nan" is not a finite float`，convert.go:130） | convert.go:122-127 |
| `bool` | 仅 `"true"`/`"false"` | convert.go:128-136 |
| `object` / `array` | 合法 JSON 且顶层类型匹配；内部数字保留 `json.Number` | convert.go:137-166 |
| `null` | **禁止字符串转换（已实现，G18）**：唯一正当来源是 JSON/YAML `null` 或 `--set-json x=null`；`""` 与 `"null"` 报 `value ... cannot be converted to null`，exit 4 | convert.go:175 |

### 8.3 结构化值兼容性检查

_列：检查 | 规则 | 证据。_

| 检查 | 规则 | 证据 |
|------|------|------|
| `Assert`（值 vs `\| type`） | string/int(integral)/float(numeric)/bool/object/array/null 严格匹配 | convert.go:179-220 |
| `schemaCompatible`（`\| type` vs schema） | 同名通过；array 比较 Element（任一为空放行）；int↔float 互通；其余冲突 | engine.go:403-421 |
| 两者同时存在 | 先按 schema 类型物化，再校验兼容，再按 inline 断言 | engine.go:325-362 |

### 8.4 错误文案与退出码

_列：场景 | 文案 | 退出码 | 证据。_

| 场景 | 文案 | 退出码 | 证据 |
|------|------|--------|------|
| 未知/未提供类型名 | `unsupported type %q`（含 `uint`） | 2 | expression.go:146-150 |
| 空类型名 | `type requires a type name` | 3 | expression.go:143-144 |
| type 非首个 step | `inline type must be the first pipeline step` | 3 | expression.go:89-91 |
| 重复 type | `declares more than one type` | 3 | expression.go:85-88 |
| 断言失败 | `value %q is not a valid int/bool/...` | 4 | convert.go:121-172 |
| 与 schema 冲突 | `inline type %s conflicts with schema type %s` | 4 | engine.go:353-354 |

**案例（类型名）**

| 规则 | 命令 | 输出 / exit | 标注 |
|------|------|-------------|------|
| TY1 `string` | `--set 'x=8080'` + `${x \| type string}` | `8080` | 现状 |
| TY2 `int` | `--set 'x=2^63-1（9223372036854775807）'` + `${x \| type int}` | `9223372036854775807` | 现状 |
| TY3 `float` | `--set 'x=0.5'` + `${x \| type float}` | `0.5` | 现状 |
| TY3 `float` 超范围 | `--set 'x=1e309'` + `${x \| type float}` | `value "1e309" is not a valid float`，exit 4 | 现状 |
| TY4 `bool` | `--set 'x=true'` + `${x \| type bool}` | `true` | 现状 |
| TY5 `array<int>` | `--set-json 'x=[1,2]'` + `${x \| type array<int> \| join ","}` | `1,2` | 现状 |
| TY5 嵌套 array | `${x \| type array<array<int>>}` | `nested array types are not supported`，exit 2 | 现状 |
| TY6 `null` | `--set-json 'x=null'` + `${x \| type null}` | 空，exit 0 | 现状 |
| TY6 `null` 字符串 | `--set-json 'x=""'` + `${x \| type null}` | 现状：`value "" cannot be converted to null`，exit 4 | 现状 |
| TY7 `uint` | `--set 'x=1'` + `${x \| type uint}` | `unsupported type "uint"`，exit 2 | 现状（不提供） |
| TY8 宽度名 | `${x \| type int64}` | `unsupported type "int64"`，exit 2 | 现状（不提供） |

> schema 与 inline `| type` 的完整交互规则（并集、顺序、冲突、相容性判定表与理由）见
> [§16 schema 与 inline 规则](12-schema-and-inline-rules.md)。

## 附录：多行值与缩进（P5/P7/P8 的边界）

多行值有两种意图，**必须由模板显式选择**，不能靠猜：

| 意图 | 写法 | 续行位置 |
|------|------|----------|
| **结构内容**（YAML 块），要嵌在父键下 | 序列化器加 `align` | 与首行同列（对齐到表达式所在列） |
| **结构内容**，列由序列化侧决定 | `${x \| toYaml N \| indent K}` | 由 `indent` 决定（引用行留列 0） |
| **原样文本**（PEM、脚本、别人给的片段） | 不加任何东西 | **原样**：源文本自己的位置（通常列 0） |

三件事各管一段，互不越界：

| 阶段 | 谁 | 管什么 |
|------|----|--------|
| 序列化 | `toYaml N` / `toJson` | 文档**内部**每深一层缩几格 |
| 文本加工 | `indent K` | 在这段文字**当前格式之上**每行再加 K 格（相对追加） |
| 插入 | 替换层 | 仅当值带 `align` 标记时，把每行对齐到表达式所在列 |

**互斥**：`align` 与 `indent` 不能同时出现（列会被定义两次）→ KindUsage exit 2；
`indent` 只接受普通标量，遇到带标记的序列化结果即报错。

**边界情况**：

- 表达式不在行首（如 `key: ${x}`）或前导含 tab 时，列不可复现，即使带 `align` 也不补缩进（原样插入）；
- 原样插入（不带 `align`）只适合**单行**值，或插到"后续行与父键同列也不破坏结构"的位置；
  YAML 的块标量内容必须比父键更深，因此**嵌在键下的多行值必须带 `align`**（或改用 `indent K`）。

`[FACT]` 依据：`extended/engine.go` 的 `spanColumn`/`alignLines`、`pipeline` 的
`markAligned`/`formatIndent`；fixture `formatters/toyaml_align_default`、
`formatters/toyaml_align_width`、`formatters/toyaml_raw_keeps_column`、
`formatters/toyaml_align_with_indent_rejected`、`formatters/toyaml_bad_argument`；
示例 `templates/scenarios-yaml.tmpl` 的 `object_nested_block`（align）与
`verbatim.string_type_multiline_verbatim`（原样，续行停在列 0），
并由 `examples` 的解析守护保证产物是合法 YAML。
