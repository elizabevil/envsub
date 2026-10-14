> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §3。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 3. 值类型系统与取值范围

**本章解决**：定义值树中每种值的类型、解码、物化、编码、运算符、schema 与边界行为，并给出
可运行案例。

值树整数表示法 **N1**：`int64` 内用 `int`，`uint64` 内用 `uint64`，超出 `uint64` 用**精确整数**
（无内建上限）；小数按是否声明类型区分：声明 `float` 走 float64（§3.7），未声明按字面量保存
（§3.11）。

以下 §3.1–§3.9 每类一张主表，**列均为**：维度 | 规则与现状（含状态与证据）。九个维度固定为：
值域、JSON 解码、YAML 解码、`.env`/`--set` 物化、toJson 输出、toYaml 输出、运算符存在性判定、
schema 校验、边界与特殊规则。每张主表后紧跟 **案例** 小节，_列：命令 | 输出 | 标注_；标注
`现状` 表示当前实测行为，`目标（待实现）` 表示规则要求但代码未落地。

### 3.1 null

| 维度 | 规则与现状 |
|------|------------|
| 值域 | 单值 `null` ✅ |
| JSON 解码 | 字面量 `null` → `nil`（jsonv2.go:101-106）✅ |
| YAML 解码 | 空标量 / `null` / `~` → `nil`（yamlv3.go:133-150；yamlv3_test.go:51）✅ |
| .env / --set 物化 | `.env` 无法表达 null；`--set-json x=null` → `nil`（set.go:108-113）✅ |
| toJson 输出 | `null`（rules_test.go:87）✅ |
| toYaml 输出 | `null` ✅ |
| 运算符存在性判定 | **存在但判空**：`${X:-fb}` 触发、`${X?msg}` 因存在不报错（engine.go:556-557）✅ |
| schema 校验 | `type:null` 通过；其它类型报 `/path`（jsonschema.go:25-35）✅ |
| 边界与特殊规则 | typed nil map/slice 同样判空（engine.go:560-563），但编码为 `{}`/`[]`（V1）✅ |

**案例（null）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json x=null` + `${x}` | 空串，exit 0 | 现状 |
| `--set-json x=null` + `${x:-d}` | `d` | 现状 |
| `--set-json x=null` + `${x \| toJson}` / `${x \| toYaml}` | `null` / `null` | 现状 |
| YAML `x: null` + `${x?e}` | 空串，exit 0（存在，不报错） | 现状 |

### 3.2 bool

| 维度 | 规则与现状 |
|------|------------|
| 值域 | `true` / `false` ✅ |
| JSON 解码 | `true`/`false` → `bool`（jsonv2.go:95-100）✅ |
| YAML 解码 | 仅 YAML 1.2 core：`true`/`false`；`yes`/`on` 是字符串（rules_test.go:358-371）✅ |
| .env / --set 物化 | 仅小写 `"true"`/`"false"`（normalize.go:217-225；convert.go:128-136）✅ |
| toJson 输出 | `true` / `false` ✅ |
| toYaml 输出 | `true` / `false` ✅ |
| 运算符存在性判定 | **`false` 非空**（engine.go:564-565）✅ |
| schema 校验 | `type:boolean` ✅ |
| 边界与特殊规则 | `"True"`/`"1"`/`"yes"` 均非 bool（convert_test.go:76-79）✅ |

**案例（bool）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json x=true` + `${x} ${x \| toJson} ${x \| toYaml}` | `true true true` | 现状 |
| `--set-json x=false` + `${x:-d}` | `false`（false 非空） | 现状 |
| `--set x=true` + `${x \| type bool}` | `true` | 现状 |
| `--set x=yes` + `${x \| type bool}` | `value "yes" is not a valid bool`，exit 4 | 现状 |

### 3.3 string

