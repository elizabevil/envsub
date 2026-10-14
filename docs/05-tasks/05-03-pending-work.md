

## 6. 待办 E：CLI 环境变量污染防护 —— ✅ 已完成

| 选项 | 语义 | 备注 |
|------|------|------|
| `--no-process-env` | 不读取进程环境；`Env` 只含 `--env-file` 内容 | 覆盖 `render.go:324` 的 `EnvFromOS()` 调用点 |
| `--process-env-prefix PREFIX` | 只装入指定前缀的进程变量（白名单），其余忽略 | 与 `--no-process-env` 互斥（建议 exit 2） |

实现：`values.EnvFromOSFiltered(prefix)`（空前缀=全部）；`render.Request.Values` 增
`NoProcessEnv`/`ProcessEnvPrefix`，装配 `tree.Env` 时按开关过滤；`Request.Validate()` 对互斥组合返回
`KindUsage`(2)；CLI 注册两个开关。验证：`TestEnvFromOSFiltered`、`TestValidateEnvSources`，以及 CLI
冒烟（默认 `HOME` 生效 / `--no-process-env` → `none` / 前缀白名单外走默认值 / 互斥 exit 2 /
`--help` 列出两项）。`--process-env-prefix` **只约束进程环境，不筛 `.env`**（2026-10-10 确认）。
文档：`04-cli.md` 选项表 C10、`13-materialization.md` §12 改为'已提供'。

## 7. 待办 F：去掉 union 物化的重复实现 —— ✅ 已完成

原先 `schema.convertDeclared` 与 `engine.normalizeBySchemaMembers` 是同一算法（SI11）的两份实现。
现在：**算法只存在于 `schema`**——新增导出方法 `func (s *Schema) NormalizeString(path, text string)
(any, bool, error)`（包一层 `convertDeclared`），引擎的字符串分支改为调用它；引擎里剩下的
成员循环只做"**已类型化值的成员检查**"（非字符串无需转换，因此没有第二份转换逻辑）。
对拍用例：`TestNormalizeBySchemaMembersMatchesSchema`（7 种 schema/值组合，两侧结果必须一致）。

### F2：walker 遍历骨架收敛 —— ✅ 已完成

新增 `internal/schema/walk.go`：`propertyVisitor` 只回答两个问题（缺失键怎么办、字符串怎么办），
遍历骨架（`utils.SortedKeys(properties)` + `values.AppendSegment` + map/[]any 递归 + items 迭代）只有一份：
`walkProperties` / `walkItems` / `walkValue`。`applyDefaults` 与 `materializeNode` 各自退化为
一个 visitor。

顺带修掉一个真实缺陷：物化时字符串转换成功后原先 `continue`，**不递归**进刚解析出的集合，
因此 `.env` 的 `APP_PORTS='["1","2"]'` + `items: {type: integer}` 会因元素仍是字符串而校验失败；
现在解析出的集合与内联写法的集合走同一套后续处理。用例
`TestMaterializeConvertsElementsOfParsedArray`（`go test ./internal/schema/`）。

## 8. 待办 G：测试面补齐 —— ✅ 已完成

| 缺口 | 现状 | 建议 |
|------|------|------|
| `internal/app`、`cmd/envsub` | ✅ **已补齐**：`internal/app/app_test.go` 覆盖参数装配、输入/输出形态、`--validate-only`、`--explain`、退出码 2/3/4/5/6 与诊断文案（覆盖率 **75%**，目标 ≥60%）；`cmd/envsub/main_test.go` 覆盖 `run()` 的委派与退出码（50%，`main` 的 `os.Exit` 行不可测） | 见 §11 |
| `test/e2e/testdata/` 的 env 开关 fixture | ✅ **已完成**：`no_process_env`、`env_prefix`、`env_prefix_positive`、`env_options_conflict`（含 `expect.err` + `exit` 2）；追加 `legacy_flag_values`、`legacy_flag_env_prefix`（旧名 → `unknown flag` + exit 2）与 `env_file_nonstandard`（`--dotenv-file` 忽略文件名） | 见 `06-10` §9.1、`06-12` §6 |

