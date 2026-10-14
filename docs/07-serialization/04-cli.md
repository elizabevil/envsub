> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §6。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 6. CLI 行为规则（C 系列）

**本章解决**：CLI 值来源（两档，C2 已删除）、优先级与覆盖顺序、与 schema/inline type 的交互、策略开关、错误
分类与已知差异。**CLI 层不做类型猜测**；类型由 §8（`| type`）与 §9（schema）定义。

### 6.1 值来源两档（C1、C3；C2 已删除）

_列：ID | 档位 | 语义 | 状态。_

| 编号 | 档位 | 语义 | 状态 |
|------|------|------|------|
| C1 | `--set KEY=VALUE` | **存字符串**；类型只由 schema 声明或 `\| type xxx` 决定 | ✅ |
| C2 | ~~`--set-string KEY=VALUE`~~ | **已删除**：与 `--set` 重复且与 schema 声明冲突；强制字符串改用 `--set-json KEY='"value"'` | 已删除 |
| C3 | `--set-json KEY=<JSON>` | JSON 字面量 → 精确类型树（int64 → uint64 → 精确整数） | ✅ |

**`-f/--values-file` 的边界**：`--values-file` 合并进**根命名空间**，只接受
`.yaml`/`.yml`/`.json`；`.env` 必须走 **`--dotenv-file`**。传 `.env` 给
`--values-file` 报 `KindUsage` exit 2（"use --dotenv-file for .env files"），不再被静默当作 dotenv 来源。
`[FACT]`（来源：2026-10-10 定案、2026-10-10 更名；fixture `values_rejects_dotenv`、单测 `TestRunValuesFileRejectsDotenv`）

**`--dotenv-file` 按 flag 语义强制 dotenv**：格式由 flag 指定，**与文件名/扩展名无关**——
`.env`、`debug.env`、`.env.local`、`.env.production`、无扩展名的 `production` 都以 dotenv 解析。
判定不再经过 `source.ForFile`。该文件写入**根命名空间**。`[FACT]`（来源：2026-10-10 修正；
fixture `env_file_nonstandard`、单测 `TestRunEnvFileIgnoresExtension`、`TestDotenvLoaderIgnoresFileName`）

**C3 命名理由**

- 名字描述的是**值的语法**（右侧必须是合法 JSON 字面量），不是结果类型：`--set-json x=8080` 得到 JSON number，
  `--set-json x='"8080"'` 得到 JSON string，`--set-json x=null` 得到 null。
- 与 Helm 3.10+ 同名同义（`--set-json` = 按 JSON 解析该键的值），用户既有习惯可直接迁移；
  对比之下 `--set-typed` / `--set-value` / `--set-literal` 都没有生态先例，且"typed"无法说明用什么语法。
- 与 C1 构成清晰的一对：**默认字符串（`--set`）、需要类型时给 JSON（`--set-json`）**；
  删除 `--set-string` 后这层对比反而更清楚（此前 `--set` 与 `--set-string` 行为几乎相同，是混乱来源）。
- 若将来需要 YAML 字面量，可按同一命名法加 `--set-yaml`，不冲突。
- 非法 JSON（`--set-json` 与 `.env` 物化）统一为 KindType exit 4（G9）；赋值语法错误仍为 KindUsage exit 2。