| 维度 | 规则与现状 |
|------|------------|
| 值域 | 任意合法 UTF-8，可以为空 ✅ |
| JSON 解码 | JSON 字符串；非法 UTF-8 拒绝（Q3a；jsonv2_test.go:224-241；sonic.go:47-70）✅ |
| YAML 解码 | 带引号标量保持 string，即使形如数字（yamlv3.go:134；yamlv3_test.go:50）✅ |
| .env / --set 物化 | `.env` 原样字符串；`--set` 存字符串（set.go:104-110）✅ |
| toJson 输出 | 加引号 + JSON 转义；数字样字符串必须加引号（Q1）✅ |
| toYaml 输出 | 按需加引号 / 块标量 ✅ |
| 运算符存在性判定 | 空串 `""` 判空（engine.go:558-559）；见 §5.3 bash 语义 ✅ |
| schema 校验 | `type:string`；`minLength`/`maxLength`/`pattern` 由库执行 ✅ |
| 边界与特殊规则 | 非法 UTF-8 编解码均报错 exit 4；长度无内建上限 ✅ |

**案例（string）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set x=8080` + `${x} ${x \| toJson}` | `8080 "8080"` | 现状 |
| `--set-json 'x="<a>&b"'` + `${x \| toJson}` | `"<a>&b"`（不转义 HTML） | 现状 |
| `.env` 行 `A="line\nbreak"` + `${A}` | 两行文本（`\n` 被解码） | 现状 |
| JSON 文件含非法 UTF-8 字节 + 渲染 | `jsontext: invalid UTF-8 ...`，exit 4 | 现状 |

### 3.3.1 日期/时间类标量（YAML `!!timestamp`）

YAML 会把 `2024-01-01`、`2024-01-01T10:00:00Z`、`2024-01-01 10:00:00`（以及显式 `!!timestamp`）解析为
**`!!timestamp` tag**。值文法**没有时间类型**（§3 的值类只有 null/bool/string/int/uint64/精确整数/
LiteralNumber/float64/array/object），因此这类标量**按字符串处理**，保留源文件的原始拼写。`[FACT]`

| 维度 | 规则与现状 |
|------|------------|
| YAML 解码 | `!!timestamp` → **原始文本字符串**（`yamlv3.go` 的 `scalarValue`）；不产出 `time.Time` ✅ |
| JSON / `.env` / `--set` | 本来就是字符串（JSON 无日期类型）✅ |
| 规避 | 无需规避；如需强制字符串可写引号或 `!!str` |
| 契约测试 | `TestScalarFormsStayInTheValueGrammar`（所有常见标量形式必须落在值文法类型内）、`TestTypeMatrixDecode` 的日期/时间行（YAML 与 JSON 同值同型）、`internal/render` 的 `TestRunRendersYAMLDateScalar`；gap 见 `09-gaps` G24 |

### 3.3.2 其他 YAML 形态的归一化

YAML 的部分写法会被解析成结构化或改写过的值。归一化后的类型必须仍落在 §3 的值类内：`[FACT]`

| 写法 | 归一化结果 | 说明 |
|------|------------|------|
| `!!binary aGk=` | `string` = `hi` | **唯一会改写内容**的形态：按 base64 解码后作为字符串；需要原样文本请加引号或用 `!!str` |
| `!!set {a, b}` | `object` = `{a: null, b: null}` | 集合语义用"成员存在 + null 值"表达 |
| `!!omap` / `!!pairs` | `array`（元素为 object） | 有序映射退化为数组 |
| 非字符串标量键（`1:`、`true:`） | 键**字符串化**（`"1"`、`"true"`） | JSON 对象键只能是字符串；不报错 |
| 序列/映射做键（`? [a, b]`） | **拒绝**：`cannot unmarshal !!seq into string` | 非标量键无法表达 |
| `<<` merge key | 合并进目标映射（`yamlx` 处理） | 与 YAML 1.1 语义一致 |
| 日期/时间戳 | `string`（原始拼写） | 见 §3.3.1 |

### 3.3.3 输入 BOM（字节序标记）

Windows 编辑器保存文件时默认写入 UTF-8 BOM（`EF BB BF`）。**所有解码入口统一剥掉开头的 BOM**，
不再出现"YAML 能读、JSON 报 `invalid character '\ufeff'`"这类按格式分叉的行为：`[FACT]`

| 入口 | 位置 |
|------|------|
| JSON（两个后端） | `jsonx.Unmarshal` 漏斗 → `utils.TrimBOM` |
| YAML | `yamlx.Decode` / `yamlx.Unmarshal` |
| `.env` | `source.ParseDotenv` |
| `--set-file` 的文件内容 | `load.setOneFile` |

规则：**只剥开头**的 BOM（文件中间的同样字节是内容，保留）；UTF-16 BOM 不处理（该文件不是合法 UTF-8，
按非法 UTF-8 拒绝）；模板文件不属于"解码入口"，其字节原样参与渲染。测试：
`utils.TestTrimBOM`、`jsonx.TestUnmarshalValueAcceptsLeadingBOM`、`yamlx.TestDecodeAcceptsLeadingBOM`、
`source.TestParseDotenvStripsLeadingBOM`、`load` 的 `--set-file` BOM 行、`test/encoding` 的 BOM 矩阵行；
gap 见 `09-gaps` G25。

### 3.4 int

| 维度 | 规则与现状 |
|------|------------|
| 值域 | **取能精确表示该值的最窄类型**（N5）：`int64` 内（`2^63-1`，9223372036854775807）→ `int`；`uint64` 内（`2^64-1`，18446744073709551615）→ `uint64`；超出 → 精确整数；无内建上限（N1/N5）✅ |
| JSON 解码 | `json.Number` → `NormalizeNumbers` → `int`/`uint64`/`ExactInt`（number.go:37-55）✅ |
| YAML 解码 | yaml.v3 节点 → `int`；超 `int64` → `uint64`；超 `uint64` → 精确整数（yamlv3.go:145-149）✅ |
| .env / --set 物化 | `\| type int`：`Atoi` → `ParseUint(10,64)` → `codec.New`（convert.go:112-120）；schema `integer` 同（normalize.go:201-209）✅ |
| toJson 输出 | 裸 JSON number（rules_test.go:40-45）✅ |
| toYaml 输出 | 裸 `!!int` 标量 ✅ |
| 运算符存在性判定 | `0` **非空** ✅ |
| schema 校验 | `type:integer`；`minimum`/`maximum` 越界报 `/path`（schema/minimum_violation）✅ |
| 边界与特殊规则 | `2^63-1` 精确、不被 float 化（rules_test.go:392-397）✅；负值是 `int` 的普通合法值 |

**案例（int）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json x=8080` + `${x} ${x \| toJson} ${x \| toYaml}` | `8080 8080 8080` | 现状 |
| `--set 'x=2^63-1（9223372036854775807）'` + `${x \| type int}` | `9223372036854775807` | 现状 |
| `.env BIG=2^64-1（18446744073709551615）` + `${BIG \| type int}` | `18446744073709551615` | 现状 |
| `--set-json x=-5` + `${x}` | `-5`（负值合法） | 现状 |

