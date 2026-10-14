---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-09
---

# 06-08 expression 解析优化（A/B 数据）

> 背景：评审要求"重新测试验证"。本文只记录**同基准、同参数**的前后对比与复现方式，
> 结论区分"确定性指标"（allocs/op、B/op）与"噪声指标"（含整体渲染的 ms/op）。

## 1. 方法

- 改前：`git worktree add .tmp/base bb9cd61^`（优化前的 `expression.go`），把**同一份**
  基准文件 `expression_bench_test.go`、`benchmark_test.go` 复制进去；
- 改后：当前工作树；
- 命令（两侧一致）：`go test -run '^$' -bench <pattern> -benchmem -count 3 ./internal/engine/extended/`。

## 2. 解析级（确定性：区间不重叠）

| 基准 | 改前 | 改后 | 变化 |
|------|------|------|------|
| `ParseExpression/default`（`${A:-x}`） | 114.4 / 109.8 / 109.5 ns；72 B；**4 allocs** | 90.6 / 73.6 / 68.8 ns；48 B；**2 allocs** | 时间 ≈ −34%，allocs −50% |
| `ParseExpression/typeInt` | 139.5 / 137.3 / 143.8 ns；96 B；**5 allocs** | 81.0 / 79.6 / 79.8 ns；64 B；**2 allocs** | 时间 ≈ −43%，allocs 5→2 |
| `ParseExpression/multiStep` | 424.6 / 410.6 / 404.5 ns；448 B；**11 allocs** | 204.9 / 215.9 / 211.2 ns；224 B；**3 allocs** | 时间 ≈ −49%，allocs 11→3 |
| `SplitPipes/default` | 71.5 / 66.2 / 68.1 ns；40 B；**3 allocs** | 23.7 / 23.9 / 23.5 ns；16 B；**1 alloc** | 时间 ≈ −65%，allocs 3→1 |

## 3. 渲染级（分配确定性；时间受噪声支配）

| 基准 | 改前 | 改后 | 结论 |
|------|------|------|------|
| `RenderLargeTemplateOperators`（200k `${VAR:-x}`，每变量解析一次） | 130.98 / 146.25 / 144.85 ms；**73.98 MB/op；1,533,703 allocs** | 129.36 / 130.00 / 129.16 ms；**69.18 MB/op；1,133,703 allocs** | 分配 **−400,000（−26%）**、内存 −6.5%；时间区间仅勉强分离（≈ −8%），不足以单独作为结论 |
| `RenderLargeTemplate`（200k `${VAR}`，走 `simpleSpanName` 快路径，**不经过解析**） | 37.2 / 38.9 / 37.3 ms；400,021 allocs | 41.6 / 43.2 / 50.4 ms；400,021 allocs | 该路径没有代码改动、allocs 完全相同，却在后一批里慢了 12–30% ⇒ **本机计时跨批不可比** |

## 4. 结论

- **有提升，且可确定的部分是**：解析函数快约 35–50%、分配减半；渲染含解析的场景每变量少 2 次分配
  （200k 变量共少 400,000 次，−26%），内存 −6.5%。
- **不能宣称的部分**：整体渲染耗时的百分比。同一批次内解析微基准区间不重叠（可信），但
  跨批次的渲染 ms/op 被环境噪声主导（未改动路径也漂移 12–30%）。

## 5. 复现

```bash
# 解析级 A/B
git worktree add --detach .tmp/base bb9cd61^
cp internal/engine/extended/{expression_bench_test.go,benchmark_test.go} .tmp/base/internal/engine/extended/
(cd .tmp/base && go test -run '^$' -bench 'BenchmarkParseExpression|BenchmarkSplitPipes' -benchmem -count 3 ./internal/engine/extended/)
go test -run '^$' -bench 'BenchmarkParseExpression|BenchmarkSplitPipes' -benchmem -count 3 ./internal/engine/extended/
git worktree remove --force .tmp/base

# 渲染级
go test -run '^$' -bench 'BenchmarkRenderLargeTemplate$|BenchmarkRenderLargeTemplateOperators' -benchmem -benchtime 3x -count 3 ./internal/engine/extended/

# 正确性（可见用例）
go test -run 'TestSplitPipesCases|TestSplitPipesRejectsUnclosedQuote|TestParseExpressionCases' -v ./internal/engine/extended/
```

## 非目标（Non-Goals）

- 不引入解析缓存（CLI 每模板渲染一次；`Engine` 为进程级单例且并行渲染会并发解析，缓存需并发安全且有界）。`[FACT]`
- 不在本文评估 `values.ParsePathPrefix` 的 1 次分配（另见提交信息中的后续机会）。`[FACT]`

## 6. 快路径 span 名零拷贝（已实施，附 A/B）

### 6.1 剖析定位（实施前）

`BenchmarkRenderLargeTemplate`（200k `${VAR}`）的内存剖析（`-test.memprofile -test.memprofilerate 1`）：

| 分配点 | 对象数（5 次迭代） | 每变量 | 性质 |
|--------|-------------------|--------|------|
| `simpleSpanName` 的 `string(raw)` | 1,200,000 | 1.2 | **生产**：快路径把 span 的 `[]byte` 转成 `string` |
| `values.(*Tree).ResolveName`（内联归属） | 1,200,000 | 1.2 | **生产**：同一转换/调用链 |
| `fmt.Sprintf`（`benchmarkTemplate`/`benchmarkTree`） | 799,895 | 0.8 | **测试 setup**，与生产无关 |

改法（不引入 unsafe）：

1. `simpleSpanName` 返回 `[]byte`（span 的子切片），不再 `string(raw)`；
2. `values.Tree` 增加 `ResolveNameBytes(raw []byte) (any, bool)`，内部用 `t.Root[string(raw)]` /
   `t.Env[string(raw)]`——Go 对"map 下标里的 `[]byte`→`string` 转换"有免分配优化；
