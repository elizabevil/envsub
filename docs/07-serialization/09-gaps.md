> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §13。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 13. 现状与规则的差异清单

**本章解决**：逐条记录"现状 vs 规则"的差异、是否统一与结论，供实现排期。

### 13.1 差异（G 系列）

_列：ID | 现状（file:line / 实测） | 规则要求 | 是否需统一 | 结论/状态。_

| 编号 | 现状（file:line / 实测） | 规则要求 | 是否需统一 | 结论 / 状态 |
|------|--------------------------|----------|------------|-------------|
| G1 | `sonic` 重复成员名 last-wins（sonic.go:20-43） | Q3b 后端等价 | 否 | **已决为 sonic 唯一例外**（T9）；换后端行为变化须在用户文档固化 |
| G2 | ~~`\| type float` 对超出 float64 精确表示的整数静默丢精度~~ | F7：超出 float64 精确表示必须报错 | 已消除 | **已消除**：`types.checkFloatExact`（convert.go）对字符串、`uint64`、`ExactInt`、`json.Number` 一致报错；`2^53` 边界通过 |
| G3 | 超出 `float64` 的字面量 `1e309` | F6：未声明按字面量保留；F7：转 `float` 时报错 | 否 | **已消除**：未声明走 `codec.LiteralNumber` 保留原文；`\| type float` 报错 exit 4 |
| G4 | 两通道拼写：`${x}` 定点展开、`toJson`/`toYaml` 最短往返 | 各自明确即可 | **否** | **已决：不统一（有意）**——无 formatter 是标量文本化（避免指数），formatter 是序列化；两套规则已写入 §3.7 与 §10 |
| G5 | YAML `.nan`/`.inf` 曾被接受为 NaN/Inf；`toYaml` 曾输出 `.nan`，`toJson` 抛库层错误 | F8：三路径无条件拒绝 | 否 | **已消除**：解码拒绝（yamlx/yamlv3.go:149-150，`non-finite number ... is not allowed`）；编码经 `codec.RejectNonFinite`（codec/exact.go:63）被 jsonx 两后端与 yamlx/marshal.go:24 调用 |
| G6 | `:=`/`=` 写回的是渲染后的字符串 | 各路径能力一致 | **否** | **已决：维持并写入规范**——赋值写入字面量文本，类型由后续 `\| type` 或 schema 决定（与 C1 的 `--set` 一致） |
| G7 | 浮点拼写曾不保留：`1.50`→`1.5`、`1.0`→`1` | F6：未声明小数保留拼写；声明 `float` 按最短往返 | 否 | **已消除**：未声明数字按 F6 保留原文；声明 `float` 仍按最短形式（`1.0`→`1`） |
| G8 | `--set` 文案与实现不一致 | 行为与文案一致 | 否 | **已解决**：帮助文案改为 raw string；`SetTyped` 已随 `--set-string` 删除 |
| G9 | `--set-json` 非法 JSON 曾为 KindUsage=2；`.env` 非法 JSON 为 KindType=4 | 错误分类一致 | 是 | **已统一为 KindType exit 4**：`values.ErrInvalidJSON`（values/set.go:30）由 `render.setOne` 映射（render/render.go:437）；赋值语法错误仍为 KindUsage exit 2 |
| G10 | `.env` 结构化值只能写单行 JSON | — | **否** | **已决：维持**——`.env` 是按行方言；多行结构请用 YAML/JSON 值文件或 `--set-json` |
| G11 | 联合类型 `type: ["integer","null"]` 时 `DeclaredType` 返回空 | 支持联合类型 | 否 | **已实现**：取第一个非 `null` 成员作为物化目标，`null` 仅作可空标记（全为 null 时返回 null）；测试 `TestDeclaredTypeUnion`、`TestMaterializeUnionType`（`go test ./internal/schema/`）；**SI11 已落地**：联合按成员集合物化（可物化成员按序尝试，含 string 时失败即保留字符串），兼容性判定亦看成员集合，见 `12-schema-and-inline-rules.md` §2 SI11 |
| G12 | `$ref`/`allOf` 处的 `default` 不生效（仅遍历 `properties`/`items`） | 语义一致 | 否 | **已实现**：`Load` 生成 `s.walk = expandSchema(Raw)`（本地 `$ref` 按 JSON Pointer 展开、`allOf` 分支合并，先出现者胜，seen+深度保护循环），默认值/物化/类型查询/路径查询四个 walker 统一读 `s.walk`；`Raw` 与编译后的 schema 不变（校验语义不受影响）；外部 `$ref` 由编译器拒绝（`TestExternalRefIsRejected`）。测试 `TestApplyDefaultsThroughRef`、`TestApplyDefaultsThroughAllOf`、`TestRefChainAndCycle`、`TestMaterializeThroughRef` |
| G13 | `array<T>` 嵌套校验两侧不一致 | 两处一致 | 否 | **已实现（拒绝嵌套）**：抽取共用文法 `types.ArrayElement`（哨兵错误 `ErrUnclosedArray`/`ErrEmptyArrayElement`/`ErrNestedArray`），inline 的 `ParseType` 与 schema 的 `normalizeTypeName` 共用；`array<array<…>>` 与 `array<array>` 两侧报同一句 `nested array types are not supported`，`array<object>` 放行。对拍用例 `TestArrayAliasGrammarIsShared`（`go test ./test/encoding/`） |
| G14 | ~~嵌套 env fallback（`APP_HOSTS`→`app.hosts`）不参与物化/校验~~ | 评审决定**不保留路径推导** | — | **已撤销（功能移除）**：不再存在 `路径 → ENV_NAME` 推导。**v0.11 更新（方案 C）**：环境读取只剩 `${@process.NAME}`；`.env` 写入根；`${NAME}` 恒读根、**不再回退环境**；因此 `envFallback`（`materialize.go`/`defaults.go`）一并移除。测试 `TestNoPathDerivationForNestedPaths`、`TestEnvName`（已删除）；过时 fixture `sources/env_nested_path_fallback`、`env_file_wins` 已删除/替换。见 `06-review/06-15` §3.1 D3/D6/D13 |
| G15 | `.env` 非法 JSON（exit 4）与 `--set-json` 非法 JSON（原 exit 2）不一致 | 错误分类一致 | 否 | **已解决**：统一为 `KindType` exit 4 |
| G16 | `pipeline.Names()` 不含 `type` | — | **否** | **已决：维持**——`type` 是内联类型断言而非 formatter；已在 `pipeline.Names()` 注释说明 |
| G17 | ~~`--set-string` 值仍会按 schema 物化~~ | **已消除**：该 flag 已删除，`--set` 按 schema 物化是唯一且符合规则的行为（原缺陷描述：materialize.go:58-77 对所有字符串一律转换） | 否 | 已消除 |
| G18 | `\| type null` 曾把 `""` 隐式转为 `nil` | `null` 保留，但禁止字符串隐式转换；`""` 必须报错 exit 4 | 是 | **已消除**：`value "" cannot be converted to null`，exit 4（types/convert.go:175） |
| G19 | `--on-empty` 仅接受 `ok\|error` | — | **否** | **已决：维持并文档化**——帮助文案已写 `ok\|error`；空值的 keep/zero 语义由 `--on-missing` 覆盖 |
| G20 | 结构体字段标签 `omitempty` 对**数值 0** 的处理两后端不一致：`encoding/json/v2` **保留**（`{"whole":0}`），`sonic` **省略**（`{}`） | Q3b：两后端输出等价 | 是 | **已按编码侧约束统一**：需要"数字可省略"的字段一律用 `*int` + `omitempty`（本轮 `--explain` 的 `line` 即如此）；指针/字符串/切片两后端行为一致，无需改后端。测试：`TestMarshalOptionalFieldsAgree`（共享）、`TestJSONv2MarshalKeepsOmitemptyZeroInt`、`TestSonicMarshalOmitsOmitemptyZeroInt`（各后端一条） |
| G21 | schema `default` 落在**根命名空间**，于是压过 `.env` 与进程环境（早期实现）；与 `03-03` §3.5"default 最低"的建议相反 | 「**显式指定 > 隐式包含**」＋「`default` 只是 schema 兜底」 | 是 | **已修正**：该 key 有任何来源时不填 default —— `default` 由"一层来源"降为**兜底**。**v0.11 定稿顺序**：`--set`/`--set-json` > `--dotenv-file` > `-f/--values-file` > `default`（进程环境退出层序）。测试 `TestApplyDefaultsDefersToEnvironment`、fixture `schema_default_is_fallback`；规则见 `04-cli.md` §6.2 |
| G22 | `.env` 键归根后进入 schema 校验面：`.env` 里的宿主变量（`PATH`/`HOME`/`CI`）在 `additionalProperties:false` 下会被拒收（实测：`.env PORT=8080\\nPATH=/usr/bin` + 只声明 `PORT` 的严格 schema → 现状 exit 0，归根后 exit 4） | **已决（O1=(a)）**：照常纳入校验 | 是 | **已定案**：`.env` 与值文件同源、同规则（"根里的键都要过 schema"）；代价写入迁移说明。e2e fixture `dotenv_schema_strict`（未声明键 → exit 4）。见 `06-15` §6 O1、§7 R7 |

### 13.2 已决事项索引（T 系列）

_列：ID | 结论 | 状态。_

| 编号 | 结论 | 状态 |
|------|------|------|
| T1 | `sonic` 后端归一化 typed nil 为 `{}`/`[]`，保住 V5 | ✅ |
| T2 | 精确整数类型归属解析规则层，落在叶子包 `internal/codec` | ✅ |
| T3 | 每种类型给出显式精确语义与取值范围，见 §3 | ✅ |
| T4 | 类型名只提供 `int`（精确整数，含负数，无内建上限）；不提供 `uint` 与宽度类型名 | ✅ 已决 |
| T5 | 运算符只做存在性/空值判断；`:=`/`=` 写回字符串 | ✅ |
| T6 | `NaN`/`±Inf` 三路径报错（= F8） | ✅ |
| T7 | 深度上限继承库行为（约 10000 层），不叠加自有上限 | ✅ |
| T8 | 不增加宽度类型名 | 已决 |
| T9 | 接受 `sonic` 重复成员名为唯一例外 | ✅ |


### G13 需求讨论：`array<T>` 是否支持嵌套（含"object 套娃"）

**先澄清两点"不是问题"的部分**：

1. **object 套娃本身已完全可用**：`object` 是叶子类型名，内层结构由 JSON Schema 的嵌套
   `properties` 描述（inline `| type object` 只断言"是对象"，按 SI5 不描述形状）。值树里
   `map[string]any` 任意嵌套、`toJson`/`toYaml` 输出、jsonschema/v6 校验都已支持。
2. **`array<object>` 也已支持**（`Element = object`）；真正缺的只有 `array<array<…>>`。

**若要支持嵌套数组，需要的操作（按依赖顺序）**：

| # | 操作 | 影响面 | 说明 |
|---|------|--------|------|
| 1 | `types.Declared` 改为**递归**（`Element` 从 `string` 变为 `*Declared` 或等价的嵌套表示） | `internal/types`、`internal/schema`、`internal/engine/extended` 及全部 `Declared{...}` 字面量与断言/比较/打印点（估 30–60 处） | 当前模型是扁平的（`Name + Element string`），根本无法表达两层元素 |
| 2 | 共用解析器支持平衡嵌套（SI1），递归下降解析 `<...>` | `types.ParseType` + `schema.normalizeTypeName` 共用一份 | 顺带修掉现在的分叉：inline 拒绝、schema 静默降级 |
| 3 | schema 别名 → 嵌套 `items` 的**递归**映射；JSON Schema 的嵌套 `items` → 递归 `Declared` 的反向映射 | `internal/schema`（normalize/materialize/lookup） | 否则"声明支持"但物化/断言只做一层 |
| 4 | 物化与 `Assert` 递归到每层元素 | `internal/types/convert.go`、`internal/schema/materialize.go` | `.env` 的 JSON 字符串按每层元素逐个断言 |
| 5 | **formatter 语义决策**：`join` 目前只接受标量元素、明确拒绝嵌套数组/对象 | `internal/pipeline` | 支持嵌套类型后会立即暴露"能校验但不能 join"的不对称，需要额外规则（拒绝？要求元素级 formatter？） |
| 6 | 文档与测试：SI1/§8 类型表、`09-gaps`、嵌套数组正/负 fixture、对拍用例 | 文档 + `test/` | 按 §7 规则 5/6 |

