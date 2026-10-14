---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-09
---

# 06-09 解析/处理路径优化 review（待办 D）

> 方法：每条路径先落盘基准、再按"占相关工作量 >5% 才改"判断（`docs/prompts/_common.md` §7 规则 6）。
> 参照量：渲染一个变量 ≈183 ns、一次大渲染（200k 变量）≈36.6 ms
> （`BenchmarkRenderLargeTemplate`，allocs 已降到 21）。

## 1. `source.ParseDotenv` —— 结论：不改

`BenchmarkParseDotenv`（1000 键，混合裸值/双引号/单引号，`internal/source/dotenv_bench_test.go`）：

| 指标 | 值 | 换算 |
|------|----|------|
| ns/op | 190 µs（= 189,713–195,246） | **190 ns/键** |
| allocs/op | 1,657 | 1.66/键 |
| B/op | 288 KB | 288 B/键 |

- 与"渲染一个变量 183 ns"同量级，因此**大 `.env` + 少量变量**的场景下解析确实占比较高，但**优化空间已被测量排除**：
  每行 1 次 `bufio.Scanner.Text()`（扫描缓冲复用，必须物化一行字符串）+ 结果 map 增长 ≈0.66 次/键；
  预分配结果 map 最多省 ~10 次分配/1000 键（<1%），低于阈值。
- 继续深入需要 `unsafe.String` 或自管行缓冲（把 `Text()` 的拷贝也省掉），收益与风险不匹配。

## 2. `yamlx.Decode` —— ✅ 已优化（标量直解 + 键快路径）

`BenchmarkDecode`（1000 个服务 × 标量 + 嵌套 map + 序列，≈150 KB，`internal/yamlx/decode_bench_test.go`）：

| 指标 | 值 | 换算 |
|------|----|------|
| ns/op | 10.1 ms（10,132,667–10,454,228） | 10.1 µs/服务 |
| allocs/op | 121,059 | **121/服务** |
| B/op | 6.42 MB | 6.4 KB/服务 |

- 占"一次大渲染"的 **28%**，远超 5% 阈值 ⇒ 值得优化。
- 主因（代码位置）：`yamlx.scalarValue`（yamlv3.go）对非 `!!str` 标量调用 `node.Decode(&value)`，
  走 `yaml.v3` 的**反射解码**路径，每个标量至少一次分配 + 一次反射；`!!int`/`!!float`/`!!bool`/`!!null`
  完全可以用 `strconv` 直接解析节点文本，语义更可控（且与既有规则一致：整数阶梯 int64→uint64→ExactInt、
  非有限值拒绝、`.5`/`5.`/`1_000` 退回 float64）。
- 次要：映射/序列可按 `len(node.Content)` 预分配；重复键集合可复用。
- **验证要求**（实施时）：现有 160 个 engine fixture、`internal/yamlx` 全部用例、`test/yamlx`、
  `test/e2e` 与所有 golden 逐字不变；`BenchmarkDecode` 同批 A/B；`just test-all` 全绿。
- 预估：去掉每标量反射后 allocs/服务可降一个量级（121 → 十几），ns/op 同步下降；需实测确认。

## 3. 其余待办（待测）

| 路径 | 状态 |
|------|------|
| `schema` 归一化/校验 | 待落基准（`codec.ToJSON` 仅在含精确整数时复制，预期不改） |
| `values` 合并/克隆 | 待落基准（多文件覆盖场景） |
| 输出写入 / streaming 窗口 | 待抽查（已用 `bufio` + `os.CreateTemp`，读取缓冲已复用） |
| 引擎通用路径 | `ParsePathPrefix` 仍有 1 次分配/表达式（`BenchmarkParseExpression` 可量化） |
| CLI 启动 | 实测 ~5.8 ms（预算 100 ms），维持 |

## 4. `schema.Validate` —— 结论：不改（瓶颈在第三方库内）

`BenchmarkValidateLargeTree`（200 个 typed 属性全部存在）：

| 指标 | 值 | 换算 |
|------|----|------|
| ns/op | 139–154 µs | 0.7 µs/属性 |
| allocs/op | 1,990 | **10/属性** |
| B/op | 57.9 KB | 289 B/属性 |

判定依据：① 与"渲染 200 个变量 ≈37 µs"相比，校验确实更贵（小渲染场景占比高）；② 但 1990 次分配
发生在 `santhosh-tekuri/jsonschema/v6` 的校验器内部（本项目只在 `Validate` 里做一次
`codec.ToJSON`，且仅在树含精确整数时才复制，本例不触发），**从我们这侧没有可改点**；
③ 替代方案等于更换校验库，属 ADR 级决策，超出优化范围。

## 5. `values.MergeInto` —— 结论：不改

`BenchmarkMergeInto`（50 个值文件 × 100 键 = 5000 键叠加）：

| 指标 | 值 | 换算 |
|------|----|------|
| ns/op | 160–167 µs | 32 ns/键 |
| allocs/op | **18** | 0.004/键 |
| B/op | 24.5 KB | 5 B/键 |

判定依据：分配几乎为零（原地写入 map），时间全部花在 5000 次 map 插入上，已接近下限 ⇒ 不改。

## 6. 输出写入 / streaming 窗口 —— 结论：不改（由既有门禁数据支撑）

