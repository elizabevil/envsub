---
status: draft-ai
owner: [ 待指定 ]
version: 0.2
last-updated: 2026-10-08
---

# 06-06 JSON 数字精度核查与解码方案对照（Round 5）

## 1. 结论摘要

- `encoding/json/v2` 对 `any` 目标**默认把数字解成 `float64`**，2^53 以上的整数丢精度；v2 没有
  `UseNumber` 开关（那是 v1 `Decoder` 的方法）。`[FACT]`
- **生产实现只使用标准库 API**：`jsonv2.Unmarshal` + 其文档化的 `WithUnmarshalers` 扩展点，
  用类型化 unmarshaler 把数字落成 `json.Number`，容器仍由 v2 解析（带同一 hook 递归）。
  没有自建解析层。`[FACT]`
- 被拒绝的备选：**单遍 `jsontext` token 遍历（自建树）**。同一负载 3.55 ms vs 12.04 ms
  （快 3.4×），但属于"自行封装一层解析逻辑"，与"优先标准库 API"规则冲突，故不作生产实现。
  它保留在 `test/jsonx/walker_test.go`，与生产实现做功能等价与性能对照，随时可复现。`[FACT]`
- 该规则已写入 [`docs/prompts/_common.md`](../../prompts/_common.md) 第 7 节。`[FACT]`

## 2. 查证：v2 的数字默认行为

文档原文（Go 1.27.1，`go doc encoding/json/v2`）：

> v1 and v2 may still lose precision when unmarshaling into an **any** interface value, where
> unmarshal **uses a float64 by default** to represent a JSON number.

实测（同一份文档，不同量级）：

| 输入 | `any` 解成 | 实际值 | 具体类型目标 |
|------|-----------|--------|-------------|
| `8080` | float64 | `8080` | `uint64`/`json.Number` → 精确 |
| `9007199254740993`（2^53+1） | float64 | `9007199254740992`（舍入） | 同上 |
| `18446744073709551615`（u64 max） | float64 | `1.8446744073709552e+19` | 同上 |
| `-9223372036854775808`（i64 min） | float64 | `-9.223372036854776e+18` | 同上 |

补充：`jsontext` 的选项只有 `AllowInvalidUTF8`、`AllowDuplicateNames`、`WithIndent`、
`WithIndentPrefix` 等，**没有任何数字类型开关**；v1 包级 `Unmarshal` 默认同样是 float64。`[FACT]`

## 3. WithUnmarshalers 的真实语义（决定实现形状）

实测（Go 1.27.1）：

- 目标为 `*any`（整份文档）：hook **只被调用一次**，收到整份文档的原始字节；hook 若不处理，
  结果就是 `nil`——它接管的是"整个值"，不是"只拦数字"。
- 目标为 `map[string]any`：hook **按每个值调用**，但**容器内部的嵌套值不会自动再进来**，
  必须由 hook 自己把容器交回 v2 并带上同一 hook。

因此生产实现里有且只有一处"分派"逻辑（`decodeAny`，按 `jsontext.Value.Kind()` 选择具体
Go 类型），其余解析全部由 v2 完成。`[FACT]`

## 4. 方案对照（同一负载：364 KB / 5000 对象）

| | 生产：v2 `Unmarshal` + hook | 备选：单遍 token 遍历 | `-tags=sonic` |
|---|---|---|---|
| 精度（u64 / 2^53+） | 精确 | 精确 | 精确 |
| 查重 / 非法 UTF-8 / 尾随内容 | 拒绝（标准库） | 拒绝（标准库 `jsontext`） | 拒绝 |
| 自建解析层 | **无** | 有（`decodeValue`/`decodeObject`/`decodeArray`） | 无 |
| 时间/op | 12.04 ms（30.97 MB/s） | **3.55 ms**（105.22 MB/s） | **2.45 ms**（152.44 MB/s） |
| 分配 | 110,114 | 75,035 | 45,014 |
| 实现行数（解码部分） | ~70 | ~130 | 0（直接透传） |

复现：

```bash
go test -v ./test/jsonx/                              # 功能等价 + 计时报告
go test -run XXX -bench=. -benchmem ./test/jsonx/     # 生产 vs token 遍历
go test -tags=sonic -run XXX -bench=. -benchmem ./test/jsonx/   # 追加 sonic 对照
```

第三方案"接受 float64"被拒绝：u64 级 ID、时间戳会静默损坏，违反项目"整数保持整数"的既有
契约（`.env` JSON 规则、`--set-json`、JSON/YAML values 加载、schema 校验都依赖它）。`[FACT]`

## 5. 功能与性能测试案例（落盘，不再删除）

| 文件 | 内容 |
|------|------|
| `test/jsonx/walker_test.go` | 被拒绝的 token 遍历实现（保留备查，与生产实现同源同语义） |
| `test/jsonx/compare_test.go` | `TestCorpusEquivalence`（遍历 `internal/jsonx/testdata` 全部 fixture，断言两者接受/拒绝一致且树完全相等）、`TestBothImplementationsPreserveBigIntegers`、`TestTimingReport`（普通 `go test -v` 即打印对照）、`BenchmarkUnmarshalValueProduction` / `BenchmarkUnmarshalValueWalker` |
| `internal/jsonx/decode_test.go` | `decodeAny` 防御分支、`Unmarshal(*any)` 与 `UnmarshalValue` 一致性 |

对照结果（2026-10-08，本机，benchtime=100x）：

```text
BenchmarkUnmarshalValueProduction-10   12042770 ns/op   30.97 MB/s   3768586 B/op   110114 allocs/op
BenchmarkUnmarshalValueWalker-10        3545097 ns/op  105.22 MB/s   2992755 B/op    75035 allocs/op
（-tags=sonic）BenchmarkUnmarshalValueProduction-10   2446807 ns/op  152.44 MB/s  4049848 B/op  45014 allocs/op
```

## 6. 回归护栏

`internal/jsonx/jsonv2_test.go` 的 `TestJSONv2DefaultsToFloat64ForAny`（fixture：
`testdata/big_integers.json`）断言：

1. `jsonv2.Unmarshal` 把 `9007199254740993` 解成 float64 并舍入为 `9007199254740992`；
2. `jsonx.UnmarshalValue` 对同一文档返回精确的 `json.Number`。

失败含义：标准库改了 `any` 的默认数字类型 → 第 3 节的 hook 可能不再必要，需要重新评估。`[FACT]`

## 7. 开发规则（已写入提示词）

`docs/prompts/_common.md` 第 7 节新增：

1. **优先使用标准库/依赖库既有 API** 实现功能，不要自行封装一层解析逻辑；确需自建时必须写明
   既有 API 无法满足的具体原因与实测数据。
2. **方案对比实验必须落盘可复现**：对照实现放 `test/`（功能等价测试 + 基准），数据与结论写
   `docs/99-archive/06-review/`，决策写 ADR；禁止"测完即删"。
3. 引用性能/体积数据必须给出复现命令。

## 7.1 与序列化规范的关系

本文解决的是 JSON 侧精度（`json.Number` 全链路）；值模型、YAML 侧与"空值语义"的统一规范
（含超出 uint64 的取直、`null` 与空集合的区分）见
[序列化规则](../../07-serialization/00-index.md)。`[FACT]`

## 8. 遗留

- 默认后端的 JSON 吞吐相对 sonic 降低约 4.9×，这是自觉取舍；需要吞吐时用 `-tags=sonic`。`[INFERENCE]`
- 维护约定：备选 token 遍历若将来重新采纳（例如 JSON 吞吐成为瓶颈），需同步 ADR-004 §8.3 与本节。`[FACT]`
