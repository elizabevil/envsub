---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-10
---

# 06-15 命名空间与分层重构（方案 C 定案）

> 用途：定案"路径命名空间 + 值来源分层"，作为后续代码与测试改动的唯一依据。
> 本轮是**破坏性重构**：明确不考虑历史兼容（用户 2026-10-10 指示"项目快速迭代，只考虑最优方案，
> 不考虑历史影响"）。`[FACT]`
> 状态：**方案已获用户确认（C 方案、`.env > -f`、O1=(a)、O2=(a)、O3=(a)）；代码、测试与文档同步已完成（见 §8.1 验证证据）。**

## 0. 结论速览

1. **`@` 是路径的唯一命名空间记号**，只允许出现在路径首字节，且只允许 `@process` 一个命名空间。`[FACT]`
2. **`.env` 写入根命名空间**（不再是独立命名空间），层序为
   `--set`/`--set-json` > `--dotenv-file` > `-f/--values-file` > schema `default`。`[FACT]`
3. **进程环境退出层序**，只能通过 `${@process.NAME}` 显式读取。`[FACT]`
4. **取消"根缺失 → 读环境"的分层便利**：`${NAME}` 恒读根。`[FACT]`
5. **顶层特殊键可达**：`${["a.b"].k}`、`${["a-b"]}`、`${["env"].K}`；`env` 从保留标识符降为普通键。`[FACT]`
6. `--env-file` 更名为 **`--dotenv-file`**（不留别名）；`--no-process-env` / `--process-env-prefix` 保留，
   作用域收窄为 `@process` 命名空间。`[FACT]`

定案来源：2026-10-10 用户答复（"1. 记号选 @。2. 不保留'根缺失→读环境'。"、"采纳 C 方案"、
"且 .env > -f"）。`[FACT]`

## 1. 背景：为什么要做这次重构

评审与实测（2026-10-10，命令见 §7）暴露了三个同源缺陷：命名空间靠**保留普通标识符**实现，
于是"根对象里同名/特殊键"无法表达，且不同来源挤进同一个名字。`[FACT]`

| 缺陷 | 实测现象 | 复现 |
|------|----------|------|
| 顶层特殊键不可达 | `${["a.b"]}` → exit 3 `expected identifier at offset 0`；顶层 `"a.b"` 配 `${a.b}` → exit 6 | §7 R3、R4 |
| 顶层特殊键**静默错值** | 顶层 `"a-b"` 配 `${a-b}` → **exit 0、输出 `b`**（被读成 `a` 的默认值） | §7 R5 |
| `env` 命名空间冲突 | values 顶层 `env` 的子键永不达；`--set env.K=v` 写的是 `root.env.K`，`${env.K}` 读不到 | §7 R1、R2 |
| 文档与实现不一致 | `04-cli` §6.2 写成单一链 `--set > 值文件 > .env > 进程环境 > default`；实测 `-f` 与 `.env` 同键时**两者并存不互压**（`${K}` 取 `-f`、`${env.K}` 取 `.env`） | §7 R6 |

## 2. 定案：语法与分层

### 2.1 路径文法（四行规则 + 三条约束）

```
${NAME}            根对象，裸标识符
${app.port}        根对象，点号路径
${["a.b"].k}       根对象，顶层特殊键（首段为引导索引，键可为任意字符串）
${@process.NAME}   进程环境命名空间（显式、只读、恒 string）
${@process["MY-VAR"]} 进程环境，名字含点/短横等非标识符字符
```

| # | 约束 | 违反后果 |
|---|------|----------|
| G1 | `@` 只允许出现在路径**首字节**，且只允许出现一次 | `${a@b}`、`${env.X@}` → `KindTemplate` exit 3 |
| G2 | `@` 后只接受**命名空间名**（当前唯一合法值 `process`） | `${@env.K}`（未知命名空间）、`${@["a.b"]}` → exit 3 |
| G3 | 首段裸标识符**不再有特殊含义**：`env` 就是根对象里的普通键 | 无（这是设计目标） |

数字键规则：`["0"]` 恒为**键**，`[0]` 恒为**数组下标**，不做"整数键自动转下标"。`[FACT]`（用户 2026-10-10
答复第 3 问"暂不讨论"→ 本方案取无隐式规则的默认，见 §6 开放项 O3）

### 2.2 分层（唯一配置面 + 唯一只读命名空间）

