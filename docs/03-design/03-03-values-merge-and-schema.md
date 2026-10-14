---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# Extended 设计 · 03 值来源、多文件与 Schema

> 本文是 `03-00-index.md` 的拆分部分。

### 3.4 类型来源与转换

- 环境变量、`.env` 文件的原始值均为 string。`[INFERENCE]`
- YAML/JSON values 文件保留原生类型。`[INFERENCE]`
- CLI `--set` 默认按 schema 做类型解析；`--set-string` 强制字符串；`--set-json` 传入 JSON 原生值。`[INFERENCE]`
- 类型解析规则建议：
    - `string`：原样。
    - `int`：仅接受可选符号加数字字符，拒绝空白和浮点。`[ASSUMPTION]` 验证：确认是否允许 `" 1 "`；owner：技术负责人。
    - `float`：十进制浮点数，解析规则与 Go `strconv.ParseFloat` 对齐。`[INFERENCE]`
    - `bool`：只接受 `true` / `false`；不接受 `1/0`、`yes/no`。`[FACT]`（来源：需求提出者确认，2026-09-29）
    - `array` / `object`：来自 string 时必须为合法 JSON；`.env` 中数组使用 JSON 字符串，例如 `HOSTS='["a","b"]'`。`[FACT]`
      （来源：需求提出者确认允许 JSON 数组，2026-09-29）
- `.env` / 环境变量 JSON 数组规则（已确认）：
    1. `.env` 解析后默认所有值仍是 string。
    2. 只有当 schema 声明该 key 为 `array` 或 `object` 时，才对该 string 做 JSON 解析。
    3. JSON 值必须单行、合法；推荐 `.env` 用单引号包裹、内部用双引号 JSON：`HOSTS='["a","b"]'`。
    4. JSON 解析失败按类型错误处理，指出文件、行、key、原始值。
    5. 如果 schema 声明为 `string`，则 `HOSTS='["a","b"]'` 仍是字符串，不自动解析为数组。
    6. 未声明 schema 时，`.env` 不自动推断数组；如需强类型，用 YAML/JSON values 或 `--set-json`。
       `[FACT]`（来源：需求提出者逐条确认，2026-09-29）
- 结构化 values 中出现类型不匹配时，默认直接报错，不做隐式转换。`[INFERENCE]`

### 3.5 多文件加载与覆盖

已确认规则：

- 后加载文件覆盖先加载文件。`[FACT]`（来源：需求提出者确认）
- `object` 递归深合并。`[INFERENCE]`
- `array` 默认整体替换，不做追加。`[FACT]`（来源：需求提出者确认）
- 标量/null 直接覆盖。`[INFERENCE]`
- CLI `--set*` 优先级最高。`[FACT]`（来源：需求提出者确认）

建议默认加载顺序（从低到高）：
****

1. schema 中的 `default`（**兜底**：只在该 key 于根命名空间缺失时生效）。`[FACT]`
2. 结构化 values 文件（YAML/JSON），按命令行顺序；后加载覆盖先加载。`[FACT]`
3. `.env` 文件（`--dotenv-file`），按命令行顺序；写入**根命名空间**，压过 values 文件（局部层压共享基线）。`[FACT]`
4. CLI `--set`、`--set-json`，按组顺序（`--set` → `--set-json`）。`[FACT]`

原则：**显式指定 > 隐式包含**；`default` 只是 schema 兜底，不压过任何实际来源。`[FACT]`（来源：需求提出者确认）

进程环境**不在上述层序内**：它不再参与 `${NAME}` 解析，只能经 `${@process.NAME}` 显式读取，
并受 `--no-process-env` / `--process-env-prefix` 约束（无 `--merge-env`）。`[FACT]`（来源：
`07-serialization/13-materialization.md` §13/§14——不保留路径推导，环境读取为分层覆盖；2026-10-10 定稿）

### 3.6 Schema 与类型校验

> 复用决策见 `docs/04-decisions/04-03-adr-003-json-schema-reuse.md`；可行性调研见
> `docs/reference/REF-03-json-schema-reuse-research.md`。

已确认方案：

