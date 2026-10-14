---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-09
---

# 物化模型（materialization）

> 本文回答：**为什么要物化、什么时候物化、物化成什么、联合类型怎么处理**，以及它与 schema 校验、
> inline `| type`、null 语义的关系。规则条文见 [§16 schema 与 inline 规则](12-schema-and-inline-rules.md)
> （SI1–SI11），本文补充**背景与理由**。

## 1. 术语

| 术语 | 含义 |
|------|------|
| 物化 | 把**字符串拼写**按声明类型转成值树的原生类型（`"8080"` → `int 8080`） |
| 类型校验 | 用 JSON Schema 判断值是否合法（`Validate`，由 `jsonschema/v6` 执行） |
| 类型断言 | 模板里的 inline `\| type`，在**使用点**声明/收窄类型（SI2/SI7） |
| 声明驱动 | 只有存在类型声明时才转换；没有声明一律保持字符串 |

三者关系：**物化选方向 → 校验判合法 → 断言收窄**（顺序见 §3）。

## 2. 为什么需要物化：来源是异构的

| 来源 | 进入值树时 | 需要物化 |
|------|-----------|----------|
| `.env` 文件 | 一律**字符串**（按行方言；结构化值写单行 JSON，G10） | ✅ |
| `--set key=value` | 字符串（C1：存原文） | ✅ |
| 进程环境变量 | 字符串 | ✅ |
| 模板赋值 `:=` / `=` | 文本字符串（C6） | ✅ |
| `--set-json key=<json>` | JSON 原生类型（含 null） | ❌ |
| YAML / JSON 值文件 | 原生类型（int/uint64/ExactInt/float/bool/null/集合） | ❌ |

不物化就会出现"同一模板、两个来源、两种结果"：

```
.env:  PORT=8080        ${PORT | toJson}  →  "\"8080\""   # 字符串，带引号
yaml:  port: 8080       ${port | toJson}  →  8080          # 数字
```

物化的目的就是抹平这种差异，让 `.env` 与 YAML 走同一套语义。

## 3. 什么时候物化：渲染前、校验前

```
internal/render/render.go:364   sch.Materialize(tree)
internal/render/render.go:367   sch.Validate(tree)
```

**顺序不可交换**：`.env` 的 `"8080"` 必须先变成 `int 8080`，否则会被 schema
`type: ["integer","null"]` 判为类型不符。这也是 SI3 的顺序（先 schema 物化 → 相容性检查 → inline 断言）。

## 4. 没有声明就不转换（no silent coercion）

这是整个设计的地基。`.env` 里的 `"007"`、`"1.0"`、`"2026-10-09"` 若被自动识别为数字/日期，
会造成静默的数据损坏（前导零丢失、`1.0` 变 `1`、版本号被改写）。因此：

| 有没有声明 | 行为 |
|-----------|------|
| 无任何声明 | **保持字符串**；未声明的十进制也保留字面量文本（F6：`1.0` → `1.0`） |
| schema 声明了类型 | 按 schema 物化 |
| inline `\| type` 声明了类型 | 按 inline 断言/物化（作用在原始字符串上） |
| 来源自带类型 | 直接用原生类型，但仍受统一规则约束（NaN/±Inf 拒绝、超大整数走 `ExactInt`） |

## 5. 物化成什么：类型阶梯

| 目标 | 结果 |
|------|------|
| `integer` | `int64` 范围内 → `int`；超出 → `uint64`；再超出 → `codec.ExactInt`（精确十进制文本） |
| `float` / `number` | 严格 float64：非有限值拒绝、超出范围/精度丢失拒绝；未声明时保留 `codec.LiteralNumber` 原文 |
| `boolean` | `true`/`false`（YAML 的 `yes`/`on` 仍是字符串，见 yamlx） |
| `array` / `object` | 按单行 JSON 解析（G10），再按元素/schema 递归处理 |

## 6. 联合类型（`type` 是数组）

联合是**析取**，但物化需要**一个方向**，于是有两层工具：

| 工具 | 作用 | 用途 |
|------|------|------|
| `declaredTypeName(node)` | 折叠成**单个**名字（第一个非 `null` 成员） | 兼容性检查等需要单名的调用方 |
| `DeclaredTypes(path)` / `declaredTypes(node)` | 给出**成员集合**（按书写顺序）+ 是否允许 `string` | 物化与兼容性判定的真实依据（SI11） |

**SI11 规则**：可物化成员（`integer`/`number`/`boolean`/`array`/`object`）按书写顺序依次尝试，
第一个成功即用；全部失败且联合含 `"string"` → **保留字符串**（交校验裁决）；不含 `"string"` 才报错；
`"string"` **不参与正向偏好**，只作兜底 ⇒ 它的书写位置不影响结果。

实测对照（修复前 → 修复后）：

