---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-13
---

# 示例与测试架构（examples 重构方案）

> **状态：已实施（2026-10-13）。** 本文是 `examples/` 重构与 `test/e2e` 命名对齐的方案；下文即为
> 实施后的实际结构。实施小结见 §10。
>
> 背景：`examples/` 此前是"按功能切片"（`conformance.tmpl` 按规则族分段、justfile 按旗标切片），
> 验收只断言"退出码 + 字节数"，既没有完整流程，也无法发现内容错误。本方案把示例改造成
> **一个共享数据集 + 一批自洽案例 + 一组按问题组织的配方**，并让示例层与 e2e 层用同一套文件名。

## 1. 目标与四层职责

测试分四层，每层只做自己那件事，避免"把单测搬进示例"：

| 层 | 职责 | 不做什么 |
|----|------|----------|
| 单元测试（各包 `*_test.go`） | 函数、边界、错误分类 | 不覆盖 CLI 组合 |
| fixture（`internal/engine/extended/testdata/`） | 规则级标准答案（规则编号 ↔ 用例） | 不做跨命令对照 |
| e2e（`test/e2e/testdata/`） | 精确契约：argv、退出码、stdout/stderr、目录快照 | 不重复示例的表达方式 |
| **示例（`examples/`）** | **命令级与场景级"看得见的效果"**：同一套数据在不同命令下的结果 | 不追求覆盖每条规则编号 |

示例层内部按四个测试维度组织：

| 维度 | 内容 | 表达方式 |
|------|------|----------|
| D1 完整 CLI 命令 | `render`（全部旗标组合）、`version`、`completion SHELL`，以及已定案的优先级规则 | 同一数据 + 按问题分组的配方 |
| D2 渲染场景 | 值来源（进程环境 / `.env` / 值文件 / `--set*` / schema default）× 类型 × schema 物化与校验 | 同一数据 + 案例模板 |
| D3 辅助命令 | `--validate-only`、`--check`、`--report=unused`、`--explain`、`--error-format=json` | **与 D2 共用同一套数据**，只换命令 |
| D4 失败路径 | 每个 `diag.Kind` 与退出码 | 同一数据 + `cases/errors/*` |

## 2. 目录结构

```
examples/
  justfile                  执行层：公共 source 前缀 + 按"回答什么问题"组织的配方
  README.md                 目录说明、变量命名图例、案例 ↔ data 对照表
  data/                     共享数据集（唯一一份，全部配方复用）
    values-base.yaml          值文件基线（层 L3）
    values-prod.yaml          值文件覆盖层（同键覆盖、map 深合并、数组替换）
    values-extra.json         JSON 值文件（大整数、字面量拼写）
    app.env                   .env（层 L2，进根，高于值文件）
    extra.env                 第二个 .env（验证"后写胜"）
    schema.yaml               schema：default / 物化 / 约束（enum、range、pattern、length）
    schema-invalid-data.yaml  仅用于触发校验失败
    invalid-syntax.yaml       解析失败用例
  secrets/
    tls.key                   --set-file 的输入（含尾换行，验证"去单个尾换行"）
  templates/                  跨案例共用的模板（被多个配方复用，故不放进 cases/）
    scenarios-json.tmpl        + scenarios-json.golden + .parse=json
    scenarios-yaml.tmpl        + scenarios-yaml.golden + .parse=yaml
    precedence.tmpl            + precedence.golden
    realworld.tmpl             + realworld.golden
    unused.tmpl                + unused.golden + unused.unused
    stdin.tmpl                 + stdin.golden
    filters.tmpl               给 --only-vars/--ignore-vars 用
    values-long-flag.tmpl      给 --values-file 长名用
    batch/                     给 --output-dir --recursive 用
    precedence.tmpl            只含优先级相关的键
    realworld.tmpl             写实案例：一份真实服务配置
    stdin.tmpl                 stdin 场景
    unused.tmpl                只引用部分键（给 --report=unused 用）
  cases/                     一个渲染案例 = 模板 + 它的期望（成对，不分离）
    scenarios-yaml/  input.tmpl  expect.out  expect.yaml  process.env
    precedence/   input.tmpl  expect.out  process.env
    realworld/    input.tmpl  expect.out
    unused/       input.tmpl  expect.unused
    stdin/        input.tmpl
    errors/
      missing/    input.tmpl  expect.exit  expect.err
      syntax/     input.tmpl  expect.exit  expect.err
      type/       input.tmpl  expect.exit  expect.err
      schema/     input.tmpl  expect.exit  expect.err
      io/         input.tmpl  expect.exit  expect.err
  tools/digest/main.go        跨平台目录比对与摘要（替代 shell 的 diff/wc/sha256sum）
```

