---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-10
---

# 06-14 会话交接：envsub 一轮大改动的状态与剩余任务

> 用途：迁移会话上下文。新模型读本文件即可继续剩余工作，无需重看聊天记录。
> 仓库：`/home/debi/IdeaProjects/envsub`（Go 1.27 配置渲染器，module `github.com/elizabevil/envsub`）。

## 0. 最重要的三件事

1. **工作区有大量未提交改动**（36 个文件 `git diff` 修改 + 约 20 个未跟踪新文件，见 §6）。开始前先 `git status` 通读，不要 `git checkout`/`git restore`（提示词明文禁止丢弃未提交改动）。
2. **改用户可见行为前必须先讨论确认**（`docs/prompts/_common.md` §7.10，本会话中途被用户要求补进去的规则）。当前仍有几个未决问题见 §4。
3. **验证口径**：`just check`、`just test-all`（含 perf 门禁）本会话最后一次运行均 exit 0；e2e fixture 36 个。

---

## 1. 项目定位（一句话）

envsubst 的"超集"：单静态二进制、类型校验 + 多文件分层 + 受限模板引擎；P0 已 26/27（仅 TASK-026 试用发布未做）。不是 Helm/Helmfile 替代品，但值文件覆盖、`--set`/`--set-json`、pipeline 语义刻意对齐 Helm。

## 2. 本会话已实现（均已测试落盘）

### 2.1 `--explain`（JSONL，stderr）
- 引擎侧 `engine.PathEngine` + `extended.Engine.Paths(src)` 已存在；本会话补渲染层与 CLI。
- 输出：**固定 JSONL → stderr**，每变量一行。字段：`template`（单模板也输出）、`variable`（`${env.NAME}` 归一为 `NAME`）、`source`、`kind`（`set|dotenv|yaml|json|schema|process|missing`）、`file`/`line`（仅文件来源，`line` 用 `*int`）、`shadowed`（被覆盖来源，高→低，不截断）。
- CLI：`--explain[=PATH]`（裸 = all，`*` 哨兵；`--explain=app.port` 精确匹配过滤，可重复）。
- **已决：不提供 `--explain-out`**；落盘用 `2>`。`--explain` 不输出值（密钥安全）；stdout 逐字节不变。
- 关键文件：[internal/render/explain.go](internal/render/explain.go)、[internal/render/render.go](internal/render/render.go)（`emitExplain`/`renderOne`）。
- 测试：`internal/render/explain_test.go`；e2e `explain_env_file`/`explain_multi_env`/`explain_set_wins`/`explain_missing`/`explain_batch`/`explain_path`/`explain_space_form`；`TestExplainStdoutUnchanged`、`TestExplainValidateOnly`。

### 2.2 `Options` 重构（方案 B：拆两层）
- `internal/cli/render.go`：`renderFlags`（按 `input/values/output/policies/explain` 分组，仅解析）+ `request(args, stderr)`。
- `internal/render/options.go`：`Request{Input, Values, Output, Policies, Explain, ValidateOnly}` + `Request.Validate()`（互斥与策略校验收敛到一处）。
- 旧的 20 字段平面 `Options` 已删除。测试：`internal/render/render_features_test.go`（字面量改为分组）、`env_options_test.go`。

### 2.3 值来源优先级定稿 + `default` 兜底（G21）
- **定稿顺序（先命中者胜）**：`--set`/`--set-json` > YAML/JSON 值文件 > `.env`（后者覆盖前者）> 进程环境 > schema `default`（**兜底，非来源**）。
- `default` 由"根命名空间的一层"降为兜底：`schema.ApplyDefaults` 先查环境命名空间，任何来源（含进程环境）有值就不填 default。
- 测试：`internal/schema/schema_test.go::TestApplyDefaultsDefersToEnvironment`；e2e `schema_default_is_fallback`。
- 文档：`04-cli.md` §6.2、`09-gaps.md` G21、`06-review/06-13`。

### 2.4 CLI 改名（无别名、无弃用）
- `--values` → **`-f, --values-file`**（方案 C）。
- `--env-prefix` → **`--process-env-prefix`**（语义不变：只筛进程环境，**不筛 `.env`**）。
- 旧名已彻底移除：`--values`/`--env-prefix` → `unknown flag` exit 2。
- 关键文件：`internal/cli/constants.go`（`flagValuesFile`/`flagProcessEnvPrefix`）、`internal/cli/render.go`、`internal/render/options.go`（`Values.ProcessEnvPrefix`）。
- 测试：`internal/cli/render_test.go::TestRenderHasNoLegacyFlagNames`、`TestRenderHelpNamesFlagArguments`；e2e `legacy_flag_values`/`legacy_flag_env_prefix`。

