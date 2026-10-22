---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# Go 代码开发规范

> 规范依据：Google Go Style Guide（Guide / Decisions / Best Practices）、Effective Go、Go Code Review Comments。  
> 当规范未覆盖时，先查官方与 Google 风格指南，不凭经验猜测。

## 1. Go 版本

- `go.mod` 使用 `go 1.27.3`。`[FACT]`（2026-10-22 更新）
- 代码必须使用 Go 1.27.3 可编译的语法与标准库。`[FACT]`
- 不为了兼容旧版本而使用废弃语法；旧写法应升级到现代惯用法。`[FACT]`

## 2. 强制工具链

- `gofmt`
- `goimports`
- `go vet`
- `golangci-lint`
- `govulncheck`
- `go test ./...`
- `go test -race ./...`（涉及并发后）
- `go test -fuzz`（解析器与模板扫描器）

## 3. 格式化、导入与命名

- 所有 Go 文件必须 `gofmt`/`goimports`。`[FACT]`
- import 分组：标准库、空行、第三方、空行、本项目。`[INFERENCE]`
- 包名使用小写、简短、有意义的名词，避免 `util`、`common`、`helpers`。`[FACT]`
- 避免重复：函数名/方法名/参数名不要重复上下文。`[FACT]`
- 不要为 getter 加 `Get` 前缀。`[FACT]`
- 导出标识符必须有以名称开头的 doc comment。`[FACT]`
- 错误字符串小写开头，不加句尾标点。`[FACT]`

### 3.1 注释分两类（依据 Go Code Review Comments "Doc Comments" 与 Google Go Style Guide "Doc comments" / "Commentary"）`[FACT]`

| | **doc comment**（包、导出符号、行为不显然的未导出符号） | **实现注释**（函数体内） |
|---|---|---|
| 是否强制 | **必须** | 可选 |
| 写什么 | **契约**：做什么、参数与返回值含义、错误语义、边界、调用方要注意什么 | 代码表达不出的约束：为什么必须这样、为什么不能那样、外部硬限制 |
| 长度 | 完整句子，**该长就长**，不为简短牺牲契约 | **一行**；写不下说明它属于 doc 或设计文档 |
| 禁止 | 实现细节、方案沿革、版本与评审编号 | 复述代码、复述接口实现、教程式解释 |

推论（本仓库的执行口径）：

- 包注释每包**恰好一条**，说明该包负责什么、不负责什么；散在函数上的“边界关系”并进包注释。
- 方法实现某个接口时不再逐方法写 `// X implements Y`，除非行为有额外约束。
- 一眼能看懂的函数不给注释（如 getter、字段搬运、简单的 `if err != nil` 分支）。
- 注释里**不得**出现设计沿革（“原来是 X，现在 Y”）、版本号（v0.11）、评审编号（06-review/06-17）；
  这些属于 commit message 与 `docs/06-review`。`[FACT]`
- 未完成的功能留 `// TODO(owner): …`（已知缺陷用 `FIXME`）作为占位：它是 IDE TODO 面板的
  唯一来源，不得当作沿革注释删除；实现完成后删除，不得改写成“已完成”。`[FACT]`
- 可执行清单与提交前自检命令见 `docs/prompts/_common.md` 第 7 节第 11 条。`[FACT]`

## 4. 必须使用现代 Go 语法

- 用 `any`，不用 `interface{}`。`[FACT]`（Google Decisions: Use any）
- 用 `min` / `max` / `clear` 内置函数，避免手写循环。`[INFERENCE]`
- 用 `slices` / `maps` / `cmp` / `iter` 标准库，不用旧 `sort` 写法。`[INFERENCE]`
- `for i := range n`（Go 1.22+）替代 `for i := 0; i < n; i++`。`[INFERENCE]`
- 用 `errors.Join` 组合错误。`[INFERENCE]`
- 用 `errors.Is` / `errors.As` 判断错误。`[FACT]`
- 用 `%w` 包装错误；不要用 `%v` 包装后丢失链。`[FACT]`
- 用 `strings.Cut` / `CutPrefix` / `CutSuffix`，避免手写 `Index` 拼接。`[INFERENCE]`
- 用 `time.Since(t)`，不用 `time.Now().Sub(t)`。`[INFERENCE]`
- 用 `os.ReadFile` / `os.WriteFile` / `os.CreateTemp` / `os.MkdirTemp`。`[FACT]`
- 用 `io.ReadAll` / `io.Discard` / `io.NopCloser`，不用 `io/ioutil`。`[FACT]`
- 用 `log/slog` 记录结构化日志；新代码不用全局 `log`。`[INFERENCE]`
- 随机数安全场景用 `crypto/rand`；一般随机用 `math/rand/v2`，不用旧 `math/rand` 全局。`[FACT]`
- `context.Context` 必须是函数第一个参数；不要放进 struct。`[FACT]`（Google Decisions）

## 5. 禁止或避免的旧写法