**`data/` 与 `templates/` 的区别**（易混，明确写死）：

| | `data/` | `templates/` 与 `cases/*/input.tmpl` |
|---|---|---|
| 内容 | `*.yaml` / `*.json` / `.env` / `schema.yaml`，**不含占位符** | `*.tmpl`，**含 `${...}`、`$NAME`、`${@process.X}`** |
| 谁读它 | `-f` / `--dotenv-file` / `--schema` / `--set-file` | 命令的位置参数 |
| 生命周期 | **共享**：改一处会影响所有配方 → 必须有意为之 | **自洽**：改它只影响自己的期望产物 |

`templates/` 与 `cases/*/input.tmpl` 的分工：**被多个配方复用的模板放 `templates/`**（`scenarios`/`precedence`/`realworld`/`unused`/`stdin` 各被 2–3 条配方使用），**只服务一条案例且带专属 `process.env` 的放 `cases/`**。

## 3. 文件命名（示例层与 e2e 层统一）

一层一族的命名，`expect.*` 前缀标识"期望"：

| 文件 | 含义 | 比对方式 |
|------|------|----------|
| `input.tmpl` | 模板（该案例的唯一输入；stdin 场景下由配方用 `-` 传入） | — |
| `process.env` | 该案例固定的进程环境，逐行 `KEY=VALUE` | 覆盖宿主环境 |
| `expect.exit` | 期望退出码 | 精确 |
| `expect.out` | 期望 stdout | 精确（整文件） |
| `expect.err` | 期望 stderr | **子串** |
| `expect.unused` | 期望 `--report=unused` 的输出 | 精确（整文件） |
| `<name>.golden` | 期望 stdout | 精确（整文件） |
| `<name>.parse` | `json` 或 `yaml`：产物必须是该格式 | 解析校验（不是比对） |
| `unused.unused` | 期望 `--report=unused` 输出 | 精确（整文件） |

> **实施时的调整**：模板与它的 golden 放在同一目录（`templates/<name>.tmpl` 与 `templates/<name>.golden`），
> 不再单设 `cases/<场景>/` 一层——那会与 `templates/` 重复一遍同样的文件。`cases/` 因此只保留 `errors/`。
| `expect.dir/` | 期望目录快照（批量输出、原地写回） | 目录树逐文件 |

规则：

- 三个 `expect.*` 均**可选**；全部缺失表示"只要求 exit 0 且无输出"。
- `expect.err` 一律子串比对（长文案脆弱）；**精确文案由 `test/e2e` 层承担**。
- 一个案例只描述**一个模板 + 它的期望**；多命令串联由 **justfile 配方**完成（即"同一数据、不同命令"）。

### 3.1 e2e 对齐（本次一并改）

`test/e2e` 的 harness 只改一处常量，并重命名 fixture 里的文件：

| e2e 现状 | 改为 | 影响 |
|----------|------|------|
| `exit` | `expect.exit` | `e2e_test.go` 的 `caseExitFile` 常量；24 个 fixture 内的 `exit` 文件重命名 |
| `expect.out` / `expect.err` / `expect.dir` / `process.env` / `input.tmpl` / `args` / `modified` / `result/` / `special` | 不变 | — |

改完后两层读法一致，README 只写一次。**`e2e` 仍保留 `args`**（示例层的"命令"写在 justfile 里，e2e 层写在 `args` 里）——这是两层唯一有意的差异，原因：示例要让用户看到命令，e2e 要能独立复现。

## 4. 变量命名图例（`<类型>_<来源>_<效果>`）

模板里的键名本身就是断言标签，词表固定：

| 段 | 取值 |
|----|------|
| 类型 | `string` `int` `float` `bool` `array` `object` `null` |
| 来源 | `type`（类型行为）、`src`（来源存在性）、`prec`（优先级胜出）、`process`（进程环境）、`schema`（物化/约束）、`fmt`（formatter） |
| 效果 | `root` `default` `materialized` `kept` `sorted` `joined` `explicit` `miss` `wins` `over_values` `over_dotenv` `from_string` `enum_ok` `min_items` 等 |

示例：