### 3.5 uint64（值类型，非对外类型名）

| 维度 | 规则与现状 |
|------|------------|
| 值域 | `0` .. `2^64-1`（18446744073709551615）✅ |
| JSON 解码 | 超 `int64` 且落 `uint64` → `uint64`（number.go:43-45）✅ |
| YAML 解码 | 超 `int64` → `uint64`（yamlv3_test.go:44）✅ |
| .env / --set 物化 | `ParseUint(10,64)`（convert.go:115-117）✅ |
| toJson 输出 | 裸数字（rules_test.go:42-43）✅ |
| toYaml 输出 | 裸数字 ✅ |
| 运算符存在性判定 | 非空 ✅ |
| schema 校验 | `type:integer` 通过 ✅ |
| 边界与特殊规则 | `\| type float` 若无法精确表示必须报错（F7）：`value ... cannot be represented exactly as a float64`，exit 4（FB3 ✅） |

**案例（uint64）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json x=2^64-1（18446744073709551615）` + `${x \| toJson}` | `18446744073709551615` | 现状 |
| YAML `x: 2^64-1（18446744073709551615）` + `${x}` | 精确十进制 | 现状 |
| `.env X=2^64-1（18446744073709551615）` + `${X \| type int}` | 精确十进制 | 现状 |
| `--set-json x=12345678901234567890` + `${x \| type float}` | 现状：`value 12345678901234567890 cannot be represented exactly as a float64`，exit 4 | 现状 |

### 3.6 超过 uint64 的精确整数

| 维度 | 规则与现状 |
|------|------------|
| 值域 | 任意精度十进制整数，无内建上限（N1）✅ |
| JSON 解码 | `json.Number` → `codec.ExactInt`（number.go:46-48；rules_test.go:131-140）✅ |
| YAML 解码 | yaml.v3 回退 `float64` 后按节点文本构造精确整数（yamlv3.go:145-149）✅ |
| .env / --set 物化 | `codec.New` → `ExactInt`（convert.go:118-120；schema_test.go:303-336）✅ |
| toJson 输出 | `ExactInt.MarshalJSON` 裸数字（exact.go:45-50；rules_test.go:47-49）✅ |
| toYaml 输出 | `yamlIntNode` 隐式 `!!int` 裸标量（yamlx/marshal.go:64-66,80-82）✅ |
| 运算符存在性判定 | **非空**（T5）✅ |
| schema 校验 | 送校验前 `codec.ToJSON` 转 `json.Number`（exact.go:83-88；schema_test.go:271-298）✅ |
| 边界与特殊规则 | 下游（如只支持 `2^53`，9007199254740992 的 JS）可能无法表示，属使用方约束；`\| type float` 对精确整数报错 exit 4（FB3 ✅） |

**案例（精确整数）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json x=2^64（18446744073709551616）` + `${x \| type float}` | 现状：`cannot be represented exactly as a float64`，exit 4 | 现状 |
| `--set-json x=123456789012345678901234567890` + `\| toJson` / `\| toYaml` | 裸数字，无引号 | 现状 |
| YAML `x: 2^64（18446744073709551616）` + `${x}` | 精确十进制 | 现状 |
| `--set-json x=2^64（18446744073709551616）` + `${x \| type float}` | 现状：`cannot be represented exactly as a float64`，exit 4 | 现状 |