```
根命名空间（唯一配置面，按层合并，先命中者胜）
  1  --set / --set-json      组顺序固定：--set → --set-json；组内后写胜，与书写顺序无关
  2  --dotenv-file           多个文件时后者覆盖前者
  3  -f / --values-file      多个文件时后者覆盖前者
  4  schema default          兜底，不是来源

@process 命名空间（只读，恒 string，不参与任何层序）
  进程环境，受 --no-process-env / --process-env-prefix 约束（白名单保留原名，不剥离）
```

与旧版的关键差别：旧版是**两条并列链**（根链 + 环境链），`.env` 与 `-f` 分处两链、互不覆盖；
新版 `.env` 与 `-f` 处于**同一条链**的两个层，`.env` 在上。`[FACT]`

### 2.3 `--set` 键的文法

`--set`/`--set-json` 的键就是**根路径表达式**，与模板中的根写法同一套文法：`env.K` ⇔ `${env.K}`、
`["env"].K` ⇔ `${["env"].K}`。因此 `--set @process.K=v` → `KindUsage` exit 2（键表达式里没有 `@`）。
环境变量的注入通道只有进程环境与 `--dotenv-file`；**不新增 `--set-env`**（理由见 §5 候选 D）。`[FACT]`

## 3. 行为差异清单与预期效果

### 3.1 行为差异（旧 → 新）

| # | 项目 | 旧行为 | 新行为 | 是否用户可见 |
|---|------|--------|--------|--------------|
| D1 | `${env.K}` | 环境命名空间（`.env` + 进程），与根并列 | **根对象里的普通路径**（`env` 是普通键） | ✅ |
| D2 | `${K}` 根缺失时 | 回退到环境变量 `K` | **不回退**，按缺失处理（`--on-missing`，默认 exit 6） | ✅ |
| D3 | `.env` 的值所在处 | 环境命名空间 | **根命名空间**（层序 2） | ✅ |
| D4 | `-f` 与 `.env` 同键 | 并存不互压：`${K}` 取 `-f`、`${env.K}` 取 `.env` | `.env` 胜（层序 2 > 3） | ✅ |
| D5 | `.env` 键与 schema | 不受 schema 约束（不在根） | **进入 schema 校验与物化面**（见 §6 O1） | ✅ |
| D6 | 进程环境读取 | `${env.K}` / `${K}` 回退 | 仅 `${@process.K}` | ✅ |
| D7 | `--no-process-env` / `--process-env-prefix` | 影响环境命名空间（含 `.env` 之外的层） | **只影响 `@process`**；`.env` 完全不受影响（语义更干净） | ✅（语义澄清） |
| D8 | 旗标名 `--env-file` | 存在 | **移除**，改名 `--dotenv-file`；旧名 → `unknown flag` exit 2 | ✅ |
| D9 | `${["a.b"]}` 顶层特殊键 | exit 3 / 不可达 | 可用 | ✅ |
| D10 | ~~`${a-b}`（顶层 `"a-b"`）改为 exit 3~~ | `${a-b}` 输出 `b` | **保持不变**：`-` 是既有的默认值运算符（`${NAME-default}`），`${a-b}` 的行为是正确的"取 `a`，缺失则用字面量 `b`"；把它改为语法错误需要废掉运算符，收益不抵破坏。键名为 `"a-b"` 的顶层键必须写 `${["a-b"]}`（与 jq `.foo-bar` 是减法、`.["foo-bar"]` 才是键同理） | ✅ 决策已更正 |
| D11 | `--set env.K=v` | 写入 `root.env.K`（当时不可达） | 写入 `root.env.K`，`${env.K}` **可读**（同格同义） | ✅ |
| D12 | `--set @process.K=v` | （无此写法） | `KindUsage` exit 2 | ✅ |
| D13 | schema 物化中的环境兜底 | `envFallback` 会取环境命名空间的同名值（`materialize.go:128`、`defaults.go:31`） | **移除**；`@process` 是显式读取，不参与 schema 兜底 | ✅ |
| D14 | `${env.K}` 的 explain 路径 | 归一为 `K`（去 `env.` 前缀） | 归一为普通根路径 `env.K`；`@process.K` 保持带记号 | ✅ |
| D15 | `--process-env-prefix` 与 `.env` | `.env` 不受前缀影响（已如此） | 不变（现在由"`.env` 不在命名空间内"从结构上保证） | — |