| 键名 | 含义 |
|------|------|
| `string_type_root` | string，来自根命名空间的值文件 |
| `string_src_from_default` | string，仅由 schema `default` 提供 |
| `string_prec_dotenv_over_values` | string，`.env` 覆盖值文件同键 |
| `string_prec_set_over_dotenv` | string，`--set` 覆盖 `.env` |
| `int_process_materialized` | int，来自 `@process` 并经 `\| type int` 物化 |
| `bool_schema_from_string` | bool，由字符串 `"true"` 经 schema 物化 |
| `float_type_literal_kept` | float，未声明字面量保留拼写（`1.50`） |
| `array_type_json_joined` | array，经 `toJson` 或 `join` 输出 |
| `object_type_yaml_sorted` | object，`toYaml` 按键排序 |
| `null_type_preserved` | null，经 `type: null` 保留 |

**替代**：不再使用 `name` / `port` / `hosts` 这类"场景值"，它们量小且不明确。

**守护（实施时新增）**：新增测试断言

- 每个键名匹配 `^(string|int|float|bool|array|object|null)_(type|src|prec|process|schema|fmt)_[a-z0-9_]+$`；
- `data/` 的键覆盖**每个类型 × 每个来源至少 1 个**，缺失即失败。

## 5. 两个 `scenarios` 模板：按产物语法分文件

`toJson` 与 `toYaml` 产出**两种不同语法**的文档。把一个 JSON 片段和一个 YAML 片段混在同一个文件里
既难读、也无法校验产物语法，因此拆成两个模板，且**每个模板的产物都是该语法的合法文档**：

| 模板 | 产物 | 由谁保证 |
|------|------|----------|
| `templates/scenarios-json.tmpl` | 合法 JSON | `cases/scenarios-json/expect.json`：把渲染结果交给 JSON 解析器 |
| `templates/scenarios-yaml.tmpl` | 合法 YAML | `cases/scenarios-yaml/expect.yaml`：把渲染结果交给 YAML 解析器 |

这不是"重复两遍"，而是**两种编码路径的对照**：同一批数据、同一批操作符，在两个模板里各走一次。
JSON 侧顺便成为"转义正确性"的测试——JSON 模板里任何内联的字符串都必须经 `| toJson` 或 `| quote`，
否则产物就不是合法 JSON，守护会立刻失败。

两个模板的 region 划分保持一致（便于对照）：

| region | 内容 |
|--------|------|
| `types` | 7 种类型各一行 |
| `sources` | 同一逻辑键在四个来源各一份 |
| `precedence` | 同键多层，验证"谁赢"（C4/C5/C10/G21） |
| `process` | 进程环境只能经 `@process` 读到 |
| `positions` | 同一变量在不同位置（顶层/嵌套/数组元素/default word/formatter 参数位） |
| `operators` | O1–O9 两侧（命中/未命中）、`$$`、`$1` |
| `schema` | 物化与约束（见 §5.2） |
| `pipeline` | P1–P6 及其链式与错误位置（见 §5.1） |

`scenarios-json.tmpl` 示意：

```jinja
{
  "types": {
    "string_type_root": ${string_type_root | toJson},
    "int_type_materialized": ${int_type_materialized | type int},
    "float_type_literal_kept": ${float_type_literal_kept},
    "bool_type_dotenv": ${bool_type_dotenv | type bool},
    "array_type_joined": ${array_type_json_joined | type array<string> | join "," | toJson},
    "object_type_sorted": ${object_type_yaml_sorted | type object | toJson},
    "null_type_preserved": ${null_type_preserved | type null}
  },
  "positions": {
    "in_default_word": ${string_src_from_default | toJson},
    "in_formatter_arg": ${array_type_json_joined | join "-" | toJson}
  }
}
```

`scenarios-yaml.tmpl` 示意：

```jinja
types:
  string_type_root: ${string_type_root}
  int_type_materialized: ${int_type_materialized | type int}
  object_type_yaml_sorted: |
${object_type_yaml_sorted | type object | toYaml | indent 4}
  array_fmt_indent: |
${array_type_json_joined | type array<string> | toYaml | indent 2}

operators:
  default_colon: ${string_src_from_set:-word}
  assign_written: ${string_prec_written:=assigned}
  alternate_plus: ${string_type_root+word}
```

> 模板注释里**不要写 `${...}` 字面量**：注释也会被求值，会变成真实引用（本轮已踩过）。
> 两份模板里同名变量保持一致，差异只在**编码形式**（`toJson` vs 原生 YAML），这样对照时一眼能看出编码差异。

