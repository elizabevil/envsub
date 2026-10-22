---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-10
---

# 06-12 `--values-file` 的 `.env` 边界与 CLI 帮助参数名

> 用户报告两点：`-f, --values-file` 的帮助文案与实现语义不符；`test/e2e/testdata/explain_batch/`
> 下的手工 `Justfile`/`result/` 使案例不易复核。本文记录诊断、决定与落盘证据。

## 1. 诊断（实测）

### 1.1 `--values-file` 接受 `.env`，但语义是"根命名空间"

```
$ printf 'K=from_dotenv_passed_as_values\n' > app.env
$ printf 'K=${K}\n' > input.tmpl
$ envsub render --values-file app.env input.tmpl
K=from_dotenv_passed_as_values      # exit 0
```

`source.ForFile` 对 `.env` 返回 `Loader{Env: true}`，`--values-file` 分支照单全收，`values.MergeInto`
把键写进**环境命名空间**——与 `--env-file` 行为重叠，却走不同的优先级/记账路径。帮助与文档都写
"YAML/JSON values files"，属实现与契约不符。`[FACT]`（来源：本轮实测，2026-10-10）

### 1.2 CLI 帮助泄漏 Go 类型名

`envsub render --help` 对所有 `StringArray` flag 显示 `stringArray`（如
`-f, --values-file stringArray  YAML/JSON values files`）。pflag 只有在 usage 里有反引号参数名时才
显示语义名。`[FACT]`（来源：`pflag@v1.0.10/flag.go` 的 `UnquoteUsage`）

### 1.3 fixture 目录内混入手工产物

`test/e2e/testdata/explain_batch/` 下出现 `Justfile` 与 `result/`（手工运行 `--output-dir result`
产生）。二者会被 e2e runner 的 `copyDirectory` 原样复制，且可能被误提交。`[FACT]`
（来源：`test/e2e/e2e_test.go` 的 `copyDirectory`）

## 2. 候选方案与权衡（Alternatives & Trade-offs）

| 方案 | 做法 | 结论 |
|------|------|------|
| **A 拒绝（采用）** | `--values-file` 遇到 dotenv loader 报 `KindUsage` exit 2，提示改用 `--env-file` | 契约与帮助/文档一致；两种来源不再重叠 |
| B 接受并文档化 | 帮助改成"YAML/JSON/.env"，说明 `.env` 进 env 命名空间 | **未采用**：同一件事两条命令、两套优先级记账，歧义留给用户 |
| C 路由到根 | 把 `.env` 的值也写进根命名空间 | **未采用**：改变 `.env` 的既有语义（环境命名空间）与优先级（§14），破坏 `env_file_wins` 等既有契约 |

`[INFERENCE]`（依据：A 的改动面仅限一个校验点与帮助文案，且不与 §13/§14 的分层模型冲突。）

## 3. 决定与实现

1. `internal/render/render.go`：`--values-file` 分支在 `ForFile` 之后拒绝 `loader.Env`，错误文案
   `--values-file accepts YAML/JSON value files, not <path>: use --env-file for .env files`（exit 2）。`[FACT]`
2. `internal/cli/render.go`、`internal/cli/root.go`：所有取值 flag 在 usage 里加反引号参数名
   （`FILE`/`KEY=VALUE`/`KEY=JSON`/`VAR`/`POLICY`/`PREFIX`/`DIR`/`LEVEL`），帮助不再出现
   `stringArray`/裸 `string`。`[FACT]`
3. fixture 卫生：`Justfile` 移到 `.tmp/fixture-scratch/`（未提交区，保留原物），删除手工生成的
   `result/`；`.gitignore` 增加 `/test/e2e/testdata/*/result/`，避免同类产物被提交。`[FACT]`
4. `explain_batch/expect.err` 去掉多余的行尾空行（此前 3 个换行）。`[FACT]`

## 4. 证据与复现命令

```bash
go test -count=1 -run TestRunValuesFileRejectsDotenv ./internal/render/
go test -count=1 -run TestRenderHelpNamesFlagArguments ./internal/cli/
go test -count=1 ./test/e2e/            # fixture values_rejects_dotenv（exit 2 + expect.err）
just check

# 手工确认帮助
envsub render --help | grep -- '--values-file'
#   -f, --values-file FILE   YAML/JSON values files (FILE, repeatable)

# 手工确认拒绝
printf 'K=1\n' > app.env; printf 'K=${K}\n' > i.tmpl
envsub render --values-file app.env i.tmpl   # exit 2
```

`[FACT]`（`just check` 本轮退出码 0）

## 5. 非目标（Non-Goals）

1. **不改变 `--env-file` 的语义与优先级**：`.env` 仍走环境命名空间，仍按 `.env` > 进程环境。
2. **不新增文件格式**：不引入 TOML（`--values-file x.toml` 仍报"cannot determine value file format"），
   也不为 `.env` 增加第二入口。
3. **不保留旧名与别名**：`--values`、`--env-prefix` 已重命名为 `--values-file`、
   `--process-env-prefix`，**不提供弃用别名**；`--set-string` 等历史删除项不回滚。
4. **不在本轮给 CLI 帮助做 golden**：帮助文本随版本变化，只断言语义参数名（`TestRenderHelpNamesFlagArguments`），
   不逐字节固定整页输出。
5. **不改 e2e runner 的 fixture 约定**：不为 fixture 引入 `Justfile` 更新机制；golden 仍由 `args` +
   `expect.*` 描述。

## 6. 决定与待确认（2026-10-10 更新）

- ✅ **已确认：`--values-file` 只支持 values 层，拒绝 `.env` 是正确的**——§3 的实现（`KindUsage` exit 2）
  保留；§5 非目标第 1、2 条不变。`[FACT]`（来源：需求提出者确认，2026-10-10）
- ✅ **已决（2026-10-10）：values 旗标采用 `-f, --values-file`**（方案 C），**不保留 `--values`
  别名、无弃用期**；同时 `--env-prefix` 重命名为 `--process-env-prefix`（语义不变：只筛进程环境，
  不筛 `.env`）。`[FACT]`（来源：需求提出者确认；证据：`TestRenderHelpNamesFlagArguments`、
  `go test -count=1 ./test/e2e/`、全仓无旧名残留）
- **待定 B：`--env-file` 的文件名识别**。✅ **已决（2026-10-10）：`--env-file` 按 flag 语义强制
  dotenv，忽略文件名/扩展名**。`source.DotenvLoader()` 取代该路径上的 `ForFile`，于是 `.env`、
  `debug.env`、`.env.local`、`.env.production`、无扩展名的 `production` 全部可用。证据：单测
  `TestDotenvLoaderIgnoresFileName`、`TestRunEnvFileIgnoresExtension`，fixture `env_file_nonstandard`。`[FACT]`
- **仍待办**：是否为 e2e fixture 提供统一的 `just test-update-e2e` 重生成入口——当前重生成入口分散
  （引擎 fixture 的 `just test-update`、examples 的 `digest-update`）。`[TODO]` owner：技术负责人。
- `[ASSUMPTION]` 使用者不会依赖"用 `--values-file` 传 `.env`"这一未文档化行为。验证方式：若收到反馈，
  再回到方案 B/C 评估。owner：产品负责人。