- 直接复用 **JSON Schema Draft 2020-12**，不自研 schema 语法。`[FACT]`（来源：ADR-003）
- schema 文件支持 `.json`、`.yaml`、`.yml`，先解析为 JSON 兼容数据树。`[ASSUMPTION]`
- 校验引擎使用 `github.com/santhosh-tekuri/jsonschema/v6`。`[FACT]`（来源：ADR-003）
- 薄适配层负责：YAML/JSON 加载、类型别名归一化、`default` 应用、env/.env 字符串物化、`x-*` 扩展读取、未知关键字策略、远程
  `$ref` 禁用。`[INFERENCE]`
- 类型别名（内部归一化为 JSON Schema 标准类型）：
    - `int` -> `integer`
    - `float` -> `number`
    - `bool` -> `boolean`
    - `array<T>` -> `{ type: array, items: { type: T } }`
    - `object` -> `{ type: object }`
    - `null` -> `null`
    - `string` -> `string`
- `default` 在 JSON Schema 中只是注解，由薄适配层在合并 values 后、校验前主动应用。`[FACT]`（来源：
  `docs/reference/REF-03-json-schema-reuse-research.md`）
- 未知非 `x-*` 关键字默认 **不报错**，以便 schema 复用到其它系统；可选 `--unknown-keywords=warn|error` 提高严格度。`[FACT]`
  （来源：需求提出者确认）
- `x-sensitive` 等 `x-*` 扩展由薄适配层读取，不修改标准 JSON Schema 语义。`[INFERENCE]`
- 远程 `$ref` 默认禁用，只允许本地文件或内联 `$defs`。`[INFERENCE]`

标准 JSON Schema 示例：

```yaml
$schema: https://json-schema.org/draft/2020-12/schema
type: object
properties:
  app:
    type: object
    required: [port]
    properties:
      port:
        type: integer
        default: 8080
        minimum: 1
        maximum: 65535
        x-sensitive: false
      ratio:
        type: number
        default: 0.5
        minimum: 0.0
        maximum: 1.0
      hosts:
        type: array
        items:
          type: string
        default: []
        minItems: 1
      debug:
        type: boolean
        default: false
```

#### 3.6.1 快速类型声明

建议同时支持两种声明方式：

1. **schema 文件（推荐）**：适合生产、复用、默认值、required 与复杂约束。示例：
   ```yaml
   $schema: https://json-schema.org/draft/2020-12/schema
   type: object
   properties:
     app:
       type: object
       required: [port]
       properties:
         port:
           type: integer
           default: 8080
         hosts:
           type: array
           items:
             type: string
           default: []
         debug:
           type: boolean
           default: false
   ```
   `[INFERENCE]`

2. **模板内联 `| type`（快速）**：适合少量变量或临时模板。
    - `${PORT | type int}`
    - `${HOSTS | type array<string> | toJson}`
    - `${DEBUG | type bool}`
      `[FACT]`（来源：需求提出者确认使用内联 `| type`）

内联 `| type` 语义：

- 先按 `--on-missing` 策略解析 path。
- 如果来源是 string，按目标类型规则解析。
- 如果已是结构化类型且与目标类型不兼容，直接报错。
- `type` 不产生新值，只做类型断言/转换，输出原值给后续 formatter。
- `type` 不提供 default/required/schema 约束；复杂校验仍由 schema 文件负责。
- 同时使用 schema 文件和内联 `| type` 时，二者取并集，值必须同时满足；内联不得放宽 schema。`[FACT]`（来源：需求提出者确认）

#### 3.6.2 内联 `| type` 与 schema 的优先级

已确认原则： **schema 是全局契约，内联 `| type` 是局部断言；schema 与内联取并集，schema 的
required/default/约束优先，交集冲突报错。**`[FACT]`（来源：需求提出者确认，2026-09-29）

优先级规则：

- schema 无类型 + 无内联 `| type`：不强制类型，按原始值处理。
- schema 无类型 + 内联 `T`：内联类型作为该插值的类型依据，用于 string 解析与类型校验。
- schema 有类型 `S` + 无内联 `| type`：使用 schema 类型 `S`。
- schema 有类型 `S` + 内联 `T` 且兼容：二者取并集，值必须同时满足；schema 的 required/default/约束优先级更高，内联只做类型断言/补类型。
- schema 有类型 `S` + 内联 `T` 且交集冲突：直接报错，不静默覆盖。
- 同一个插值出现多个 `type`：直接报错。