### 5.1 操作符与 formatter 覆盖矩阵（两个 scenarios 模板都必须齐全）

**操作符（O 系列，9 个）** —— 每个操作符都要有"命中/未命中"两侧：

| 规则 | 表达式 | 场景 | 期望 |
|------|--------|------|------|
| O1 | `${A}` / `$A` | 存在 / 空 / 缺失 | 值 / 按 `--on-empty` / 按 `--on-missing` |
| O2 | `${A:-word}` | 空 或 缺失 | `word` |
| O3 | `${A-word}` | 缺失（空仍取值） | `word` |
| O4 | `${A:=word}` | 缺失或空 → **写回**；已存在 → 不写 | `word` / 原值 |
| O5 | `${A=word}` | 缺失 → 写回；已存在 → 不写 | `word` / 原值 |
| O6 | `${A:+word}` | 存在且非空 | `word`（`0`/`false` 也算存在且非空） |
| O7 | `${A+word}` | 存在（含空） | `word` |
| O8 | `${A:?word}` | 缺失或空 | 报错 exit 6，`word` 为消息 |
| O9 | `${A?word}` | 缺失 | 报错 exit 6 |

另需覆盖：`$$`（转义为字面 `$`）、`$1`/`$2`（保持字面量）、嵌套 word（默认值里再引用变量）、
同一变量的**写回可见性**（`${A:=x}` 之后的 `${A}` 读到的必须是写回值）、
以及 `${A:-x}` **不写回**（后续引用仍是原值）。

**formatter（P 系列，6 个）** —— 覆盖参数有无、链式、以及错误路径：

| 规则 | 形式 | 必须覆盖 |
|------|------|----------|
| P1 | `${array \| join}` / `${array \| join "sep"}` | 默认 `,`；自定义分隔符（**参数必须加引号**）；非数组 → exit 4 |
| P2 | `${x \| toJson}` | 对象键排序、无尾换行、HTML 不转义、精确整数与字面量拼写保真 |
| P3 | `${x \| toYaml}` | 2 空格缩进、映射键排序、去尾换行、序列风格 |
| P4 | `${scalar \| quote}` | Go 转义；结构化值 → exit 4 |
| P5 | `${x \| indent N}` / `${x \| indent 0}` | 每行加 N 空格；0 等价原样；缺参/非数字 → exit 2 |
| P6 | `${x \| type T}` | 位置约束（**必须是链首**，否则 exit 3）；7 种类型名；`uint`/`int64` 等不支持名 → exit 2；与 schema 冲突 → exit 4 |

链式必须成对覆盖：`type → formatter`（如 `${ports \| type array<int> \| join "-"}`）、
`formatter → formatter`（如 `toYaml → indent`）、以及**错误位置**（`formatter → type` 报链首约束）。

### 5.2 schema 覆盖矩阵（`data/schema.yaml` 必须齐全）

schema 白名单共 60 个关键字，按"对本项目是否实际生效"分三类。**A 类必须逐条出现在示例里**：

**A 类：实际参与行为（必须覆盖）**

| 类别 | 关键字 | 示例中要有 |
|------|--------|------------|
| 类型与别名 | `type`、`int`/`float`/`bool`、`array<T>` | 7 种类型各一；别名与 `array<T>` 各一 |
| 校验 | `enum`、`const`、`required` | 命中 + 未命中（未命中 → exit 4） |
| 数值 | `minimum`、`maximum`、`exclusiveMinimum`、`exclusiveMaximum`、`multipleOf` | 各一，含边界值 |
| 字符串 | `minLength`、`maxLength`、`pattern` | 各一，含边界 |
| 数组 | `items`、`minItems`、`maxItems`、`uniqueItems`、`prefixItems`、`contains`、`minContains`、`maxContains` | 各一 |
| 对象 | `properties`、`required`、`patternProperties`、`additionalProperties`、`propertyNames`、`minProperties`、`maxProperties`、`dependentRequired`、`dependentSchemas` | 各一 |
| 组合 | `allOf`、`anyOf`、`oneOf`、`not`、`if`/`then`/`else` | 各一（含"不满足"的反例） |
| 结构 | `$ref`、`$defs`、`definitions`、`$anchor`、`$id`、`$schema` | 各一（`$ref` 至少指向 `$defs` 与嵌套 `properties`） |
| 默认值 | `default` | 顶层、嵌套对象、数组元素、**不得覆盖已有来源**（G21） |
| 未知关键字策略 | `x-*` 扩展 | 一个 `x-*`（放行）+ 一个非 `x-*` 未知键（用 `--unknown-keywords` 三态各跑一次） |