| 输入 | 修复前 | 修复后 |
|------|--------|--------|
| `["integer","string"]` + `"hello"` | ❌ `expected an integer`（**误拒** schema 允许的值） | ✅ 保留 `"hello"` |
| `["string","integer"]` + `"8080"` | `"8080"`（字符串） | ✅ `int 8080`（顺序差异消除） |
| `["integer","string"]` + `"8080"` | `int 8080` | ✅ `int 8080` |
| `["integer","null"]` + `"8080"` | `int 8080` | ✅ 不变 |
| `["integer","boolean"]` + `"hello"` | ❌ 报错 | ❌ 仍报错（string 本不合法） |

**为什么跳过 `null`**：`null` 不是可转换目标（`isScalarType` 与 `isStructuredType` 都不含它），
选它只能制造假阴性；返回 `""`（当成未声明）又会悄悄削弱约束（假阳性）。选任一非 null 成员则保证
"物化成功 ⇒ 满足联合"。

## 7. `null`、`required`、`""` 三者的区别

| 状态 | 写法 | `declaredTypeName` | 含义 |
|------|------|--------------------|------|
| 未声明类型 | 没有 `type` | `""` | 任何类型皆可 |
| 可为 null | `["integer","null"]` | `"integer"` | 值是 integer 或 null（JSON Schema 的可空惯用法；2020-12 没有 `nullable`） |
| 只允许 null | `"null"` / `["null"]` | `"null"` | 只接受 null |

`required` 管**键的存在性**，`type` 管**值**，两者正交 —— `required:["x"] + ["integer","null"]`
= "键必须出现，值可为 null，也可为整数"。在三个阶段的表现：

| 阶段 | 行为 |
|------|------|
| 物化 | 只看类型，不看 `required`（`"8080"` → int） |
| 校验 | 缺键 → required 失败；`null` → 通过；整数 → 通过；未物化的 `"8080"` → 类型失败 |
| 渲染 | `null` 是"**找到了但值为空**"：`isEmptyValue(nil)==true` → 走 `--on-empty`（默认 `ok`）；**不触发** `--on-missing`（键存在） |

## 8. object 内部不参与物化的"深处"

`declaredTypeName` 只回答**本层**声明的类型，不递归、不看 `items`/`properties`。因此：

- `{type: object}` → `object`；`{type: array, items: {type: object}}` → `array<object>`；
- 对象内部的数组/嵌套对象**完全交给 JSON Schema 与 `jsonschema/v6` 递归校验**；
- formatter 也不遍历对象内部：`toJson`/`toYaml` 整体序列化，`join` 只接受顶层 `[]any` 且拒绝嵌套元素。

结论：**object 内含数组（任意深度）不受类型声明限制影响**（G13 补充）。

## 9. 边界与已知限制

| 项 | 状态 |
|----|------|
| `array<array<…>>` | **已决并实现：两侧统一拒绝**（G13，共用文法 `types.ArrayElement`） |
| 嵌套 env fallback（`APP_HOSTS`→`app.hosts`）纳入物化与 `--validate-only` | **已实现（G14）**：按 schema 声明路径用 `values.EnvName` 查 env；仅当真找到值时才合成已声明的对象父层；根值优先 |
| 本地 `$ref`/`allOf` 处的 `default` | **已实现（G12）**：`expandSchema` 展开 `$ref`、合并 `allOf`，四个 walker 统一读 `s.walk`；`Raw`/编译结果不变 |
| 外部 `$ref` | 不处理：由 schema 编译器直接拒绝（不联网、不加依赖） |
| 多可物化成员的真歧义（如 `["integer","number"]`） | 按**书写顺序**取第一个成功者（确定性） |
| 算法实现份数 | **一份**：`schema.NormalizeString`（物化与渲染共用）；引擎仅对已类型化值做成员检查 |
| 遍历骨架份数 | **一份**：`walkProperties`/`walkItems`（默认值与物化各自只提供 visitor）；字符串解析出的集合与内联集合同样继续处理 |

## 10. 验证与复现

| 内容 | 测试 | 命令 |
|------|------|------|
| 联合物化不误拒 / 顺序无关 / 结构化成员 / 严格拒绝 | `TestMaterializeUnionNeverRejectsLegalString`、`TestMaterializeUnionArrayMembers` | `go test ./internal/schema/` |
| 单名折叠（含 `["null"]`、`["null","integer"]`） | `TestDeclaredTypeUnion`、`TestMaterializeUnionType` | `go test ./internal/schema/` |
| 兼容性判定看成员集合 / 渲染期规范化 | `TestSchemaCompatibleMembersHonoursUnion`、`TestNormalizeBySchemaMembersKeepsLegalString` | `go test ./internal/engine/extended/` |
| `array<T>` 文法两侧一致（SI1/G13） | `TestArrayAliasGrammarIsShared` | `go test ./test/encoding/` |
| 端到端（含 schema fixture） | `TestE2E/values_schema*` | `just test-e2e` |

## 11. 案例（Examples）

> 以下案例都可以用仓库里的 fixture 形态复现（`test/e2e/testdata/` 的 `args` + `input.tmpl` + `expect.out`）。
> 命令中的 `--schema` 即 schema 选项；`.env` 的顶层键会被物化（嵌套 env fallback 见 §9 G14）。

### 案例 1：同一份值，`.env` 与 YAML 结果一致（顶层键）

```bash
# schema.yaml
type: object
properties:
  PORT:  {type: integer}
  DEBUG: {type: boolean}
  HOSTS: {type: array}

# .env
PORT=8080
DEBUG=true
HOSTS='["x","y"]'

# input.tmpl
${PORT | toJson} ${DEBUG | toJson} ${HOSTS | toJson}

envsub render --env-file .env --schema schema.yaml input.tmpl
# 8080 true ["x","y"]
```

`.env` 里三个值进树时都是字符串，物化后分别是 int / bool / `[]any`，与 YAML 值文件写法结果一致。
（与 e2e fixture `values_schema` 同形。）

### 案例 2：没有声明 → 保持字符串（no silent coercion）

```bash
# .env
PORT=8080
VERSION=1.0
CODE=007

# 不加 schema，也不加 | type
# input.tmpl
${PORT | toJson} ${VERSION | toJson} ${CODE | toJson}
# "8080" "1.0" "007"
```

`"007"` 的前导零、`"1.0"` 的字面量都**不被改写**（F6）；只有声明存在时才转换。

### 案例 3：可空联合与 `null` 的真正来源

```bash
# schema.yaml
type: object
properties:
  TIMEOUT: {type: ["integer","null"]}

# ① 字符串来源：按非 null 成员物化
# .env
TIMEOUT=30
envsub render --env-file .env --schema schema.yaml input.tmpl   # 30

# ② null 只能来自 JSON/YAML
envsub render --set-json TIMEOUT=null --schema schema.yaml input.tmpl   # null
```

注意 `.env` 里写 `TIMEOUT=null` 得到的是**字符串** `"null"`，会被"integer 或 null"拒绝——
`null` 是 JSON 值，不是字符串拼写（§7）。

### 案例 4：联合含 `string` 时不误拒（SI11）

```bash
# schema.yaml
type: object
properties:
  TAG: {type: ["integer","string"]}

# .env
TAG=8080     →  ${TAG | toJson} = 8080      （先尝试 integer，成功）
# .env
TAG=hello    →  ${TAG | toJson} = "hello"   （integer 失败 → 保留字符串，校验仍通过）
```

修复前第二例会以 `expected an integer` 失败（见 §6 实测对照表）。

### 案例 5：非法值 → exit 4 与文案形态

```bash
# schema.yaml
type: object
properties:
  PORT: {type: integer}
# .env
PORT=eighty

envsub render --env-file .env --schema schema.yaml input.tmpl
# stderr 形如：PORT at <来源> (eighty): expected an integer, got "eighty"
# 退出码：4（type）
```

### 案例 6：object 内含数组不受类型声明影响

```bash
# schema.yaml
type: object
properties:
  app:
    type: object
    properties:
      hosts: {type: array, items: {type: string}}
# values.yaml
app:
  hosts: [a, b]
# input.tmpl
${app.hosts | join ","}      # a,b
```

对象内部形状由 JSON Schema 递归校验负责，`declaredTypeName` 只回答 `app` 是 `object`（§8）。

### 案例 7：库级对拍（无需 CLI）

| 案例 | 测试 |
|------|------|
| `["integer","string"]` + `"hello"` 保留字符串；`["string","integer"]` + `"8080"` 得 int | `TestMaterializeUnionNeverRejectsLegalString` |
| `["array","string"]` + `'["a","b"]'` 得两元素数组 | `TestMaterializeUnionArrayMembers` |
| inline `string` 与 schema `["integer","string"]` 相容；渲染期保留合法字符串 | `TestSchemaCompatibleMembersHonoursUnion`、`TestNormalizeBySchemaMembersKeepsLegalString` |

## 候选方案与权衡（Alternatives & Trade-offs）

| 方案 | 做法 | 为何不采用 |
|------|------|-----------|
| **A（当前）** | 声明驱动：只在有 schema/inline 声明时物化，联合按成员集合尝试 + `string` 兜底 | — |
| B | 内容推断：看到 `"8080"` 就当整数、看到日期就转时间 | 静默损坏数据（前导零、`1.0`、版本号）；与 F6 冲突 |
| C | 一律按 schema 折叠后的**第一个成员**转换 | 实测会误拒联合里其他合法成员，且结果依赖书写顺序（§6） |
| D | 联合下完全不物化，全部交给校验 | `.env` 的 `"8080"` 无法通过 `type: integer`，等于废掉 `.env`+schema 的组合 |

## 非目标（Non-Goals）

