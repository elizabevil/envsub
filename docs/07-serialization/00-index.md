---
status: draft-ai
owner: [ 待指定 ]
version: 0.9
last-updated: 2026-10-09
---

# 序列化规则（规范性）

> 目录：`docs/07-serialization/`（多篇；本文件为总索引）。

> 本文件为总索引。

> 本文是**规范性文档**：定义值树与 JSON/YAML 文本之间的强制语义，以及值来源解码/物化、模板
> 操作符、formatter 管道、`| type` 断言、schema 校验、CLI 行为的验收标准。每条规则带唯一编号，
> 可被 `test/` 与评审直接引用。
> 方案选型记录见 **附录 A**；业界惯例核查见 **附录 B**。
> 决策来源：需求提出者 2026-10-08 / 2026-10-09 确认项（见 §1.3 变更记录）。

编号与引用约定：

- 文档引用统一写 `§x.y` 或规则 ID（如 `§6.2`、`C2`）；跨文档引用必须用 Markdown 链接。
- 代码引用统一写 `包/文件.go:行`，`internal/<包>/` 前缀见 §1.1「相关实现」。
- 状态标记：✅ 已实现并有测试固化；**待实现** 规则已定、代码未落地；**不提供** 有意不实现；
  **已决例外** 有意保留的差异。
- 每个案例标注 **现状**（当前实测行为）或 **目标（待实现）**（规则要求、尚未实现）。

## 1. 文档说明

**本章解决**：界定规范范围、状态与版本历史，避免读者误用。

### 1.1 范围与相关实现

_列：项 | 内容。_

| 项 | 内容 |
|----|------|
| 范围 | 值树 ↔ JSON/YAML 文本；值来源解码与物化；模板操作符；formatter 管道；`\| type`；schema 关键字/默认值/校验；CLI 行为；输出形态；后端等价 |
| 非范围 | 模板语法词法（见 [03-02 值模型与模板语法](03-02-value-model-and-template-syntax.md)）、CLI flag 定义（见 [03-08 CLI 设计](03-08-cli-design.md)） |
| 相关实现 | `internal/values`、`internal/types`、`internal/codec`、`internal/jsonx`、`internal/yamlx`、`internal/schema`、`internal/pipeline`、`internal/engine/extended`、`internal/render`、`internal/cli` |
| 相关测试 | `test/encoding/`（规则一致性）、`test/jsonx/`、`test/yamlx/`（后端/编码器对照）、`internal/*_test.go`、`internal/engine/extended/testdata/`、`test/e2e/` |

### 1.2 状态与落地

_列：项 | 内容。_

| 项 | 内容 |
|----|------|
| 状态 | `draft-ai`；规范已定案，实现按 §14 落地（步骤 1–11 全部完成） |
| 规范性来源 | 需求提出者决策 + 实现与实测（每格标注 `file:line` 或「实测」，案例标注 现状/目标） |
| 效力 | 本文件是相关功能（数字类型、空值、`toJson`/`toYaml`、值来源物化、CLI）的验收前提；与代码冲突时以本文件为准并登记 §13 |

### 1.3 变更记录

_列：版本 | 日期 | 变更 | 影响章节。_