兼容性规则：

- `T == S`：通过。
- 标量类型之间不做隐式兼容；`int` 与 `float` 视为不同类型，`0` 与 `0.0` 也视为不同类型。`[FACT]`（来源：需求提出者确认，2026-09-29）
- `array<T>` 与 `array<U>` 要求 `T == U`；`object` 视为 object 的宽类型，具体 properties 由 schema 约束。`[INFERENCE]`
- schema 的 required/default/约束优先级高于内联 `| type`；内联不能补足 required，也不能覆盖 default。`[INFERENCE]`
- 特别地，为了方便使用，`int` 与 `float` 均支持零值填充；若零值场景出现 `int`/`float` 冲突或歧义，以 `0` 为准。`[FACT]`
  （来源：需求提出者确认，2026-09-29；仅限零值/歧义场景，普通类型不兼容仍报错）

处理顺序建议：

1. 解析 path。
2. 模板显式操作符 `${path:-default}` 等。
3. schema `default`。
4. schema `required` 校验：required 缺失直接报错，不自动填零。
5. `--on-missing` 策略：`error` / `empty` / `keep` / `zero`。
6. 确定类型：优先 schema 类型；若 schema 无类型则用内联 `| type`。
7. string 按类型解析；检查 schema 与内联类型兼容性。
8. schema 约束校验（enum、range、pattern 等）。
9. formatter pipeline 依次执行，最后输出。

补充限制：

- 如果提供 `| type`，它必须紧跟 path，作为 pipeline 的第一个操作符；出现在其他 formatter 之后时直接报错。`| type` 可以缺省，不写时按
  schema 或无类型策略处理。`[FACT]`（来源：需求提出者确认，2026-09-29）
- `${path | type T | toJson}` 合法；`${path | toJson | type T}` 非法。`[INFERENCE]`

#### 3.6.3 Schema 约束能力

直接复用 JSON Schema 2020-12 的约束关键字。`[FACT]`（来源：ADR-003）

P0 建议覆盖：

- 通用：`enum`、`const`、`default`、`required`。
- `integer` / `number`：`minimum`、`maximum`。
- `string`：`minLength`、`maxLength`、`pattern`。
- `array`：`items`、`minItems`、`maxItems`。
- `object`：`properties`、`additionalProperties`、`required`。

P1 增强：

- `exclusiveMinimum`、`exclusiveMaximum`、`multipleOf`。
- `uniqueItems`。
- `minProperties`、`maxProperties`。
- `format` 断言：`hostname`、`ipv4`、`uri`、`duration` 等。
- `allOf`、`anyOf`、`oneOf`、`not`、`if/then/else`、`$ref`、`$defs` 的深入支持。`[TODO]` owner：技术负责人。

示例：

```yaml
$schema: https://json-schema.org/draft/2020-12/schema
type: object
properties:
  app:
    type: object
    required: [port]
    properties:
      port:
        type: integer
        default: 8080
        minimum: 1
        maximum: 65535
      mode:
        type: string
        enum: [dev, staging, prod]
      name:
        type: string
        minLength: 3
        maxLength: 63
        pattern: "^[a-z][a-z0-9-]*$"
      retries:
        type: integer
        default: 3
        minimum: 0
        maximum: 10
      hosts:
        type: array
        items:
          type: string
        minItems: 1
        maxItems: 10
```

校验顺序：

1. 类型解析/断言。
2. `enum` / `const`。
3. 数值范围、字符串长度/正则、数组长度等约束。
4. 对象 required/properties。
5. 全部通过后再渲染。

错误信息应包含：值路径、约束名、期望、实际、来源文件与行号（可获取时）。`[INFERENCE]`

校验流程：

1. 加载并合并全部值来源。`[INFERENCE]`
2. 按 schema 填充缺失键的默认值。`[INFERENCE]`
3. 对来自 string 的值按 schema 做解析。`[INFERENCE]`
4. 执行类型、必填、枚举、范围等校验。`[INFERENCE]`
5. 校验失败：报错退出，不渲染输出。`[INFERENCE]`
6. 校验通过：开始模板渲染。`[INFERENCE]`

## 非目标（本部分）

- 不替代整体设计评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `03-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
