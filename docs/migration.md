---
status: active
owner: 产品负责人
version: 0.1
last-updated: 2026-10-22
---

# 迁移指南：从 GNU envsubst 到 envsub

envsub **不是** `envsubst` 的 drop-in 替代品：它多了一层"值来源"的概念，变量名不再直接对应
进程环境。本文说明差在哪、怎么迁，以及迁过去会看到哪些错误。

二期会提供兼容模式与自动转换命令（见 §8）。在那之前，本文是唯一的迁移依据。

## 1. 三分钟迁移

大多数用法只需要做两件事：

**第一步：把"环境变量"改成"值文件"。** 这是最常见的迁移动作。

```bash
# 原来
PORT=8080 envsubst < app.conf.tmpl > app.conf

# 改成
cat > values.yaml <<'EOF'
port: 8080
EOF
envsub render -f values.yaml app.conf.tmpl > app.conf
```

**第二步：确认模板里没有依赖进程环境的裸名。** 如果有，把它们改成 `${@process.NAME}`（见 §2）。

如果你的模板只用 `$VAR` / `${VAR}`，并且变量本来就来自一个值文件，那么**模板一个字都不用改**。

## 2. 最重要的一条差异：裸名读值树，进程环境走 `@process`

这是最容易踩的一条，也是与 `envsubst` 语义差别最大的一条。

| 写法 | envsub 读的是 |
|---|---|
| `${PORT}` | **值树**：值文件、`.env` 文件、`--set` |
| `${@process.PORT}` | **进程环境** |

```console
$ echo 'x=${PORT}' | PORT=8080 envsub render -
variable "PORT" is not set
$ echo $?
6
```

`envsubst` 会把 `${PORT}` 替换成进程环境里的 `8080`；envsub 不会——它认为 `PORT` 是一个配置项，
而配置项来自值文件。要读进程环境必须显式写命名空间：

```console
$ echo 'x=${@process.PORT}' | PORT=8080 envsub render -
x=8080
```

命名空间只有一个：`@process`。它只读、只写进程环境，不参与值来源的层序。

进程环境还可以整段放开或收紧：

```bash
envsub render --no-process-env app.tmpl              # 完全不读进程环境
envsub render --process-env-prefix APP_ app.tmpl     # 只放开 APP_ 开头的变量（保留原名，不剥离前缀）
```

## 3. 语法对照

| 写法 | GNU envsubst | envsub | 说明 |
|---|---|---|---|
| `$VAR`、`${VAR}` | 支持 | 支持 | 语义不同，见 §2 |
| `${VAR:-default}` | 不支持 | 支持 | 未设置**或为空**时用 default |
| `${VAR-default}` | 不支持 | 支持 | 仅未设置时用 default |
| `${VAR:+other}` | 不支持 | 支持 | 有值且非空时用 other |
| `${VAR+other}` | 不支持 | 支持 | 仅未设置时用 other |
| `${VAR:?msg}` | 不支持 | 支持 | 未设置或为空时报错退出（exit 6） |
| `${VAR:=default}` | 不支持 | 支持 | 把 default **写回值树**，后续引用看到它 |
| `${VAR=default}` | 不支持 | 支持 | 同上，仅未设置时写回 |
| `$$` | 保留为 `$$` | 变成 `$` | envsub 把 `$$` 当作转义；`$1` 两者都保持字面量 |
| 未设置的变量 | 替换为空串 | **报错退出 6** | 用 `--on-missing=empty` 改回空串 |
| 变量白名单 | 位置参数 `envsubst '$VAR'` | `--only-vars` / `--ignore-vars` | |
| `SHELL-FORMAT` 环境变量 | 支持 | 不支持 | |
| 类型化取值 | 不支持 | 支持 | YAML/JSON 值文件保留 int/float/bool/list/map |
| JSON Schema 校验 | 不支持 | 支持 | `--schema` |
| 写回文件 | 不支持 | 支持 | `-i` 原子替换、`--output-dir` 批量 |

envsub 还多了一组**格式化管道**，GNU 没有对应物：

```text
${value | join ", "}     列表拼成字符串
${value | toJson}        JSON
${value | toYaml}        YAML（可带嵌套宽度 N 与 align）
${value | quote}         加引号
${value | indent 2}      缩进
${value | type int}      内联类型断言
```

完整语法见 [`../07-serialization/03-operators.md`](07-serialization/03-operators.md) 与
[`../07-serialization/05-formatters-and-types.md`](07-serialization/05-formatters-and-types.md)。

## 4. 值来源与层序

envsub 的值来自多个来源，优先级从高到低：