- 禁止 `github.com/.../ioutil` / `io/ioutil`。`[FACT]`
- 禁止用 `panic` 处理普通错误；错误应返回给调用者。`[FACT]`
- 禁止用 `init()` 做注册/副作用；使用显式注册函数并在 `app` 组装。`[FACT]`
- 避免全局可变状态；依赖通过参数或 struct 注入。`[FACT]`
- 避免裸返回（naked returns），除非函数非常短。`[FACT]`
- 避免把 `string` / interface 指针化来“省字节”。`[FACT]`
- 避免在调用链中间随意 `context.Background()`；优先从调用方传递。`[FACT]`
- 避免 `os.IsNotExist` 等旧判断，改用 `errors.Is(err, fs.ErrNotExist)`。`[INFERENCE]`
- 避免 `sort.Slice` / `sort.Strings`，改用 `slices.SortFunc` / `slices.Sort`。`[INFERENCE]`
- 避免 `for` 循环里 `x := x` 捕获变量；Go 1.22+ 已修复循环变量语义。`[INFERENCE]`

## 6. 字符串常量与工具函数

- 常用字符串字面量必须提取为 `const`，例如引擎名、flag 名、默认策略、日志消息键。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 禁止在多个文件中重复写同一业务字符串。`[FACT]`
- 常用重复逻辑提取为函数，放入 `internal/utils` 或对应领域包：
    - `internal/utils`：通用、无业务语义的小工具。
    - 领域逻辑优先放对应包，例如 `diag.NotImplemented`。`[INFERENCE]`
- 示例：
    - `engine.NameExtended`
    - `engine.UnsetError`
    - `cli.flagOutput`
    - `render.logRenderStart`
    - `utils.IsStdio`
    - `diag.NotImplemented`
- `utils` 不得反向依赖业务包；只依赖标准库。`[FACT]`

## 7. 错误处理

- 错误只处理一次：要么立即处理，要么返回给调用方。`[FACT]`
- 错误包装要提供上下文，例如 `fmt.Errorf("load schema %q: %w", path, err)`。`[FACT]`
- 错误字符串小写，不加标点。`[FACT]`
- 不允许用 `_` 丢弃错误，除非有注释说明为什么安全。`[FACT]`
- 错误分类与退出码统一走 `internal/diag`。`[INFERENCE]`

## 8. 测试规范

- 测试优先：先写 fixture 与 golden，再实现功能。`[FACT]`
- 使用标准库 `testing`；需要结构 diff 时用 `github.com/google/go-cmp/cmp`。`[FACT]`
- 表驱动测试；每个 case 有明确名称。`[FACT]`
- 使用 `t.Helper()`、`t.Cleanup()`、`t.TempDir()`、`t.Setenv()`。`[FACT]`
- 并发相关代码必须 `go test -race`。`[INFERENCE]`
- 解析器、扫描器、`.env` 解析器需要 fuzz target。`[INFERENCE]`
- Golden 更新必须通过显式 `UPDATE_GOLDEN=1`，并人工 review diff。`[INFERENCE]`

## 9. 并发规范

- 不为并发而并发；先证明有收益。`[INFERENCE]`
- 每个 goroutine 必须有明确生命周期与退出条件。`[FACT]`
- 共享状态用 channel 或 `sync` 保护；优先避免共享。`[INFERENCE]`
- 不启动无法取消的后台 goroutine。`[INFERENCE]`
- 使用 `context` 取消长任务与 IO。`[FACT]`

## 10. 泛型规范

- 只在能简化 API 或减少重复时使用泛型。`[FACT]`（Google Decisions: Generics）
- 不用泛型造 DSL 或做“聪明”抽象。`[FACT]`
- 能用接口模型表达时，优先接口。`[FACT]`
- 导出泛型 API 前需要额外 review。`[FACT]`

## 11. 依赖与包边界

- 依赖必须纯 Go、无 CGO、多平台可编译。`[FACT]`
- 新增运行时依赖必须写明理由，并验证 Windows/macOS/arm64。`[FACT]`
- 核心逻辑放 `internal/`；公开库 API 只在 `pkg/` 稳定后暴露。`[INFERENCE]`
- 引擎、来源、Schema、Writer 通过接口解耦，具体实现可替换。`[FACT]`
- 禁止 `internal/engine/extended` 反向依赖 CLI。`[INFERENCE]`

## 12. 提交前检查

- `just fmt`
- `just vet`
- `just test`
- `just lint`
- `govulncheck ./...`
- 新增/修改逻辑必须有对应 fixture 或测试。`[FACT]`

## 13. 非目标（Non-Goals）

- 不追求满足所有 Google 内部风格变体。`[ASSUMPTION]`
- 不为旧 Go 版本保留兼容写法。`[FACT]`
- 不用自定义 lint 规则替代官方/Google 规范。`[INFERENCE]`

## 14. 参考

- Google Go Style Guide: https://google.github.io/styleguide/go/
- Google Go Decisions: https://google.github.io/styleguide/go/decisions
- Google Go Best Practices: https://google.github.io/styleguide/go/best-practices
- Effective Go: https://go.dev/doc/effective_go
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments
