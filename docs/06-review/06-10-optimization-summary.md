---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-09
---

# 06-10 优化总结（体积 / 性能 / 代码质量）

> 汇总本轮全部优化与检查结果。每条都有落盘基准或测试，复现命令见对应文档。
> 结论区分**确定性指标**（allocs/B/体积）与**噪声指标**（整体 ms/op）。

## 1. 体积与依赖

| 项 | 结果 | 证据 |
|----|------|------|
| YAML 编码统一到 `go.yaml.in/yaml/v3` | 二进制 **5.35 → 5.16 MB（−184 KB，−3.6%）**；移除 `sigs.k8s.io/yaml` 与 `go.yaml.in/yaml/v2` | ADR-004 §10、06-07；`just size` |
| 默认/sonic 体积 | 5,177,607 B / 8,864,007 B（本轮优化期间**逐字节不变**） | `just size`、`just size sonic` |

## 2. 渲染与解析性能（确定性指标为准）

| 优化 | 指标 | 改前 → 改后 |
|------|------|-------------|
| expression 解析（`splitPipes` 零拷贝切片 + Steps 预分配） | `ParseExpression/default` | 116.7 ns、4 allocs → **61.6 ns、2 allocs** |
| 渲染快路径 span 名零拷贝（`[]byte` 贯通 + `ResolveRootName`/`ResolveEnvName`） | `RenderLargeTemplate`（200k 变量） | allocs/op 400,021 → **21**；B/op 36.40 → **29.99 MB** |
| 同上（门禁侧） | `test/perf` RSS 比值 | 1.40x → **1.05–1.12x**；render 0.41–0.43x |
| yamlx 标量按 tag 直解 + 键快路径 | `Decode`（1000 服务） | allocs/op 121,059 → **94,056（−22%）**；B −25%；ns ≈ −14% |
| 路径解析（`ParsePath` 预分配 / `ParsePathPrefix` 零收集） | `ParsePath` / `ParsePathPrefix` | 5 → **2 allocs**（−45% 时间）/ 5 → **0 allocs**（≈6×） |

## 3. 已检查但**有意不改**（均有数据）

| 路径 | 实测 | 结论 |
|------|------|------|
| `source.ParseDotenv` | 190 ns/键、1.66 allocs/键 | 预分配收益 <1%，必要物化无法避免 |
| `schema.Validate` | 139–154 µs/200 属性、10 allocs/属性 | 分配在 `jsonschema/v6` 内部，本项目侧无可改点 |
| `values.MergeInto` | 160 µs/5000 键、**18 allocs** | 原地写入，已接近下限 |
| 输出写入 / streaming | render 0.43x、RSS 1.12x、输出 7,666,670 B 与 GNU 基线逐字节一致 | 已用 `bufio` + 原子写、读取缓冲复用 |
| CLI 启动 | **4.1–4.5 ms**（预算 100 ms） | 远低于预算 |

## 4. 正确性与规范（同批落地）

| 项 | 结果 |
|----|------|
| 序列化规则步骤 1–11 | nil 遵循库默认 `{}`/`[]`、`codec.ExactInt` 精确整数贯通、`NaN`/`±Inf` 三路径拒绝、`float` 仅 float64 范围、`codec.LiteralNumber` 未声明字面量保留（`1.0`→`1.0`）、删除 `--set-string`、删除 `\| type null` 的 `""`→nil、非法 JSON 统一 exit 4 |
| 测试面 | `test/e2e` 18 fixture、`test/encoding` 规则矩阵与类型表、`test/jsonx`、`test/yamlx`、`test/perf` + 各包用例；两后端 22 包全绿 |
| 规则文档 | `docs/07-serialization/` 15 篇（v0.9），章节拆分且全局 § 编号不变 |

## 5. 代码质量

- 重复字符判断 7 → 1 helper（`codec.IsDigit`）；`".eE"` 7 → 1（`numberMarks`）；
  JSON kind 判断改用 `jsontext` 具名常量；手写 JSON number 状态机 → `jsontext` 校验；
- 解析审计：5 处可换标准库的替换（`maps.Copy`、`strings.Cut`、`bytes.Count` 等），
  必须自研的文法补"为什么不能用标准库"注释 + 反例/分配数据；
- 三处审计发现修复：缺失值文件专用文案、`streamWindow` 读取缓冲复用、
  `diag.Error` 携带结构化 `Location`（删除从错误文案反解 offset，字段 6 → 4）；
- `justfile`：缓存目录兜底 + `caches` 依赖，保证全新 clone 可直接 `just build`；
  新增 `clean-cache`/`clean`。

## 6. 流程规则（`docs/prompts/_common.md` §7）

1. 优先标准库，自建需给理由与数据；2. 对照实验落盘；3. 数据附复现命令；
4. **任何检查/优化 review 都必须整体落盘**（基准或测试 + 06-review 记录 + 05-tasks 状态），
   **"不优化"同样需要数据支撑**；5. 性能任务先落基准、区分确定性与噪声指标、收益 <5% 结论为不改；
