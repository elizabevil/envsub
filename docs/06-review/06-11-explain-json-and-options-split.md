---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-10
---

# 06-11 结构变更与 `--explain` JSON 化（本轮记录）

> 记录本轮两项改动：`render.Options` 拆分为"CLI 旗标层 + `render.Request`"，以及
> `--explain` 从 TSV 改为固定 JSONL 并支持 `--explain=PATH` 过滤。
> 依据 `docs/prompts/_common.md` §7（代码实现与实验约束）：验证落盘为测试、结论落盘到本文件、
> 状态同步到 `docs/05-tasks/`。

## 1. 背景与目标

- `render.Options` 有 20 个平面字段、14 个方法、64 处内部引用，同时扮演"cobra 旗标面板"与
  "内部请求对象"两个角色，`Templates`（位置参数）与 `ExplainTo`（运行时 IO）也混在其中。
  `[FACT]`（来源：`internal/render/render.go` 重构前实测，见 §5 复现命令）
- `--explain` 需要固定 JSON 输出，并支持指定某个模板变量的渲染路径。`[FACT]`（来源：用户本轮决策）

## 2. 结论摘要

| 变更 | 结论 | 证据 |
|------|------|------|
| 结构 | CLI 旗标层与内部请求层分离（方案 B）；`renderFlags` 与 `render.Request` 各自分组 | `just check`、`docs/05-tasks/05-03-pending-work.md` §12 |
| 校验 | 互斥与策略值校验统一到 `Request.Validate()`（CLI 与库调用方共用） | `TestValidateEnvSources`、`TestRunOptionValidation` |
| 输出 | `--explain[=PATH]` 固定 JSONL → stderr；删除 TSV、批量前缀、遮蔽列截断 | `TestExplainTemplateWritesJSONL`、6 个 e2e fixture |
| 依赖 | 发现并规避 json 两后端对 `omitempty` 数值 0 的差异（G20） | `TestMarshalOptionalFieldsAgree` 等 |

## 3. 变更 1：`Options` → CLI 旗标层 + `render.Request`

### 候选方案与权衡（Alternatives & Trade-offs）

| 方案 | 做法 | 结论 |
|------|------|------|
| A 仅分组 | `Options` 保留 6 个子结构，cobra 直接绑定 | **未采用**：仍是同一个类型承担两种角色，旗标增删会直接改动内部模型 |
| **B 拆分（采用）** | `internal/cli.renderFlags`（解析）→ `render.Request`（校验+执行） | 旗标层与内部模型独立演进；CLI 只做一层映射 |
| C 只拆文件 | 字段不动，按职责分文件 | **未采用**：只改善排版，20 字段的平面结构问题不变 |

`[INFERENCE]`（依据：三种方案对"旗标变更影响面"的比较。推断 B 的长期维护成本最低，代价是多一份
结构与一层映射。）

### 实现要点

- `internal/render/options.go`：`Request{Input, Values, Output, Policies, Explain, ValidateOnly}` 六组；
  `Validate()` 收敛原先分散在 `render.go` 的 `validate()` 与 `validateEnvSources()`。
- `internal/cli/render.go`：`renderFlags` 同名分组，`request(args, stderr)` 装配；
  cobra 只解析，不校验。
- 复用既有 API：`--explain` 的"简单标识符"判定改用 `values.IsSimpleIdentifier`，删除了本轮一开始
  在 `render` 里重复实现的同名函数（遵守 §7.1）。`[FACT]`

## 4. 变更 2：`--explain[=PATH]` 固定 JSONL

### 旗标面的候选方案与权衡

| 方案 | 形式 | 结论 |
|------|------|------|
| 独立过滤器 | `--explain` bool + `--explain-path PATH` | **未采用**：多一个开关；与本轮确定的"裸 `--explain` = all"不符 |
| **可选值（采用）** | `--explain[=PATH]`，裸 = all（`*` 哨兵） | 一个旗标表达 all 与过滤；代价是路径必须写 `=` |
| 逗号列表 | `--explain-path a,b` | **未采用**：模板路径允许引号键，`app["my,key"]` 含逗号，切分破坏路径 |

`[FACT]`（来源：用户本轮决策；pflag 可选值语义见 `github.com/spf13/pflag@v1.0.10/flag.go` 的
`NoOptDefVal` 处理）