已完成部分见 `docs/06-review/06-10-optimization-summary.md` §9（`TestAllOfPrecedenceIsFirstWins`、
`TestRefPointerPathResolvesNestedDefinition`）。

## 9. 性能复查记录

`docs/06-review/06-10-optimization-summary.md` §8：G11–G14 / F / F2 / 待办 E 之后逐项复核，
**所有确定性指标与基线持平**，`just test-all` 门禁 render 0.43x、RSS 1.11x、startup 4.75ms、
基线输出逐字节一致 ⇒ 本轮重构无性能回退。

## 10. 待决问题（等评审决定）

| # | 问题 | 选项 | 影响 |
|---|------|------|------|
| 1 | `--process-env-prefix` 的**正例** e2e 怎么落 | ✅ **已完成**：e2e runner 支持 `env` 注入文件，`env_prefix_positive` 钉住"白名单命中→生效、前缀外→缺失" | — |
| 2 | `--process-env-prefix` 是**保留前缀**还是**剥掉前缀** | ✅ **已定**：保留原名（白名单闸门，不剥离）；`--process-env-prefix` 作用域收窄为 `@process` 命名空间 | 见 `06-15` §2.2/§4 |
| 3 | `internal/app` / `cmd/envsub` 的 CLI 测试深度 | ✅ **已定：方案 (a)**——app 层只测装配（flag → `render.Request`）与退出码，命令矩阵（`--output-dir`/`--in-place`/stdin/glob/batch）由 e2e 的 60 个 case 端到端保证，避免同一契约在两处维护（依据 `06-17` 的 CLI/runner 分离）。补强：输出模式三选一的装配有专门用例（`internal/cli`） | 见 `03-07`、`06-17` |
| 4 | 错误 fixture 是否覆盖**全部退出码** | ✅ **基本完成**：e2e 已覆盖 2/3/4/5/6；仅 1（内部错误，无法稳定触发）记为已知缺口 | 退出码是用户可见契约 |
| 5 | `test/e2e` fixture 规模 | ✅ **已定：不设上限、不分组**——用例见 `test/e2e/testdata/`（全量 `just test-e2e`，单跑 `just test-e2e-case NAME`），文档不再写死数量；**触发条件**：用例数 >100 或热耗时 >2s 时重新评审 | 热耗时约 0.5s |
| 6 | **环境变量映射契约** | ✅ **已实现**：路径推导已移除（`envName` 仅保留单段同名；`envMetaName` 删除；物化自动收窄）；环境读取只剩 `${@process.NAME}`，`.env` 写入根，层序 `--set` > `--dotenv-file` > `-f` > `default`。见 `13-materialization.md` §13/§14、`04-cli.md` §6.2 |
| 7 | **`.env` 与进程环境优先级** | ✅ **已定案**：`.env` 与进程环境不同格（`.env` 进根、进程环境只在 `@process`），"谁压谁"不存在 | 见 `13-materialization.md` §12/§14 |
| 8 | **`env.NAME` 前缀去留** | ✅ **已定**：`env` 降为**根对象里的普通键**，`${env.NAME}` 不再是命名空间写法；进程环境写 `${@process.NAME}`。原"是否弃用兼容形式"问题消失 | 见 `06-15` §2.1/§3.1 D1 |
| 9 | **`--explain` 来源输出** | ✅ **已实现（JSONL 版）**：引擎侧变量枚举（`engine.PathEngine` + `extended.Engine.Paths`）＋渲染层 `Request.Explain{Enabled, Paths, Writer}`（来源栈在 `buildValues` 记录，`explainTemplate` 编码 JSONL 并做精确路径过滤）＋CLI `--explain[=PATH]`（stderr，裸 = all）＋7 个 e2e fixture 与 stdout 逐字节回归。**已决（2026-10-10）：不提供 `--explain-out`，固定 stderr + JSONL，落盘用 `2>`**。规格见 `13-materialization.md` §15，取舍与实测见 `06-review/06-11`。`variable` 用规范拼写、不再做 `env.` 归一；`shadowed` 已升级为**结构化帧数组**（ADR-005），
含被覆盖的 schema `default` |
| 10 | **命名空间重构的三个开放项** | ✅ **已裁决**：O1 `.env` 键**照常纳入 schema 校验**（`additionalProperties:false` 会拒收 `.env` 里的 `PATH` 等，属既定代价）；O2 `--dotenv-file` 键字符集**不放宽**；O3 `["0"]` 恒为键、`[0]` 恒为下标 | 见 `06-15` §6；实现按此执行 |