**B 类：由库处理、示例只要出现一次**：`format`、`contentEncoding`、`contentMediaType`、
`contentSchema`、`title`、`description`、`examples`、`deprecated`、`readOnly`、`writeOnly`、
`$comment`、`$vocabulary`、`unevaluatedProperties`、`unevaluatedItems`、`$dynamicRef`、
`$dynamicAnchor`、`contains` 的边界组合。

**C 类：对本项目不生效（示例里要写清"为什么不测"）**：`unevaluated*` 与 `$dynamic*` 在
`jsonschema/v6` 下按库语义处理，样例只保留一处并注明来源。

另按规则文档的已知边界，示例里要**显式呈现**这几条（都属于"实际场景"）：

| 规则 | 场景 |
|------|------|
| SC6/SC7/SC8 | `default` 只补 `properties`/`items` 中缺失的键；已存在不覆盖；嵌套递归；数组只补已存在元素 |
| SC9 | `$ref` 指向的定义里的 `default` **不生效**（现状，需在示例中固化） |
| SC10 | 校验只覆盖 root 命名空间 |
| SC11 | `ExactInt` 在校验前转 `json.Number`，值不被改动 |
| SC12 | 校验错误扁平化为至多 8 条 `at /ptr: reason`，`KindSchema`=exit 4 |
| G21 | 有实际来源时 `default` 不填（`--on-missing` 之外的第二重兜底语义） |
| SC4 | `array<T>` 逐元素断言；嵌套 array → exit 2 |
| TY6/F6 | `null` 保留、未声明小数字面量保拼写（`1.50`） |

## 6. justfile 配方（按"回答什么问题"组织）

公共 source 前缀**只在顶部定义一次**，全部配方逐字复用（这就是"同一套数据、不同命令"的强制形式），并有一条自查配方校验前缀一致。

| 配方 | 回答的问题 | 用的命令 / 数据 |
|------|-----------|------------------|
| `data` | 数据集里有哪些键、命名图例是什么 | 不执行（列表 + 图例） |
| `scenarios-json` | 无进程环境时，JSON 侧全类型/全来源/全位置/全管道渲染成什么 | `render` + `templates/scenarios-json.tmpl`（产物须可解析为 JSON） |
| `scenarios-yaml` | 同上，YAML 侧 | `render` + `templates/scenarios-yaml.tmpl`（产物须可解析为 YAML） |
| `scenarios-with-process` | 加上进程环境后两个模板的差异 | 同一份 + `--process-env-prefix ENVSUB_` |
| `precedence` | 层序（C4/C5/C10/G21）谁赢 | 同一份 + `templates/precedence.tmpl` |
| `as-files` | 分层仅来自文件（base/prod/json） | 同一份 + `templates/realworld.tmpl` |
| `as-sets` | 分层仅来自 `--set*` / `--set-file` | 同一份 + `templates/realworld.tmpl` |
| `as-defaults` | 全靠 schema default 兜底 | 同一份 + `templates/realworld.tmpl` |
| `with-dotenv` / `without-dotenv` | `.env` 加入/排除的差异 | 同一份 |
| `auxiliary-validate` | 同一数据只校验不写 | `--validate-only` |
| `auxiliary-check` | 同一数据与提交产物比对 | `--check`（期望 exit 7） |
| `auxiliary-unused` | 同一数据里哪些键没人引用 | `--report=unused` |
| `auxiliary-explain` | 同一数据每个值来自哪里 | `--explain` |
| `auxiliary-stdin` | stdin 只读一次 | `-` |
| `failures-missing/syntax/type/schema/io/usage/check` | 每个退出码一例（6/3/4/4/5/2/7） | `cases/errors/*` |
| `acceptance` | 完整流程一次跑完，做目录级内容比对 | 全部命令串联 |
| `digest-update` | 有意变更后重生成期望产物 | `tools/digest` |

分组不按"旗标"也不硬按"命令"，而按**要观察的问题**；`just --list` 需要能一眼看出每条配方回答什么。

## 7. 平台中立

```make
# 只在这里定义二进制名与路径
bin := if os_family() == "windows" { root / "dist" / "envsub.exe" } else { root / "dist" / "envsub" }
```

