---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-12
---

# 06-16 `go fix` 现代化建议全量记录（Go 1.27.2）

> 用途：把 `go fix` 的原始建议落盘留档（`_common.md` §7.3/§7.6：结论与证据要可复跑，
> 不能只留在临时工作区或聊天记录里）。这些建议**已全部应用**并通过 `just check` / `just test-all`。

## 1. 复现命令

```bash
export GOCACHE=/tmp/gocache                                  # 沙箱内默认 ~/.cache/go-build 只读
export GOTMPDIR=$PWD/.tmp/gotmp                              # 同上，GOTMPDIR 必须可写
go fix -diff ./...                                           # 只看建议；exit 1 = 有建议
go fix ./...                                                 # 应用
go fix -diff ./...                                           # 复跑应为空
```

`go fix` 自带 25 个分析器（`go tool fix help` 列出，含 `any`、`errorsastype`、`stringscutprefix`、
`stringsseq`、`slicessort`、`slicescontains`、`mapsloop`、`forvar`、`testingcontext`、`minmax`、
`newexpr`、`rangeint`、`omitzero`、`reflecttypefor`、`waitgroupgo`、`stditerators` 等）。
本次给出的建议覆盖其中 7 类。

## 2. 建议原文（应用前的 diff，7 个文件 / 208 行）

```diff
--- internal/diag/errors.go (old)
+++ internal/diag/errors.go (new)
@@ -27,8 +27,7 @@
-	var de *Error
-	if errors.As(err, &de) {
+	if de, ok := errors.AsType[*Error](err); ok {
@@ -46,8 +45,7 @@
-	var de *Error
-	if errors.As(err, &de) {
+	if de, ok := errors.AsType[*Error](err); ok {
@@ -148,7 +146,7 @@
-	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...), Location: Location{Offset: offsetUnknown}}
+	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...), Offset: offsetUnknown}
@@ -156,7 +154,7 @@
-	return &Error{Kind: kind, Err: err, Location: Location{Offset: offsetUnknown}}
+	return &Error{Kind: kind, Err: err, Offset: offsetUnknown}
@@ -179,7 +177,7 @@
-		de = &Error{Kind: KindInternal, Err: err, Location: Location{Offset: offsetUnknown}}
+		de = &Error{Kind: KindInternal, Err: err, Offset: offsetUnknown}

--- test/e2e/e2e_test.go (old)
+++ test/e2e/e2e_test.go (new)
@@ -281,8 +281,8 @@
-	if strings.HasPrefix(arg, "@") {
-		return filepath.Join(work, strings.TrimPrefix(arg, "@"))
+	if after, ok := strings.CutPrefix(arg, "@"); ok {
+		return filepath.Join(work, after)
@@ -417,7 +417,7 @@
-	for _, line := range strings.Split(string(data), "\n") {
+	for line := range strings.SplitSeq(string(data), "\n") {
@@ -439,7 +439,7 @@
-	for _, line := range strings.Split(string(data), "\n") {
+	for line := range strings.SplitSeq(string(data), "\n") {
@@ -490,8 +490,7 @@
-	var exitErr *exec.ExitError
-	if errors.As(err, &exitErr) {
+	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {

--- test/perf/perf_test.go (old)
+++ test/perf/perf_test.go (new)
@@ -17,7 +17,7 @@
-	"sort"
+	"slices"
@@ -304,7 +304,7 @@
-	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
+	slices.Sort(sorted)

--- internal/codec/literal.go (old)
+++ internal/codec/literal.go (new)
@@ -6,6 +6,7 @@
+	"slices"
@@ -85,10 +86,8 @@
-		for _, child := range typed {
-			if containsJSONNumberType(child) {
-				return true
-			}
-		}
+		if slices.ContainsFunc(typed, containsJSONNumberType) {
+			return true
+		}

--- test/encoding/types_test.go (old)
+++ test/encoding/types_test.go (new)
@@ -66,7 +66,6 @@
-		tt := tt
@@ -129,7 +128,6 @@
-		tt := tt
@@ -205,7 +203,6 @@
-		tt := tt
@@ -260,7 +257,6 @@
-		tt := tt
@@ -290,7 +286,6 @@
-		tt := tt

--- internal/engine/extended/engine_features_test.go (old)
+++ internal/engine/extended/engine_features_test.go (new)
@@ -34,9 +35,7 @@
-	for key, value := range entries {
-		tree.Root[key] = value
-	}
+	maps.Copy(tree.Root, entries)

--- internal/engine/extended/stream_test.go (old)
+++ internal/engine/extended/stream_test.go (new)
@@ -57,8 +57,7 @@
-	ctx, cancel := context.WithCancel(context.Background())
-	defer cancel()
+	ctx := t.Context()
```