## 11. CLI 测试矩阵清单（已执行；评审定为"完整测试"）

目标：把 `internal/app` 与 `cmd/envsub` 从 0 覆盖率提到"命令矩阵级"，并沉淀为可回归用例。

| 组 | 用例 | 断言 |
|----|------|------|
| 输入 | 单文件 / 多文件 / glob / 目录（`--recursive` 开与关）/ stdin（`-`） | 输出内容、文件数、目录镜像关系 |
| 输出 | stdout / `--output`（单模板）/ `--output-dir`（多模板镜像）/ `--in-place`（原子写 + 权限保留） | 内容与路径；错误组合（同 `--output` 与 `--output-dir`）→ exit 2 |
| 校验 | `--validate-only`（有/无 schema；与输出选项互斥） | 不写文件；exit 0/4 |
| 值来源 | `--set`、`--set-json`、`--env-file`（多文件后者覆盖前者）、`--no-process-env`、`--process-env-prefix`、`--explain` | 取值与优先级、explain 行 |
| 退出码 | 1（内部，无法稳定触发，记为已知缺口）、2（用法）、3（模板）、4（类型/schema）、5（IO）、6（缺失/策略） | 每个码至少一条 CLI 级用例 |
| 诊断 | 模板错误定位（文件:行）、缺失变量文案、未知关键字策略 | stderr 文案子串 + 退出码 |

实现方式：优先补 `test/e2e/testdata/` fixture（用户可见行为）；`internal/app` 侧重"参数装配与校验"
（Options 构造、路径解析、批处理编排），用包内单测覆盖，目标覆盖率 ≥60%。

**落地情况（`internal/app/app_test.go`，覆盖率 75%）**：

| 组 | 覆盖 |
|----|------|
| 输入 | ✅ 单文件（`TestRunRenderInputs/file`）、多文件（`TestRunRenderOutputs/output dir`）、glob、目录（`--recursive` 开与关）、stdin（`-`） |
| 输出 | ✅ stdout、`--output`、`--output-dir`（镜像路径）、`--in-place`；权限保留由 `internal/output` 单测与 e2e `in_place` 覆盖 |
| 校验 | ✅ `--validate-only` 有/无 schema；与输出选项互斥 → exit 2 |
| 值来源 | ✅ `--set`、`--values-file`（多文件后者覆盖前者）、`--env-file`（强制 dotenv，多文件后者覆盖前者）、`--no-process-env`、`--process-env-prefix`（互斥）、`--explain`；`--set-json`／`--process-env-prefix` 正例由 e2e（`set_json_big_int`／`env_prefix_positive`）与单测覆盖 |
| 退出码 | ✅ 2/3/4/5/6；1（内部）无稳定触发，仍为已知缺口 |
| 诊断 | ✅ 模板错误定位（`broken.tmpl:2`）、缺失变量文案；未知关键字策略由 schema/engine fixture 覆盖 |

## 12. 本轮结构变更（已落盘）