引用：Helm `install/upgrade` 的 `--set-json`（[Helm 文档](https://v3.helm.sh/es/docs/v3/helm/helm_install/)）。

编号映射（向后兼容旧引用）：CLI1 = C1，CLI2 = C2，CLI3 = C3，CLI4 ⊂ C12，CLI5 ⊂ C12。

**C2 删除理由（历史记录）**：`--set` 本身即字符串语义、类型由 schema 或 `| type` 决定，因此
`--set-string` 唯一可能的差异（绕过 schema 物化，即 G17）与 schema 声明相冲突，属病态场景；
需要"强制字符串"时用 `--set-json KEY='"value"'` 即可。Helm 需要 `--set` 是因为它做类型
猜测，本项目不做（C12）。**结论：不提供 `--set-string`，统一到 `--set`**；
备选方案：若保留，则必须实现"跳过 schema 物化"（§14 步骤 10 备选）。

语义选择对照：

_列：想要的语义 | 用法 | `${x}` | `\| toJson`。_

| 想要的语义 | 用法 | `${x}` | `\| toJson` |
|------------|------|--------|-------------|
| 数字 | `--set-json x=8080`，或 `--set x=8080` + `\| type int` | `8080` | `8080` |
| 像数字的字符串 | `--set x=8080`（默认档）；强制字符串可用 `--set-json x='"8080"'` | `8080` | `"8080"` |
| 字符串（默认档） | `--set x=8080` | `8080` | `"8080"` |

**案例（C1–C3）**

| 规则 | 命令 | 输出 / exit | 标注 |
|------|------|-------------|------|
| C1 | `--set 'x=8080'` + `${x \| toJson}` | `"8080"` | 现状 |
| ~~C2~~ | 已删除；原用例：`--set 'N=8080'` + schema `integer` + `${N \| toJson}` | `8080`（按 schema 物化） | 已删除 |
| C2 | `--set-json 'x="8080"'` + `${x \| toJson}` | `"8080"`（强制字符串的替代写法） | 现状 |
| C3 | `--set-json 'x=18446744073709551616'` + `${x \| toJson}` | `18446744073709551616` | 现状 |

### 6.2 优先级与覆盖顺序（C4–C5）

_列：ID | 规则 | 证据。_

| 编号 | 规则 | 证据 |
|------|------|------|
| C4 | 同一来源内**后写入覆盖先写入**；CLI 两档按固定组顺序应用（`--set` → `--set-json`），与命令行参数书写顺序无关 | render.go:416-433 |
| C5 | 值来源总优先级见下表 | 实测 |

**定稿顺序（自上而下，先命中者胜）**：

```
根命名空间（唯一配置面）
  --set / --set-json  >  --dotenv-file  >  -f/--values-file  >  schema default

@process 命名空间（只读，恒 string，不参与层序）
  ${@process.NAME}  ← 进程环境，受 --no-process-env / --process-env-prefix 约束
```

值来源优先级（**上高下低**，同 key 冲突时高者胜）：
_列：优先级 | 来源 | 说明。_

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1（最高） | `--set` / `--set-json` | 写入**根命名空间**；组顺序固定（`--set` → `--set-json`），组内后写入者胜，与书写顺序无关 |
| 2 | `--dotenv-file`（`.env`） | **根命名空间**；多个文件时**后出现者覆盖先出现者** |
| 3 | `-f` / `--values-file` | 根命名空间；后一个文件覆盖前一个，object 深合并，数组/标量替换 |
| 4（兜底，非来源） | schema `default` | 只在本 key **任何来源都没有**时填入；见下方说明 |

**进程环境不在层序内**：它只通过 `${@process.NAME}` 显式读取，不再参与 `${NAME}` 解析。
因此 `${K}` 永不因宿主环境里恰好有 `K` 而改变取值。`[FACT]`（来源：2026-10-10 用户定案；见 `06-15`）

**`.env` 高于值文件**：`.env` 是"本机/本部署"的局部层，值文件是仓库内的共享基线，局部压基线。`[FACT]`

**`default` 是兜底，不是一层来源**：它只在该 key 于**根命名空间**缺失时才填入
（`.env` 与 `-f` 都在根内，故都被尊重）。理由是"**显式指定 > 隐式包含**"，而 `default` 只是 schema 的兜底，
不应压过任何实际来源。`[FACT]`（见 §13.1 G21）

其他优先级：

_列：关系 | 规则 | 证据。_

| 关系 | 规则 | 证据 |
|------|------|------|
| schema ↔ inline `\| type` | 先按 schema 类型物化，再做兼容性检查，最后按 inline 断言；冲突报错 | engine.go:325-362；§8.3 |
| `type` ↔ formatter | `type` 必须是管道**第一个**步骤；放在 formatter 之后报 KindTemplate=3 | expression.go:89-91 |
| `:=`/`=` 写回 ↔ 后续引用 | 写回后同一次渲染的后续引用可见；`${A:-d}` 不写回 | policies/assign_default、default_does_not_assign |

**案例（C4–C5）**

| 规则 | 命令 | 输出 | 标注 |
|------|------|------|------|
| C4 | `--set 'x=1' --set 'x=2' --set-json 'x=3'` + `${x}` | `3` | 现状 |
| C4 | `--set-json 'x=3' --set 'x=1'` + `${x}` | `3`（组顺序与书写顺序无关） | 现状 |
| C4 | `--set 'x=1' --set 'x=2'` + `${x}` | `2`（同组后写胜） | 现状 |
| C5 | `-f v1.yaml -f v2.yaml`（两文件同 key） | 后一个文件的值 | 现状 |
| C5 | `--dotenv-file a.env --dotenv-file b.env`（同 key） | `b.env` 的值（后出现者胜） | 现状；fixture `explain_multi_env`／§14 |
| C5 | `-f v.yaml`（`K: from_values`）+ `--dotenv-file e.env`（`K=from_dotenv`）+ `${K}` | `from_dotenv`（`.env` 层更高） | ****；fixture `dotenv_beats_values`，见 `06-15` §3.1 D4 |
| C5 | 进程环境 `K=from_proc`，无 `.env`／值文件／`--set`，模板 `${K}` | **缺失**（不再回退环境） | ****；fixture `no_env_fallback` |
| C5 | 进程环境 `K=from_proc` + `${@process.K}` | `from_proc` | ****；fixture `process_namespace` |
| C10 | `--no-process-env` | 关 | 完全不读取进程环境（`@process` 为空）；`--dotenv-file` 内容不受影响 |
| C10 | `--process-env-prefix PREFIX` | 空 | 只暴露带该前缀的**进程**变量（白名单，保留原名）；与 `--no-process-env` 互斥（exit 2） |
| C5 | schema `default` + `--dotenv-file` 同 key | `.env` 的值（default 只在该 key 无任何来源时兜底） | ****仍在根内比较；fixture `schema_default_is_fallback` |

### 6.3 与 schema / inline type 的交互（C6–C7）

_列：ID | 规则 | 证据。_

| 编号 | 规则 | 证据 |
|------|------|------|
| C6 | 字符串值先由 `Materialize` 按 schema 类型物化；渲染时若仍有 schema 类型则 `applyType` 再转换；inline `\| type` 与 schema 类型冲突时报 `inline type %s conflicts with schema type %s`，exit 4 | engine.go:325-362；§8.3 |
| C7 | `type` 必须是管道第一个步骤；重复 `type` 或位置错误报 KindTemplate=3 | expression.go:85-91 |

**案例（C6–C7）**

| 规则 | 命令 | 输出 / exit | 标注 |
|------|------|-------------|------|
| C6 | `--set 'N=8080'` + schema `N: integer` + `${N}` | `8080`（被物化为 int） | 现状 |
| C6 | 同上 + `${N \| type int}` | `8080` | 现状 |
| C6 | 同上 + `${N \| type string}` | `N: inline type string conflicts with schema type integer`，exit 4 | 现状 |
| C6 | 同上 + `${N \| type float}` | `8080`（int↔float 互通） | 现状 |
| C7 | `${N \| quote \| type int}` | `inline type must be the first pipeline step`，exit 3 | 现状 |

### 6.4 策略开关（C8–C9）

**`--only-vars` / `--ignore-vars` 的匹配规则**：带 `.` 的 pattern 匹配**过滤键**的
整体或路径前缀；不带 `.` 的 pattern 匹配**根段**。过滤键与根段都由 `internal/values` 的语法解析得出，
不再由引擎手写拆分（依据 `06-review/06-17` D4）。由此：

| pattern | 模板写法 | 是否命中 | 说明 |
|---------|----------|----------|------|
| `app` | `${app.port}` | ✅ | 裸 pattern 匹配根段 |
| `app.port` | `${app.port}` | ✅ | 带 `.` 必须整串或前缀 |
| `a.b` | `${["a.b"].k}` | ✅ | 根路径按**不带引号**的键比较 |
| `@process.NAME` | `${@process.NAME}` | ✅ | 命名空间路径**保留 `@` 记号** |
| `process.NAME` | `${@process.NAME}` | ❌ | 少了 `@` 就是另一条根路径 |

_列：ID | 开关 | 取值 | 行为。_

| 编号 | 开关 | 取值 | 行为 |
|------|------|------|------|
| C8 | `--on-missing` | `error`（默认） | 变量未设置 → KindMissing，exit 6（`variable "A" is not set`） |
| C8 | | `empty` | 未设置 → 输出空串，exit 0 |
| C8 | | `keep` | 未设置 → 原样输出占位符（嵌套 default word 时整体保持），exit 0 |
| C8 | | `zero` | 未设置 → 按 schema/inline 类型输出零值（int `0`、float `0`、bool `false`、string 空、array `[]`、object `{}`、null 空）；**无类型声明时报 KindPolicy，exit 6** |
| C8 | | 其它 | KindUsage，exit 2（`invalid --on-missing value %q: want error, empty, keep or zero`） |
| C9 | `--on-empty` | `ok`（默认） | 空值正常输出/继续 |
| C9 | | `error` | 空值 → KindPolicy，exit 6（`variable "A" is empty`） |
| C9 | | `keep` / `zero` | **当前不支持** → KindUsage，exit 2（`invalid --on-empty value %q: want ok or error`）→ G19 待定 |

补充规则：

- `--on-empty` 只作用于普通引用 `${A}`；带操作符的 `${A:-d}` 等不受影响。
- `--on-empty=error` 对 `0`/`false` 不触发（它们非空，OV2）。

**案例（C8–C9）**

| 规则 | 命令 | 输出 / exit | 标注 |
|------|------|-------------|------|
| C8 | `${A}`（未设置），默认 | exit 6 | 现状 |
| C8 | `--on-missing=empty` + `${A}` | 空，exit 0 | 现状 |
| C8 | `--on-missing=keep` + `${A}` | `${A}`，exit 0 | 现状 |
| C8 | `--on-missing=zero` + `${A}`（无类型） | `--on-missing=zero requires a schema type or an inline \| type`，exit 6 | 现状 |
| C8 | `--on-missing=zero` + schema `A: integer` + `${A}` | `0` | 现状 |
| C8 | `--on-missing=bogus` | exit 2 | 现状 |
| C9 | `--on-empty=error` + `--set-json 'A=""'` + `${A}` | exit 6 | 现状 |
| C9 | `--on-empty=error` + `--set-json 'A=0'` + `${A}` | `0`，exit 0（0 非空） | 现状 |
| C9 | `--on-empty=keep` | exit 2（G19） | 现状/待定 |

### 6.5 错误分类（C10）

_列：ID | 规则 | 证据。_

| 编号 | 规则 | 证据 |
|------|------|------|
| C10 | 所有错误按 `diag.Kind` 分类并映射到固定退出码；完整目录见 §11 | diag/errors.go:111-136 |

**案例（C10）**：见 §11 每行的"文案示例"，均可用 exit code 断言。

### 6.5.0 从文件取值（`--set-file KEY=FILE`）

| 编号 | 规则 | 证据 |
|------|------|------|
| C16 | 值 = 文件字节，作为**字符串**写入根；结尾的**单个换行（含 CRLF）被去掉**，其余字节保留（多行值内部换行不变） | `load.setOneFile` |
| C16 | 层内顺序：`--set` → `--set-json` → `--set-file`，同键时后者胜（与书写顺序无关） | fixture `set_file`、单测 `TestSetFileWinsOverSet` |
| C16 | 文件不可读 → KindIO exit 5（不是用法错误）；赋值形式错误 → KindUsage exit 2 | fixture `set_file_missing` |
| C16 | 与 `.env`/值文件里的字符串一样可被 schema 或 inline `\| type` 物化 | 单测 `TestSetFileReadsTheFilesBytes` |

**案例（C16）**

| 命令 | 输出 / exit | 标注 |
|------|-------------|------|
| `--set-file TOKEN=token.txt`（文件内容 `s3cr3t\n`） | `s3cr3t` | fixture `set_file` |
| `--set TOKEN=x --set-file TOKEN=token.txt` | 文件的值 | 单测 |
| `--set-file TOKEN=absent.txt` | `read --set-file absent.txt: ...`，exit 5 | fixture `set_file_missing` |
| `--set-file TOKEN` | `assignment "TOKEN" must be in KEY=VALUE form`，exit 2 | 单测 |

### 6.5.1 输出过期检测（`--check`）

| 编号 | 规则 | 证据 |
|------|------|------|
| C13 | `--check` 把渲染结果与现有目标**逐字节比较**，不写任何文件；存在过期目标时以 `KindDiff` 报错并 exit **7** | `internal/output/check.go`；`internal/render/render.go` |
| C13 | 必须有输出目标：`-o` / `-i` / `--output-dir` 之一；否则 KindUsage exit 2 | `render.Request.Validate` |
| C13 | 与 `--validate-only` 互斥（KindUsage exit 2） | 同上 |
| C13 | 目标文件**不存在**计为过期（渲染会创建它），不是用法错误 | `CheckWriter.Write` |

**案例（C13）**

| 命令 | 输出 / exit | 标注 |
|------|-------------|------|
| `--check -o out.txt`，`out.txt` 与渲染结果一致 | 无输出，exit 0 | fixture `check_matches` |
| `--check -o out.txt`，`out.txt` 内容过期 | `1 of 1 outputs are outdated: out.txt`（stderr），exit 7 | fixture `check_outdated` |
| `--check -o new.txt`，`new.txt` 不存在 | `1 of 1 outputs are outdated: new.txt`，exit 7；文件**不被创建** | 单测 `TestCheckReportsOutdatedAndWritesNothing` |
| `--check --output-dir outdir a.tmpl b.tmpl`，仅 `a` 过期 | `1 of 2 outputs are outdated: outdir/a.tmpl`，exit 7 | fixture `check_batch_outdated` |
| `--check`（无输出目标） | `--check needs an output target: --output, --in-place or --output-dir`，exit 2 | fixture `check_without_target` |
| `--check --validate-only` | `--check cannot be combined with --validate-only`，exit 2 | fixture `check_with_validate_only` |

### 6.5.2 机器可读错误（`--error-format`）

| 编号 | 规则 | 证据 |
|------|------|------|
| C14 | `--error-format=json` 时失败以**单个 JSON 对象**写 stderr，字段固定为 `kind`/`exitCode`/`message`；退出码与 text 形态**完全一致** | `internal/diag/report.go`；`internal/app/app.go` |
| C14 | 默认 `text`；非法取值 → KindUsage exit 2，且在任何 IO 之前拒绝 | `cli.ValidateErrorFormat` |
| C14 | `kind` 取值为 `diag.Kind`（internal/usage/template/type/schema/io/missing/policy/diff），未分类错误为 `internal`+exit 1 | `diag.NewReport` |

**案例（C14）**

| 命令 | 输出 / exit | 标注 |
|------|-------------|------|
| `--error-format=json` + 缺失变量 | `{"kind":"missing","exitCode":6,"message":"variable \"X\" is not set"}`，exit 6 | fixture `error_format_json` |
| 同上（不带 `--error-format`） | `variable "X" is not set`，exit 6 | 现有 fixture |
| `--error-format=json --check -o out.txt`（过期） | `{"kind":"diff","exitCode":7,...}`，exit 7 | 单测 + fixture `check_outdated` |
| `--error-format=yaml` | `invalid --error-format value "yaml": want text or json`，exit 2 | fixture `error_format_invalid` |

### 6.5.3 未使用值报告（`--report=unused`）

| 编号 | 规则 | 证据 |
|------|------|------|
| C15 | 列出**值文件里**从未被任何模板引用的叶子，一行一个写到 stderr，按路径排序 | `internal/render/unused.go` |
| C15 | 只统计 `--values-file` 的条目：schema `default`、`--set`/`--set-json`、`--dotenv-file`、`@process` 都不是"可删除的死配置"，不列出 | `meta.Kind == values.SourceValues` |
| C15 | 引用**子树**即使用其下全部值（`${extra \| toJson}` 使 `extra[0]`/`extra[1]` 为已用）；被 `--only-vars`/`--ignore-vars` 排除的引用**不算**使用 | 单测 `TestReportUnusedListsValueFileLeaves`、`TestReportUnusedHonoursVariableFilters` |
| C15 | 拼写与 `--explain` 的 `variable` 一致（`values.CanonicalPath`），需要引号的键写作 `["a.b"]` | `canonicalPath` |
| C15 | 非法取值 → KindUsage exit 2 | fixture `report_invalid` |

**案例（C15）**

| 命令 | 输出 / exit | 标注 |
|------|-------------|------|
| `--report=unused` + `${app.port}`，值文件有 `app.unused` 与 `extra: [a,b]` | stderr `app.unused` / `extra[0]` / `extra[1]`；stdout 不变，exit 0 | fixture `report_unused` |
| 同上但模板引用 `${extra \| toJson}` | 只列 `app.unused` | 单测 |
| `--report=path` | `invalid --report value "path": want unused`，exit 2 | fixture `report_invalid` |

### 6.5.4 值不入诊断（脱敏，C17）

| 编号 | 规则 | 证据 |
|------|------|------|
| C17 | 诊断/错误**不得回显值本身**：物化失败只报路径与来源（`H at .env:1: expected a JSON array: ...`），不回显 `notjson` | `schema.materializeError`；单测 `TestMaterializeErrorHidesTheValue` |
| C17 | `--explain` 只报来源、从不报值；`--report=unused` 只报路径 | `13-materialization.md` §15 |
| C17 | `-i`/`-o` 的原子写回经 `0600` 临时文件 + rename，目标权限位保留；临时文件对 group/other 不可读 | `output.AtomicReplace`；单测 `TestAtomicReplaceWritesThroughARestrictedTempFile` |
| C17 | 解析器（模板扫描、路径、`.env`）有 fuzz target，任意字节不得 panic | `test/fuzz`；`just --justfile test/justfile test-fuzz` |

**案例（C17）**

| 命令 | 输出 / exit | 标注 |
|------|-------------|------|
| `.env` 有 `TOKEN=sup3r-s3cret` + schema 要求 `array` | 错误含 `TOKEN` 与 `.env:1`，**不含** `sup3r-s3cret` | 单测 `TestMaterializeErrorHidesTheValue` |
| `-i` 渲染 `0755` 的模板 | 结果仍 `0755`；过程中临时文件无 group/other 位 | 单测 `TestInPlaceWriterPreservesPermissions`、`TestAtomicReplaceWritesThroughARestrictedTempFile` |
| `just --justfile test/justfile test-fuzz` | 四个目标各 fuzz 30s，无 crash | CI/本地门禁 |

### 6.6 已知差异（C11）

_列：ID | 差异 | 现状 | 状态。_

| 编号 | 差异 | 现状 | 状态 |
|------|------|------|------|
| C11 | 非法 JSON 退出码 | 已统一：`--set-json` 与 `.env` 物化均 KindType exit 4；赋值语法错误 KindUsage exit 2 | 已统一 |

**案例（C11）**

| 命令 | 输出 / exit | 标注 |
|------|-------------|------|
| `--set-json 'x=notjson'` | `set x: invalid JSON value: ...`，exit 4 | 现状 |
| `.env` 行 `H=notjson` + schema `array` | `H at .env:1: expected a JSON array: ...`，exit 4（**不回显值**，见 C17） | 现状 |

### 6.7 不做类型猜测（C12）

_列：ID | 规则 | 理由。_

| 编号 | 规则 | 理由 |
|------|------|------|
| C12 | CLI 层**必须**保持"声明类型决定类型"：不根据字符串内容猜测类型；**暂不引入** `--set-int`/`--set-float`/`--set-bool` | 猜测会静默损坏：`1.20`→`1.2`、`007`→`7`、大整数丢精度、`1e3`→`1000`；`--set-json` 已覆盖显式类型化需求 |

**案例（C12）**：`--set 'x=1.20'` + `${x \| toJson}` → `"1.20"`（字符串原样保留，不变成 `1.2`），现状。