## 3. 归类与理由

| 分析器 | 改动 | 为什么更好 |
|--------|------|-----------|
| `errorsastype` | `errors.As` → `errors.AsType[T]`（Go 1.26+ 的泛型方法/函数） | 少一个局部变量、少一次取地址、类型由参数表达，读起来是"取出 T 类型的错误" |
| `embedlit` | 复合字面量里的嵌入字段 `Location: Location{Offset: …}` → `Offset: …` | Go 允许直接提升字段名，写法更短 |
| `stringscutprefix` | `HasPrefix`+`TrimPrefix` → `CutPrefix` | 一次扫描而不是两次 |
| `stringsseq` | `range strings.Split(...)` → `range strings.SplitSeq(...)` | 迭代器，免去一次性切片的分配 |
| `slicessort` / `slicescontains` / `mapsloop` | `sort.Slice` → `slices.Sort`；显式循环 → `slices.ContainsFunc` / `maps.Copy` | 标准库已提供同等语义的通用版本，少写循环 |
| `forvar` | 删除 `tt := tt` | Go 1.22+ 每轮迭代已有独立变量 |
| `testingcontext` | `context.WithCancel(context.Background())` → `t.Context()` | 由测试框架负责取消时机，不会忘记 cancel |

## 4. 未给出的建议（说明没有漏掉）—— 以及为什么

`go fix` 复跑为空，即这 25 个分析器在当前代码上**没有**其它可应用项。逐项排查过的高频候选与结论：

| 候选 | 为什么这里不适用 |
|------|------------------|
| `rangeint`（`for i := 0; i < n; i++` → `for i := range n`） | 代码里的计数循环大多需要读写 `i` 之外的游标（scanner、parser 的 `p.pos`），不是"遍历 n 次" |
| `minmax` | 未发现可用 `min`/`max` 简化的 if/else（数值比较集中在 schema 校验，语义不止取极值） |
| `newexpr`（`new(expr)`） | 未见 `x := v; p := &x` 形状 |
| `waitgroupgo` | 并行渲染用的是 `errgroup`/channel 而非裸 `sync.WaitGroup` |
| `reflecttypefor` | 未使用 `reflect.TypeOf` |
| `stditerators` / `backward` | 未使用 `Len/At` 风格 API，也没有反向遍历切片 |
| `omitzero` | `omitempty` 出现在 `--explain` 的 `*int` 字段上，`omitzero` 不改变该行为（G20 已按指针约束统一） |
| `atomictypes`（`atomic.Int64` 等） | 未使用 `sync/atomic` |
| `unsafefuncs` | 未使用 `unsafe` 指针算术 |

## 5. 非目标（Non-Goals）

- 本文件不替代 `docs/99-archive/06-review/06-15` 的实现评审结论，只记录工具链建议原文与应用结果。`[FACT]`
- 不引入"为用泛型而用泛型"的重构：本轮没有为 `Tree`/`Schema` 增加泛型方法，因为它们的元素类型是
  `any`/`map[string]any`，类型参数会带来装箱或重复实例化而没有可读性收益。`[INFERENCE]`
- 不修改 `go.mod` 的 `go` 指令或引入新依赖。`[FACT]`

## 6. 验证

```bash
export GOCACHE=/tmp/gocache GOTMPDIR=$PWD/.tmp/gotmp
gofmt -l ./cmd ./internal ./test   # 空
go vet ./...                       # 通过
go fix -diff ./...                 # 空
just check                         # exit 0
just test-all                      # exit 0（perf：render 0.42x、RSS 1.17x、startup 5.87ms、基线逐字节一致）
```
