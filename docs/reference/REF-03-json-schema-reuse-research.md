---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# JSON Schema 复用可行性调研

> 目的：回答 schema 能否直接复用 JSON Schema、YAML 能否承载、成熟 Go 校验库能否满足需求，以及未实现关键字的处理策略。  
> 调研日期：2026-09-29。来源见文末。

## 1. 结论摘要

- `[INFERENCE]` 建议直接复用 **JSON Schema Draft 2020-12**，不再自研一套 schema 语法。
- `[FACT]` JSON Schema 定义在 JSON 数据模型上；YAML 可以作为承载格式，但需要先归一化为 JSON 兼容类型。
- `[FACT]` 最成熟的 Go 候选是 `github.com/santhosh-tekuri/jsonschema/v6`，完整支持 Draft 2020-12
  测试套件；但默认不会为未知关键字报错，也不会自动应用 `default`。
- `[FACT]` `github.com/kaptinlin/jsonschema` 支持默认值应用与 `Extra` 扩展保留，但版本较新，需评估生产成熟度。
- `[FACT]` JSON Schema 2020-12 规范要求实现 MUST refuse unknown/unprocessed keywords；但当前 Go 库通常忽略或剥离未知关键字。
- `[FACT]` 需求提出者确认：未知非 `x-*` 关键字默认不报错，因为用户可能复用 schema 文件到其它系统；可选严格模式再报错。
- `[INFERENCE]` 因此建议：用成熟校验库做核心校验，外面加一层 schema 加载/归一化/未知关键字策略/默认值应用/敏感标记读取层。

## 2. 问题 1：JSON Schema 2020-12 与 YAML/JSON

- `[FACT]` JSON Schema 的标准 dialect URI 为 `https://json-schema.org/draft/2020-12/schema`。
- `[FACT]` JSON Schema 本身是 JSON 文档；规范基于 JSON 数据模型，不要求 schema 文件必须以 `.json` 扩展名保存。
- `[FACT]` YAML 是 JSON 的超集/可承载格式之一；大量工具使用 JSON Schema 校验 YAML 数据，例如 Helm 的 `values.schema.json`。
- `[ASSUMPTION]` 本项目建议接受 `.json`、`.yaml`、`.yml` 三种 schema 文件扩展名，先解析为 JSON 兼容的 `any` 数据树，再交给
  JSON Schema 校验器。
- `[TODO]` YAML 归一化边界需要明确：
    - YAML 1.1 的 `yes/no/on/off`、日期、时间戳、binary tag、锚点/别名、重复 key 等。
    - 建议限制为 YAML 1.2 Core Schema + JSON 兼容类型；非 JSON 类型直接报错或显式转换。
    - owner：技术负责人。

## 3. 问题 3：成熟 Go JSON Schema 校验库能否满足需求

### 3.1 候选库对比

| 库                              | Draft 支持                | 关键能力                                                                         | 风险/限制                                                 |
|---------------------------------|---------------------------|----------------------------------------------------------------------------------|-----------------------------------------------------------|
| `santhosh-tekuri/jsonschema/v6` | 4、6、7、2019-09、2020-12 | 完整测试套件、format assertions、content assertions、自定义 vocabulary、错误层级 | 不自动应用 default；未知关键字不报错；YAML 解析需外部处理 |
| `kaptinlin/jsonschema`          | 4、6、7、2019-09、2020-12 | 默认值感知 `Unmarshal`、`Extra` 保留未知关键字、精确 JSON number、自定义 format  | 版本较新（v0.9.x），生态与生产验证不如 santhosh           |
| `qri-io/jsonschema`             | 2019-09（旧 draft 7）     | 自定义关键字、性能曾宣称较好                                                     | 不是 2020-12 主线；维护活跃度较低                         |
| `xeipuuv/gojsonschema`          | draft 4/6/7               | 使用广、示例多                                                                   | 不支持 2019-09 / 2020-12                                  |
| `invopop/jsonschema`            | 2020-12（生成）           | 从 Go struct 生成 JSON Schema                                                    | 不是校验器，不适合作为本需求核心                          |

### 3.2 本地验证结果

以下为 2026-09-29 本地原型测试结果：

- `[FACT]` `santhosh-tekuri/jsonschema/v6 v6.0.3` 可编译并校验 Draft 2020-12 schema，`integer`/`minimum`/`maximum`/
  `enum`/`required` 正常工作。
- `[FACT]` 该库遇到未知关键字 `foo`、`bar` 或自定义 `x-sensitive` 时不报错；schema 编译成功，未知关键字在验证中被忽略。
- `[FACT]` 该库不会自动应用 `default`；需要引擎自己填充默认值。
- `[FACT]` `kaptinlin/jsonschema v0.9.10` 可通过 `Schema.Unmarshal` 应用 `default`，例如 `{}` 被填充为
  `{name:"world", port:8080}`。
- `[FACT]` `kaptinlin/jsonschema` 默认会剥离未知关键字；设置 `PreserveExtra = true` 后，`x-sensitive`、`foo` 等会保留在
  `Schema.Extra`，但不会报错。
- `[FACT]` 两个库都支持 Draft 2020-12 的基础校验关键字；但“未知关键字必须报错”需要项目自己加策略层。

### 3.3 能否满足需求

