---
status: draft-ai
owner: [ 待指定 ]
version: 0.2
last-updated: 2026-10-08
---

# Extended 设计 · 04 Unset、CLI 与控制结构

> 本文是 `03-00-index.md` 的拆分部分。

### 3.7 未设置与空值策略

#### `--on-missing` 的功能目标

`--on-missing` 定义“引用的变量/路径不存在”时的行为，避免 envsubst 类工具静默替换为空导致配置损坏。`[INFERENCE]`

建议选项：

- `--on-missing=error`（推荐默认）：缺失变量/路径直接报错，停止渲染，不写输出。适合生产与 CI。`[ASSUMPTION]`
- `--on-missing=empty`：缺失变量/路径替换为空字符串，不输出任何字符；周围模板文本仍会输出。适合 GNU envsubst 迁移或纯文本模板。
  `[INFERENCE]`
- `--on-missing=keep`：保留原始占位符，例如 `${app.port}` 原样输出；适合多阶段渲染或后续工具继续处理。`[INFERENCE]`
- `--on-missing=zero`：按 schema 类型填充 Go 零值；仅对非 required 字段生效。`[FACT]`（来源：需求提出者确认，2026-09-29）

零值映射建议：

| 类型       | Go 零值 |
|------------|---------|
| `string`   | `""`    |
| `bool`     | `false` |
| `int`      | `0`     |
| `float`    | `0.0`   |
| `array<T>` | `[]`    |
| `object`   | `{}`    |
| `null`     | `null`  |

`--on-missing=zero` 的约束：

- 必须能从 schema 获知类型；没有声明类型时无法推断，应报错。`[INFERENCE]`
- `schema.required: true` 且无 default 时，直接报错，不自动填零值；需要零值请显式写 `default: 0` 或把字段设为非 required。
  `[FACT]`（来源：需求提出者确认，2026-09-29；安全大于约定）
- 填零值后仍需通过 schema 约束；例如 `minimum: 1` 的 `int` 填 `0` 会校验失败。`[INFERENCE]`
- required 字段永远不自动填零，边界数据安全优先于零值约定。`[FACT]`（来源：需求提出者确认，2026-09-29）
- `--on-missing=empty` 与 schema 非 string 类型冲突时，建议报类型错误并提示改用 `zero` 或 default。`[ASSUMPTION]`

优先级与处理顺序建议：

> 注：以下是**处理阶段**顺序，不是值来源优先级。值来源优先级见 `07-serialization/04-cli.md` §6.2：
> `--set`/`--set-json` > YAML/JSON 值文件 > `.env` > 进程环境 > schema `default`（兜底）。`[FACT]`
> （来源：需求提出者确认，2026-10-10）

1. 模板显式操作符：`${path:-default}`、`${path-default}`。
2. schema `default`（只对缺失键生效）。
3. schema `required` 校验：required 且无 default 时直接报错。
4. `--on-missing` 策略：`error` / `empty` / `keep` / `zero`。
5. 类型断言与 schema 约束校验。
6. 渲染输出。

- `${path:?message}`、`${path?message}` 始终报错。`[INFERENCE]`
- `--on-empty` 只控制“已设置但为空”的情况，默认 `ok`；schema 可用 `minLength: 1` 或 `nonEmpty` 约束为空值错误。`[INFERENCE]`
- P1 条件/循环中的缺失路径行为需在 P1 设计时单独定义，默认建议沿用 `error`。`[TODO]`

默认值仍待确认：建议 `--on-missing=error`，待需求提出者确认。`[ASSUMPTION]`

### 3.8 CLI 草案