### 3.7 float（声明 float64）

| 维度 | 规则与现状 |
|------|------------|
| 值域 | 显式 `\| type float` **仅限** IEEE-754 binary64 的**有限**值（F7）；非有限值无条件拒绝（F8）✅ |
| JSON 解码 | 声明为 float 时：含 `.`/`e`/`E` → `float64`（number.go:51-53）；未声明见 §3.11 |
| YAML 解码 | `.nan`/`.inf` 被拒绝：`non-finite number ".nan" is not allowed`，exit 4（yamlv3.go:149-150）✅（F8） |
| .env / --set 物化 | `strconv.ParseFloat`（convert.go:122-127）；`nan`/`inf`/`Infinity` 被拒绝：`value "nan" is not a finite float`，exit 4（convert.go:130）✅（F7/F8） |
| toJson 输出 | 声明 `float` 时最短往返十进制（`1.0`→`1`）；可指数；`-0` 保留符号；未声明字面量见 §3.11 ✅ |
| toYaml 输出 | 与 toJson 语义一致；未声明字面量见 §3.11 ✅ |
| 运算符存在性判定 | `0` **非空** ✅ |
| schema 校验 | `type:number`；按 IEEE 比较，无十进制容差（F5）✅ |
| 边界与特殊规则 | 见 §3.10；`NaN`/`±Inf` 解码/物化报错，编码经 `codec.RejectNonFinite`（exact.go:63）在 jsonx 两后端与 yamlx 报错，exit 4 ✅（F8） |

**案例（float）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set x=0.5` + `${x \| type float}` | `0.5` | 现状 |
| `--set-json x=1.0e30` + `${x}` / `${x \| toJson}` | `1000000000000000000000000000000` / `1e+30` | 现状 |
| `--set x=1e309` + `${x \| type float}` | `value "1e309" is not a valid float`，exit 4 | 现状 |
| `--set-json x=1e309` + `${x \| type float}` | 现状：`value 1e309 cannot be represented as a finite float64`，exit 4 | 现状 |
| `--set x=nan` + `${x \| type float}` | 现状：`value "nan" is not a finite float`，exit 4 | 现状 |

### 3.8 array

| 维度 | 规则与现状 |
|------|------------|
| 值域 | 0..n 个元素，无内建上限 ✅ |
| JSON 解码 | `[]any` ✅ |
| YAML 解码 | `[]any`（yamlv3.go:44-53）✅ |
| .env / --set 物化 | 仅当 schema 声明 `array` 或用 `\| type array` 时解析 JSON 字符串（materialize.go:60-66；convert.go:147-166）✅ |
| toJson 输出 | JSON 数组 `["a","b"]` ✅ |
| toYaml 输出 | 块序列（顶层 `- a`）✅ |
| 运算符存在性判定 | **空数组判空**（engine.go:560-561）✅ |
| schema 校验 | `type:array` + `items`/`minItems`/`maxItems`/`uniqueItems` ✅ |
| 边界与特殊规则 | `join` 拒绝嵌套数组与对象元素（pipeline.go:151-155）✅ |

**案例（array）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json 'x=[1,2]'` + `\| toJson` / `\| toYaml` / `\| join ","` | `[1,2]` / 块序列 / `1,2` | 现状 |
| `--set-json 'x=[]'` + `${x:-d}` | `d`（空数组判空） | 现状 |
| `--set-json 'x=[]'` + `${x}` | `array value requires a formatter`，exit 4 | 现状 |
| `.env H=["a","b"]` + schema `array` + `${H \| join ","}` | `a,b` | 现状 |

