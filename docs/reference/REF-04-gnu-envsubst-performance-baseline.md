---
status: draft-ai
owner: [ 待指定 ]
version: 0.2
last-updated: 2026-10-08
---

# GNU envsubst 性能基线参考

> 用途：为 envsubst 升级版的性能目标提供参考基线。  
> 基线环境：本机 GNU envsubst 0.23.1，2026-09-29。

## 1. 测试方法

- 模板：生成 10,466,670 字节文本，约 200,000 行，每行包含 `${HOST_N}` 变量。
- 命令：`envsubst < template > /dev/null`。
- 启动：空输入调用 200 次，统计平均时间。
- 内存：使用 `resource.getrusage(RUSAGE_CHILDREN).ru_maxrss` 记录峰值。
- 环境：本机 Linux，GNU envsubst 0.23.1。`[FACT]`

## 2. 测量结果

| 指标                     | GNU envsubst 基线 |
|--------------------------|-------------------|
| 10MB / 200k 变量模板耗时 | 约 0.062 秒       |
| 峰值 RSS                 | 约 11,220 KB      |
| 空输入平均启动时间       | 约 0.63 毫秒      |

`[FACT]`（来源：本地基准测试，2026-09-29）

## 3. 目标建议

建议将“与 envsubst 同量级”落为可测试目标：

- 10MB / 200k 变量模板：耗时不超过 GNU envsubst 基线的 5 倍。`[ASSUMPTION]`
- 峰值内存：不超过 GNU envsubst 基线的 5 倍。`[ASSUMPTION]`
- 空输入启动：不超过 100 毫秒。`[ASSUMPTION]`
- 验证方式：CI benchmark 使用同一模板与机器重复测量。`[FACT]`（已实现，见第 4 节）

## 4. 门禁实现（2026-10-08）

- 位置：`test/perf/`（`go test ./test/perf`，快捷入口 `just test-perf`，CI job `perf`）。`[FACT]`
- 方法：`TestMain` 用当前源码构建 envsub，并生成与本文件 §1 同规格的
  10,466,670 字节 / 200,000 行模板；GNU envsubst 与 `envsub render --on-missing=empty`
  各预热一次后重复 N 次（默认 7，CI 9），取中位数比较；峰值 RSS 由
  `RUSAGE_CHILDREN`（`ProcessState.SysUsage`）读取，非 unix 平台跳过该比值。`[FACT]`
- 断言：耗时 ≤ 5×；峰值 RSS ≤ 5×；空输入启动 ≤ 100 ms（绝对值）。另有
  "GNU 与 envsub 输出逐字节一致"的前置断言，避免"渲染错误所以更快"的假通过。`[FACT]`
- 可调项：`PERF_RUNS`、`PERF_MAX_TIME_RATIO`、`PERF_MAX_RSS_RATIO`、
  `PERF_MAX_STARTUP_MS`；`go test -short` 跳过整个门禁。`[FACT]`
- 缺少 GNU envsubst 时比值类断言跳过（启动预算仍强制）；CI 的 perf job 先
  `apt-get install gettext-base`，因此门禁在 CI 上始终生效。`[FACT]`
- 实测（2026-10-08 本机，重复运行）：耗时 0.61–1.09×、峰值 RSS 1.81–2.25×、
  启动 10.7–16.3 ms，三项均在限值内。`[FACT]`

## 5. 非目标（Non-Goals）

- 该基线不代表所有平台与机器的绝对值。`[ASSUMPTION]`
- 不承诺扩展模式与 GNU envsubst 完全同等性能；只要求同量级。`[INFERENCE]`
- 不把该基线作为功能正确性的替代验证。`[INFERENCE]`