6. 禁止 `sleep`/轮询空等后台任务。

## 7. 仍待处理（需产品决策，见 `07-serialization/09-gaps.md`）

| 项 | 内容 |
|----|------|
| G11 | schema 联合类型 `type: ["integer","null"]` 的物化/校验（待实现） |
| G14 | 嵌套 env fallback 纳入物化与 `--validate-only`（待实现） |
| G12 | `$ref`/`allOf` 处的 `default` 生效（待评估） |
| G13 | `array<T>` 在 schema 与 inline 两侧诊断统一（待统一） |
| G4/G6/G10/G16/G19 | 已决：维持现状并写明规范（见 09-gaps 结论列） |

## 8. 性能复查（G11–G14 / F / F2 / 待办 E 之后）

复查动机：本轮对 schema 层做了较大重构（`expandSchema`+`s.walk`、visitor 化遍历、union 成员集合、
env 回退、两个 CLI 开关），需确认渲染热路径与解析路径**没有回退**。

确定性指标（allocs/B）——与各自基线逐项对比：

| 基准 | 基线 | 复查 | 结论 |
|------|------|------|------|
| `RenderLargeTemplate`（200k 变量） | 21 allocs / 29,999,200 B | **21 allocs / 29,999,220 B** | 持平 |
| `ParsePath` | 2 allocs / 168 B | **2 allocs / 168 B** | 持平 |
| `ParsePathPrefix` | 0 allocs / 0 B | **0 allocs / 0 B** | 持平 |
| `Decode`（1000 服务） | 94,056 allocs | **94,056 allocs** | 持平 |
| `ParseDotenv`（1000 键，source_test） | 1,657 allocs | **1,657 allocs** | 持平 |
| `MergeInto`（50×100 键） | 18 allocs | **18 allocs** | 持平 |
| `ValidateLargeTree`（200 属性） | 1,990 allocs | **1,990 allocs** | 持平 |
| `ParseExpression/default` | 1 alloc / 16 B（上次优化后） | **1 alloc / 16 B / 45.0 ns** | 持平 |

门禁（`just test-all`，7 次取中）：

```
perf: render GNU=62.97ms envsub=27.28ms ratio=0.43x limit=5.0x
perf: rss    GNU=11.8MiB envsub=13.1MiB ratio=1.11x limit=5.0x
perf: startup envsub=4.75ms budget=100ms
perf: baseline output matches byte for byte (7666670 bytes)
```

复现：

```bash
go test -run '^$' -bench 'BenchmarkRenderLargeTemplate|BenchmarkParseExpression|BenchmarkDecode|BenchmarkParsePath' \
  -benchmem ./internal/engine/extended/ ./internal/yamlx/ ./internal/values/
just test-all
```

结论：**本轮重构未引入性能回退**；`RenderLargeTemplate` 仍是 21 allocs/op（每变量 0 分配），
输出与 GNU 基线逐字节一致。

## 9. 测试补充（本轮）

| 用例 | 覆盖的行为 |
|------|-----------|
| `TestAllOfPrecedenceIsFirstWins` | 两个 `allOf` 分支声明同名属性时**先出现者胜**（此前只写在文档与提交信息里，未验证） |
| `TestRefPointerPathResolvesNestedDefinition` | `$ref` 可指向**嵌套位置**（`#/$defs/wrapper/properties/inner`），不限于整个定义 |

### 9.1 e2e fixture（已补齐）

| fixture | args 要点 | 断言 | 为什么确定 |
|---------|-----------|------|-----------|
| `no_process_env` | `render --no-process-env --env-file .env input.tmpl` | 模板 `${PATH:-missing} ${FROM_FILE}` → `missing yes` | `PATH` 在任何 exec 环境中都存在，被 `--no-process-env` 排除后必然走默认值；`.env` 仍生效 |
| `env_prefix` | `render --process-env-prefix ENVSUB_FIXTURE_ONLY_ --env-file .env input.tmpl` | 同上 → `missing yes` | 无进程变量带该前缀，`PATH` 被白名单排除 |
| `env_options_conflict` | `render --no-process-env --process-env-prefix ENVSUB_FIXTURE_ONLY_ input.tmpl` | `expect.err` 含 `mutually exclusive`，`exit` = 2 | 互斥校验在装配 `Env` 前触发 |

复现：`go test -count=1 -run 'TestE2E/(no_process_env|env_prefix|env_options_conflict)' -v ./test/e2e/`

仍待补（`05-03` 待办 G）：`internal/app` 与 `cmd/envsub` 覆盖率为 0；`--process-env-prefix` 的**正例**
（匹配前缀的进程变量确实生效）目前只有单测覆盖，见 `05-03` §10 待决问题 1。