| 任务 | 状态 | 证据 / 复现 |
|------|------|-------------|
| **待办 H：`Options` 拆分为 CLI 旗标层 + `render.Request`（方案 B）** | ✅ 已完成 | `internal/render/options.go`（六组 + `Validate()`）、`internal/cli/render.go`（`renderFlags` + `request()`）；`go test -count=1 ./internal/render/ ./internal/app/ ./cmd/envsub/`、`just check`；结论见 `06-review/06-11` §3 |
| **待办 I：`--explain` 固定 JSONL + `--explain[=PATH]` 过滤** | ✅ 已完成；**不提供 `--explain-out`（2026-10-10 已决）** | `internal/render/explain.go`、CLI `--explain` 可选值；7 个 fixture（`explain_env_file`/`explain_multi_env`/`explain_set_wins`/`explain_missing`/`explain_batch`/`explain_path`/`explain_space_form`；已按当前口径更新 `shadowed` 与 `variable`）；`go test -count=1 ./test/e2e/`；规格见 `13-materialization.md` §15 |
| **待办 J：json 两后端 `omitempty` 数值 0 差异（G20）** | ✅ 已规避并落盘 | 需可省略的数字改用 `*int`；测试 `TestMarshalOptionalFieldsAgree`（共享）、`TestJSONv2MarshalKeepsOmitemptyZeroInt`/`TestSonicMarshalOmitsOmitemptyZeroInt`（各后端）；`go test -count=1 ./internal/jsonx/ && go test -count=1 -tags=sonic ./internal/jsonx/`；gap 见 `09-gaps.md` G20 |
| **待办 K：CLI 值来源旗标的边界与命名** | ✅ **已完成**（2026-10-10）：`.env` 边界（`--values-file` 只支持 values 层，拒绝 `.env`）；`--env-file` 强制 dotenv（忽略文件名）；values 旗标定为 **`-f, --values-file`**（方案 C，**不保留 `--values` 别名**）；`--env-prefix` → **`--process-env-prefix`**（只筛进程环境，行为不变）；**旧名一律移除、无弃用别名** | 结论见 `06-review/06-12` §5/§6；代码 `internal/cli/constants.go`/`render.go`、`internal/render/options.go`/`render.go`、`internal/source/loader.go`；fixture `values_rejects_dotenv`、`env_file_nonstandard`、`env_prefix*`、`legacy_flag_values`、`legacy_flag_env_prefix`；测试 `TestRunValuesFileRejectsDotenv`、`TestRenderHelpNamesFlagArguments`、`TestRenderHasNoLegacyFlagNames`、`TestDotenvLoaderIgnoresFileName`、`TestRunEnvFileIgnoresExtension`、`TestRunExitCodes`（含旧名 exit 2 两行）；`just check` / `just test-all` |
| **待办 N：CLI 改名的测试补齐** | ✅ 已完成 | 新增 e2e `legacy_flag_values`/`legacy_flag_env_prefix`（旧名 → `unknown flag` + exit 2）、`env_file_nonstandard`；新增单测 `TestRenderHasNoLegacyFlagNames`（旧名未注册、`-f` 仍绑定 `--values-file`）、`TestDotenvLoaderIgnoresFileName`、`TestRunEnvFileIgnoresExtension`；`TestRunExitCodes` 增 2 行；全量 `just check` / `just test-all` exit 0 |
| **待办 L：`default` 降为兜底（显式 > 隐式）** | ✅ 已完成（用户确认"方案 A 必须"） | `internal/schema/defaults.go`（`envFallback` 命中则不填）、测试 `TestApplyDefaultsDefersToEnvironment`、fixture `schema_default_is_fallback`（原 `schema_default_precedence` 已替换）；`go test -count=1 ./internal/schema/ ./test/e2e/`、`just check`；规则见 `04-cli.md` §6.2、gap G21、结论见 `06-review/06-13` |
| **待办 M：统一文档中的来源优先级描述** | ✅ 已完成（纯文档，代码未改） | 定稿链 `--set`/`--set-json` > `--dotenv-file` > `-f/--values-file` > schema `default`，进程环境不入链（只在 `@process`），已同步 `04-cli.md` §6.2、`00-index.md` C5、`02-sources.md` §4.2、`13-materialization.md` §12/§14/§15、`03-03` §3.5、README；`.env` 高于值文件与来源优先级的历史讨论见 `06-review/06-13` §2.1 |

## 13. P1/P2 候选需求（决策台账）

需求全景与可实现性判定在 [`02-05`](../02-product/02-05-requirements-registry.md)，
**任务（目标/验收/依赖/风险）在 [`05-04`](05-04-p1-p2-task-breakdown.md)**；
本节只保留"开工前要定的事"这份决策台账，避免与任务清单重复。