1. **不做自动类型推断**：除 schema / inline `| type` / 来源自带类型外，一律保持字符串。
2. **不改变 JSON/YAML 解码语义**：物化只作用于字符串拼写；解码侧的 NaN/±Inf 拒绝、精确整数规则见 §3/§5。
3. **不解析外部或远程 `$ref`**（不联网、不新增依赖）；本地 `$ref` 的 default 见 §9 G12。
4. **不把 `required`/`default`/`enum` 等约束搬进模板期**：约束由校验阶段（`jsonschema/v6`）裁决（SI5）。
5. **不为 `array<array<…>>` 增加语法**：已决为两侧统一拒绝（G13）。

## 12. 环境变量暴露与优先级（污染防护）

- **优先级**：`.env` 进根、进程环境只在 `@process`，两者不同格，因此不存在"谁压谁"；完整来源顺序见
  [`04-cli.md` §6.2](04-cli.md)。契约由 `internal/render/layering_test.go::TestBuildValuesLayerOrder`
  与 `TestProcessNamespaceReadsHostEnvironment` 覆盖，e2e fixture 为 `dotenv_beats_values`、
  `env_file_wins`、`no_env_fallback`、`process_namespace`。
- **暴露范围**：`tree.Env` 仍是 `os.Environ()` 的快照（受前缀白名单约束），但**只有** `${@process.NAME}` 能读到它；裸 `${NAME}` 恒读根，因此宿主环境无法再改变一个根的取值。`[FACT]`（`06-15` §3.1 D6）
- **名字撞车**：没有路径推导，也没有"简单名回退环境"，所以"别人的 `APP_HOSTS` 影响我的 `app.hosts`"
  与"宿主 `NAME` 覆盖我的 `${NAME}`"这两类意外都不存在。`[FACT]`
- **限制进程环境的开关（语义已澄清）**：`--no-process-env` 让 `@process` 为空；`--process-env-prefix PREFIX` 只把带该前缀的**进程**变量放进 `@process`（白名单，保留原名）；两者互斥（usage，exit 2）。**`.env` 文件完全不受这两个开关影响**（它进根）。`[FACT]`
- **`--process-env-prefix` 的精确语义**：它只是**进程环境变量名的白名单闸门**，**不拼接、不剥离**前缀——`APP_PORT` 以原名 `APP_PORT` 入表，模板用 `${@process.APP_PORT}` 精确读取；`--process-env-prefix APP_` 不会让顶层属性 `port` 或 `${@process.port}` 命中 `APP_PORT`。`[FACT]`（fixture `env_prefix`、`env_prefix_positive`）

## 13. 环境变量映射（需求讨论，待评审决断）

> **前置约定**：根是唯一配置面（`.env` 写入根），进程环境仅经 `${@process.NAME}` 读取。
> 因此下表里的 `env.NAME` 一律读作 **`@process.NAME`**。本节讨论的是**显式映射**
> （把 `APP_PORT` 写回 `app.port`），与命名空间归属正交，属独立后续议题。`[FACT]`

### 现状：环境变量进入值树只有三条路

| 方式 | 规则 | 例子 | 歧义 |
|------|------|------|------|
| 顶层同名 | **精确同名** | schema 属性 `PORT` ← env `PORT` | 无（但要求两处名字一致） |
| `env.NAME` | 显式命名空间 | `${env.PORT}` | 无 |
| **嵌套路径推导** | ≥2 段：点 → 下划线 + **全大写**（`values.EnvName`） | `app.port` ← `APP_PORT` | **有** |

推导只在第三条路生效，且仅在根值缺失时。它带来三个问题：

1. **多对多无法表达**：同一个 env 名 `PORT` 在不同项目里可能对应 `app.port`、`mongo.port`、
   `service.port`；同一个路径也可能希望读不同 env 名（多环境/多租户）。
2. **自定义名无处安放**：`app_port`、`mongo_port`、`MY_SERVICE_PORT` 这类命名与本规则无关。
3. **意外命中**：与推导名相同的无关进程变量（别的工具设的 `APP_PORT`）会被当成 `app.port`。

**结论：`路径 → env 名` 只能作为便利约定，不能作为契约。** 最初的功能目标其实是
"环境变量映射到配置文件"，而这需要**显式映射**。

### 候选方案

| 方案 | 做法 | 优点 | 代价 |
|------|------|------|------|
| **A（推荐）** | **显式映射**：`--env-map ENV=path`（可重复）或 `--env-map-file map.yaml`（`ENV: path`）；只读取映射到的名字；`env.NAME` 保留 | 零歧义、支持任意自定义名、天然防污染（白名单即映射表）；与 G14 的物化/校验自然衔接 | 新选项 + 文档 + fixture；自动便利消失 |
| B | 推导降级为**可选回退**（默认关，`--env-fallback` 打开） | 兼容现状；默认可预测 | 两套规则并存，歧义仍在（只是默认收敛） |
| C | 只保留 `env.NAME` + 顶层同名，**删除推导** | 最简、零歧义 | 破坏现有 `.env` + 嵌套 schema 用法（G14 的意义消失） |
| D | 前缀 + **剥离**（`--process-env-prefix APP_` 下 `APP_PORT` → `PORT`） | 名字更短 | 与点号路径叠加仍二义（`app.port` vs `app_port`），只是把问题搬家 |

