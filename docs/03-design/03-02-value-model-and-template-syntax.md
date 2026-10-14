---
status: draft-ai
owner: [ 待指定 ]
version: 0.2
last-updated: 2026-10-10
---

# Extended 设计 · 02 值模型与模板语法

> 本文是 `03-00-index.md` 的拆分部分。

### 3.1 值模型与命名空间

建议采用两个顶层命名空间：

- **`@process`**：只读的**进程**环境变量，类型均为 string，只能经 `${@process.NAME}` 读取
  。`[FACT]`（来源：[`06-15`](../06-review/06-15-namespace-and-layering.md) §2）
- 根命名空间：由 `.env`（`--dotenv-file`）、结构化 values 文件、CLI `--set*` 合并后的值树。`[FACT]`

路径规则：

- 未加引号的标识符：`[A-Za-z_][A-Za-z0-9_]*`； **不允许 `-`**，避免与 `${path-default}` 的 `-` 操作符冲突。`[INFERENCE]`
- `@` 只允许作为**路径首字节**，用于选择命名空间（当前唯一合法值 `@process`）；出现于其它位置报 `KindTemplate` exit 3。`[FACT]`
- 路径： `("@" namespaceName)? ( identifier | "[" quoted-string "]" ) ( "." identifier | "[" integer "]" | "[" quoted-string "]" )*`。`[INFERENCE]`
- 含 `-`、`.`、`/`、空格等特殊字符的 YAML/JSON key，必须使用 quoted-string：`${app["my-key"]}`、
  `${annotations["app.kubernetes.io/name"]}`；**顶层**特殊键用引导索引：`${["a.b"].k}`、`${["a-b"]}`。`[FACT]`
- 示例：`${app.port}`、`${hosts[0]}`、`${app.labels["env"]}`、`${@process.PORT}`、`${@process["MY-VAR"]}`、`${["a.b"].k}`。`[FACT]`
- **无分层回退**：`${NAME}` **恒读根**，根缺失即按缺失处理（`--on-missing`），不读环境变量。`[FACT]`
- `${env.NAME}` 不再是命名空间写法： `env` 是根对象里的**普通键**（`${env.NAME}` = 根里 `env` 的 `NAME`）。`[FACT]`

### 3.2 内部类型

一期建议支持以下运行时类型：

| 类型       | 说明                                |
|------------|-------------------------------------|
| `null`     | 空值 / 配置缺省                     |
| `string`   | Go `string`                         |
| `bool`     | Go `bool`，只接受 `true` / `false`  |
| `int`      | Go `int`；P0 不区分 int8/16/32/64   |
| `float`    | Go `float64`                        |
| `array<T>` | Go slice / JSON array，元素类型为 T |
| `object`   | `map[string]any` / JSON object      |

整体值树建议定义为 `any`，每个节点保存值、类型和来源信息。类型名称与 Go 基础类型保持一致，`null` 作为配置缺省语义补充。
`[INFERENCE]`

### 3.3 模板语法

#### 3.3.1 基础替换

- `$NAME`：简单标识符替换，等价于 `${NAME}`。`[INFERENCE]`
- `${path}`：路径替换。`[INFERENCE]`
- `$1`、`$2` 不作为变量，按字面量输出。`[FACT]`（依据：避免 a8m #38 的误替换问题）
- 变量名以字母或下划线开头，数字开头不视为变量。`[INFERENCE]`

#### 3.3.2 转义

