[English](README.md) | 简体中文

# envsub

**扩展配置渲染器。** GNU `envsubst` 的超集：保留你熟悉的 `${VAR}` 模板语法，同时补上类型化取值、
多来源分层、JSON Schema 校验、安全原地写回，以及适合 CI 的过期检测。

```console
$ envsub render -f values.yaml app.tmpl
server:
  host: 10.0.0.1
  port: 8080
  tags: web, prod
```

> **从 `envsubst` 过来？** 裸名 `${NAME}` 读的是 envsub 的**值树**——值文件、`.env` 文件与
> `--set`——而不是你的 shell 环境变量。进程环境是一个独立的只读命名空间，要用
> `${@process.NAME}` 显式访问。这是上手前唯一需要知道的事，详见
> [值从哪来](#值从哪来)。

## 为什么不用 `envsubst`

`envsubst` 只做字符串替换。真实配置需要更多：

| | GNU `envsubst` | envsub |
|---|---|---|
| 取值 | 只有字符串 | 类型化：int、float、bool、list、map、null |
| 来源 | 进程环境 | 环境变量、`.env` 文件、YAML/JSON 文件、`--set`、JSON Schema 默认值 |
| 分层 | 单一来源 | 确定的优先级，后来的来源胜出 |
| 校验 | 无 | JSON Schema Draft 2020-12，报错带具体路径 |
| 写回 | 无 | 原子替换原地写回、批量输出、过期检测 |
| 格式化 | 无 | `toJson`、`toYaml`、`join`、`quote`、`indent` |
| 缺值 | 变成空串 | 报错、留空、保留原样、写零值——由你选 |

## 安装

```bash
# 从 release 安装（linux/darwin/windows × amd64/arm64）
go install github.com/elizabevil/envsub/cmd/envsub@latest

# 或从源码构建
git clone https://github.com/elizabevil/envsub && cd envsub
just build        # → dist/envsub
```

构建需要 Go 1.27。发布的二进制是静态链接的，除此之外不依赖任何东西。

## 快速上手

写一个模板和一份取值：

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

渲染：

```console
$ envsub render -f values.yaml app.tmpl
server:
  host: 10.0.0.1
  port: 8080
  tags: web, prod
```

`PORT` 保持整数 `8080`——不会自动加引号，也不会做类型转换。想要字符串就明说：
`${PORT | type string}`。

## 用例

### 替换环境变量

进程环境要显式通过 `@process` 命名空间访问：

```bash
cat > config.tmpl <<'EOF'
host: ${@process.HOST}
EOF

HOST=10.0.0.1 envsub render config.tmpl
# host: 10.0.0.1
```

也可以一次放开整个前缀——变量名保持原样，前缀只是一道闸门：

```bash
cat > prefixed.tmpl <<'EOF'
host: ${@process.APP_HOST}
EOF

APP_HOST=10.0.0.1 envsub render --process-env-prefix APP_ prefixed.tmpl
# host: 10.0.0.1
```

模板和输出都随你指定：

```bash
envsub render config.tmpl                 # 模板输出到 stdout
envsub render -o out.yaml config.tmpl     # …或写到文件
cat config.tmpl | envsub render -         # …或从 stdin 读模板
```

### 从文件读类型化的值

```bash
cat > deploy-values.yaml <<'EOF'
replicas: 3
enabled: true
limits: {cpu: 500m, memory: 1Gi}
EOF

cat > deploy.tmpl <<'EOF'
replicas: ${replicas}
enabled: ${enabled}
limits: ${limits | toJson}
EOF

$ envsub render -f deploy-values.yaml deploy.tmpl
replicas: 3
enabled: true
limits: {"cpu":"500m","memory":"1Gi"}
```

### 叠加多个来源

后来的来源胜出。完整层序是 `--set` > `.env` 文件 > 值文件 > schema `default`。

```bash
envsub render \
  -f values.yaml \
  --dotenv-file .env \
  --set PORT=9090 \
  app.tmpl
```

### 用 JSON Schema 校验

```bash
cat > schema.yaml <<'EOF'
type: object
properties:
  port: {type: integer, minimum: 1}
EOF

cat > port.tmpl <<'EOF'
port: ${port}
EOF

envsub render --schema schema.yaml -f good-values.yaml port.tmpl
```

违反 schema 的值会带着出错的路径失败，退出码 4：

```bash
cat > bad-values.yaml <<'EOF'
port: 0
EOF
```

```console
$ envsub render --schema schema.yaml -f bad-values.yaml port.tmpl
schema validation failed: at /port: minimum: got 0, want 1
$ echo $?
4
```

### 决定缺值时怎么办

```bash
envsub render --on-missing=error   app.tmpl   # 报错（默认）
envsub render --on-missing=empty   app.tmpl   # 渲染成空
envsub render --on-missing=keep    app.tmpl   # 保留 ${VAR} 原样
envsub render --on-missing=zero    app.tmpl   # 类型的零值
```

模板里有的必填、有的可选时，用 `keep`。`zero` 需要知道类型——用 schema 或内联
`| type` 给一个，否则它不知道"零"是什么：

```bash
envsub render --on-missing=zero app.tmpl                  # 报错：没有类型
echo 'port: ${PORT | type int}' | envsub render --on-missing=zero -
# port: 0
```

### 渲染整个目录

```bash
envsub render -f values.yaml --output-dir rendered/ --recursive templates/
```

目录结构会被镜像。加 `--strip-suffix .tmpl` 可以去掉输出名上的模板后缀。

### 安全地原地写回

```bash
envsub render --in-place config.yaml
```

替换是原子的：新内容先写进临时文件，再 rename 覆盖原文件，所以中途失败不会留下半个文件。
文件权限会保留。

### 格式化一个值

```bash
envsub render -f values.yaml -o - <<'EOF'
${config | toYaml}
${config | toJson}
${hosts | join ", "}
${name | quote}
${config | toYaml 4}
EOF
```

### 在 CI 里检查生成物是否过期

`--check` 拿目标文件和"当前取值本该渲染出的内容"比对，不写任何东西。它需要知道目标文件，
所以要配合指明目标的输出选项：

```bash
cat > check.tmpl <<'EOF'
port: ${port}
EOF

cat > check-values.yaml <<'EOF'
port: 8080
EOF

envsub render -f check-values.yaml -o config.yaml check.tmpl   # 先生成
envsub render --check -f check-values.yaml -o config.yaml check.tmpl
```

目标文件不一致时退出码是 7，这正适合放进流水线：

```bash
envsub render --check -f check-values.yaml -o config.yaml check.tmpl || exit 1
```

```bash
cat > other-values.yaml <<'EOF'
port: 9090
EOF
```

```console
$ envsub render --check -f other-values.yaml -o config.yaml check.tmpl
1 of 1 outputs are outdated: config.yaml
$ echo $?
7
```

### 看清一个值是从哪来的

```bash
envsub render --explain --no-process-env \
  -f values.yaml --dotenv-file .env --set PORT=9090 app.tmpl
```

每个被引用的变量在 stderr 上得到一行 JSON，说明哪一层胜出、遮蔽了哪些层——**不包含值本身**，
所以可以直接贴进工单。

```console
{"template":"app.tmpl","variable":"PORT","origin":{"kind":"set","method":"flag","source":"PORT","file":"","line":0},"shadowed":[{"kind":"envfile","method":"file","source":".env","file":".env","line":1}]}
```

## 模板语法

```text
$NAME ${NAME}                     变量引用
${app.port}                       嵌套路径
${app["my-key"]}                  带连字符的键
${hosts[0]}                       数组元素
${["a.b"].k}                      顶层带点的键
${@process.PORT}                  进程环境（只读）

${path:-default}                  未设置或为空时用默认值
${path-default}                   未设置时用默认值
${path:+alt}                      有值且非空时用备选值
${path+alt}                       有值时用备选值
${path:?message}                  未设置或为空时带消息失败
${path?message}                   未设置时带消息失败
${path:=default}                  把默认值写回值树
${path=default}                   未设置时把默认值写回值树

$$                                字面量 $
$1                                保持字面量

${hosts | join ", "}              formatter 管道
${PORT | type int}                内联类型断言
${name | quote}                   给字符串加引号
${app | toJson}                   JSON
${app | toYaml}                   YAML
${app | toYaml 4}                 指定嵌套宽度的 YAML
${app | toYaml align}             对齐到表达式所在列的 YAML
${app | toYaml 4 align}
${app | toJson | indent 2}        链式 formatter
```

`toYaml` 可选地接受嵌套宽度和/或 `align`；`toJson` 接受 `align`。`align` 会让结果的每一行
都从表达式所在的那一列开始，这正是让一块内容缩进在父键下面的做法。不加 `align` 时，多行值
按原样插入，后续行保持自己的位置。

内联 `| type` 断言与 schema 取并集：满足其中之一即可，两个都不满足才报错。给了类型时，
它必须是管道的第一个步骤。

## 值从哪来

```text
--set / --set-json / --set-file
  >  .env 文件（--dotenv-file）
  >  YAML/JSON 值文件（-f/--values-file）
  >  schema default
```

同一类里后写的文件胜出；schema 的 `default` 只填补没有任何来源提供的键。

进程环境是一个**独立的只读命名空间**。它永远不会填进裸名——要用就显式用：

```bash
envsub render --no-process-env app.tmpl          # 完全忽略它
envsub render --process-env-prefix APP_ app.tmpl # 只放开 APP_* 变量
```

`.env` 文件一律按 dotenv 解析，跟文件名无关；它们的键落在根命名空间。

## CLI 参考

```text
envsub render [flags] [template...]
envsub version                                      以 JSON 输出构建信息
envsub completion bash|zsh|fish|powershell         生成 shell 补全脚本

值来源
  -f, --values-file FILE      YAML/JSON 值文件，可重复
      --dotenv-file FILE      dotenv 文件（按 dotenv 解析，文件名随意），可重复
      --schema FILE           JSON Schema（Draft 2020-12），JSON 或 YAML
      --set KEY=VALUE         原始字符串值（由 schema 或内联 | type 定型）
      --set-json KEY=JSON      原生 JSON 值
      --set-file KEY=FILE      从文件读入的值，保持字符串
      --no-process-env         忽略进程环境
      --process-env-prefix P   只放开带此前缀的进程变量

渲染
  -o, --output FILE           单个输出文件
  -i, --in-place              原子替换每个模板
      --output-dir DIR        批量输出目录（镜像输入布局）
      --strip-suffix EXT      去掉批量输出名上的这个后缀，可重复
      --recursive             递归遍历模板目录
      --validate-only         装载、物化、校验；不写任何东西
      --check                 报告已过期的目标；不写任何东西
      --explain[=PATH]        在 stderr 上输出 JSON 来源报告（PATH 可限定范围，可重复）
      --report=unused         在 stderr 上列出模板没引用到的值文件条目

策略
      --on-missing=error|empty|keep|zero    默认 error
      --on-empty=ok|error                   默认 ok
      --unknown-keywords=ignore|warn|error  默认 ignore
      --only-vars VAR...                    只替换这些变量
      --ignore-vars VAR...                  绝不替换这些变量

全局
      --log-level=debug|info|warn|error     默认 warn
      --error-format=text|json              默认 text
```

`--error-format=json` 会让 stderr 上的每个失败都是单个 JSON 对象，含 `kind`、`exitCode`
与 `message`，方便工具消费。

## 退出码

| 码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 内部错误或未分类错误 |
| 2 | 用法错误 |
| 3 | 模板语法错误 |
| 4 | 类型或 schema 校验错误 |
| 5 | 文件或 I/O 错误 |
| 6 | 缺值或取值策略错误 |
| 7 | 目标已过期（`--check`） |

## 开发

```bash
just check      # gofmt + vet + lint + 单测/fixture + e2e + 示例
just fmt
just lint       # golangci-lint v2，见 .golangci.yml
just test
just test-race
just test-fuzz  # fuzz 每个解析器（8 个目标，各 30s）
just test-perf  # 性能门禁，需要 GNU envsubst（gettext-base）
just bench
just size       # 二进制体积报告
just build-sonic  # 换成 sonic JSON 后端的同一个二进制（更快、更大）
```

有些共享工具链把默认缓存挂成只读，所以 `just` 把 `GOCACHE`/`GOTMPDIR` 指向仓库内的
`.tmp/` 目录。

### JSON 后端

JSON 走 `internal/jsonx`，有两种编译期实现：

| | 默认（标准库 `encoding/json/v2`） | `-tags=sonic` |
|---|---|---|
| linux/amd64 二进制 | 5.03 MiB | 8.55 MiB |
| 启动 | 4.6 ms | 9.5 ms |
| REF-04 峰值 RSS | 1.30× GNU | 1.80–2.25× GNU |
| JSON 解码（364 KB） | 12.04 ms | 2.45 ms |

默认后端让二进制小 41%，启动与内存也更好；当 JSON 吞吐比体积更重要时用 `-tags=sonic`。
体积数字是在 `go.mod` 指定的 Go 版本上测的，换一个 Go 版本会差几百 KiB，所以要在同一工具链内比较。

YAML 走 `internal/yamlx`，只用 `go.yaml.in/yaml/v3` 一个库：解码遍历节点（整数保持整数、
重复键与合并键都有处理），`toYaml` 编码同一棵树。

## 文档

规范性的行为定义在 [`docs/07-serialization/`](docs/07-serialization/00-index.md)——
每条规则都有编号、可复制的例子与对应测试。完整地图见
[`docs/README.md`](docs/README.md)；被取代的材料在
[`docs/99-archive/`](docs/99-archive/README.md)。

`examples/` 里有覆盖每类规则的可运行案例：

```bash
cd examples
just            # 列出所有案例
just demo       # 全部跑一遍
just full       # 全部类型、来源、操作符与 formatter
```

## 许可证

MIT，见 [LICENSE](LICENSE)。