### 待决点（请评审决定）

1. 主契约选 **A / B / C**？（D 单独或与 A 组合？）
2. 映射写在哪里：CLI `--env-map`、映射文件（便于版本化）、还是两者都支持？
3. **优先级次序**：显式映射 / `.env` 文件 / `env.NAME` / 顶层同名 / 推导回退 谁先谁后？
4. 映射目标路径在 schema/值树中不存在（拼错）→ 报错（推荐）还是告警？
5. `env.` 显式命名空间是否保留？
6. 若保留推导：默认开还是关？
7. 若选 A，`--process-env-prefix` 的定位是否简化为"仅诊断/审计"（白名单由映射表承担）？

> 未决断前不改动实现；现有 `--no-process-env` / `--process-env-prefix` 与推导规则保持现状（有测试与文档）。

### 设计溯源：这条推导当初为什么存在

| 出处 | 原文 / 含义 |
|------|-------------|
| `03-design/03-02-value-model-and-template-syntax.md:26` | 明确写成**兼容回退**：`${NAME}` 在根命名空间未找到且是简单标识符时回退环境变量 |
| `03-design/03-03-values-merge-and-schema.md:51` | 操作系统环境变量**默认不自动合并**进根命名空间，只通过 `${env.*}` 或简单名回退读取 |
| `internal/values/path.go:239`、`tree.go:115` | 代码注释：根优先 → 直接 `env.NAME` → conventional SCREAMING_SNAKE_CASE 回退；点号路径映射为大写下划线 |
| `00-raw/00-04-priority-and-json-schema.md:87` | 扩展引擎已支持 `${app.port}`、`env.PORT` ⇒ 属要保留的既有能力 |
| `03-design/03-06-open-questions-and-review.md:38` | **一直被列为待定问题**：环境变量是否仅通过 `env.*` 与回退读取 |

动机归纳（三条，本质都是降低上手成本）：

1. **零值文件可用**：只有 `.env` / 环境变量的项目也能渲染 `${app.port}`，不必先写 YAML 值文件；
2. **少写前缀**：`${PORT}` 这类简单名不必处处写成 `${env.PORT}`；
3. **顺应惯例**：环境变量惯用 SCREAMING_SNAKE_CASE，于是把路径机械映射为大写加下划线。

代价：把**便利回退**当成了**映射契约** ⇒ 多对多歧义（`PORT` → `app.port` / `mongo.port`）、
自定义名无法表达（`app_port`）、无关变量意外命中（污染）。

**结论**：§13 的方案选择实际上是在回答 `03-06` 那个悬而未决的问题——建议以显式映射为契约（方案 A），
推导降级为可选的兼容回退（方案 B 的开关），并在 `03-06` 标注该问题已由本文回答。

### 评审决定（不保留推导）与实测事实

**决定**：**不保留 `路径 → env 名` 推导**（它属隐式逻辑，容易造成误判）。环境的读取只保留：
显式 `${env.NAME}`、`.env` 文件（按原样键名）、进程环境（受 `--no-process-env` / `--process-env-prefix` 约束）。
**优先级（已修复）**：`--set` > **YAML/JSON 值文件** > `.env` > 进程环境 > schema `default`（定稿见 [`04-cli.md` §6.2](04-cli.md)；环境命名空间内部见 §12）。

实测（`.tmp` 下临时文件，CLI 冒烟）：

| 场景 | 模板 | 结果 |
|------|------|------|
| `.env` 定义 `NAME=world`，无进程环境参与 | `${NAME}` | **`world`**（`.env` 写入根） |
| 同上 + `--process-env-prefix OTHER_` | `${NAME}` | **`world`** ⇒ prefix 只过滤**进程环境**，不影响 `.env` |
| 同上 + `--no-process-env` | `${@process.NAME}` | 缺失（`--on-missing` 决定） |
| 进程环境 `NAME=world` | `${@process.NAME}` | **`world`** |
| 进程环境 `APP_PORT=9090` | `${app.port:-none}` | **`none`** ⇒ 无路径推导，无关变量不会被吃 |
| `--set K=from_set` + `.env K=from_env_file` + 进程 `K=from_process` | `${K}` | **`from_set`**（最高层） |
| `.env K=from_env_file` + 进程 `K=from_process` | `${K}` 得 `from_env_file`，`${@process.K}` 得 `from_process` | fixture `env_file_wins` 钉住两者互不干扰 |

### 移除推导的实施清单（✅ 已执行）

