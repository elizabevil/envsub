---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# ADR-003：复用 JSON Schema Draft 2020-12 + santhosh v6 + 薄适配层

## 1. 标题

ADR-003：复用 JSON Schema Draft 2020-12 + santhosh v6 + 薄适配层

## 2. 状态

Accepted（已确认），来源：需求提出者确认采用该方案，2026-09-29。`[FACT]`

## 3. 背景

- 需求要求 schema 支持类型、默认值、枚举、范围、数组、对象与复用。`[FACT]`（来源：需求提出者确认）
- 自研 schema 语法会带来解析、校验、文档、IDE 支持与生态迁移成本。`[INFERENCE]`
- JSON Schema Draft 2020-12 已是成熟标准，Go 有成熟实现。`[FACT]`（来源：
  `docs/99-archive/reference/REF-03-json-schema-reuse-research.md`）
- 需求提出者要求优先复用，而不是自己定义一套 schema。`[FACT]`（来源：需求提出者确认）

## 4. 决策

- **schema 格式**：直接使用 JSON Schema Draft 2020-12。`[FACT]`
- **schema 文件**：支持 `.json`、`.yaml`、`.yml`；先解析为 JSON 兼容数据树。`[ASSUMPTION]`
- **校验库**：使用 `github.com/santhosh-tekuri/jsonschema/v6`。`[FACT]`
- **薄适配层**：自研但保持薄，只做以下事情：
    1. YAML/JSON schema 加载与归一化。
    2. Go 风格别名归一化：`int` -> `integer`、`float` -> `number`、`bool` -> `boolean`、`array<T>` -> `array + items`、
       `object` -> `object`。
    3. `default` 主动应用。
    4. env/.env 字符串到 schema 类型的物化。
    5. `x-sensitive` 等 `x-*` 扩展读取。
    6. 未知关键字策略；默认不报错。
    7. 远程 `$ref` 默认禁用。
       `[INFERENCE]`
- **未知关键字策略**：未知非 `x-*` 关键字默认 **不报错**，以保证 schema 可以复用到其它系统；可通过显式
  `--unknown-keywords=warn|error` 提高严格度。`[FACT]`（来源：需求提出者确认）
- **x-* 扩展**：`x-sensitive` 等由薄适配层读取，不修改标准 JSON Schema 语义。`[INFERENCE]`
- **内联 `| type`**：映射为 JSON Schema 类型断言/补类型；schema 与内联取并集，schema 的 required/default/约束优先，交集冲突报错。
  `[FACT]`（来源：此前确认）

## 5. 被拒绝的方案

- **自研 schema DSL：拒绝。** 维护解析器、校验器、文档与生态成本高，且无法直接复用现有 schema 文件。`[INFERENCE]`
- **以 kaptinlin/jsonschema 作为主校验库：本次拒绝。** 其默认值应用与 `Extra` 保留有吸引力，但版本较新、生产验证不如
  santhosh；保留为备选。`[INFERENCE]`
- **未知关键字默认报错：拒绝。** 因为用户可能把同一份 schema 复用到其它系统；默认报错会降低可移植性。`[FACT]`（来源：需求提出者确认）
- **继续 JSON Schema 风格的自定义子集：拒绝。** 既然选择复用标准，就不应再维护自定义语法。`[INFERENCE]`

## 6. 后果

- 正面：可直接复用 JSON Schema、Helm/K8s 生态经验、成熟校验库；减少自研语法与校验器成本。`[INFERENCE]`
- 负面：需要薄适配层处理别名、默认值、未知关键字、YAML 归一化与字符串物化；JSON Schema 2020-12 的未知关键字规范行为（MUST
  refuse）默认被放宽。`[INFERENCE]`
- 已缓解：`--unknown-keywords=ignore|warn|error` 已实现（`schema.ParseUnknownPolicy`、`load.Spec.UnknownKeywords`、
  `render.Policies.SchemaUnknown`），默认 `ignore` 时仍会把未知关键字记入日志；规则见 `04-cli.md`。
  残留风险：JSON Schema 2020-12 的未知关键字规范行为默认被放宽属**有意选择**。`[FACT]`
- 兼容性：可为将来兼容 Helm `values.schema.json` 留出路径。`[INFERENCE]`

## 7. 相关文档

- `docs/99-archive/reference/REF-03-json-schema-reuse-research.md`
- `docs/03-design/03-00-index.md`
- `docs/99-archive/01-context/01-00-index.md`

## 非目标（Non-Goals）

- 一期不承诺支持 JSON Schema 全部 dialect 与全部关键字。`[ASSUMPTION]`
- 不支持远程 `$ref`、网络 schema 拉取作为默认行为。`[FACT]`（来源：需求提出者已接受默认不访问网络）
- 不修改标准 JSON Schema 关键字的语义；项目扩展统一使用 `x-*`。`[INFERENCE]`
- 不把本工具绑定到 Helm/K8s 专用 schema 生态。`[INFERENCE]`