### 3.9 object

| 维度 | 规则与现状 |
|------|------------|
| 值域 | 0..n 个键；键为合法 UTF-8 字符串 ✅ |
| JSON 解码 | `map[string]any`；重复成员名默认后端拒绝（Q3b）✅ |
| YAML 解码 | `map[string]any`；重复键拒绝（yamlv3.go:88-90）；支持 `<<` 合并键（yamlv3.go:65-105）✅ |
| .env / --set 物化 | schema 声明 `object` 或 `\| type object` 时解析 JSON 字符串（materialize.go:60-66）✅ |
| toJson 输出 | 紧凑 JSON，键排序（jsonv2.go:132-134；sonic.go:20-23）✅ |
| toYaml 输出 | 块映射，键排序（rules_test.go:198-204）✅ |
| 运算符存在性判定 | **空对象判空**（engine.go:562-563）✅ |
| schema 校验 | `type:object` + `required`/`properties`/`additionalProperties` 等 ✅ |
| 边界与特殊规则 | `sonic` 重复成员名 last-wins（T9 唯一例外，§12）**已决例外** |

**案例（object）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json 'x={"b":1,"a":2}'` + `\| toJson` | `{"a":2,"b":1}`（键排序） | 现状 |
| `--set-json 'x={}'` + `${x:-d}` | `d`（空对象判空） | 现状 |
| `.env H={"a":1}` + schema `object` + `${H \| toJson}` | `{"a":1}` | 现状 |
| JSON 文件 `{"a":1,"a":2}` + 渲染 | `jsontext: duplicate object member name "a"`，exit 4 | 现状 |

### 3.10 浮点与拒绝规则（F 系列）

**本章节解决**：声明 `float` 的表示、范围、非有限值与编码形态。

_列：ID | 规则 | 现状（实测 / file:line） | 状态。_

| 编号 | 规则 | 现状（实测 / file:line） | 状态 |
|------|------|--------------------------|------|
| F1 | `float` 为 IEEE-754 binary64；不做十进制精确化，不引入 decimal | convert.go:122-127 | ✅ |
| F2 | `float` 编码为最短往返十进制，允许指数形式 | rules_test.go:255-258 | ✅ |
| F3 | 整数样小数**不得丢小数点**：未声明字面量 `1.0` 输出 `1.0`；声明 `float` 后按最短往返（`1.0`→`1`） | codec/literal.go、values/number.go、yamlx；见 §13.1 G7 | ✅ |
| F4 | `float` 物化接受十进制/指数，拒绝空串、含空白、非数字文本 | convert.go:122-127 | ✅ |
| F5 | 浮点比较按 IEEE，无十进制容差 | jsonschema/v6 | ✅ |
| F6 | **未声明小数按字面量保存**：`1.0`/`1.50`/`1e3` 原样保留，不经过 float64（`number` 不是值类型，见 §3.11） | codec/literal.go、values/number.go、types/convert.go、yamlx | ✅ |
| F7 | `float` **仅限 float64 范围**；超出范围（如 `1e309`）或无法精确表示的整数 → 报错 exit 4 | convert.go:130,202-211,330-373（checkFloatExact） | ✅ |
| F8 | `NaN`/`+Inf`/`-Inf` **无条件拒绝**：解码、物化、编码三条路径都必须报错 exit 4 | yamlx/yamlv3.go:149-150、types/convert.go:130,202-211、schema/normalize.go:218、codec/exact.go:63 | ✅ |

**浮点精度说明（为什么 `0.1` "看起来精确"）**：`float64` 是二进制分数，`0.1` 无法用有限二进制位
精确表示，存储的是最近邻 ≈`0.1000000000000000055511151231257827`；输出用"最短往返"算法，
打印回来仍是 `0.1`，所以看起来精确；算术会暴露差异（`0.1+0.2 == 0.30000000000000004`）。
规则：不做十进制算术、不引入 decimal；要**原样保留字面量**走未声明数字路径（F6）。`[FACT]`

**边界补充**：

_列：ID | 边界 | 现状（实测） | 规则 | 状态。_

| 编号 | 边界 | 现状（实测） | 规则 | 状态 |
|------|------|--------------|------|------|
| FB1 | 高精度小数按 `float64` 舍入 | `1.50` 当前 → `1.5`；`0.1` 存储为最近邻 | 声明 `float` 允许二进制舍入（F1）；未声明路径由 F6 保拼写 | ✅ 规则 |
| FB2 | 超出 `float64` 范围的**字面量** | `1e309` 未声明时三通道保留原文（codec.LiteralNumber）；转 `float` 报错 exit 4 | 未声明按 F6 保留；转 `float` 按 F7 报错 | ✅ |
| FB3 | `\| type float` 对超出 float64 精确表示的整数 | **已实现（v0.7）**：字符串、`uint64`、精确整数、`json.Number` 四种来源一致报错 | F7：三种来源一致报错 | ✅ |

**案例（浮点边界）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json x=1.0` + `\| toJson` | 现状：`1.0` | 现状 |
| `--set-json x=1.50` + `\| toJson` | 现状：`1.50` | 现状 |
| `--set-json x=1e3` + `\| toJson` | 现状：`1e3` | 现状 |
| `--set-json x=1e309` + `\| toJson` | `1e309`（未声明按字面量） | 现状 |
| YAML `x: .nan` + `\| toJson` | 现状：解码期 `non-finite number ".nan" is not allowed`，exit 4 | 现状 |