| 位置 | 现状 | 移除后 |
|------|------|--------|
| `internal/values/path.go` | `envName()` 推导 | ✅ 仅保留单段精确名（点号/索引路径返回 false）；`EnvName` 保留但语义收窄 |
| `internal/values/tree.go` | 解析只读根；环境由 `@process` 显式读取 | ✅ 见 §12 |
| `internal/schema/materialize.go` | `envFallback` 走 `values.EnvName` | ✅ 自动收窄为**顶层精确同名**（无需改动） |
| 测试 | `env_name_test.go`、`g14_env_fallback_test.go`、`union_materialize_test.go`、engine fixture | ✅ 已改写为精确名；`TestEnvName` 换成 `TestEnvKey`/`TestFirstKey`，`TestMaterializeTopLevelEnvFallback` 换成 `TestMaterializeConvertsRootStrings` + `TestProcessNamespaceIsNotAMaterializationSource`，`TestNoPathDerivationForNestedPaths` 反向钉住不推导；engine fixture 的 testdata `env.yaml` 全部改用 `${@process.…}` |
| 文档 | `03-02:26`、`03-03:51`、`03-06:38`、本文件 §6/§13、`04-cli` 相关表述 | 同步为"无推导；环境读取三条路 + `env.` 显式" |

**（已澄清）**：不存在"精确回退"这一独立概念——环境读取就是分层覆盖（见 §14）。

### `.env` 与进程环境的读法

| 项 | 内容 |
|----|------|
| 结论 | `.env` 是根层、进程环境是独立命名空间，两者不互相覆盖 |
| 评审期望 | `--set` > **`.env`** > 进程环境（`.env` 是项目内的显式配置，应当压过宿主的进程环境） |
| 代码位置 | `internal/render/render.go` 的 `tree.Env = mergeEnv(loaded.Env, tree.Env)` —— 进程环境是后写入方，胜出 |
| 修复方式 | 反转合并顺序（`.env` 后写入，即 `mergeEnv(tree.Env, loaded.Env)` 语义），并补 e2e fixture：`.env K=from_env_file` + 进程 `K=from_process` → 期望 `from_env_file` |
| 影响面 | 仅影响同一个键同时出现在 `.env` 与进程环境的场景；`--set` 仍最高优先 |

### 待决断：`env` 命名空间的来源可区分性

评审指出的原始困扰（**已确认成立**）：`.env` 与进程环境共用 `env` 命名空间，用户系统里本就有
`NAME`，临时新建 `.env` 改 `NAME` 时，`${env.NAME}` 到底读的是哪一个**无从判断**。优先级修复后
（`.env` 胜）**取到的值是确定的**，但"这个值来自 `.env` 还是宿主环境"仍不可见。

| 候选 | 做法 | 权衡 |
|------|------|------|
| A | 保持单一 `env` 命名空间 + 让来源可查（诊断/`--explain` 显示胜出来源：文件名+行号或 process） | 不破坏模板；需新增诊断入口 |
| B | 拆命名空间：`dotenv.NAME`（仅 `.env`）、`process.NAME`（仅进程环境），`env.NAME` 保留为"生效值" | 语义最清楚；新增两个命名空间与文档 |
| C | 仅在被遮蔽时告警（同一键两来源都存在 → 日志/警告一行） | 改动小；仍是隐式，只是可见 |

需要决定：选 A / B / C，或都不做（仅依赖 §12 的优先级）。

## 14. 两个命名空间

- **根命名空间**：`--set` / `--set-json` > `--dotenv-file` > `-f/--values-file` > schema `default`
  （完整表见 [`04-cli.md` §6.2](04-cli.md)）。裸 `${NAME}` 只读这里，根里没有就是缺失，
  不存在回退到环境的路径。
- **`@process` 命名空间**：进程环境的只读快照，只经 `${@process.NAME}` 读取，永远是字符串，
  不参与层序，也不会被裸 `${NAME}` 命中。
- 读法对应：`${NAME}` / `${app.port}` / `${["a.b"]}` 读根；`${@process.NAME}` 读进程环境；
  `${env.NAME}` 是根对象里名为 `env` 的普通路径。

## 15. `--explain` 规格（已实现）

**目标**：让"这个值到底从哪来"可判定，尤其是 `.env` 与进程环境同名、或多个 `--env-file` 叠加时。

### 选项与输出通道

| 项 | 规定 |
|----|------|
| 选项 | `--explain[=PATH]`：裸 `--explain` 报告全部已引用变量；`--explain=PATH` 只报告该路径（可重复） |
| 通道 | **stderr**（stdout 仍是纯渲染结果，逐字节不变） |
| 格式 | **JSONL**（每变量一行 JSON 对象），无其它格式 |
| 退出码 | 不受影响（渲染照常执行；`--explain` 不改变任何取值行为） |
| 与 `--validate-only` | 允许并用（先解释后校验，顺序固定） |
| 与渲染的先后 | 报告在渲染循环**之前**写出：模板文本只读一次，避免 stdin/大文件被读两遍（`06-review/06-17` D2）。代价是模板赋值写回的值在本次报告里看不到（未决项，见下） |
| 值 | **不输出值**（值可能是密钥），只输出来源 |
| `--explain-out FILE` | **不提供**（2026-10-10 已决）：固定写 stderr，落盘用 shell 重定向 `2> explain.jsonl`；也不加 table/YAML 等格式 |