| 版本 | 日期 | 变更 | 影响章节 |
|------|------|------|----------|
| 0.1 | 2026-10-08 | 初版：数字 N1–N4、空值 V1–V5、浮点 F1–F5、输出 D/Q、待确定项 T1–T9 | 全文 |
| 0.2 | 2026-10-08 | 补 `\| type`、schema、操作符、formatter、落地清单 | 全文 |
| 0.3 | 2026-10-09 | CLI 独立成章（C 系列 + 优先级/交互/策略/错误）；`number` 明确为非值类型（F6）；`float` 严格范围（F7）与非有限值无条件拒绝（F8）；`""` 判空标注 bash 兼容语义（OV9）；`\| type null` 保留并登记 G18；补规则索引、术语表、错误目录 | §2、§3、§5、§6、§8、§11、§13 |
| 0.4 | 2026-10-09 | 补全可运行案例（§3–§9 每规则均有案例，标注 现状/目标）；`uint` 定为**不提供**并删除对应落地步骤；`--set` 列为**建议删除（待确认）**；清理历史编号痕迹 | §2、§3、§4、§5、§6、§7、§8、§9、§13、§14 |
| 0.5 | 2026-10-09 | 按章节拆分为多篇（docs/07-serialization/） | 00-index |
| 0.6 | 2026-10-09 | 删除 `--set-string`，统一到 `--set`（flag、`Options.SetStrings`、`values.SetTyped` 全部移除）；G17 随之消除；优先级表去掉该档 | §6.1、§13.1、§14 |
| 0.7 | 2026-10-09 | 落地 F7/F8/Q7/T6/OV8/TY3/TY6（NaN/±Inf 三路径拒绝、float 范围校验、删除 null 字符串隐式转换）；G9 非法 JSON 统一为 KindType exit 4；FB3 整数精度余项（v0.8 补齐） | §3、§6、§8、§13、§14 |
| 0.9 | 2026-10-09 | 记录 Go 1.27 中 `encoding/json`(v1) 与 v2 的关系：v1 = v1 语义 + v2 引擎，默认值仍不同（重复键/非法 UTF-8/键序/越界文案）；纯文法校验统一用 `jsontext`（§12.3） | §10、§12 |
| 0.8 | 2026-10-09 | 步骤 9 落地：新增 `codec.LiteralNumber`，未声明数字字面量保留原文（F3/F6/Q6/FB2）；步骤 8 落地：`test/encoding/types_test.go` 类型矩阵 | §3、§7、§13、§14 |
| 0.10 | 2026-10-10 | CLI 契约修订：值来源优先级定稿（`--set`/`--set-json` > 值文件 > `.env` > 进程环境 > `default` 兜底，G21）；`--values` → **`-f, --values-file`**、`--env-prefix` → **`--process-env-prefix`**（旧名移除、不保留别名）；`--env-file` 强制 dotenv（忽略文件名）；`--explain` 改为固定 JSONL 并支持 `--explain[=PATH]` | §6.1、§6.2、§13.1、§14、§15 |
| 0.11 | 2026-10-10 | **命名空间与分层重构（方案 C，定案）**：`@` 成为唯一命名空间记号（`${@process.NAME}`，只允许首字节）；`.env` 写入**根**且 **`.env` > `-f`**，层序改为 `--set`/`--set-json` > `--dotenv-file` > `-f/--values-file` > schema `default`；**进程环境退出层序**；**取消 `${NAME}` → 环境的分层便利**；顶层特殊键 `${["a.b"].k}` 可达；`--env-file` → **`--dotenv-file`**（不留别名）。方案与实测见 `06-review/06-15` | §6.1、§6.2、§13、§14、§15 |

## 2. 术语、规则用语与规则索引

**本章解决**：统一术语、规范用语，并提供全部规则编号的可检索索引。

### 2.1 术语表

_列：术语 | 含义 | 证据。_

| 术语 | 含义 | 证据 |
|------|------|------|
| 值树 | 渲染器内部值表示：`map[string]any` / `[]any` / 标量 | engine.go:102-103 |
| 声明类型 | 由 schema `type` 或模板 `\| type` 显式声明的类型 | engine.go:325-362 |
| 字面量 | 值来源文本中出现的原始数字/字符串写法，如 `1.50`、`1e3` | values/number.go:37-55 |
| 未声明数字 | 既无 schema `type` 也无 `\| type` 的数字值，统称 `number`（**不是值类型**，见 §3.11） | §3.11 |
| 精确整数 | 超出 `uint64` 的整数，以十进制文本无损保存 | codec/exact.go:14-17 |
| 字面量数字 | 未声明小数以其字面量文本保存的表示（实现待定，见 §3.11） | §3.11 |
| 物化 | 把字符串按声明类型转成值树类型 | schema/materialize.go:16-21 |
| 归一化 | 把解码器产物转成规范 Go 类型（`NormalizeNumbers`/`types.Normalize`） | values/number.go:18-35 |
| 存在性 | 路径/键是否有值（与内容无关，见 §5.3） | tree.go:87-92 |
| 空值性 | 值存在时内容是否为空（见 §5.3） | engine.go:554-567 |
| 后端 | `jsonx` 的两个实现：标准库 `encoding/json/v2`（默认）与 `sonic`（`-tags=sonic`） | jsonx/jsonv2.go:1、jsonx/sonic.go:1 |
| 库默认 | 未施加本项目额外归一化时编码库的原生输出 | jsonx/sonic.go:104-125 |