| 编号 | 候选 | 优先级 | 开工前需要定的事 |
|------|------|--------|------------------|
| REQ-SRC-06 | TOML 值文件 | P1 | 是否引入新依赖（ADR-004 需补决策）；与 `.env`/YAML 的语义差异 |
| REQ-SRC-08 | 值引用/插值 | P1 | 求值顺序、循环诊断、与 schema 物化的先后；是否破坏"无隐式逻辑"——设计材料见本文 §14 |
| REQ-SRC-09 | profiles/environments | P1 | 用"声明式覆盖层"还是新语法；与 `-f`/`--dotenv-file` 的合并点 |
| REQ-SRC-11 | HTTP(S) 值来源 | P2 | 与 K1 冲突：`--allow-remote` 的开关名、超时/大小/重定向上限、是否进日志 |
| REQ-TPL-04 | include/partials | P1 | `--include-dir` 白名单、深度上限（候选 3 层）、循环检测与错误格式 |
| REQ-TPL-05 | 受限条件/循环 | P1 | 保持"受限"的边界（无递归、无函数定义）与确定性 fixture |
| REQ-OUT-04 | `--diff`（⏸ 待需求） | P1 | 实现时作为 `--check` 的输出模式；是否内置 unified diff（外部 `diff`/`git diff` 已可替代） |

## 待办 O：`just lint` 的工具链版本（已解决）

**问题**：系统预装的 `golangci-lint`（2.13.2）用 **go1.27.0** 构建，而项目工具链与标准库是
**go1.27.2**；类型检查器只能读它自己那代或更早的 export 数据，于是报

```text
cannot decode "internal/goarch", export data version 5 is greater than maximum supported version 4
```

**解决**：`tools/lint/golangci-lint.sh` 用 `go install` 构建一次固定版本，位置**跟随标准约定**：

| 场景 | 二进制 | 缓存 | 模块/构建缓存 |
|------|--------|------|----------------|
| 正常开发机 | `$GOBIN`（即 `$GOPATH/bin`，本项目为 `~/go/bin`） | `~/.cache/golangci-lint` | 环境默认（`~/go/pkg/mod`、`~/.cache/go-build`） |
| 受限运行器（默认缓存只读） | **主工作区**的 `.tmp/lint/bin`（按 git common dir 定位，所有 worktree 共享），可用 `LINT_HOME` 覆盖 | 主工作区 `.tmp/lint/lintcache` | `GOCACHE`/`GOTMPDIR` 用主工作区 `.tmp/`；**模块缓存仍优先用环境默认**，只有默认不可写时才退到 `.tmp/lint/gopath/pkg/mod` |

- 判定"可写"用**实际写入**（`touch`）而不是权限位：只读挂载下权限位仍显示可写，只有写入返回 `EROFS`；
- PATH 或 `$GOBIN` 里已有该版本时**直接复用**，不构建、不下载；
- **缓存必须可跨次复用、且不得指向 `/tmp`**：早期实现在回退时把 `GOPATH`/`GOMODCACHE` 指到
  当前 worktree 的 `.tmp/lint/gopath`，于是每个 worktree、每次重建都要把依赖重新下载一遍。
  现在回退 home 固定在**主工作区**、模块缓存优先复用环境默认，`justfile` 的提示也从 `/tmp` 改为
  `GOCACHE=$PWD/.tmp/gocache GOTMPDIR=$PWD/.tmp/gotmp`；
- 版本不符才构建一次（首次约 45s）；`justfile` 的 `lintPin`（`v2.14.0`）与脚本内 `version` 必须同步；
- 不使用 `/tmp`：它是 tmpfs，部分运行器每次调用清空，会每次重建。

**lint 已进入静态测试**：`just check` 现在是
`caches fmt-check fmt-just vet lint test test-e2e`，lint 失败即门禁失败。
`just test-race` 同时覆盖 `./...`（含 sonic 后端）。

**缓存归属**：`justfile` **不强制任何缓存变量**（原先会把 `GOCACHE`/`GOTMPDIR`/
`GOLANGCI_LINT_CACHE` 指到 `.tmp`，现已移除，走环境默认）。受限运行器需自行给出可写位置：

```bash
GOCACHE=/tmp/gocache GOTMPDIR=/tmp/gotmp just check
```


## 本轮修复的 18 项 lint 问题（`golangci-lint v2.14.0`，配置不变）