`PATH` 是**过滤器**，不是求值目标：只在**模板已引用的名字**里筛选。比较前两侧都做**规范写法归一**
，
然后**按字节精确相等**（不做前缀、不做 glob，大小写敏感）。
命中 0 个变量时静默输出空、exit 0（stderr 是纯 JSONL 通道，不混警告）。

> pflag 的可选值 flag 只在 `=` 形式下取值：路径必须写 `--explain=app.port`；
> `--explain app.port`（空格）不会消费 `app.port`，它会被当成模板路径。

### 行格式（JSONL，稳定、可做 golden）

每条一行，字段顺序固定。模型与理由见
[ADR-005](../04-decisions/04-05-adr-005-explain-origin-model.md)：`kind` 记**层位**，
`method` 记**入口写法**，`source` 是该 kind 下的**地址**，`file`/`line` 是**物理位置**。

```json
{"template":"a.tmpl","variable":"K","origin":{"kind":"envfile","method":"file","source":".env","file":".env","line":1},"shadowed":[{"kind":"schema","method":"default","source":"#/properties/K/default","file":"schema.json","line":0}]}
{"template":"a.tmpl","variable":"@process.PORT","origin":{"kind":"process","method":"env","source":"PORT","file":"","line":0},"shadowed":[]}
```

| 字段 | 含义 |
|------|------|
| `template` | 模板路径（stdin 为 `-`）；**单模板也输出** |
| `variable` | 模板里引用的路径的**规范拼写** |
| `origin` | **胜出帧**：链尾（最高优先级） |
| `shadowed` | 被胜出者覆盖的帧，**按优先级从高到低**；**恒输出**，无覆盖为 `[]` |

帧的五个字段**恒在**（空值 `""`/`0`，不用 `omitempty`）：

| 字段 | 含义 |
|------|------|
| `kind` | 层位：`schema` / `values` / `envfile` / `set` / `process` / `assign` / `missing` |
| `method` | 入口写法：`default` / `file` / `flag` / `json` / `env` / `assign` / `missing` |
| `source` | 地址：文件路径 · JSON 指针（`#/properties/K/default`）· 环境变量名 · 键名 · 模板路径 |
| `file` | 物理位置所在文件；无文件来源为空 |
| `line` | 1 基行号；不可得为 `0` |

各 kind 的取值表：

| kind | method | source | file / line |
|------|--------|--------|-------------|
| `schema` | `default` | JSON 指针 | schema 文件 / 0 |
| `values` | `file` | 文件路径（yaml/json 由扩展名区分） | 该文件 / 0 |
| `envfile` | `file` | 文件路径 | 该文件 / 有 |
| `set` | `flag` \| `json` \| `file` | 键名 | `--set-file` 时=被读文件，否则空 |
| `process` | `env` | 环境变量名 | 空 / 0 |
| `assign` | `assign` | 模板路径 | 模板文件 / 有 |
| `missing` | `missing` | 空 | 空 / 0 |

- 顺序：**按模板中首次出现顺序**去重输出（确定性，不排序）；
- 链**不截断**，且包含**被覆盖的 schema `default`**（`kind:"schema"` 帧在链尾）；
- 多个 `.env`/多个 `-f` 时**必须体现"后覆盖前"**（按参数数组顺序）。

**当前限制**：`assign` 帧的元数据会写进值树，但报告在渲染循环**之前**写出，因此
`${K:=v}` 写回的值在本次报告里仍显示为 `missing`（见 §16 未决）。

### 不变式（e2e 断言，见 `test/e2e` 的 `TestExplainFramesFollowADR005`）

| # | 不变式 |
|---|--------|
| I1 | `kind` 属于闭集；`shadowed` 每帧同 |
| I2 | `method` 属于该 kind 的允许集合 |
| I3 | `source` 只表达地址：`values`/`envfile` 帧等于 `file`；`schema` 为指针、`process` 为变量名、`set` 为键名 |
| I4 | 五个字段恒在，不得 `omitempty` |
| I5 | `origin` 是链尾，`shadowed` 近者在前，二者不重复同一帧 |
| I6 | 被覆盖的 schema `default` 必须出现在链里 |
| I7 | 行信封只有 `template`、`variable`、`origin`、`shadowed`，且不输出值 |

### 数据来源（实现依据）