### JSONL 契约（与 `13-materialization.md` §15 一致）

每条一行：`template`、`variable`、`origin`、`shadowed`（高→低，不截断）；帧的字段为
`kind`/`method`/`source`/`file`/`line`。**不输出值**（值可能是密钥）。详见 ADR-005。`PATH` 是精确匹配的过滤器，只在模板已引用的
名字里筛选；`${env.NAME}` 与 `NAME` 归一后比较。`[FACT]`

## 5. 发现：json 两后端对 `omitempty` 数值 0 不一致（G20）

### 实测

同一结构体 `type record struct{ Whole int \`json:"whole,omitempty"\` }`，零值序列化：

| 后端 | 输出 |
|------|------|
| `encoding/json/v2`（默认） | `{"whole":0}` |
| `sonic`（`-tags=sonic`） | `{}` |

`[FACT]`（来源：本轮实测；已落盘为测试，见 §6）

### 处理

`--explain` 的 `line` 改为 `*int` + `omitempty`：nil 时两后端都省略、非 nil 时都输出，
从而保证 `--explain` 在两种构建下逐字节一致。未改动任何后端。`[FACT]`
Gap 记录：`docs/07-serialization/09-gaps.md` G20。

## 6. 证据与复现命令

```bash
# 结构与行为回归（默认 json/v2 后端）
just check

# 含性能门禁的完整门禁
just test-all

# 本轮新增/修改的关键用例
go test -count=1 -run TestExplainTemplateWritesJSONL ./internal/render/
go test -count=1 -run TestExplainTemplateFiltersByExactPath ./internal/render/
go test -count=1 -run TestExplainerSource ./internal/render/
go test -count=1 -run TestRunOptionValidation ./internal/render/
go test -count=1 -run TestRunInvalidSetJSONIsTypeError ./internal/render/
go test -count=1 ./internal/app/ ./cmd/envsub/
go test -count=1 ./test/e2e/

# 两后端一致的确认（G20）
go test -count=1 -run TestMarshalOptionalFieldsAgree ./internal/jsonx/
go test -count=1 -run TestJSONv2MarshalKeepsOmitemptyZeroInt ./internal/jsonx/
go test -count=1 -tags=sonic -run TestSonicMarshalOmitsOmitemptyZeroInt ./internal/jsonx/

# 端到端 fixture（用户可见行为）
ls test/e2e/testdata/explain_*
```

`[FACT]`（`just check` 本轮退出码 0；上列命令均可直接重跑）

## 7. 非目标（Non-Goals）

1. **不在本轮改动 JSON 解码/编码后端的语义**：G20 只在编码侧约束（可选数字用指针），不修改
   `internal/jsonx` 两后端的既有行为，也不新增后端。
2. **不为 `--explain` 增加值输出**：值可能是密钥；本轮只输出来源，`--explain-values` 之类的开关
   不在范围内。
3. **不把 `--explain` 做成求值入口**：`PATH` 只在模板已引用的名字里过滤，不解析任意路径、不触发
   缺失策略、不改变渲染结果与退出码。
4. **不引入新的输出格式，也不做 `--explain-out`**：只保留 JSONL → stderr；需要落盘用 shell 重定向
   （`2> explain.jsonl`）。报告与渲染结果同进程写出，单开一个输出文件不带来额外能力。`[FACT]`
   （来源：2026-10-10 决定）
5. **不做性能优化承诺**：本轮为结构调整，不宣称耗时/内存收益；等价性由门禁证明。

## 8. 待确认（TODO）

- `[已决]` `shadowed` 已改为**结构化帧数组**，与胜出帧同构（`kind`/`method`/`source`/`file`/`line`），
  见 ADR-005；`source` 与 `file` 不再互相重复表达（2026-10-13）。
  owner：产品/技术负责人。
- ✅ **已决（2026-10-10）：暂不提供 `--explain-out`，也不加 table/YAML 等格式**；`--explain` 固定
  JSONL → stderr，落盘交给 shell 重定向。如后续确有"审计报告需要带模板汇总"的需求再重新评估。
  `[FACT]`（来源：用户本轮决定）
- `[ASSUMPTION]` 模板赋值（`:=`/`=`）产生的名字报告为 `missing` 可接受。验证方式：若使用者反馈
  误导，再为其记账来源。owner：技术负责人。