| 类别 | 数量 | 修复 |
|------|------|------|
| errcheck | 12 | `examples/tools/digest`：写 stdout/stderr 的 `fmt.Fprintf`/`Fprintln` 与 `file.Close` 显式 `_ =`（输出流错误无处上报，与既有取舍一致） |
| unused | 1 | `internal/engine/extended/engine.go`：删除**重复的死代码** `pipelineWord` 自由函数（方法版在用；它比死代码多保留了 keep 信号语义） |
| ineffassign | 1 | `internal/values/path.go` `Canonical`：`afterIndex = true` 在循环尾部必然被清除，属无效赋值，删除并注释说明 |
| staticcheck | 1 | `internal/engine/extended/expression.go`：`switch { case field == alignArgument }` 改为 tagged switch |
| revive | 2 | `test/fuzz`：`f.Fuzz` 回调未使用 `t` 的改为 `_` |
| revive | 1 | `internal/render`：`RenderEngine` 名字有 stutter，但改为 `Engine` 会与导入的 engine 包冲突，保留并加 `//nolint:revive` 说明 |

## 14. 待办 Q：值引用 / 插值（REQ-SRC-08）——设计记录

**状态**：未排期（`REQ-SRC-08` P1 / ❌ / 🟡；任务 `TASK-116`，风险"高"）。**触发条件**：出现真实需求
"配置值内部派生/复用"，且 `.env` 与 `--set` 的替代被判定不可接受。本节自包含，接手者无需重新收集材料。

### 14.1 现状实测（当前二进制）

| 写法 | 结果 | 结论 |
|------|------|------|
| 值文件 `url: https://${host}/` | 原样输出 `https://${host}/` | 值文件里的 `${...}` **是字面量** |
| `.env` 的 `B=${A}` | 原样 `${A}` | `.env` 不做变量展开 |
| `--set 'x=${PORT}'` | 原样 `${PORT}` | `--set` 也不展开 |
| `.env` 写 `app.port=8080` | `parse ...: line 1: invalid key "app.port"` | `.env` 键**不含点号**（O2 已定不放宽）→ 无法表达嵌套目标 |

### 14.2 本仓对照案例（可直接复现）

```
base.yaml:  host: a.example
            url:  https://a.example/      # 手工派生
prod.yaml:  host: b.example              # 只覆盖 host
t.tmpl:     host=${host}\nurl=${url}
```

现状输出：`host=b.example` / `url=https://a.example/` —— **静默失配，无任何报错**。
这就是"派生值"的真实痛点：**痛点今天已存在**，不是插值引入的。

同一份数据在三种模型下的结果：

| 模型 | `url` 结果 | 是否修复失配 | 代价 |
|------|-----------|--------------|------|
| 今天（字面量） | `https://${host}/` | — | 无（不可用） |
| (a) 加载时单遍（每文件读入即展开） | `https://a.example/` | ❌ 仍失配 | 无循环检测，顺序敏感 |
| (b) 合并后定点迭代 | `https://b.example/` | ✅ | 循环检测 + 迭代上限 + 求值时机契约 |

### 14.3 与当前设计的最大差异

| # | 现有设计 | 插值之后 | 受影响的既有契约 |
|---|----------|----------|------------------|
| 1 | **值 = 纯数据，唯一求值面是模板** | 求值面变两个：数据层也能求值 | K7（"模板不是编程语言"，ADR-001）、`03-operators.md` §5（操作符是**模板**规则） |
| 2 | **层序 = 覆盖**（`MergeInto`：对象递归合并、数组与标量替换，无回头） | 层序 = 求值（结果改由依赖与求值时机决定） | `04-cli.md` §6.2 层序表、`load.Load` 固定顺序 |
| 3 | **血缘 = 一条链**（`origin` 是链尾、`shadowed` 近者在前、五字段恒在） | 血缘 = 图（值依赖值） | ADR-005 的 I1–I7；`--explain` 一条 JSONL 解释一个变量 |
| 4 | **`--report=unused` = 值文件里没有模板引用的叶子** | 多出一类边（值 → 值）：被别的值引用的键不再算死配置 | `04-cli.md` C15、`TestReportUnusedIgnoresNonFileSources` |
| 5 | **类型只在物化一处决定**（schema / `\| type`） | 新增"插值产物 → 再物化"阶段，先后必须钉死 | G21（`default` 只在无任何来源时兜底）、`schema.ApplyDefaults`/`Materialize` |
| 6 | 值来自可点名的来源，脱敏与诊断靠"来源" | `@process` 的值被搬进值树某一行，**洗掉原始出处** | 脱敏规则、`--explain` 的 `source` 语义（I3） |