### 2.5 `--env-file` 强制 dotenv
- `source.DotenvLoader()` 取代该路径的 `ForFile`，**忽略文件名/扩展名**：`.env`/`debug.env`/`.env.local`/`.env.production`/无扩展名均可用。
- 测试：`internal/source/loader_test.go::TestDotenvLoaderIgnoresFileName`、`internal/render/render_features_test.go::TestRunEnvFileIgnoresExtension`；e2e `env_file_nonstandard`。

### 2.6 json 两后端差异（G20）
- `omitempty` 对**数值 0** 不一致：`encoding/json/v2` 保留（`{"whole":0}`）、`sonic` 省略（`{}`）。
- 规避：需可省略的数字用 `*int` + `omitempty`（`--explain` 的 `line` 即如此）。
- 测试：`internal/jsonx/jsonx_test.go::TestMarshalOptionalFieldsAgree`（共享）、`jsonv2_test.go::TestJSONv2MarshalKeepsOmitemptyZeroInt`、`sonic_test.go::TestSonicMarshalOmitsOmitemptyZeroInt`。gap `09-gaps.md` G20。

### 2.7 文档与状态同步
- 新增 `06-review/06-11`（explain/结构）、`06-12`（--values-file 边界 + 帮助参数名）、`06-13`（优先级/default）。
- `07-serialization/00-index.md` 新增 v0.10 变更记录；`05-03-pending-work.md` §12 待办 H~N 已标注完成；`docs/prompts/_common.md` §7.10 新增"改行为前先讨论"规则。

## 3. 当前生效的 CLI 面

```
value sources
  -f, --values-file FILE...   YAML/JSON values files, repeatable
      --env-file FILE...      dotenv files (parsed as dotenv, any name), repeatable
      --schema FILE           JSON Schema (Draft 2020-12), JSON or YAML
      --set KEY=VALUE         raw string override (typed by schema or | type)
      --set-json KEY=JSON     native JSON value
      --no-process-env        ignore the process environment (dotenv files still load)
      --process-env-prefix P  expose only process variables with this prefix (dotenv unaffected)
```

## 4. 剩余未决问题（按优先级，含已给的调查结论）

### 4.1 【当前主线】顶层特殊键 + `env` 命名空间冲突（方案 1+2 已采纳，子问题未定）

**背景（本会话实测）**：
- 路径段非引号只允许 `[A-Za-z_][A-Za-z0-9_]*`；嵌套特殊键可用引号 `${app["my-key"]}`/`${app["my.key"]}`。
- **顶层特殊键不可达**：`${["a.b"]}` → exit 3 `expected identifier at offset 0`（`parsePrefix` 要求首段是标识符）；顶层 `"a.b"` 配 `${a.b}` → exit 6（取不到）；顶层 `"a-b"` 配 `${a-b}` → **静默**变成 `${a` 的 `-` 默认值 → 输出 `b`。
- **`env` 冲突**：`${env}` 取根对象、`${env.X}` 走环境命名空间（`compilePath` envOnly），所以 values 文件顶层 `env: {...}` 的子键**永不达**；`--set env.K=x` 写的是 `root.env.K`，`${env.K}` 读不到它（等于 `--set` 输给了 `.env`，违反"`--set` 最高"）。
- 进程环境特殊名可用引号读取：`${env["MY-VAR"]}`、`${env["MY.VAR"]}` ✅；`${env.MY-VAR}` ❌ 静默变默认值。

**用户已采纳 1+2**：加 jq 风格**引导索引** `${["a.b"]}`、`${["a-b"]}`，并用它解决 `env` 冲突。

**子问题（需继续与用户确认）**：
- Q1：`["env"]` 该指向哪一侧？我的建议：**根对象用引号 `${["env"].K}`，命名空间保持裸 `${env.K}`**（向后兼容、纯新增）。用户原话反过来问"根对象使用 `["env"]` 还是命名空间使用？"——需要给出明确结论。
- Q2：**是否把 `--set` 拆成 `--set`(设值) 与 `--set-env`(设环境) 两个功能**？（用户说"当前已实现部分"，即 `--set`/`--set-json` 已存在但只写根命名空间。）我此前建议是"拒绝 `env` 保留前缀"（`--set env.K=` → KindUsage exit 2），业界 Helm `--set` 也只写 `.Values`、环境注入靠 `-e`/`--env-file`。

**实施清单（Q1/Q2 定后执行）**：
- `internal/values/path.go::parsePrefix` 允许以 `[` 开头（引导索引段）；`compilePath` envOnly 判定保持不变（`${env.K}` 行为不动）。
- 校验（若采纳"拒绝保留前缀"）：`setOne` 里键首段为 `env` → KindUsage exit 2。
- 测试：`path_test.go` 增 `${["a.b"]}`/`${["a-b"]}`/`${["env"].K}`；e2e fixture 增 1 个（顶层 `"a.b"` + `env:` 对象）；`--set env.K` 拒绝用例。
- 文档：§3 路径语法、§13 `env` 说明、`04-cli.md` §6.1。