### 3.2 预期效果（验收时要能逐条演示）

| # | 预期 | 验证方式 |
|---|------|----------|
| E1 | `${K}` 在 `.env K=v`、`-f K=v2` 时得 `v`（`.env` 胜） | e2e fixture `dotenv_beats_values` |
| E2 | `-f K=v2` + `.env K=v` + `--set K=v3` 得 `v3` | 单测 `TestBuildValuesLayerOrder` |
| E3 | 无任何来源时 `${K}` **不**读取同名进程环境变量，走缺失逻辑 | 单测 + e2e `no_env_fallback` |
| E4 | `${@process.K}` 读到进程 `K`；`--no-process-env` 下缺失 | e2e `process_namespace` / `no_process_env` |
| E5 | 顶层 `"a.b"` 用 `${["a.b"]}` 可读、用 `${a.b}` exit 6；顶层 `"a-b"` 用 `${["a-b"]}` 可读、用 `${a-b}` 按默认值运算符处理（输出字面量 `b`，见 D10） | e2e `toplevel_special_keys` |
| E6 | values 顶层 `env: {K: v}` + `${env.K}` = `v`；`${@env.K}` exit 3 | e2e `root_env_key` |
| E7 | `--set env.K=v` 后 `${env.K}` = `v` | 单测 + fixture |
| E8 | `--set @process.K=v` exit 2；`--env-file` exit 2（旧名） | 单测 `TestRenderHasNoLegacyFlagNames` 扩展 + e2e |
| E9 | `.env` 键在严格 schema（`additionalProperties:false`）下被校验（当前决策见 O1） | e2e schema 交互 fixture |
| E10 | `--explain` 的 `variable` 用规范写法且可粘回模板；`shadowed` 仍按高→低列出 | `internal/render/explain_test.go` + e2e `explain_*` 更新 |

## 4. 非目标（Non-Goals）

- 不引入 include / 条件 / 循环 / profiles 等模板能力。`[FACT]`
- 不实现 `--env-map`（"把 `APP_PORT` 映射到 `app.port`"）。它是"路径写回"的独立议题，与本轮命名空间重构正交。`[FACT]`
- 不改变 `--on-missing` / 退出码语义（除 D10 的静默错值修正为语法错误）。`[FACT]`
- 不改变模板其余文法（默认值/存在性运算符、`| type`、formatter 管道）。`[FACT]`
- 不改变进程环境的**名字**规则（白名单保留原名，不剥离前缀）。`[FACT]`

## 5. 候选方案与权衡（含被拒方案）

| 候选 | 内容 | 结论 |
|------|------|------|
| **C（采纳）** | `.env` 写根；`@process` 唯一命名空间；`@` 记号 | 唯一满足"一个来源一个规范名 + 不造第三概念"的方案 |
| A | 单 `env` 命名空间（`.env`+进程合并），靠 `--explain` 显示胜出来源 | **拒绝**：`env` 仍是"生效值"第三概念；同一 `.env` 键的值可能来自进程环境，仍不可区分 |
| B | 拆 `dotenv.NAME` / `process.NAME`，`env.NAME` 作生效值 | **拒绝**：同一来源出现两种写法且要再定序，把分层从树搬进模板 |
| D | 新增 `--set-env`（把 `--set` 拆成 value/env 两个功能） | **拒绝**：需要新增值来源与优先级档位、定义它与 `--no-process-env`/`--process-env-prefix` 的关系、`--explain` 溯源；且名字合法域与 `.env` 不一致（`--set-env 'a.b=v'` 造出模板只能用 `@process["a.b"]` 读的名字）。`--dotenv-file` 已覆盖该需求 |
| E | 换记号：`$env` | **拒绝**：`$` 在扩展引擎中已是变量引导符（`$NAME`、`$$` 转义，`scanner.go:13,190`），`${$env}` 会让 `$` 在 `${}` 内外各一套规则 |
| F | 保留 `env` 为保留标识符，用 `["env"]` 救根（06-14 §4.1 的原建议） | **拒绝**：特例留在最常用的标识符位置；`env` 在数据里永远无法用裸写法表达 |
| G | `.env` 归根但排在 `-f` 之下 | **拒绝（用户已定 .env > -f）**：`.env` 是本机/本部署的局部层，应压仓库内共享基线 |

## 6. 开放项（已全部裁决）

