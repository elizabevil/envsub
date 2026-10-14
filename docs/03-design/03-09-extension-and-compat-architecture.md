---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 扩展与兼容架构

## 1. 目标

- 一期只实现 extended 原生模式，但所有可替换点必须通过接口暴露。`[FACT]`（来源：需求提出者确认，2026-09-29）
- P2 的 GNU/a8m/drone 兼容模式作为不同实现接入，不修改 CLI 与核心渲染流程。`[FACT]`
- 避免兼容逻辑侵入类型、值来源、Schema 与输出层。`[INFERENCE]`

> **实现状态**：本文是 **P2 的预留设计**。当前实现只有一个引擎：CLI 接收一个引擎
> （`cli.RenderEngine` 只要 `Name()`），`internal/app` 注入 `extended.New()`，
> 流式/流读取/路径枚举能力由 `render` 对具体值做类型断言发现。
> 文中的 `Registry`/`Capabilities` 草案在多引擎需求出现时再落地（连同选择开关一起）。`[FACT]`

## 2. 设计原则

- 依赖倒置：CLI 依赖接口，不依赖具体引擎。`[INFERENCE]`
- 能力声明：每个实现声明自己支持的能力集合。`[INFERENCE]`（**当前未实现**，见上方状态说明）
- 默认实现：一期注册 `extended`；P2 再注册 `gnu`、`a8m`、`drone`。`[FACT]`
- 共享基础设施：值来源、类型系统、Schema、Writer、Formatter 尽量复用。`[INFERENCE]`
- 行为隔离：兼容模式只影响模板语法/求值语义，不改变 CLI 主流程。`[INFERENCE]`

## 3. 建议包结构

```text
internal/engine/
├── engine.go          # Engine 接口、Capabilities、Registry
├── extended/          # P0 扩展模式实现
├── gnu/               # P2 预留
├── a8m/               # P2 预留
└── drone/             # P2 预留

internal/source/       # ValueSource 接口与 .env/YAML/JSON 实现
internal/schema/       # Validator 接口与 JSON Schema 实现
internal/types/        # 类型模型与物化
internal/pipeline/     # Formatter 注册表
internal/output/       # Writer 接口：stdout/file/in-place/batch
internal/app/          # 组装 Registry、CLI、Reader、Writer
```

## 4. 核心接口草案

### 4.1 Engine 接口

```go
type Capabilities struct {
    Defaults       bool
    Arrays         bool
    TypeValidation bool
    Escaping       bool
    ControlFlow    bool
}

type Input struct {
    Template []byte
    Values   *values.Tree
    Schema   *schema.Schema
    Options  Options
}

type Engine interface {
    Name() string
    Capabilities() Capabilities
    Render(ctx context.Context, in Input) ([]byte, error)
}
```

`[INFERENCE]` 具体签名在实现时确认，但接口边界必须保留：`Engine` 隔离模板语义。

### 4.2 ValueSource 接口

```go
type ValueSource interface {
    Name() string
    Load(ctx context.Context, r io.Reader) (*values.Tree, error)
}
```

- `.env`、YAML、JSON、CLI `--set*` 各自实现。`[INFERENCE]`
- 多文件合并属于 `values` 层，不属于 Engine。`[INFERENCE]`

### 4.3 Validator 接口

```go
type Validator interface {
    Validate(ctx context.Context, tree *values.Tree) error
}
```

- P0 使用 JSON Schema 实现。`[FACT]`
- P2 兼容模式可自行决定是否跳过类型校验。`[INFERENCE]`

### 4.4 Writer 接口

```go
type Writer interface {
    Write(ctx context.Context, target string, data []byte) error
}
```

- stdout、file、in-place、batch 各自实现。`[INFERENCE]`
- Engine 只返回字节，不直接写文件。`[INFERENCE]`

### 4.5 Formatter 注册表

```go
type Formatter func(any, ...string) (string, error)
type Registry map[string]Formatter
```

- `join`、`toJson`、`toYaml`、`quote`、`indent` 注册为内置。`[FACT]`
- P1/P2 可增加只读 formatter，但不改变 Engine 接口。`[INFERENCE]`

## 5. 兼容模式接入方式

- P0：注册 `extended`，CLI 默认调用它。`[FACT]`
- P2：
    - 新增 `gnu`、`a8m`、`drone` 三个 Engine 实现。
    - CLI 增加 `--compat=NAME`，通过 `Registry` 解析 Engine。
    - 兼容模式共享 ValueSource / Writer；是否共享 Schema Validator 由各引擎 Capabilities 声明。`[INFERENCE]`
- 兼容测试：每个 Engine 实现共享 conformance 测试套件，必要时对 GNU/a8m/drone 运行对照测试。`[INFERENCE]`

## 6. 依赖方向

```text
cmd/envsub
  → internal/app
      → internal/cli
      → internal/engine.Registry
          → internal/engine/extended
          → P2: internal/engine/gnu / a8m / drone
      → internal/source
      → internal/schema
      → internal/output
      → internal/pipeline
```

- 反向依赖禁止：Engine 不依赖 CLI，Writer 不依赖 Engine。`[INFERENCE]`

## 7. 示例：CLI 选择 Engine

```go
eng, err := registry.Get("extended")
if err != nil { ... }
out, err := eng.Render(ctx, engine.Input{...})
if err != nil { ... }
writer.Write(ctx, target, out)
```

P2 只需增加 `registry.Register("gnu", ...)` 和 CLI 参数映射。`[INFERENCE]`

## 8. 非目标（Non-Goals）

- 本文不实现 P2 兼容语义。`[FACT]`
- 不把兼容模式作为 P0 范围。`[FACT]`
- 不承诺接口在 v0 阶段稳定；**评审时点为进入 v1 前**。`[FACT]`