```text
--set / --set-json / --set-file
  >  .env 文件（--dotenv-file）
  >  YAML/JSON 值文件（-f/--values-file）
  >  schema 的 default
```

- 同一类里**后写的胜**。
- schema 的 `default` 只填补**没有任何来源提供**的键。
- 进程环境不在层序里（§2）。

`.env` 文件一律按 dotenv 解析，跟文件名无关；`--values-file` 只收 YAML/JSON，收 `.env` 会直接报错
（§6）。

## 5. `.env` 的代价：它的键会进 schema 校验面

`.env` 的键落在**根命名空间**，并且**和值文件的键一样要过 schema 校验**。这带来两个后果：

**一、宿主变量不能放在 `.env` 里。** 如果 schema 用了 `additionalProperties: false`，
`.env` 里的 `PATH`、`HOME`、`CI` 这类与配置无关的变量会被判为非法：

```console
$ envsub render --schema s.json --dotenv-file .env4 - <<< 'x=${PATH}'
schema validation failed: at /: additional properties 'PATH' not allowed
$ echo $?
4
```

`.env` 应该是"这个部署需要的配置"，不是"这个机器的环境"。机器上已有的变量已经在进程环境里，
用 `${@process.NAME}` 读。

**二、`.env` 的值会按 schema 类型物化。** 同名但类型不符会直接失败：

```console
$ envsub render --schema s.json --dotenv-file .env5 - <<< 'x=${port}'
port at .env5:1: expected an integer, got "notanint"
$ echo $?
4
```

## 6. 常见错误与含义

| 错误信息 | 退出码 | 原因 | 怎么办 |
|---|---|---|---|
| `variable "X" is not set` | 6 | `X` 在值树里没有 | 加值文件/`.env`/`--set`；或本意是读进程环境，改成 `${@process.X}`；或用 `--on-missing=empty` |
| `variable "X" is empty` | 6 | `X` 存在但为空，且 `--on-empty=error` | 换 `--on-empty=ok`，或给个默认值 |
| `--on-missing=zero requires a schema type or an inline \| type` | 6 | `--on-missing=zero` 不知道"零"是什么 | 给 schema 类型，或写 `${X \| type int}` |
| `--values-file accepts YAML/JSON value files, not .env: use --dotenv-file for .env files` | 2 | 把 `.env` 传给了 `-f` | 改用 `--dotenv-file` |
| `schema validation failed: at /path: ...` | 4 | 值不满足 schema | 看 `at /path` 定位到具体键 |
| `expected an integer, got "..."` | 4 | 字符串值物化不成 schema 的类型 | 改值，或在模板里用 `\| type` 换类型 |
| `unknown command "x" for "envsub"` | 2 | 子命令名拼错 | 看 `Did you mean this?` 提示 |
| `unclosed variable expression` | 3 | `${` 没有对应的 `}` | 检查模板 |
| `read template ...: no such file` | 5 | 模板文件不存在 | 检查路径 |

## 7. 兼容基线在哪

envsub 与 GNU/a8m/drone envsubst 的**逐项行为差异**记录在
[`reference/REF-02-envsubst-compatibility-baseline.md`](reference/REF-02-envsubst-compatibility-baseline.md)。
那张表是二期兼容模式的依据，也回答"为什么这里不跟 GNU 一样"。

一句话总结差异来源：GNU/a8m/drone 假定"变量就是环境里的字符串"，envsub 假定"变量是配置树里的
一个有类型的节点"。前者简单，后者能校验。

## 8. 二期计划

- **兼容模式**（TASK-201）：`--compat=gnu|a8m|drone`，用一套 conformance 测试钉住与各家的行为。
- **转换命令**（TASK-202）：`envsub migrate` / `envsub convert`，把现有模板与 `.env` 转成 envsub 的写法。
- 二期的触发条件是"出现明确的迁移需求"。一期没有实现这些，所以本文是当前的迁移依据。

## 9. 相关文档

- [`04-decisions/04-02-adr-002-compatibility-and-migration.md`](04-decisions/04-02-adr-002-compatibility-and-migration.md)：一期不做兼容的决策与后果
- [`reference/REF-02-envsubst-compatibility-baseline.md`](reference/REF-02-envsubst-compatibility-baseline.md)：各家 envsubst 的行为基线
- [`07-serialization/04-cli.md`](07-serialization/04-cli.md)：CLI 契约（C 系列规则）
- [`07-serialization/13-materialization.md`](07-serialization/13-materialization.md)：值来源、物化与命名空间
- [`../README.md`](../README.md)：快速上手与用例