**收益**：仅"允许在 schema 别名与 inline 里写 `array<array<int>>`"。真实配置里嵌套数组很少见
（矩阵类字段），而需要时用户可以在 schema 里直接写 JSON Schema 的嵌套 `items`——**校验侧已经支持**
（jsonschema/v6），缺的只是我们自己的逐元素 `Assert` 深度。

**成本**：操作 1 是**结构性改动**（类型模型 + 全仓字面量），操作 3/4 是把递归语义贯穿到物化，
操作 5 会引入新的 formatter 规则需求。

**推荐（选项 A，成本低、立即消除缺陷）**：

- 按 **SI1** 抽取**共用解析器**（这一条无论选哪个方案都要做）；
- `array<array<…>>` 在 **schema 与 inline 两侧统一拒绝**，共用同一文案与退出码
  （建议 `KindUsage`(2)，与 `unsupported type` 同类）；
- `array<object>` 继续放行；
- 在文档里写明**逃生通道**：需要嵌套数组时用 schema 的 JSON Schema 原生 `items` 嵌套
  （校验由库负责），并注明我们逐元素 `Assert` 的深度限制。

**若将来确实需要（选项 B）**：迁移路径就是上表 1→6，且因解析器已共用（SI1），改动集中在
"类型模型递归化 + 映射递归化"两处，不会再有第三份文法。

#### G13 补充：object 内存在数组（任意深度）**不影响**类型声明设计

结论：**不需要递归 `Declared`，也不需要新语法**。理由与依据：

1. **我们的类型模型从不描述对象内部形状**：`types.Declared` 只有 `Name + Element`（一层元素名），
   而 `DeclaredType(path)` 对对象属性返回的也只是"这一层是什么"——`{type: object}` → `object`，
   `{type: array, items: {type: object}}` → `array<object>`。对象的内部结构（属性、嵌套数组）
   完全交给 JSON Schema 与 `jsonschema/v6` 递归校验，不经过 `Declared`。
2. **formatter 不遍历对象内部**（你指出的关键）：`toJson`/`toYaml` 直接把整个值序列化，
   对嵌套无感知；`join` 只接受**顶层 `[]any` 且元素为标量**，嵌套数组/对象会被它明确拒绝
   （`internal/pipeline`）。因此"object 里包数组"永远不会把嵌套元素送到 formatter 上。
3. **物化路径同样是浅层的**：`.env` 里 `APP_GROUPS=[{"name":"a","hosts":["x"]}]` 按 JSON 解析
   后就是完整的嵌套树；`DeclaredType` 给 `array<object>`，逐元素只断言"是对象"（浅层），
   深层结构由 `Validate`（库）负责——这正是 SI5 的分工：**schema 管形状与约束，inline 管顶层类型**。
4. 因此 G13 的缺口严格限定为：**`array<…>` 的"元素"本身又是 `array<…>`**（`array<array<int>>`）。
   `array<object>`、`object` 内含数组、数组内含对象、对象套对象，全部已可用。

推论：即使将来支持 `array<array<int>>`，也不会影响 object 内数组的任何行为——那部分
完全不经过我们新增的（或拒绝的）语法路径。