`[INFERENCE]` 核心校验能力可以满足：

- 类型：`object`、`array`、`string`、`number`、`integer`、`boolean`、`null`。
- 约束：`required`、`enum`、`const`、`pattern`、`minLength`、`maxLength`、`minimum`、`maximum`、`exclusiveMinimum`、
  `exclusiveMaximum`、`multipleOf`、`minItems`、`maxItems`、`uniqueItems`、`properties`、`additionalProperties`、
  `dependentRequired` 等。
- 组合：`allOf`、`anyOf`、`oneOf`、`not`、`if/then/else`、`$ref`、`$defs`。

`[ASSUMPTION]` 需要项目补齐的能力：

- Go 风格别名：`int` -> `integer`、`float` -> `number`、`bool` -> `boolean`、`array<T>` -> `array + items`。
- `default` 应用逻辑。
- 未知关键字策略。
- `x-sensitive` 等扩展读取与脱敏。
- env/.env 字符串到 schema 类型的解析/物化。
- YAML -> JSON 兼容数据树归一化。
- 远程 `$ref` 默认禁用，防止网络依赖与 SSRF。
- 内联 `| type` 到 JSON Schema 类型断言的映射。

## 4. 问题 4：未实现/未知关键字如何处理

- `[FACT]` JSON Schema 2020-12 规范要求：实现 MUST refuse to evaluate schemas which contain keywords which they do not
  know how to process or explicitly choose not to process。
- `[FACT]` 但本地测试显示 `santhosh-tekuri/jsonschema/v6` 对未知 `foo`/`bar` 不报错；`kaptinlin/jsonschema` 默认剥离未知关键字，开启
  `PreserveExtra` 后也只是保留，不报错。
- `[INFERENCE]` 如果直接使用库默认行为，会出现“schema 写了未知关键字但校验通过”的假阴性/假阳性风险。
- `[FACT]` 需求提出者确认：未知非 `x-*` 关键字默认 **不报错**，以支持 schema 文件跨系统复用；可选
  `--unknown-keywords=warn|error` 提高严格度。
- `[INFERENCE]` 建议策略：
    1. 维护项目支持的 JSON Schema 关键字白名单。
    2. 遍历 schema，识别 schema 对象级别（不是 `properties` 的 key）。
    3. 未知 `x-*`：作为项目扩展读取，例如 `x-sensitive`，从校验器输入中移除或允许忽略。
    4. 未知非 `x-*`：默认忽略；`--unknown-keywords=warn` 记录警告；`--unknown-keywords=error` 直接报错。
    5. 默认值为 `ignore`。
- `[ASSUMPTION]` 若未来希望允许完整 JSON Schema 扩展生态，可研究自定义 vocabulary 与 `$vocabulary` 声明；一期不建议。
- `[TODO]` 需要确定“支持关键字白名单”的范围；owner：技术负责人。

## 5. 推荐方案

`[INFERENCE]` 建议：

1. **schema 格式**：直接使用 JSON Schema Draft 2020-12；文件可为 JSON 或 YAML。
2. **校验库**：优先 `santhosh-tekuri/jsonschema/v6`，成熟度与规范覆盖最好。
3. **适配层（自研，但很薄）**：
    - YAML/JSON 加载为通用 `any`。
    - Go 风格类型别名归一化。
    - `default` 应用。
    - 未知关键字策略：默认忽略非 `x-*`；可选 `warn` / `error`。
    - `x-sensitive` 扩展读取。
    - env/.env 字符串物化。
    - 远程 `$ref` 禁用/白名单。
4. **扩展策略**：项目特定能力只使用 `x-` 前缀，不修改标准关键字语义。
5. **未知关键字策略**：默认忽略非 `x-*` 未知关键字；提供 `--unknown-keywords=warn|error` 可选严格模式。
6. **可选备选**：如果默认值应用、`Extra` 保留成为核心阻塞，可评估 `kaptinlin/jsonschema`，但需补充生产验证。

## 6. 非目标（Non-Goals）

- 本文不是最终选型 ADR；选型需人工确认。`[ASSUMPTION]` owner：待指定。
- 不承诺一期支持 JSON Schema 全部关键字与所有 dialect。`[ASSUMPTION]`
- 不支持远程 `$ref`、网络 schema 拉取作为默认行为。`[INFERENCE]`
- 不把 YAML 特有类型语义原样带入 JSON Schema 校验。`[INFERENCE]`

## 7. 来源

- JSON Schema 规范：https://json-schema.org/
- JSON Schema Core 2020-12：https://json-schema.org/draft/2020-12/json-schema-core
- JSON Schema Validation 2020-12：https://json-schema.org/draft/2020-12/json-schema-validation
- JSON Schema Test Suite：https://github.com/json-schema-org/JSON-Schema-Test-Suite
- santhosh-tekuri/jsonschema：https://github.com/santhosh-tekuri/jsonschema
- kaptinlin/jsonschema：https://github.com/kaptinlin/jsonschema
- qri-io/jsonschema：https://github.com/qri-io/jsonschema
- xeipuuv/gojsonschema：https://github.com/xeipuuv/gojsonschema
- invopop/jsonschema：https://github.com/invopop/jsonschema
- Helm values schema：https://helm.sh/docs/topics/charts/