```text
tool render [flags] [template...]

值来源:
  -f, --values-file FILE...       YAML/JSON values，可多次
      --dotenv-file FILE...  dotenv 文件（按 dotenv 解析，文件名任意），可多次；写入根命名空间
      --schema FILE          schema 文件
      --set KEY=VALUE        按 schema 解析
      --set-string KEY=VALUE 已删除；强制字符串改用 --set-json KEY='"value"'
      --set-json KEY=JSON    原生 JSON
      --set-file KEY=FILE    从文件读取字符串（设计草案，未实现；P1）

渲染:
      --output FILE
  -i, --in-place             安全原地写回
      --output-dir DIR       多文件输出目录
      --recursive            递归模板
      --dry-run
      --diff
      --check
      --validate-only

策略:
      --on-missing=error|empty|keep|zero
      --on-empty=ok|error
      --unknown-keywords=ignore|warn|error
      --only-vars VAR...
      --ignore-vars VAR...
      --json-errors
```

说明：

- `--unknown-keywords` 默认 `ignore`；未知非 `x-*` 关键字默认不报错，以保证 schema 可跨系统复用。`[FACT]`（来源：ADR-003）
- **2026-10-08 修订**：参数名改为直接表达「触发条件 / 作用对象」——`--unset` → `--on-missing`、
  `--empty` → `--on-empty`、`--schema-unknown` → `--unknown-keywords`、`--only` → `--only-vars`、
  `--ignore` → `--ignore-vars`；不保留旧名别名。`[FACT]`
- 一期不提供 `--compat`。`[FACT]`（来源：ADR-002）

### 3.9 受限表达式、条件、循环与 Includes（P1）

为覆盖多环境配置与 YAML/JSON 结构渲染，P1 支持受限条件、循环与 include，但必须保持严格、可预测、非图灵完备。`[FACT]`
（来源：需求提出者确认 P1 实现 include，2026-09-29）

#### 表达式

- 字面量：`true`、`false`、`null`、数字、双引号字符串。`[INFERENCE]`
- 路径：`app.port`、`hosts[0]`、`env.PORT`。`[INFERENCE]`
- 运算符：`!`、`==`、`!=`、`<`、`<=`、`>`、`>=`、`&&`、`||`、`in`、`not in`。`[INFERENCE]`
- 内置谓词：`exists(path)`、`empty(path)`、`len(path)`。`[INFERENCE]`
- 不允许用户自定义函数、递归、赋值、循环变量修改、shell/网络调用。`[INFERENCE]`

#### 条件

```text
${if app.debug}
debug: true
${elseif app.env == "prod"}
debug: false
${else}
debug: false
${endif}
```

- `${if expr}` / `${elseif expr}` / `${else}` / `${endif}` 必须成对且顺序合法。`[INFERENCE]`
- 条件表达式类型不合法或未知 path 行为由 `--on-missing` / `--strict` 策略决定。`[ASSUMPTION]`

#### 循环

```text
servers:
${for host in app.hosts}
  - ${host}
${endfor}
```

- 只支持 `for ... in array` 与 `for key, value in object`。`[INFERENCE]`
- 不支持 `while`、`break`、`continue`、递归。`[INFERENCE]`
- object 遍历默认按键排序，保证确定性输出。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 可配置最大迭代次数，防止异常模板导致资源耗尽。`[INFERENCE]`

#### Includes（P1）

- P1 实现 include/partials。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 执行时通过命令行指定允许的文件/目录；具体参数名在 P1 设计时确定。`[FACT]`
- 候选默认最大嵌套深度为 3 层，例如 A -> B -> C；P1 最终确认。`[ASSUMPTION]`
- 需要循环 include 检测、路径白名单与错误格式；禁止网络 URL、绝对路径逃逸、环境变量拼接路径。`[INFERENCE]`

#### 严格性

- 已确认所有控制指令必须单独成行（允许前导空白），避免与普通文本插值混淆；插值 `${...}` 仍可内联。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- 未闭合、顺序错误、未知 formatter/谓词、类型不匹配一律报错退出。`[INFERENCE]`
- 控制结构仍属于“受限配置渲染”，不扩展为通用编程语言。`[FACT]`（来源：需求提出者确认非目标）

## 非目标（本部分）

- 不替代整体设计评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `03-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