| 来源 | 记录方式 |
|------|----------|
| 值文件 | 加载器上报 `kind=values, method=file, source=路径` |
| `.env` | `kind=envfile, method=file`，行号来自 dotenv 解析 |
| `--set` / `--set-json` / `--set-file` | `kind=set`，`method` 分别为 `flag`/`json`/`file`；`--set-file` 另记被读文件 |
| schema `default` | **先于其它层**播种 `kind=schema, method=default, source=JSON 指针`，因此被覆盖时仍在链尾 |
| 进程环境 | `kind=process, method=env, source=环境变量名`（只记名字，不记值） |
| 模板赋值 | 写回时记录 `kind=set, method=assign, source=模板路径`（链上可见需报告改到渲染后，见上文限制） |

### 实现进度

| 部分 | 状态 |
|------|------|
| 引擎侧变量枚举 | ✅ `engine.PathEngine` 接口 + `extended.Engine.Paths(src)`：按**首次出现顺序**去重，表达式经 `parseExpression` 归约为路径（`${A \| toJson}` → `A`、`${B:-x}` → `B`）；测试 `TestPaths`（7 例，含 `env.NAME`、点号/索引、`$${A}` 转义、无变量） |
| 渲染层输出 | ✅ `render.Request.Explain{Enabled, Paths, Writer}`（`Explain` 组，Writer 默认 stderr）；`buildValues` 在加载各来源时记录每名的来源栈（`.env`（参数顺序，后覆盖前）→ 值文件 → `--set`；schema `default` 只在无任何来源时兜底，见 G21），`explainTemplate` 编码 JSONL 并做精确路径过滤；单测 `TestExplainTemplateWritesJSONL`/`TestExplainTemplateFiltersByExactPath`/`TestExplainerSource`/`TestMetaOrigin` |
| CLI 选项 | ✅ `--explain[=PATH]`（pflag 可选值，裸 `--explain` 用 `*` 哨兵表示全部）；CLI 只解析，`internal/cli` 的 `renderFlags.request()` 装配成 `render.Request`，校验统一在 `Request.Validate()` |
| e2e fixture | ✅ `explain_env_file`、`explain_multi_env`、`explain_set_wins`、`explain_missing`、`explain_batch`（多模板同构）、`explain_path`（`--explain=PATH` 过滤）、`explain_space_form`（空格形态 → exit 5）；`TestExplainStdoutUnchanged` 对拍加/不加 `--explain` 的 stdout。`shadowed` 不再含 `process`（见下） |

**实现备注**

- `--explain` 需要模板文本才能枚举引用名，因此开启时改为**内存渲染**（模板只读一次，stdin 不可回绕）；不开启时保持原有流式路径。
- 报告写出时机：普通渲染在**渲染成功后**写出；与 `--validate-only` 并用时按固定顺序**先解释后校验**。
- 遮蔽顺序：**从高到低**（最近的被覆盖者在前），与根层序（`--set`/`--set-json` > `--dotenv-file` > `-f/--values-file` > schema `default`，见 [`04-cli.md` §6.2](04-cli.md)）一致。`@process` 不参与该链。
- 可省略的 `line` 用 `*int` + `omitempty`：两后端对 `omitempty` 数值 0 的处理不同（`encoding/json/v2` 保留、`sonic` 省略，见 `09-gaps.md` G20），指针形式才能让 JSON 在两种构建下一致。
- 模板赋值（`:=`/`=`）现在会把模板与行号写入值树元数据；但报告在渲染循环**之前**写出，因此本次报告仍把该名字报为 `missing`。把报告移到渲染之后（并让 `--explain` 选择缓冲路径）是**未决项**。

### 测试要求（已落地）

- ✅ 七个 fixture（`explain_env_file`/`explain_multi_env`/`explain_set_wins`/`explain_missing`/
  `explain_batch`/`explain_path`/`explain_space_form`）覆盖来源、批量、过滤与空格形态；
- ✅ `schema_default_is_fallback`：`K` 的链为 `envfile → schema`，`D` 只有 `schema`；
- ✅ `TestExplainFramesFollowADR005`（e2e，真实二进制）：断言 I1–I7——闭集、`values`/`envfile`
  的 `source` 等于 `file`、链序、被覆盖的 default 在链尾；
- ✅ 单元测试：`internal/render/explain_test.go`（JSONL 帧字段、精确匹配、来源栈优先级、meta 回退）；
- ✅ 回归：`TestExplainStdoutUnchanged` 对拍加/不加 `--explain` 的 stdout **逐字节一致**；
- ✅ 示例：`just --justfile examples/justfile explain-sources` 渲染后由 `tools/digest --sources`
  汇总，`string_prec_multi_layer` 演示 **4 层链**（default → values → values → .env）。

> **实现状态**：`shadowed` 现在包含全部被覆盖层（含 schema `default`）；`${@process.K}` 的
> `variable` 报 `@process.K`、`origin` 为 `process/env` 且 `source` 为环境变量名；`variable` 采用
> **规范拼写**，可直接粘回模板或用作 `--explain=PATH`。复现：`go test -count=1 ./test/e2e/`。