| # | 问题 | 背景（实测） | 裁决 | 状态 |
|---|------|--------------|------|------|
| **O1** | `.env` 键归根后如何面对 schema | `.env PORT=8080\nPATH=/usr/bin` + schema 只声明 `PORT`（`additionalProperties:false`）：**现状 exit 0**（`.env` 不在根）；值文件写同样两键 **exit 4** `additional properties 'PATH' not allowed`。§7 R7 | **(a) 照常纳入校验**：规则只有一条"根里的键都要过 schema"，与值文件同源；代价是 `.env` 里不能放与配置无关的宿主变量（`PATH`/`HOME`/`CI`），写入迁移说明。`[FACT]`（用户 2026-10-10 选 (a)） | ✅ 已定 |
| **O2** | `PATH`/`HOME` 等宿主变量是否允许出现在 `--dotenv-file` | `isDotenvKey` 只允许 `[A-Za-z_][A-Za-z0-9_]*`（`dotenv.go:206`） | **(a) 保持不变**：带点/短横的名字走进程环境 `${@process["MY-VAR"]}`；本轮不改字符集 | ✅ 已定（推荐项执行） |
| **O3** | 引导索引里的数字键 | 无既有实现 | **(a) `["0"]` 恒为键**，`[0]` 恒为数组下标，不做自动转换 | ✅ 已定（推荐项执行） |

O1 的连带结论：**G22 关闭为"按 (a) 实现"**（见 `09-gaps.md`），且 e2e 需新增 `dotenv_schema_strict` fixture 钉住
"`.env` 未声明键在 `additionalProperties:false` 下 exit 4"。`[FACT]`

## 7. 复现命令与实测记录

沙箱内 `GOCACHE` 需指向可写目录（默认 `~/.cache/go-build` 只读）：

```bash
export GOCACHE=/tmp/gocache
cd /home/debi/IdeaProjects/envsub && mkdir -p .probe
printf 'env:\n  K: from_values\n' > .probe/v.yaml
printf 'K=from_dotenv\n'         > .probe/e.env
printf 'K: from_values\n'        > .probe/v1.yaml
printf 'A=${env.K}\n'            > .probe/t1.tmpl
printf 'S=${K}\n'                > .probe/t2.tmpl
printf 'A=${["env"].K}\n'        > .probe/t3.tmpl
printf 'X=${a-b}\n'              > .probe/t4.tmpl
printf 'a-b: dash\n'             > .probe/v3.yaml
```

| 编号 | 命令 | 实测结果 |
|------|------|----------|
| R1 | `go run ./cmd/envsub render --set env.K=from_set -f .probe/v.yaml .probe/t1.tmpl` | exit 6 `variable "env.K" is not set` |
| R2 | `go run ./cmd/envsub render -f .probe/v.yaml .probe/t3.tmpl` | exit 3 `expected identifier at offset 0` |
| R3 | 顶层 `"a.b"` + `${a.b}` | exit 6（未设） |
| R4 | 顶层 `"a.b"` + `${["a.b"]}` | exit 3 `expected identifier at offset 0` |
| R5 | 顶层 `"a-b"` + `${a-b}` | **exit 0，输出 `X=b`**（静默错值） |
| R6 | `go run ./cmd/envsub render -f .probe/v1.yaml --env-file .probe/e.env`（模板同时取 `${K}` 与 `${env.K}`） | `S=from_values`、`A=from_dotenv` → 两命名空间并存 |
| R7 | `--schema`（`additionalProperties:false`，只声明 `PORT`）+ `.env` 含 `PATH` | exit 0（`.env` 不受校验）；同内容值文件 → exit 4 |

清理：`rm -rf .probe`（探针目录未提交，`git status` 计数保持 58）。`[FACT]`

## 8. 落地清单（本轮执行顺序）