- `$$` 输出一个字面量 `$`。`[INFERENCE]`
- 需要输出字面量 `${` 时写作 `$${`。`[INFERENCE]`
- 一期不支持 `\$` 作为转义；`\` 保持字面量，避免与 Windows/Shell 语义冲突。`[ASSUMPTION]` 验证：确认是否必须支持反斜杠转义；owner：技术负责人。
- `${` 后不是合法表达式时，默认解析失败；不静默保留。`[INFERENCE]`

#### 3.3.3 默认值与存在性操作

默认值/存在性操作建议采用 **受限 POSIX shell parameter expansion 子集**，而不是 GNU envsubst 语义；GNU envsubst
本身不支持这些语法。Phase 2 如果实现兼容模式，再按对应基线单独处理。`[INFERENCE]`

建议支持以下操作符：

| 语法                 | 语义                                                 |
|----------------------|------------------------------------------------------|
| `${path:-default}`   | path 未设置或值为空时使用 default                    |
| `${path-default}`    | path 未设置时使用 default                            |
| `${path:+alternate}` | path 已设置且非空时使用 alternate，否则为空          |
| `${path+alternate}`  | path 已设置时使用 alternate，否则为空                |
| `${path:?message}`   | path 未设置或为空时渲染失败                          |
| `${path?message}`    | path 未设置时渲染失败                                |
| `${path:=default}`   | 作为 `${path:-default}` 的别名处理，不做环境变量写回 |
| `${path=default}`    | 作为 `${path-default}` 的别名处理，不做环境变量写回  |

说明：

- `default` / `alternate` / `message` 内部允许嵌套 `${...}`，需要支持括号配对解析。`[INFERENCE]`
- 在 `:-`、`-`、`:+`、`+`、`:?`、`?`、`:=`、`=` 之后，直到与 `${` 配对的 `}` 之前，都视为默认词；其中的 `:`、`-`、`/` 等按字面量处理。例如
  `${URL:-http://host:8080}`、`${N:--1}` 应分别得到 URL 和 `-1`。`[INFERENCE]`
- 因为未加引号路径不允许 `-`，`${path-default}` 不会与带 `-` 的 key 混淆；带 `-` 的 key 必须用 `${app["my-key"]}`。
  `[INFERENCE]`
- `:=` / `=` 不产生副作用，只影响当前渲染结果。`[INFERENCE]`
- 一期不承诺 bash 的 offset、大小写转换、字符串裁剪等操作；如后续需要，作为扩展操作符另议。`[ASSUMPTION]` 验证：确认 P0
  是否需要这些能力；owner：需求提出者。

#### 3.3.4 结构化值渲染

- 标量默认按文本输出。`[INFERENCE]`
- 已确认支持 Helm-like pipeline 渲染，避免 `:` 同时表示默认值和格式化：`[FACT]`（来源：需求提出者确认，2026-09-29）
    - `${hosts | join ","}` -> 逗号连接
    - `${hosts | toJson}` -> 紧凑 JSON 数组
    - `${app | toYaml}` -> YAML 字符串
    - `${name | quote}` -> 带引号字符串
    - `${app | toYaml | indent 2}` -> 对 YAML 每一行行首增加 2 个空格
    - 支持链式：`${hosts | join "," | quote}`。`[INFERENCE]`
- 管道右侧只允许固定内置 formatter 白名单，不允许用户自定义函数、任意代码或网络访问。`[INFERENCE]`
- 旧草案中的 `${hosts:json}` 建议弃用；`:` 只用于默认值/存在性操作符，避免与类型/默认值混淆。`[INFERENCE]`
- 示例：

```text
hosts = ${hosts | toJson}
app:
  port: ${app.port}
```

- 已确认支持 `indent`；`indent N` 要求在每一行行首插入 N 个空格，N 必须为非负整数字面量。`[FACT]`（来源：需求提出者确认，2026-09-29）
- `indent` 只接受字符串输入；非字符串应先经 `toYaml`/`toJson`/`join` 等转换成字符串。`[INFERENCE]`
- formatter 白名单与参数已定：`join`、`toJson`、`toYaml`、`quote`、`indent`（另有内联 `| type`），
  参数与转义规则见 `07-serialization/05-formatters-and-types.md`；实现见 `internal/pipeline`。`[FACT]`

## 非目标（本部分）

- 不替代整体设计评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `03-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