### 4.2 `env.NAME` 去留（05-03 §10 #8，待决断）
- 已确认 `${NAME}` 是规范形态；`${env.NAME}` 不是同义词（它跳过根、只读环境），是"显式读环境"的逃生通道。
- **建议**：保留 + 修正 `13-materialization.md` §14 那句错误表述（现写"语义与 `${NAME}` 相同"，是错的）。
- 若移除：会失去"根里同名时读环境值"的能力，且要改 4 个 engine fixture + 2 个 e2e fixture + `examples/full/config.tmpl` + `envOnly` 代码 + `--explain` 的 `env.` 归一。

### 4.3 `--process-env-prefix` 保留 vs 剥离前缀（05-03 §10 #2，待决断）
- 现状：保留原名（白名单闸门，不剥离）。剥离（`APP_PORT`→`PORT`）会与点号路径命名规则叠加二义，需产品决策。

### 4.4 长期待办（未动）
- `05-03 §10 #3`：CLI 测试深度是否继续扩（`internal/app` 已 75% 覆盖）。
- P1/P2：include、条件/循环、profiles、`--dry-run`/`--diff`/`--check`、JSON 错误报告、密钥脱敏、TOML、compat 模式、watch/插件/远程 secret、HTTP/RPC 模板输入接线（`source/input.go` 仍是 `ErrNotImplemented`）。
- P0 TASK-026：试用发布（无 git tag）。

## 5. 关键代码位置（速查）

| 内容 | 文件 |
|------|------|
| 路径解析（`parsePrefix`/`envOnly`） | `internal/values/path.go` |
| 值加载/合并/`--values-file` 拒绝 `.env` | `internal/render/render.go`（`buildValues`/`setOne`） |
| 请求分组 + 校验 | `internal/render/options.go` |
| explain JSONL + 来源栈 | `internal/render/explain.go` |
| CLI 旗标装配 | `internal/cli/render.go`、`constants.go` |
| dotenv 强制加载 | `internal/source/loader.go`（`DotenvLoader`） |
| default 兜底 | `internal/schema/defaults.go` |

## 6. 未提交文件清单（`git status --short` 全量）

修改（M）：`.gitignore`、`README.md`、`cmd/envsub/main.go`、`docs/03-design/03-03`、`03-04`、`03-08`、`docs/05-tasks/05-02`、`05-03`、`docs/06-review/06-00-index`、`06-10`、`docs/07-serialization/00-index`、`02-sources`、`04-cli`、`09-gaps`、`13-materialization`、`docs/prompts/_common.md`、`internal/cli/constants.go`、`render.go`、`root.go`、`internal/jsonx/*_test.go`（3 个）、`internal/render/env_options_test.go`、`render.go`、`render_features_test.go`、`render_test.go`、`internal/schema/defaults.go`、`schema_test.go`、`internal/source/loader.go`、`loader_test.go`、`internal/values/env_filter_test.go`、`path.go`、`test/e2e/e2e_test.go`、`test/e2e/testdata/env_*`（3 个 args）。

未跟踪（??）：`cmd/envsub/main_test.go`、`docs/06-review/06-11/06-12/06-13`、`internal/app/app_test.go`、`internal/cli/render_test.go`、`internal/render/explain.go`、`explain_test.go`、`options.go`、`test/e2e/testdata/{env_file_nonstandard, explain_batch, explain_env_file, explain_missing, explain_multi_env, explain_path, explain_set_wins, explain_space_form, legacy_flag_env_prefix, legacy_flag_values, schema_default_is_fallback, values_rejects_dotenv}`。

## 7. 复现命令

```bash
just check              # fmt + vet + 单测 + e2e（exit 0）
just test-all           # 追加 perf 门禁（exit 0）
go test -count=1 ./test/e2e/                      # e2e 36 个 fixture
go test -count=1 -tags=sonic ./internal/jsonx/    # 两后端一致性（G20）
```

---

## 附：本会话用户最新一条待答指令（原话）

> 采纳1+2. 其中2 根对象使用["env"] 还是命名空间使用？   是否拆--set 为 value与env功能？（当前已实现部分）.

→ 即 §4.1 的 Q1/Q2。下一个模型应：先给出 Q1 的明确结论（根对象用 `${["env"].K}`，命名空间保持 `${env.K}` 的推荐 + 理由），再就 Q2 给出 `--set` 拆分与否的建议与代价，**征求用户确认后**再动手，并遵守 `_common.md` §7.10 与 §7.5（落盘测试 + 报测试名与命令）。
