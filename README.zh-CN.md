[English](README.md) | 简体中文

# envsub

**扩展配置渲染器。** GNU `envsubst` 的超集：保留 `${VAR}` 模板语法，同时补上类型化取值、
多来源分层、JSON Schema 校验、安全原地写回，以及适合 CI 的过期检测。

```console
$ envsub render -f values.yaml app.tmpl
server:
  host: 10.0.0.1
  port: 8080
  tags: web, prod
```

> **从 `envsubst` 迁移？** 裸名 `${NAME}` 从 envsub 的**值树**（值文件、`.env` 文件与
> `--set`）取值，不读取 shell 环境变量。进程环境是独立的只读命名空间，通过
> `${@process.NAME}` 访问。见[进程环境](#进程环境)。

## 为什么我们更推荐 envsub

若仅需单层 `${VAR}` 替换，`envsubst` 是合适的选择。当配置需要类型化、多来源、Schema
校验和安全写回时，我们更推荐 envsub：

| | 经典 `${VAR}` 替换 | envsub |
|---|---|---|
| 取值 | 字符串 | 类型化：int、float、bool、list、map、null |
| 来源 | 进程环境 | 环境变量、`.env` 文件、YAML/JSON 文件、`--set`、JSON Schema 默认值 |
| 分层 | 单一来源 | 确定的优先级，后指定的文件优先级更高 |
| 校验 | — | JSON Schema Draft 2020-12，报错带具体路径 |
| 写回 | — | 原子替换原地写回、批量输出、过期检测 |
| 格式化 | — | `toJson`、`toYaml`、`join`、`quote`、`indent` |
| 缺值 | 空串 | 报错、留空、保留原样、写零值——可选 |

## 安装

```bash
# 从 release 安装（linux/darwin/windows × amd64/arm64）
go install github.com/elizabevil/envsub/cmd/envsub@latest

# 或从源码构建
git clone https://github.com/elizabevil/envsub && cd envsub
just build        # → dist/envsub
```

构建需要 Go 1.27。发布的二进制为静态链接，无其它运行时依赖。

## 快速上手

创建模板与值文件：

```bash
cat > app.tmpl <<'EOF'
server:
  host: ${HOST}
  port: ${PORT}
  tags: ${TAGS | join ", "}
EOF

cat > values.yaml <<'EOF'
HOST: 10.0.0.1
PORT: 8080
TAGS: [web, prod]
EOF
```

渲染模板：

```console
$ envsub render -f values.yaml app.tmpl
server:
  host: 10.0.0.1
  port: 8080
  tags: web, prod
```

`PORT` 以整数 `8080` 输出，无需引号或额外类型转换。强制为字符串时使用
`${PORT | type string}`。

## 完整案例

仓库提供可运行的数据集与模板：

```text
examples/
├── data/
│   ├── values-base.yaml
│   ├── values-prod.yaml
│   ├── values-extra.json
│   ├── app.env
│   └── schema.yaml
├── templates/
│   ├── realworld.tmpl
│   └── scenarios-*.tmpl
├── out/                 # 已提交的期望产物
└── justfile             # 每个场景一个 recipe
```

```bash
ENVSUB_STRING=from-process envsub render \
  -f examples/data/values-base.yaml \
  -f examples/data/values-prod.yaml \
  -f examples/data/values-extra.json \
  --dotenv-file examples/data/app.env \
  --schema examples/data/schema.yaml \
  --on-missing=empty \
  examples/templates/realworld.tmpl
```

`>` 表示左侧优先级高于右侧：

```text
--dotenv-file examples/data/app.env > -f examples/data/values-extra.json > -f examples/data/values-prod.yaml > -f examples/data/values-base.yaml > schema default
```

`ENVSUB_STRING` 属于进程环境（`@process`），不属于该优先级顺序。

输出：

```yaml
service:
  name: prod-string
  replicas: 7
  enabled: true
  region: prod
  endpoints: ["prod","a","c"]
  labels: {"a":2,"m":3,"z":9}
  started_by: from-process
```

每个 recipe、golden 和失败案例的对应关系见
[`examples/README.md`](examples/README.md)。

## 模板语法

```text
$NAME ${NAME}                    变量引用
${app.port}                      嵌套路径
${app["my-key"]}                 带连字符的键
${hosts[0]}                      数组元素
${["a.b"].k}                     带点号的顶层键
${@process.PORT}                 进程环境（只读）

${path:-default}                 未设置或为空时用 default
${path-default}                  仅未设置时用 default
${path:+alt}                     有值且非空时用 alt
${path+alt}                      仅已设置时用 alt
${path:?message}                 未设置或为空时报错
${path?message}                  仅未设置时报错
${path:=default}                 把 default 写入值树
${path=default}                  仅未设置时写入 default

$$                               字面量 $
$1                               保持原样

${hosts | join ", "}             formatter 管道
${PORT | type int}               内联类型断言
${name | quote}                  给字符串加引号
${app | toJson}                  JSON
${app | toYaml}                  YAML
${app | toYaml 4}                指定缩进宽度的 YAML
${app | toYaml align}            与表达式所在列对齐的 YAML
${app | toYaml 4 align}
${app | toJson | indent 2}       链式 formatter
```

`toYaml` 可带缩进宽度和/或 `align`；`toJson` 可带 `align`。`align` 让结果的每一行
从表达式所在列继续，适合嵌在父键下面。不带 `align` 时，多行值原样插入，后续行保持
自己的位置。

内联 `| type` 与 schema 是“任一满足即可”的关系：两者都不满足才报错。给类型时，
它必须是管道的第一个步骤。

## 使用说明

### render 命令模板

```text
envsub render [flags] [template...]
```

`[template...]` 接受文件、目录或 `-`（stdin）；为空时读取 stdin。

单模板示例：

```bash
envsub render -f values/base.yaml app.yaml.tmpl
```

写入单个文件：

```bash
envsub render -f values/base.yaml -o rendered/app.yaml app.yaml.tmpl
```

### 参数

按用途分组。值来源和优先级规则见后面的表。

| 分组 | 参数 |
|---|---|
| 值来源 | `-f/--values-file`、`--dotenv-file`、`--schema`、`--set`、`--set-json`、`--set-file` |
| 输出 | `-o/--output`、`--output-dir`、`--strip-suffix`、`--recursive` |
| 写回与检查 | `-i/--in-place`、`--check` |
| 策略 | `--on-missing`、`--on-empty`、`--only-vars`、`--ignore-vars`、`--unknown-keywords` |
| 报告 | `--explain`、`--report`、`--jsonl`、`--error-format` |

完整参数列表见 `envsub render --help`。

### 值来源

| 参数 | 输入 | 处理方式 |
|---|---|---|
| `-f/--values-file FILE` | YAML 或 JSON 值文件 | 合并进根命名空间；可重复，后 > 前 |
| `--dotenv-file FILE` | dotenv 文件 | 载入根命名空间；可重复，后 > 前；> `-f/--values-file` |
| `--schema FILE` | JSON Schema | 填充默认值、物化声明的类型、校验最终值树 |
| `--set KEY=VALUE` | 原始字符串 | 按字符串保存；可由 schema 或 `\| type` 转换 |
| `--set-json KEY=JSON` | JSON 字面量 | 解析为 JSON：数字、布尔、数组、对象、null |
| `--set-file KEY=FILE` | 文件内容 | 按字符串保存；去掉 BOM 和一个结尾换行 |

`--set`、`--set-json`、`--set-file` 都写入根命名空间；带点号的 key 会创建嵌套对象。
同一参数内重复指定 key 时，后指定的值优先。三个参数之间的顺序固定，与命令行出现顺序无关：

```text
--set  →  --set-json  →  --set-file
```

同一 key 由三者同时提供时，`--set-file` 的值最终生效。

示例：

```bash
# 普通字符串覆盖
envsub render --set image.tag=2026.10 app.yaml.tmpl

# JSON 类型：数组
envsub render --set-json 'app.hosts=["a","b"]' app.yaml.tmpl

# 从文件读多行密钥
envsub render --set-file tls.crt=secrets/tls.crt app.yaml.tmpl
```

`--set-json` 遇到非法 JSON 按类型错误处理（exit 4）。`--set-file` 无法读取文件时按 IO
错误处理（exit 5）；它保留内部换行，只去掉一个结尾换行和开头的 BOM。

### 值的优先级

`>` 表示左侧优先级高于右侧：

```text
--set-file > --set-json > --set > --dotenv-file (后 > 前) > -f/--values-file (后 > 前) > schema default
```

优先级示例：`--set image.tag=2026.10` > `values/prod.yaml (2026.09)` > `values/base.yaml (canary)` > `schema default`。

`@process.*` 是独立的只读命名空间，不参与该优先级顺序。`--only-vars` 和
`--ignore-vars` 只决定是否替换引用，不改变来源优先级。

### 进程环境

`${@process.NAME}` 读取一个进程环境变量。裸名 `${NAME}` 永远不读进程环境，只读值树。

| 参数 | 作用 |
|---|---|
| `--no-process-env` | 让 `@process` 命名空间为空 |
| `--process-env-prefix PREFIX` | 只暴露名称以 `PREFIX` 开头的进程变量；变量名保持原样，不剥离前缀 |

`--process-env-prefix` 只过滤 `@process` 命名空间，不读取、不重命名、也不过滤
`.env` 文件或值树。空前缀表示保留全部进程变量。`--no-process-env` 与
`--process-env-prefix` 互斥。

### 单模板

未指定输出选项时，渲染结果写入 stdout：

```bash
envsub render -f values/base.yaml app.yaml.tmpl
```

`-o/--output FILE` 写入单个文件：

```bash
envsub render -f values/base.yaml -o rendered/app.yaml app.yaml.tmpl
```

用 `-` 从 stdin 读模板：

```bash
envsub render -f values/base.yaml - < app.yaml.tmpl
```

### 批量渲染

批量输出为独立模式，生成输出目录树，不与 `-o` 或 `-i` 混用。

```bash
envsub render \
  -f values/base.yaml \
  --output-dir rendered \
  --strip-suffix .tmpl \
  --recursive templates/
```

| 参数 | 含义 |
|---|---|
| `--output-dir DIR` | 每个模板在 `DIR` 下写一个产物 |
| `--recursive` | 递归遍历模板目录 |
| `--strip-suffix EXT` | 输出名去掉 `EXT`；需要配合 `--output-dir` |

输入目录布局会镜像到 `--output-dir` 下。批量渲染与 `-o/--output` 互斥，也与
`-i/--in-place` 互斥。

### 写回与检查

`-i/--in-place` 原子替换每个输入文件：新内容先写入临时文件，再 rename 覆盖目标；
中断时不会留下部分写入的文件，权限位保留。

```bash
envsub render -i -f values/base.yaml app.yaml
```

`--check` 在内存中渲染并与现有产物比较，不写任何文件。该选项需要输出目标，必须与
`-o`、`--output-dir` 或 `-i` 组合使用。至少有一个产物过期时 exit 7。

```bash
envsub render --check -f values/base.yaml -o rendered/app.yaml app.yaml.tmpl
```

### 校验、缺值与退出码

`--schema` 填充默认值、物化声明类型并校验最终值树。`--validate-only` 做同样的加载和
校验，但不渲染。

`--on-missing` 控制缺值：

| 值 | 效果 |
|---|---|
| `error` | 报错（默认） |
| `empty` | 渲染为空串 |
| `keep` | 保留原引用 |
| `zero` | 渲染该类型的零值；需要 schema 或内联类型 |

`--on-empty=error` 在普通引用解析为空值时失败；`0` 和 `false` 不算空。默认是 `ok`。

| 退出码 | 含义 |
|---|---|
| 0 | 成功 |
| 2 | 用法或语法错误 |
| 3 | 模板错误 |
| 4 | 类型或 schema 校验错误 |
| 5 | IO 错误 |
| 6 | 缺值/空值策略错误 |
| 7 | `--check` 发现产物过期 |

### 报告

`--explain[=PATH]` 按变量向 stderr 写 JSON 行，说明最终来源和被它覆盖的层，不输出值本身。

```bash
envsub render --explain -f values/base.yaml app.yaml.tmpl
```

`--report=unused` 报告值树中定义、但模板未引用的变量。`--jsonl` 输出统一的 CI 事件流；
`--explain` 和 `--report` 会自动启用。`--error-format=json` 把错误输出为 JSON。

事件封装和完整参数参考见
[`docs/07-serialization/04-cli.md`](docs/07-serialization/04-cli.md)。

## 文档

- [`examples/README.md`](examples/README.md) — 可运行数据集、recipe、golden
- [`docs/migration.md`](docs/migration.md) — 从 GNU `envsubst` 迁移
- [`docs/07-serialization/04-cli.md`](docs/07-serialization/04-cli.md) — CLI 与协议参考
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — 构建、测试、lint 与开发说明

## 许可证

MIT，见 [`LICENSE`](LICENSE)。