### 3.11 number（未声明数字，不是值类型）

**本章节解决**：明确"未声明数字"不是新增值类型，只是"既无 schema `type` 也无 `| type`"的统称，
并规定其保存规则。

**结论（决策）**：**不新增 `Number` 值类型**。规则如下：

- 未声明**整数**仍走 `int`/`uint64`/`ExactInt`，以保持运算符、比较与 schema 校验可用（N1/N5）。
- 未声明**小数**以**字面量文本**保存，不经过 float64：`1.0` 保存并输出 `1.0`、`1.50` → `1.50`、
  `1e3` → `1e3`（F6）。
- 实现：新增叶子类型 `codec.LiteralNumber`（internal/codec/literal.go），保存原文并实现 `MarshalJSON`；YAML 侧用隐式标量 Node 输出原文（yamlx/marshal.go）。
- 一旦声明为 `float`（schema `number` 或 `| type float`），值必须收窄为 float64 并遵守 F7（超范围
  报错）。

_列：维度 | 规则与现状。_

| 维度 | 规则与现状 |
|------|------------|
| 值域 | 任意 JSON number 字面量（含超出 float64 范围），按原文保存（F6）✅ |
| JSON 解码 | `NormalizeNumbers` → `codec.LiteralNumber`，原文保留（values/number.go）✅ |
| YAML 解码 | 合法 JSON number 文本 → `codec.LiteralNumber`；`.5`/`5.`/`1_000` 退回 float64（yamlx/yamlv3.go）✅ |
| .env / --set 物化 | 默认是字符串；未声明保持字符串（不自动转数字） |
| toJson 输出 | 原文（`1.0`、`1.50`、`1e3`）✅ |
| toYaml 输出 | 原文 ✅ |
| 运算符存在性判定 | 非空（不回退 float64 判空） |
| schema 校验 | 未声明不校验；声明 `number` 后按 float64 校验（F7） |
| 边界与特殊规则 | 与 `float` 分离；`number` 不做 float64 收窄 |

**案例（number）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `--set-json x=1.0` + `${x} ${x \| toJson} ${x \| toYaml}` | 现状：`1.0 1.0 1.0` | 现状 |
| `--set-json x=1.50` + `${x \| toJson}` | 现状：`1.50` | 现状 |
| `--set-json x=1e3` + `${x \| toJson}` | 现状：`1e3` | 现状 |
| YAML `x: 1.50` + `${x \| toJson}` | 现状：`1.50` | 现状 |