| 阶段 | 内容 | 完成判据 | 状态 |
|------|------|----------|------|
| 1 文档先行 | 本文件 + 更新 `04-cli.md` §6.1/§6.2、`13-materialization.md` §12/§13/§14/§15、`02-sources.md`、`00-index.md`、`03-02`/`03-03`/`03-04`/`03-08`、`09-gaps.md` G14/G21/G22、`05-03` §10、README | 文档口径与 §2 一致 | ✅ |
| 2 代码 | `internal/values/path.go`（`@` 记号 + 首段引导索引）、`tree.go`/`merge.go`（命名空间收敛、逐键 meta 合并）、`source/dotenv.go`/`loader.go`（dotenv 写根 + `Dotenv` 字段）、`render/render.go`（层序）、`render/options.go`+`cli`（`--dotenv-file`）、`schema`（去 `envFallback`）、`render/explain.go` | `go build ./...`、`go vet ./...` | ✅ |
| 3 测试 | `internal/values/path_test.go`（`@process` 正反例、`["a.b"]`、数字键、`TestEnvKey`/`TestFirstKey`）、`internal/render/layering_test.go`、`internal/render/explain_test.go`、`internal/schema`/`internal/source`/`internal/app`/`internal/cli` 用例、engine fixture testdata 与 6 个新 e2e fixture | `just check`、`just test-all` 全绿 | ✅ |
| 4 收尾 | 06-review 记录实测数字与命令、`git status` 汇总 | 记录可复核 | ✅ |
| 5 实现评审 | `go fix` 现代化全部应用；修 F1（裸名读环境）/F2（`--set` 写命名空间）/F3（explain 特殊键写法）；见 §8.2 | `just check`、`just test-all` 复跑全绿 | ✅ |

### 8.1 验证证据（可复现）

```bash
export GOCACHE=/tmp/gocache     # 沙箱内默认 ~/.cache/go-build 只读
cd /home/debi/IdeaProjects/envsub
just check        # gofmt + just --fmt + vet + 单测 + e2e
just test-all     # 追加 perf 门禁
```

| 项 | 结果 |
|----|------|
| `just check` | exit 0；e2e 42 个 fixture 全通过（原 36 + 新增 6：`dotenv_beats_values`、`no_env_fallback`、`process_namespace`、`toplevel_special_keys`、`root_env_key`、`dotenv_schema_strict`） |
| `just test-all` | exit 0；perf：render `0.55x`（限 5.0x）、RSS `1.12x`（限 5.0x）、startup `7.48 ms`（预算 100 ms）、基线输出逐字节一致（7 667 670 bytes） |
| 新增单测 | `internal/render/layering_test.go::TestBuildValuesLayerOrder`、`TestProcessNamespaceReadsHostEnvironment`；`internal/values/path_test.go::TestParsePathRejects`、`TestEnvKey`、`TestFirstKey` |
| 改名/替换的测试 | `TestMergeEnvPrefersOverlay`（删除，`mergeEnv` 已删）→ 上面的 layering 用例；`TestEnvName` → `TestEnvKey`/`TestFirstKey`；`TestMaterializeTopLevelEnvFallback`/`TestValidateCatchesTopLevelEnvConstraintViolation` → `TestMaterializeConvertsRootStrings`/`TestProcessNamespaceIsNotAMaterializationSource`/`TestValidateCatchesRootConstraintViolation`；`TestApplyDefaultsDefersToEnvironment` → `TestApplyDefaultsDefersToRootValue` |
| 引擎 fixture | 23 个用 `env.yaml`（进程环境）的 fixture 模板改为 `${@process.…}`；`env.txt`（dotenv）改为写根；新增 `syntax/no_env_fallback`，`sources/process_namespace` |

### 8.2 实现评审（2026-10-12，Go 1.27.2）：工具链现代 + 两个真实缺陷

**A. 工具链现代化（`go fix`，Go 1.27 的升级语法工具；建议原文与归类见 [`06-16`](06-16-go-fix-modernization.md)）**

```bash
export GOCACHE=/tmp/gocache GOTMPDIR=$PWD/.tmp/gotmp   # 沙箱内默认缓存只读
go fix -diff ./...        # 先看建议；exit 1 表示有建议
go fix ./...              # 应用；之后 go fix -diff 为空
```

应用了 5 类（7 个文件，含 2 个本次改动文件）：

| 分析器 | 改动 | 文件 |
|--------|------|------|
| `errorsastype` | `var e *T; errors.As(err,&e)` → `errors.AsType[*T](err)` | `internal/diag/errors.go`、`test/e2e/e2e_test.go` |
| `stringscutprefix` | `HasPrefix`+`TrimPrefix` → `strings.CutPrefix` | `test/e2e/e2e_test.go` |
| `stringsseq` | `for _, l := range strings.Split(...)` → `for l := range strings.SplitSeq(...)`（免数组分配） | `test/e2e/e2e_test.go` |
| `slicessort` | `sort.Slice` → `slices.Sort` | `test/perf/perf_test.go` |
| `slicescontains` / `mapsloop` / `forvar` / `testingcontext` | `slices.ContainsFunc`、`maps.Copy`、删除冗余 `tt := tt`、`t.Context()` | `internal/codec/literal.go`、`internal/engine/extended/{engine_features,stream}_test.go`、`test/encoding/types_test.go` |
| `embedlit` | 复合字面量里 `Location: Location{Offset: …}` → `Offset: …` | `internal/diag/errors.go` |

