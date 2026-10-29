---
status: draft-ai
owner: [ 待指定 ]
version: 0.2
last-updated: 2026-10-08
---

# CLI 设计

## 1. 目标

- 提供稳定、可脚本化、适合 CI/CD 的 CLI。`[INFERENCE]`
- 一期只实现 P0 命令；P1/P2 预留扩展位置。`[FACT]`
- 使用 `github.com/spf13/cobra`。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 2. 命令树

```text
envsub
├── render [flags] [template...]       # P0：渲染模板
├── version                            # P0：版本信息
├── completion [bash|zsh|fish|powershell]  # P0：shell completion
├── help                               # Cobra 内置
└── P1/P2 预留：validate / migrate / convert / include?
```

一期不提供 `--compat`；P2 在 `render` 上增加兼容模式。`[FACT]`

## 3. 全局参数

| 参数               | 默认  | 说明                      |
|--------------------|-------|---------------------------|
| `--log-level=debug\|info\|warn\|error` | `warn` | 日志级别 |
| `--error-format=text\|json` | `text` | 错误输出的格式；`json` 是机器模式的兼容别名，输出 `type:"error"` 事件 |
| `--jsonl` | `false` | stderr 统一为带 `type` 的 JSONL 事件流；`--explain`/`--report` 自动启用，`--jsonl=false` 关闭 |

## 4. render 参数

### 值来源

| 参数                     | 说明                          |
|--------------------------|-------------------------------|
| `-f, --values-file FILE...`   | YAML/JSON values 文件，可多次 |
| `--dotenv-file FILE...`  | `.env` 文件（强制 dotenv 解析），可多次；写入**根命名空间**，层序高于 `-f` |
| `--schema FILE`          | JSON Schema 文件              |
| `--set KEY=VALUE`        | 按 schema 解析的类型化覆盖    |
| ~~`--set-string KEY=VALUE`~~ | 已删除；强制字符串改用 `--set-json KEY='"value"'` |
| `--set-json KEY=JSON`    | 原生 JSON 覆盖                |
| `--set-file KEY=FILE`    | 从文件读取字符串覆盖；结尾单个换行被去掉；层内顺序在 `--set-json` 之后 |

### 渲染与输出

| 参数                | 说明                         |
|---------------------|------------------------------|
| `-o, --output FILE` | 输出文件；缺省 stdout        |
| `-i, --in-place`    | 安全原地写回                 |
| `--output-dir DIR`  | 批量输出目录                 |
| `--recursive`       | 递归处理目录模板             |
| `--validate-only`   | 只做 schema/类型校验，不输出 |
| `--check`           | 与现有目标逐字节比较，不写任何文件；有过期目标时 exit 7（需要 `-o`/`-i`/`--output-dir` 之一） |
| `--report=unused`   | 把值文件中未被模板引用的叶子列到 stderr；默认自动开启 `--jsonl`，输出 `type:"unused"` 记录 |

### 策略

| 参数 | 默认 | 说明 |
|------|------|------|
| `--on-missing=error\|empty\|keep\|zero` | `error` | 缺失值策略 |
| `--on-empty=ok\|error` | `ok` | 空值策略 |
| `--unknown-keywords=ignore\|warn\|error` | `ignore` | schema 未知关键字策略 |
| `--only-vars VAR...` | - | 只替换指定变量 |
| `--ignore-vars VAR...` | - | 忽略指定变量 |

## 5. 退出码

| 退出码 | 含义                  |
|--------|-----------------------|
| 0      | 成功                  |
| 1      | 内部错误 / 未分类错误  |
| 2      | CLI 用法错误          |
| 3      | 模板语法错误          |
| 4      | 类型/Schema 校验错误  |
| 5      | 文件/IO 错误          |
| 6      | 缺失变量/空值策略错误 |
| 7      | `--check` 检测到输出与现有目标不一致（过期） |

退出码 1–6 由 `diag.Kind` 映射；7 只由 `--check` 产生，用于把"生成物过期"与"程序出错"区分开
（外部 `cmp` 用 1 表达差异，与本项目 1=内部错误冲突）。`[FACT]`

**1 与 2 的边界**：CLI 层把所有"用法"问题都归成 `KindUsage` → 2，包括拼错的子命令
（`unknown command "rendr" for "envsub"`）。落到 1 的是两类：带 `internal` kind 的内部错误，
以及**不带任何 kind** 的错误——那是工具自身的缺陷，不是用户用错了。`[FACT]`

## 6. stdout / stderr 约定

- stdout：只输出渲染结果。
- stderr：日志、警告、错误，以及 `--explain`/`--report` 报告。
- 默认 stderr 是**混合诊断流**；`--error-format=json` 是机器模式的兼容别名，会输出统一事件信封。
- `--jsonl`（`--explain`/`--report` 自动开启）时，stderr 是**纯 JSONL 事件流**：每行固定为
  `schema`/`type`/`time`/`seq`/`data` 信封，`data` 按 `explain`/`unused`/`error`/`log` 特化；
  stdout 不变。`time` 统一由 `internal/timefmt` 生成（UTC、秒级、`Z`）。
- `--jsonl` 优先于 `--error-format`；`--error-format=json` 是兼容别名；`--jsonl=false` 显式退回混合流。
- 默认不打印 bundle/进度信息。

## 7. 使用示例

```bash
envsub render -f base.yaml -f prod.yaml --schema schema.yaml -o config.yaml template.tmpl

envsub render --dotenv-file .env --set app.port=8080 --validate-only

envsub render --in-place --on-missing=error config.tmpl

envsub completion bash > /etc/bash_completion.d/envsub
```

## 8. P1/P2 预留

- P1：`--diff`、include/条件/循环相关参数。（`--dry-run` 已删除，`--check` 已实现）
- P2：`--compat=gnu|a8m|drone`、`migrate`、`convert`。
- 所有新命令通过 engine/source/schema/writer 接口接入，不修改现有 CLI 主流程。`[INFERENCE]`

## 9. 非目标（Non-Goals）

- 首版不做交互式 TUI。`[ASSUMPTION]`
- 不把 P1/P2 命令塞进 P0。`[FACT]`
- 不通过 stdin 传递 schema 作为 P0 默认能力（schema 只由 `--schema` 指定）。`[FACT]`