- `test/perf` 的 `TestPerformanceGate`：render 比值 **0.44x**（限 5.0x）、RSS 比值 **1.05x**（限 5.0x）；
- `TestRenderMatchesGNUBaselineOutput`：7,666,670 字节输出与 GNU 基线**逐字节一致**；
- 代码侧已用 `bufio` + `os.CreateTemp`+`Rename` 原子写，`streamWindow` 已复用读取缓冲（`06-08` §2）。
⇒ 无独立微基准的必要，结论"不改"，数据来自已落盘的门禁测试。

## 7. CLI 启动 —— 结论：不改

`test/perf` 的 `TestStartupWithinBudget`：**5.8 ms**（预算 100 ms，7 次取中）。

## 8. 引擎通用路径（路径解析）—— ✅ 已优化

`BenchmarkParsePath`（`app.servers[0].labels["my-key"]`）：

| 指标 | 值 | 换算 |
|------|----|------|
| ns/op | 674–1844 ns（双峰，冷热缓存差异） | ≈4–10 个变量渲染（183 ns/变量） |
| allocs/op | **5** | — |
| B/op | 488 | — |

判定：**候选优化**（分配 5 次/表达式，在非快路径模板里可观），但需要先把 5 次分配的来源逐项定位
（段切片增长、引号键、索引解析），再按 A/B 决定；`ParsePathPrefix` 的单独分配（历史报告 19.4 ns/1 alloc）
尚未独立复现，列为下一轮第一件事。

### 2.1 实施与 A/B

改动（`internal/yamlx/yamlv3.go`）：

- `scalarValue` 按 yaml.v3 已解析出的 **tag 分派**，不再对每个标量调用 `node.Decode`（反射）：
  `!!str` 直取文本、`!!null` 直接 nil、`!!bool` 按 core schema 的 true/false、`!!int` 用
  `strconv.ParseInt(text, 0, 64)` → `ParseUint` → `codec.New`（int64→uint64→精确整数阶梯，
  base 0 覆盖 0x/0o/0b 与数字分隔符）、`!!float` 先试精确整数（yaml.v3 把超 uint64 整数标成
  `!!float`）、再拒绝 `.nan`/`.inf`、再 `codec.NewLiteral`（保留 `1.5`/`1e3` 原文）、最后
  `ParseFloat`（`.5`/`5.`/`1_000` 等 YAML 专有写法）；
- 未识别的 tag/写法**回落到 `node.Decode`**，行为与库一致；
- 映射键在 `tagStr` 时直接取文本，省掉每键一次反射。

`BenchmarkDecode`（1000 服务，同基准文件，改前 = HEAD worktree，`-benchmem -count 3`）：

| 指标 | 改前 | 改后 | 变化 |
|------|------|------|------|
| allocs/op | 121,059 | **94,056** | **−22.3%** |
| B/op | 6.42 MB | 4.83 MB | −24.7% |
| ns/op | 8.92 / 9.00 / 8.93 ms | 7.76 / 7.71 / 7.69 ms | ≈ **−14%**（区间不重叠） |

可见用例：`internal/yamlx/scalar_forms_test.go`
（`TestScalarFormsDecode`：十进制/负数/0x/布尔/NULL/~/字符串/引号数字/`1.5`/`1e3`/uint64 边界/
超 uint64 精确整数；`TestScalarYAMLOnlyFormsStayUsable`：`.5`、`5.`、`1_000`、`1_000.5`、
`0o17`、`0b101`、`+.5` 必须可解码且可 JSON 序列化）。

回归：160 个 engine fixture、`internal/yamlx`、`test/yamlx`、`test/e2e` 与全部 golden 未改动且全绿。

### 8.1 实施与 A/B

改动（`internal/values/path.go`）：

- `parsePrefix(collect bool)`：需要段时**按实际分隔符计数预分配**（
  `strings.Count(input, ".") + strings.Count(input, "[") + 1`），原先 `make([]Segment, 0, 1)`
  对五段路径要扩容三次；
- `parseQuoted(collect bool)`：只取前缀时**扫描引号但不构建键字符串**（`strings.Builder`
  零写入不分配）；
- `ParsePathPrefix` 传 `collect=false`，不再为了取前缀而构建整套 `[]Segment` 与引号键。

A/B（同基准文件，改前 = HEAD worktree，`-benchmem -count 3`）：

| 基准 | 指标 | 改前 | 改后 | 变化 |
|------|------|------|------|------|
| `ParsePath`（`app.servers[0].labels["my-key"]`） | allocs / B / ns | 5 / 488 / 230–250 | **2 / 168 / 116–149** | allocs −60%、B −66%、时间 ≈ −45% |
| `ParsePathPrefix`（同路径 + ` \| toJson`） | allocs / B / ns | 5 / 488 / 230–271 | **0 / 0 / 37–38** | 零分配、≈ 6× 快 |

可见用例：`internal/values/path_test.go` 新增
`TestParsePathPrefixAgreesWithParsePath`（前缀与剩余部分可拼回原串、`ParsePath` 能解析该前缀、
空串/未闭合括号/未闭合引号/连续点号仍报错）；新增基准 `BenchmarkParsePathPrefix`。

回归：`internal/values`、160 个 engine fixture、`test/e2e` 全部通过，golden 未改动。