**B. 评审发现的真实缺陷（均已修复 + 落盘测试）**

| # | 缺陷 | 症状 | 修法 | 证据 |
|---|------|------|------|------|
| F1 | **裸 `${NAME}` 仍会读进程环境** | 解析快路径 `emitSpan` 保留了 `ResolveEnvName` 回退，与 §2 定案和文档（"取消分层便利"）矛盾：`NAME=world` 时 `${NAME}` 得 `world` | 删除该回退，快路径只读根；改用例为 `TestRenderBareNameIgnoresProcessEnv`（裸名 exit 6、`${@process.NAME}` 得值） | `internal/engine/extended/engine.go`；`e2e basic_env/output_file/unset_keep/empty_error` 改为显式 `${@process.…}` 并加 dotenv `env` 文件 |
| F2 | **`--set '@process.K=v'` 写入进程环境命名空间** | `SetPath`→`assignPath` 把 `@process.K` 当成普通键写进 `root["process"]["K"]`（旧实现），既污染根又让 `--explain` 谎报 `source:set`；契约上 `@process` 只读 | `assignPath` 增加 namespace 守卫，返回 `namespace @process is read-only: assignment paths target the root namespace`（`KindUsage` exit 2） | `internal/values/set.go`；`TestSetPathRejectsNamespace`、`app_test.go` 的 exit-code 表两行（`--set`/`--set-json`） |
| F3 | `--explain` 的特殊键写法不可粘贴回模板 | `["a.b"].k` 的 `variable` 报成元数据形式 `a.b.k`，`--explain='["a.b"].k'` 过滤不到 | 新增 `values.Canonical`（模板写法）并把 `EncodePath` 的骨架提为 `EncodeSegments`；`explainName` 改用 `Canonical` | `internal/values/path.go`、`internal/render/explain.go`；`TestCanonical`（12 例往返）、`TestExplainNameAndLookupKeys` |

**C. 顺带的清晰度收敛（无行为变化）**

- `mergeMap`：原来"先判断是否对象、再各自写 meta"让 `srcMeta` 调用重复两处、`existingMap = target[key].(map[string]any)` 二次断言；改为"先记 meta，再按对象/标量分派"，对象分支用 `existing, ok := target[key].(map[string]any)` 一次断言。
- `CloneValue`：导出版本原来只是包一层未导出函数，改为直接递归（删掉重复的一层）。
- `parseNamespace` 的错误偏移原来用 `parseIdentifier` 之后的位置，改为 `start`（`${@env.K}` 现在报 offset 1；`${@.K}` 报 `expected a namespace name after "@" at offset 1`）。
- 删除无人使用的导出辅助 `values.FirstKey`（它是实现 F2 前为 `--set` 设想的守卫，实际守卫放在 `assignPath` 后已无调用者）。
- `ResolveName` 的注释同步为"只读根"，并修正 `TestRunRendersEnvironmentVariables` → `TestRunRendersDotenvValues`（名字原来说的语义已被 v0.11 取消）。

**D. 工具链缺口（如实记录）**

- `just lint`（golangci-lint 2.13.2）在当前 Go 1.27.2 上**失败于 typecheck**：`cannot decode "internal/goarch", export data version 5 is greater than maximum supported version 4`。这是 linter 自带 typechecker 与工具链导出数据版本不兼容，**不是代码问题**；本轮以 `go vet` + `go fix`（含 25 个分析器）+ `gofmt` 替代，待 golangci-lint 升级后重跑。`[FACT]`

## 9. 非目标之外的已知缺口（本轮不修）

- 退出码 1（内部错误）仍无稳定触发用例（承 `05-03` §11）。`[FACT]`
- `--explain` 对 `@process` 的 `shadowed` 展示细节（`.env` 归根后 `.env` 与 `@process` 不再是同格竞争）需在实现阶段再定粒度。`[INFERENCE]`