3. 仅在这些**慢路径**才转 `string`：`Only`/`Ignore` 过滤器非空时（`opts.Replaces`）、
   缺失/保留/策略分支、`writeValue` 落到非 string/int/bool 的默认分支（错误文案需要路径）；
4. 新增 `writeValueBytes(w, name []byte, value any)`，string/int/bool 三个快分支零分配，
   默认分支才 `stringifyValue(string(name), value)`。

预期：常见 `${VAR}` 快路径每变量从 2 allocs 降到 0，即 `BenchmarkRenderLargeTemplate`
的 400,021 allocs/op 接近清零（模板与值树的 setup 分配不计入每个 op）。

复现剖析：

```bash
go test -c -o .tmp/prof/engine.test ./internal/engine/extended/
./.tmp/prof/engine.test -test.run '^$' -test.bench 'BenchmarkRenderLargeTemplate$' \
  -test.benchtime 5x -test.memprofile .tmp/prof/mem.out -test.memprofilerate 1
go tool pprof -top -sample_index=alloc_objects .tmp/prof/engine.test .tmp/prof/mem.out
```

### 6.2 实施与 A/B（提交见 git log）

改法：`simpleSpanName` 返回 span 子切片（`[]byte`）→ `Tree.ResolveNameBytes` 用
`m[string(raw)]` 免分配查找 → `Options.ReplacesBytes`（无过滤器时直接返回 true）→
`writeValueBytes` 仅在 string/int/bool 之外的回退分支才构造字符串；`writeValue` 与
`writeValueBytes` 共用 `writeScalar`。

`BenchmarkRenderLargeTemplate`（200k `${VAR}`，`-benchmem -benchtime 5x -count 3`，
改前用同基准文件的 HEAD worktree）：

| 指标 | 改前 | 改后 | 变化 |
|------|------|------|------|
| allocs/op | 400,021 | **200,021** | **−50%** |
| B/op | 36.40 MB | 33.20 MB | −8.8% |
| ns/op（3 次） | 52.4 / 49.7 / 55.0 ms | 28.3 / 38.9 / 37.5 ms | 区间不重叠，≈ −28% |
| `test/perf` RSS 比值 | 1.40x | **1.11x** | 门禁输出 |

### 6.3 Env 路径 string→any 装箱 —— ✅ 已用方案 A 修复

**测法**：`go test -c -gcflags=all=-l`（关闭内联）后 `-test.memprofile -test.memprofilerate 1`，
避免内联造成的归因失真。

**归因结果**（3 次迭代 × 200k 变量）：

| 位置 | 对象数 | 每变量 | 性质 |
|------|--------|--------|------|
| `values.(*Tree).ResolveNameBytes` | 800,000 | 1.33 | **生产**（真实热点） |
| `fmt.Sprintf` + `benchmarkTree` + `benchmarkTemplate` | ~1.6M | — | 测试 setup，一次性 |

**根因（已核对代码）**：`values.Tree.Env` 是 `map[string]string`（tree.go:16），而
`ResolveNameBytes`/`ResolveName` 的返回类型是 `(any, bool)`。当变量从环境命名空间命中时，
`return value, true` 会把 `string` **装箱成 `any`**，每次查找分配一次。
基准 `benchmarkTree` 恰好把 200k 变量全部放进 `tree.Env`（benchmark_test.go:24-30），
因此每变量 1 次分配 = 200,021 allocs/op，与测量完全吻合。

**两个修法（下一步二选一）**：

| 方案 | 做法 | 优点 | 代价 |
|------|------|------|------|
| A（推荐，改动小） | 新增 `Tree.ResolveEnvName(name []byte) (string, bool)`；引擎快路径先查 Root（any），未命中再查 Env 并用字符串直写（Env 值恒为字符串，只需处理 `--on-empty`） | 保持内存布局；Env 命中零分配 | 快路径多一个分支；Env 的 `--on-empty=error` 判定要在字符串分支里重复一次 |
| B | 把 `Tree.Env` 改成 `map[string]any` | 装箱只在插入时发生一次，所有路径受益 | 触及 merge/dotenv/set/parallel/stream 等约 10 处赋值与测试；常驻内存增加（每键 interface 头） |

方案 A 的验证要求：`BenchmarkRenderLargeTemplate` 同批 A/B（预期 200,021 → ~21），
并补 `ResolveEnvName` 与 `ResolveName` 对拍用例（含空串值、缺失名、非 ASCII 名）。

**已否定的两次尝试**（勿重走）：拆 `lookupName` 单次转换小函数、`writeScalar` 的 scratch 逃逸，
两者都不改变分配计数（详见提交 6e955b6）。

**实施结果（方案 A）**：`Tree` 拆出 `ResolveRootName(name []byte) (any, bool)` 与
`ResolveEnvName(name []byte) (string, bool)`；引擎快路径先查 Root（any 路径），未命中再查 Env
并**直接字符串写出**（Env 值恒为 string，空值判定在字符串分支内完成，`--on-empty=error` 语义不变）。

| 指标（`BenchmarkRenderLargeTemplate`，`-benchmem -benchtime 5x`） | 改前 | 改后 |
|------------------------------------------------------------------|------|------|
| allocs/op | 200,021 | **21**（约等于每次渲染的固定开销） |
| B/op | 33.20 MB | **29.99 MB（−9.7%）** |

对拍用例：`TestResolveNameSplitMatchesResolveName`（Root 值、Env 值、空串 Env 值、非 ASCII 名、
缺失名、空名、Root 优先、nil tree）。