- **不使用 bash 脚本**：期望产物比对与摘要改由 `examples/tools/digest`（Go 小工具）完成，配方里不再出现 `diff` / `wc` / `sha256sum` / `cat` 等平台相关命令。`examples/full/acceptance.sh` 已随重构删除。
- 进程环境在配方内固定，`@process` 结果不依赖机器。
- `digest` 只依赖 Go 工具链，与 `just build` 的前提一致。

## 8. 实施顺序

| 顺序 | 内容 | 验收 |
|------|------|------|
| 1 | 建 `data/` + `templates/` + 变量按命名图例重写 | 命名守护测试通过；每个类型 × 来源至少 1 个键；**§5.1 操作符/formatter 矩阵与 §5.2 A 类 schema 关键字逐条出现在示例中**（新增守护：从 `pipeline.Names()` 与 `knownKeywords` 反查示例覆盖）；两个 scenarios 模板的产物分别通过 JSON/YAML 解析守护 |
| 2 | 建 `cases/`（模板 + `expect.*`）与 `tools/digest` | `digest` 能比对目录并给出差异清单 |
| 3 | 重写 `examples/justfile`（公共前缀 + 按问题分组）与 README | `just --list` 可读；前缀自查通过 |
| 4 | `test/e2e` 对齐：`exit` → `expect.exit` | `just test-e2e` 全绿 |
| 5 | 增加跨层守护（示例↔规则族、旗标覆盖、命名/维度、前缀一致） | `just check` 全绿 |
| 6 | 删除 `examples/full/`（内容已并入）与 `acceptance.sh` | 仓库内无旧路径引用 |

**实施期间保持** `just check` 与 `just test-all` 全绿，每步独立提交。

## 10. 实施小结（2026-10-13）

| 项 | 结果 |
|----|------|
| 数据集 | `examples/data/` 5 个值文件 + schema + 两个失败输入；键名全部为 `<类型>_<来源>_<效果>` |
| 模板 | `scenarios-json.tmpl`（产物合法 JSON）与 `scenarios-yaml.tmpl`（产物合法 YAML），region 一一对应 |
| 案例 | `cases/errors/` 6 个失败路径（6/3/4/4/5/2），`--check` 的 exit 7 由 `auxiliary-check` 覆盖 |
| 配方 | 24 条，覆盖 CLI 命令、值来源、类型、schema、辅助命令、失败路径与完整验收；公共 source 前缀只在顶部定义一次 |
| 工具 | `examples/tools/digest` 做目录比对与格式解析，配方里不再有 `diff`/`cmp`/`wc`/`sha256sum` |
| 平台 | 只有 `bin` 变量区分平台；没有 shell 脚本（`acceptance.sh` 已删除） |
| 守护 | `test/examples` 五条：规则族对照、全部旗标出现、键名规范、类型×来源维度、formatter 全部被使用、两份模板的解析标记 |
| e2e 对齐 | `test/e2e` 的 `exit` → `expect.exit`（24 个 fixture）；新增 `examples_tree` 用目录快照钉住输出树 |
| 删除 | `examples/full/`（内容并入新结构）、`examples/full/acceptance.sh`、旧 fixture `examples_full` 与 `examples_conformance` |

实施中发现并记录（未擅自定案）：

- **`${A:-word | pipeline}` 无规则**：实测管道不作用于运算符默认值（`${M:-a | quote}` → `a`，而 `${P:+a | quote}` → `"a"`），
  示例因此避免在默认值上接管道。需要补一条 O 系列规则说明。
- **替换只缩进第一行**：`toYaml` 的多行结果无法嵌在父键下（续行会落到列 0），示例改用 `toJson` 表达嵌套结构，
  并在模板注释与 README 里写明这一限制。若要让"嵌套块标量"成为可表达形态，需要给 `indent` 或替换逻辑补能力。

## 9. 非目标

- 不改变任何生产行为；本方案只动 `examples/`、`test/e2e` 的文件命名与 `test/examples` 的守护。`[FACT]`
- 不在示例层覆盖每条规则编号（那是 fixture 层的职责）。`[FACT]`
- 本轮不处理"批量输出保留 `.tmpl` 后缀"的产品问题——它记入需求登记册（`REQ-OUT-11`），待决策后再动。`[FACT]`
- 不为 Windows 单独写一套逻辑（P2 只补二进制路径与 CI 运行器）。`[FACT]`
- 两个 scenarios 模板**自身**不是合法 JSON/YAML（含未替换的占位符）；被校验的是**渲染产物**。`[FACT]`