### 2.2 规则用语

_列：用语 | 含义。_

| 用语 | 含义 |
|------|------|
| 必须 / 禁止 | 强制要求；违反即缺陷，必须有一条测试 |
| 应当 | 期望遵守的要求；不遵守必须在本文件记录理由 |
| 可以 | 允许的实现自由 |
| 默认 | 未显式配置时的行为 |
| ✅ / 待实现 / 不提供 / 已决例外 | 规则在代码中的落地状态 |

### 2.3 规则索引

_列：ID | 一句话规则 | 状态 | 验收测试。索引只做导航，规则定义在各自主章节；同一规则不在两处定义。_

| ID | 一句话规则 | 状态 | 验收测试 |
|----|------------|------|----------|
| N1 | 整数取能精确表示的最窄类型（int/uint64/精确整数）；未声明小数按字面量 | ✅ | rules_test.go:30-73 |
| N2 | 两类整数两种格式都输出裸数字；浮点不保证拼写 | ✅ | rules_test.go:30-73 |
| N3 | JSON/YAML/.env/`--set`/`:=` 各路径整数能力必须一致 | ⚠️ 部分（G6/G11） | rules_test.go:121-183 |
| N4 | 任何路径禁止把整数静默降级为 float64 | ✅（float 断言除外，见 G2） | rules_test.go:121-183 |
| N5 | 整数阶梯即"最窄可精确表示"；范围用 `2^n` 表述 | ✅ | number_test.go:15-40 |
| F1 | `float` 为 IEEE-754 binary64，不做十进制精确化 | ✅ | rules_test.go:245-290 |
| F2 | `float` 编码为最短往返十进制，允许指数 | ✅ | rules_test.go:255-258 |
| F3 | 整数样小数不得丢小数点（`1.0` → `1.0`） | ✅ | rules_test.go TestLiteralNumberRules |
| F4 | `float` 物化接受十进制/指数，拒绝空串、空白、非数字 | ✅ | rules_test.go:325-338 |
| F5 | 浮点比较按 IEEE，无十进制容差 | ✅ | jsonschema/v6 |
| F6 | 未声明小数按字面量保存（`1.0`/`1.50`/`1e3`） | ✅ | rules_test.go TestLiteralNumberRules |
| F7 | `float` 仅限 float64 范围；超范围或无法精确表示的整数报错 exit 4 | ✅ | rules_test.go:302-345 |
| F8 | `NaN`/`±Inf` 解码、物化、编码三路径无条件拒绝 exit 4 | ✅ | rules_test.go:259-266,301-324 |
| FB1 | 高精度小数按 float64 舍入；字面量路径由 F6 保拼写 | ✅ 规则 | rules_test.go:255 |
| FB2 | 超出 float64 范围的**字面量**按 F6 保留；转 `float` 按 F7 报错 | ✅ | rules_test.go TestLiteralNumberRules |
| FB3 | `\| type float` 对超出 float64 精确表示的整数：三种来源（字符串/`uint64`/精确整数）一致报错；`2^53`（9007199254740992）为边界 | ✅（v0.7） | rules_test.go:340-380 |
| V1 | `nil`（含 typed nil）与空集合编码为 `{}`/`[]` | ✅ | rules_test.go:75-117 |
| V2 | 显式 `null` 仍输出 `null` | ✅ | rules_test.go:87 |
| V3 | 缺失路径不输出，由 `--on-missing` 决定 | ✅ | rules_test.go:121 |
| V4 | 空字符串输出 `""`，不得写成 `null` | ✅ | rules_test.go:88 |
| V5 | 两个 JSON 后端逐字节一致（重复成员名例外见 T9） | ✅ | test/jsonx |
| D1 | 映射键在所有编码器排序输出 | ✅ | rules_test.go:187-205 |
| D2 | `toJson` 无尾换行；`toYaml` 去掉尾部换行 | ✅ | pipeline.go:92-106 |
| D3 | YAML 缩进 2 空格；序列/映射风格固定 | ✅ | test/yamlx:119-139 |
| Q1 | 数字样字符串必须加引号，保证往返类型不变 | ✅ | rules_test.go:88-89 |
| Q2 | JSON 不转义 HTML（`<` `>` `&` 原样） | ✅ | 实测 |
| Q3a | 非法 UTF-8 编解码两侧都拒绝 exit 4 | ✅ | jsonv2_test.go:224-241 |
| Q3b | 重复成员名默认后端/YAML 拒绝 | ✅ | jsonv2_test.go:204-211 |
| Q4 | 空字符串输出 `""`（= V4） | ✅ | rules_test.go:88 |
| Q5 | 缺失路径不产生输出（= V3） | ✅ | engine.go:279-304 |
| Q6 | 未声明小数字面量保留拼写，不得丢小数点 | ✅ | rules_test.go TestLiteralNumberRules |
| Q7 | `NaN`/`±Inf` 编码侧也必须报错 | ✅ | rules_test.go:259-266 |
| L1 | 嵌套深度上限继承库（约 10000 层），干净报错不 panic | ✅ | rules_test.go:400-418 |
| T1 | `sonic` 归一化 typed nil 为 `{}`/`[]` | ✅ | test/jsonx `-tags=sonic` |
| T2 | 精确整数类型归属叶子包 `internal/codec` | ✅ | internal/codec |
| T3 | 每种类型给出显式精确语义与范围 | ✅ | §3 |
| T4 | 类型名只提供 `int`（精确整数，含负数，无内建上限）；**不提供** `uint` 与宽度类型名 | 已决 | §8.1 |
| T5 | 运算符只做存在性/空值判断；`:=`/`=` 写回字符串 | ✅ | §5.1 |
| T6 | `NaN`/`±Inf` 三路径报错（= F8） | ✅ | rules_test.go:259-266 |
| T7 | 深度上限继承库行为，不叠加自有上限（= L1） | ✅ | rules_test.go:400-418 |
| T8 | 不增加宽度类型名（`int32`/`int64`/…） | 已决 | §8.1 |
| T9 | 接受 `sonic` 重复成员名为唯一例外 | 已决例外 | test/jsonx |
| O1 | `${A}` 直接取值 + 策略 | ✅ | errors/missing_variable |
| O2 | `${A:-d}` unset 或空 → `d` | ✅ | defaults/default_or_empty_* |
| O3 | `${A-d}` 仅 unset → `d` | ✅ | defaults/default_unset* |
| O4 | `${A:=d}` unset 或空 → 写回 `d` | ✅ | policies/assign_default |
| O5 | `${A=d}` 仅 unset → 写回 `d` | ✅ | policies/assign_if_unset |
| O6 | `${A:+a}` 存在且非空 → `a` | ✅ | defaults/alternate_nonempty* |
| O7 | `${A+a}` 存在 → `a` | ✅ | defaults/alternate_if_set* |
| O8 | `${A:?e}` unset 或空 → 报错 `e` | ✅ | defaults/required_empty |
| O9 | `${A?e}` 仅 unset → 报错 `e` | ✅ | defaults/required_unset |
| OV1 | 算空：`nil`、`""`、空数组、空对象、typed nil | ✅ | engine.go:554-567 |
| OV2 | 不算空：`0`、`false`、uint64/精确整数、非空集合 | ✅ | 实测 |
| OV3 | typed nil 判空但编码为 `{}`/`[]` | ✅ | rules_test.go:92-93 |
| OV4 | 存在性只看路径是否解析成功 | ✅ | tree.go:87-92 |
| OV5 | 默认策略 `--on-missing=error`、`--on-empty=ok` | ✅ | cli/render.go:34-35 |
| OV6 | `:=`/`=` 是唯一写回值树的运算符 | ✅ | policies/default_does_not_assign |
| OV7 | 含 `:=`/`=` 时并行渲染回退串行 | ✅ | parallel.go:105-108 |
| OV8 | `NaN`/`±Inf` 不应出现在值树 | ✅ | §3.10 F8 |
| OV9 | `""` 判空是 **bash 兼容语义**（有意选择） | ✅ | §5.3 |
| P1 | `join`：`[]any` → 连接字符串，参数须加引号 | ✅ | pipeline/join_* |
| P2 | `toJson`：任意 → 紧凑 JSON，键排序；可选 `align` | ✅ | pipeline/tojson_*、formatters/toyaml_align_default |
| P3 | `toYaml`：任意 → YAML 块，去尾换行；可选嵌套宽度 `N` 与 `align` | ✅ | pipeline/toyaml_object、formatters/toyaml_* |
| P4 | `quote`：标量 → `strconv.Quote` | ✅ | pipeline/quote |
| P5 | `indent`：标量 + 非负整数宽度，在当前格式之上追加 | ✅ | pipeline/indent_* |
| P6 | `type`：管道内联类型断言 | ✅ | inline_type/* |
| P7 | 序列化宽度属于序列化器（`toYaml N`），文本前缀属于 `indent` | ✅ | formatters/toyaml_align_width |
| P8 | 多行值的两种意图由模板显式选择（`align` vs 原样）；`align` 与 `indent` 互斥 | ✅ | formatters/toyaml_raw_keeps_column、formatters/toyaml_align_with_indent_rejected |
| TY1 | `string` 原样 | ✅ | convert_test.go:70 |
| TY2 | `int` 精确整数、无上限、可负 | ✅ | number_test.go:69-84 |
| TY3 | `float` = float64 有限值 | ✅ | convert_test.go:75 |
| TY4 | `bool` 仅 `true`/`false` | ✅ | convert_test.go:76-79 |
| TY5 | `object`/`array`/`array<T>` | ✅ | convert_test.go:80-84 |
| TY6 | `null` 保留；**禁止字符串隐式转换**（G18） | ✅ | §14 步骤 11 |
| TY7 | **不提供** `uint` 类型名：`\| type uint` → `unsupported type` exit 2；非负约束用 schema `minimum: 0` | 不提供 | §8.4 |
| TY8 | 禁止宽度类型名（`int32`/`int64`/…） | 已决 | §8.1 |
| SC1 | core/结构关键字集合 | ✅ | normalize.go:48-68 |
| SC2 | 校验关键字集合（库执行） | ✅ | jsonschema/v6 |
| SC3 | 注解/format 关键字集合 | ✅ | jsonschema/v6 |
| SC4 | 类型别名与 `array<T>` 归一化 | ✅ | normalize.go:18-29 |
| SC5 | 名字容器子键不判未知 | ✅ | normalize.go:36-43 |
| SC6 | `default` 只补缺失键、不覆盖 | ✅ | schema_test.go:120-136 |
| SC7 | `default` 原样克隆、不做类型转换 | ✅ | defaults.go:36 |
| SC8 | 嵌套对象递归；数组只补已存在元素 | ✅ | defaults.go:44-72 |
| SC9 | `default` 只遍历 `properties`/`items`（`$ref` 等不生效） | 待定 | §9.2 |
| SC10 | 校验对象为 `codec.ToJSON(tree.Root)` | ✅ | jsonschema.go:25-35 |
| SC11 | `ExactInt` 校验前转 `json.Number` | ✅ | schema_test.go:271-298 |
| SC12 | 校验错误扁平化为至多 8 条 `at /ptr: reason` | ✅ | jsonschema.go:53-74 |
| CLI1–CLI5 | 值来源与不猜测原则（映射到 C1、C3、C12；C2 已删除） | ✅ | §6.1 |
| C1 | `--set` 存字符串，类型由 schema/`\| type` 决定 | ✅ | §6.1 |
| C2 | ~~`--set-string`~~ | **已删除（v0.6）**；`--set` 存字符串，类型由 schema 或 `\| type` 决定 | §6.1 |
| C3 | `--set-json` JSON 字面量 → 精确类型树（命名理由见 §6.1，v0.6 复核保留） | ✅ | e2e/set_json_big_int |
| C4 | 同一来源内后赋值覆盖前赋值；set 组按固定顺序覆盖 | ✅ | 实测 |
| C5 | 值来源优先级表（`--set`/`--set-json` > YAML/JSON 值文件 > `.env` > 进程环境 > schema `default`；`default` 为兜底） | ✅ | §6.2 |
| C6 | schema 与 inline `\| type` 交互：先 schema 物化、再兼容检查、再断言 | ✅ | engine.go:325-362 |
| C7 | `type` 必须是首个管道步骤，先于 formatter | ✅ | expression.go:89-91 |
| C8 | `--on-missing=error\|empty\|keep\|zero` 行为 | ✅ | §6.4 |
| C9 | `--on-empty=ok\|error` 行为（`keep`/`zero` 不支持，G19） | ✅/待定 | §6.4 |
| C10 | 错误分类与退出码（见 §11） | ✅ | diag/errors.go:111-136 |
| C11 | `--set-json` 与 `.env` 物化非法 JSON 统一为 KindType exit 4；赋值语法错误仍为 KindUsage exit 2 | 已统一（v0.7） | §6.6 |
| C12 | CLI 层**不做类型猜测**；暂不引入 `--set-int`/`--set-float`/`--set-bool` | 已决 | §6.7 |
| G1 | `sonic` 重复成员名 last-wins | 已决例外 | §13.1 |
| G2 | ~~`\| type float` 对超精度整数静默丢精度~~ | **已消除（v0.7）**：`checkFloatExact` 统一拒绝 | §13.1 |
| G3 | 超出 float64 字面量当前保留原文 | **已消除（v0.8）**：未声明按 F6 保留、转 float 按 F7 报错 | §13.1 |
| G4 | `${x}` 定点展开 vs `toJson`/`toYaml` 最短形式 | 不统一（有意） | §13.1 |
| G5 | YAML `.nan`/`.inf` 曾被接受 | 已消除（v0.7） | §13.1 |
| G6 | `:=`/`=` 写回字符串 | 待定 | §13.1 |
| G7 | 浮点拼写当前不保留 | **已消除（v0.8）**：未声明数字按 F6 保留；声明 `float` 仍按最短形式 | §13.1 |
| G8 | `--set` 文案与实现不一致 | 待定 | §13.1 |
| G9 | `--set-json` 与 `.env` 非法 JSON 退出码曾不同 | 已统一（v0.7） | §13.1 |
| G10 | JSON→结构体两条路数字表示不同 | 待定 | §13.1 |
| G11 | YAML 大整数两条路不同 | 待定 | §13.1 |
| G12 | schema 校验与物化能力不对称 | 待定 | §13.1 |
| G13 | 联合 `type` 不触发物化 | 待定 | §13.1 |
| G14 | `array<T>` schema 与 inline 语义不同 | 待定 | §13.1 |
| G15 | 嵌套 env fallback 不参与 `Validate` | 待定 | §13.1 |
| G16 | `pipeline.Names()` 不含 `type` | 无需改代码 | §13.1 |
| G17 | ~~`--set-string` 仍按 schema 物化~~ | **已消除（v0.6，随 C2 删除）** | §13.1 |
| G18 | `\| type null` 对 `""` 曾隐式转 `nil` | 已消除（v0.7） | §8.2 |
| G19 | `--on-empty` 不支持 `keep`/`zero` | 待定 | §6.4 |


### 2.4 章节 → 文件映射

_列：章节 | 文件。全局 § 编号不变；跨篇引用保持 §x.y 形式。_

| 章节 | 文件 |
|------|------|
| §1、§2 | [00-index.md](00-index.md) |
| §3（含 §3.10、§3.11） | [01-types.md](01-types.md) |
| §4 | [02-sources.md](02-sources.md) |
| §5 | [03-operators.md](03-operators.md) |
| §6 | [04-cli.md](04-cli.md) |
| §7、§8 | [05-formatters-and-types.md](05-formatters-and-types.md) |
| §9 | [06-schema.md](06-schema.md) |
| §10、§12 | [07-output-and-backends.md](07-output-and-backends.md) |
| §11 | [08-errors.md](08-errors.md) |
| §13 | [09-gaps.md](09-gaps.md) |
| §14 | [10-checklist.md](10-checklist.md) |
| §15 | [11-examples.md](11-examples.md) |
| **§16 schema 与 inline 规则（新增）** | [12-schema-and-inline-rules.md](12-schema-and-inline-rules.md) |
| **§17 物化模型** | [13-materialization.md](13-materialization.md) |
| 附录 A | [90-appendix-a-selection.md](90-appendix-a-selection.md) |
| 附录 B | [91-appendix-b-industry.md](91-appendix-b-industry.md) |
| 非目标 | [99-non-goals.md](99-non-goals.md) |