一句话：**现在"值/覆盖/链"，插值后"表达式/求值/图"**；前三条正是 K7、ADR-005 与 unused 定义的地基。

### 14.4 业界已有案例（各自选了哪条路）

| 工具 | 值能否互相引用 | 求值时机 | 循环处理 | 与本项目的关系 |
|------|----------------|----------|----------|----------------|
| envsubst（本项目祖先） | ❌ 数据层无求值 | 仅模板替换 | 不存在 | 同族（现状） |
| Helm | ❌ `values.yaml` 是纯数据 | 模板渲染时（Go template + Sprig）；派生要靠 `tpl` 或重复写 | 不存在 | 同族；"环境 → 值"已由 `${@process.NAME}` 覆盖 |
| docker compose | ⚠️ 只从**环境/`.env`** 插值（`${VAR:-d}`/`${VAR:?err}`/`$$` 转义） | 载入期单遍 | **结构上不可能** | 窄化 + 载入期的先例 |
| Kustomize | ❌ 无通用插值；提供 **`replacements`**（显式 source→target **单向复制**） | 构建期按声明复制 | 单向 ⇒ 无环 | **最贴近下面 14.5 的最小边界** |
| Ansible | ✅ Jinja2 任意引用 | **惰性**（用到才解析） | 自引用报"模板字符串递归循环"类错误 | 表达力强、错误推迟到运行期 |
| Terraform (HCL) | ✅ `locals`/`variable` 互相引用 | 由引用推导**依赖图**，计划期求值 | **硬错误**（Cycle） | 完整"图求值"路线 |
| Spring Boot | ✅ `${...}` placeholder | 惰性取用，支持默认值 `:` | 启动期报错 | 同类：靠"启动即失败"兜住 |

### 14.5 若要做：把差异压到最小的边界（建议）

1. **显式开关**（不改变既有字面量语义，避开兼容性破坏）；
2. 只做**整值引用**，且只引用**层序更早/本层更早**定义的键 → **循环结构上不可能**，不需要检测器；
3. 求值放在**合并之后、物化之前**，按层序自底向上单遍推进（等价于保证收敛的定点）；
4. 血缘新增**依赖边**（不塞进 `shadowed`），`--explain` 展开一层依赖；
5. 明确**不越 K7**：无函数、无递归、无控制流；第一步**不做字符串拼接**。

### 14.6 开工前必须先定的事（决策清单）

| # | 决策 | 选项 |
|---|------|------|
| 1 | 兼容策略 | 显式开关（`--interpolate`）／新语法（`@{path}`、`${{ }}`） |
| 2 | 求值模型 | (a) 加载时单遍／(b) 合并后定点／(c) 两阶段（见 14.2 的后果差异） |
| 3 | 循环处理 | 结构排除（只允许向后/更低层引用）／检测器 + 迭代上限 |
| 4 | 缺失引用的错误类别 | 复用"缺失"（exit 6）／规则错（exit 4）／新码 |
| 5 | 类型与转义 | 整值引用是否保类型；`$$`／`\${` 转义规则 |
| 6 | 与物化的先后 | 插值在 `default` 兜底之前还是之后 |
| 7 | 报告语义 | `--explain` 是否展开依赖边；`unused` 是否把"被值引用"算已用 |
| 8 | 脱敏 | 插值后如何保留/追踪原始来源 |

**量级参照**：本仓"结构键"重构（纯内部改动）触 31 文件、+512/−440；插值还额外需要 ADR、新规则章节、
迁移说明与新诊断 → **L/XL，且设计（第 1、2 条）重于编码**。

**替代方案**：若驱动力只是"环境变量 → 嵌套配置"，优先考虑 `--env-map`（`13-materialization.md` §13 方案 A），
成本中、不引入求值语义；`.env` 已覆盖"顶层同名"这一多数场景。
