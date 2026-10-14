---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-08
---

# 代码评审整改（Round 4：6 条评审意见）

> 输入：用户对 P0 实现的 6 条评审意见（版本信息、结构体字段、参数透传、
> 选项方法化与参数顺序、字面量常量化、schema 文件名）以及两条澄清：
> ① schema 采用"按职责拆文件 + 校验实现命名 jsonschema.go"；
> ② 引擎参数统一为 `(ctx, w, in)` / `(ctx, w, src, in)` 并删除冗余的 `name`。
> 结论：6 条全部完成，每项独立提交；干净检出上 `just check` 与 `just lint` 均通过。

## 1. 整改对照

| # | 评审意见 | 处理 | 提交 |
|---|----------|------|------|
| 1 | Go 已内置版本信息，不需要自己 `-X` 注入；后续意见：缺构建时间、git 字段不明确、没有 git hash | `internal/version` 采用 Kubernetes `version.Info` 的字段形状：`gitVersion`/`gitCommit`（完整 40 位）/`gitTreeState`（clean\|dirty）/`buildDate`/`goVersion`/`compiler`/`platform`。版本、提交、树状态、编译器、平台全部来自 `debug.ReadBuildInfo()`；Go 不嵌入构建时间，按"不注入"的立场改为取可执行文件 mtime（RFC3339），全仓库不再有任何 `-X`。`envsub version` 输出缩进 JSON（`sonic.MarshalIndent(info, "", "  ")`，2 空格，
无前缀），字段与 Kubernetes `version.Info` 对齐 | `04a74c8`、`fe076be`、`c31dcb7`、`e389f7e`、本提交 |
| 2 | `FileStream{target, file, name}` 这类字段定义不清晰 | `output`：`name`→`tempPath`、`file`→`temp`、`WriterStream.target`→`path`、`StdoutWriter.W`→`out`，类型 `StreamTo`→`DirectStream`（原名与引擎方法同名）；`values`/`engine`/`extended`/`diag`/`source`/`render` 的结构体字段全部补齐注释；并写脚本复核"无注释字段 = 0" | `8c2d8ab` |
| 3 | 避免没必要的参数展开，直接透传 | `engine.StreamEngine.WriteTo(ctx, w, in)`、`StreamReaderEngine.StreamTo(ctx, w, src, in)` 删除冗余 `name`；`extended` 内部 `emitSpan/renderToken/renderVariable/assignWord/applyPolicy/pipelineValue/requiredError/stringWord/renderChunk/streamParallel/streamSequential` 等一律只收 `engine.Input`；新增 `Input.WithTree()` 统一补齐空值树；`--only/--ignore` 匹配逻辑移到 `engine.Options.Replaces` | `2ecfca2` |
| 4 | `validateOptions`/`buildValues` 应挂到 opts 上，并调整参数顺序 | `render` 的 `validate/buildValues/loadSchema/applySets/resolveTemplates/newWriter/newStreamTargets/newStreamTarget/templateOutput/renderOne/streamReader` 全部改为 `Options` 方法（接收者统一 `opts`）；`template` 删除与 `Input` 恒等的 `Source`；新增 `destination{writer, stream}`，`renderOne` 参数由 10 个降为 6 个；每个模板的 `engine.Input` 在 `Run` 构造一次后透传 | `a60fc06` |
| 5 | 字面量常量化 | 退出码、日志级别名、JSON Schema 类型名、路径语法字符、`.env` 引号/转义/注释/空白、值文件扩展名、`array<T>` 前后缀、换行符、整数进制等改为具名常量；纯 `""` 判断保持原样（Go 中表示"无值"的惯用写法，抽常量反而降低可读性，这一点与意见有出入，理由见第 3 节） | `4fd14b3` |
| 6 | `internal/schema` 文件名需准确、按复用 JSON 的职责命名 | `schema.go` 拆为 `schema.go`（类型/策略/共用助手）、`keywords.go`（关键字与类型名常量）、`load.go`、`lookup.go`、`defaults.go`、`materialize.go`、`jsonschema.go`（复用 santhosh-tekuri/jsonschema/v6 的校验与错误渲染）；`normalize.go` 保留 | `1c95ba6` |

附带（用户指示）：仓库内唯一的 WaitGroup 用法改为 Go 1.25+ 的 `WaitGroup.Go`，
并把外部改动引入的 `line := 11` 修正回 `1`（该分支不使用 startLine，故不影响输出，
但错误初值会让后续接入行号定位时整体偏移）。提交 `7592fdc`。

## 2. 验证（2026-10-08，干净检出 HEAD=4fd14b3）

```text
git status --porcelain                       空
git worktree add HEAD → just check           exit 0（gofmt + vet + 单测/fixture + e2e）
                      → just lint            0 issues
go test -race -count=1 ./...                 15 个包全部通过
./dist/envsub version                         缩进 JSON（sonic.MarshalIndent），字段与 Kubernetes version.Info 对齐：
  {
    "gitVersion": "v0.0.0-<时间戳>-<提交>+dirty",
    "gitCommit": "<40 位提交>",
    "gitTreeState": "dirty",
    "buildDate": "2026-10-08T07:30:07Z",
    "goVersion": "go1.27.1",
    "compiler": "gc",
    "platform": "linux/amd64"
  }
```

`buildDate` 取运行中可执行文件的 mtime（UTC，RFC3339）：`just build` 与
`go build` 行为一致，justfile/CI 不再传入任何 `-X`。代价是复制二进制会改变该值
（不是原始构建时刻）——这是"完全不做构建期注入"的必然取舍。

## 3. 与评审意见的出入（供判断）

| 条目 | 意见 | 实际 | 理由 |
|------|------|------|------|
| 5 | 列举中包含 `""` | 未把空字符串抽成常量，保留 `if x == ""` 这类判断 | Go 惯例：`""` 是零值而非魔法值，`UnsetDefault = ""` 之类常量反而掩盖"默认即 error"的语义；语义化的空值来源已由 `engine.Unset*`/`Empty*` 常量表达 |
| 3 | "直接透传"未限定范围 | `engine.Input` 透传到渲染层内部；`streamWindow` 的 `windowSize`、`reader` 等非选项参数仍显式传参 | 这些参数每次调用不同（窗口大小/流），不属于"同一结构体反复展开" |

## 4. 遗留

- Round 3 报告中 D1~D6 的偏差裁决仍未收到结论（`a18e5cf`、`342b717`）。
  本轮未改动并行逻辑语义，只在 `2ecfca2` 中调整了其函数签名。
- `pending` 有界化与步骤 4 基准调参仍未做（见 06-04 第 6、7 节）。
