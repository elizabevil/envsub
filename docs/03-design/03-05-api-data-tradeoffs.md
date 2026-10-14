---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# Extended 设计 · 05 API、数据、替代方案与权衡

> 本文是 `03-00-index.md` 的拆分部分。

## 4. APIs

建议对外先提供 CLI；Go 包 API 可作为内部/库 API 保留。`[INFERENCE]`

内部渲染接口草案：

```go
type Input struct {
    Templates []string
    Values    []ValueSource
    Schema    *Schema
    Options   Options
}

type Result struct {
    Outputs []Output
    Report  Report
}

func Render(ctx context.Context, in Input) (Result, error)
```

`[INFERENCE]` 具体签名在实现设计阶段确认。`[TODO]` owner：技术负责人。

## 5. Data

- 值树：`object` 根节点，节点保存 `value`、`type`、`source`、`sensitive`。`[INFERENCE]`
- Schema：YAML/JSON 描述文件，加载后归一化为内部类型描述树。`[INFERENCE]`
- 来源追踪：每个值记录来源层级与文件路径，用于错误信息和 `--explain`。`[INFERENCE]`
- 不持久化存储；每次渲染重新加载。`[INFERENCE]`

## 6. Alternatives Considered

### 6.1 采用 `{{ ... }}` Jinja/Handlebars 风格

- 优点：表达式、条件、循环成熟；生态大。
- 拒绝原因：一期定位是 envsubst 风格文本渲染与迁移路径；`${...}` 更贴近已有模板；完整模板引擎超出非目标。`[INFERENCE]`

### 6.2 自研 schema DSL（已拒绝）

- 优点：可以完全按本工具需求定制。
- 拒绝原因：需要自研解析器、校验器、文档与 IDE 支持，成本高，且无法直接复用现成 schema 文件。`[INFERENCE]`
- 当前决策：直接复用 JSON Schema Draft 2020-12，外加薄适配层，详见 `docs/04-decisions/04-03-adr-003-json-schema-reuse.md`。
  `[FACT]`

### 6.3 对数组/对象自动做文本拼接（已拒绝）

- 优点：实现简单。
- 拒绝原因：行为不可预测，容易在不同输出格式中产生错误；当前采用 Helm-like pipeline，使用 `| toJson`、`| toYaml`、`| join`、
  `| indent` 显式格式化。`[FACT]`（来源：需求提出者确认，2026-09-29）

### 6.4 `:=` 真正写回环境变量

- 优点：与 shell/bash 语义一致。
- 拒绝原因：渲染器应避免副作用；`:=` 仅作为默认值别名，不修改进程环境。`[INFERENCE]`

## 7. Trade-offs

- 选择 `${...}` 语法：迁移友好，但需要处理嵌套默认值和转义边界。`[INFERENCE]`
- 默认 `--on-missing=error`：更安全，但可能对宽松模板造成摩擦；可用 `--on-missing=empty` 放宽。`[ASSUMPTION]`
- 数组来自 env 要求 JSON：避免逗号分隔歧义，但输入稍显冗长。`[ASSUMPTION]`
- 数组默认替换：规则简单可预测，但不支持“追加多个环境”的用法；后续可用显式 merge 策略扩展。`[ASSUMPTION]`
- 保留 `env` 命名空间：清晰，但根键不能叫 `env`；需要文档明确。`[INFERENCE]`
- 结构化值默认 JSON：对 JSON/YAML 输出友好，但对纯文本模板可能需要显式 `| join` / `| toYaml` 或报错。`[ASSUMPTION]`

## 8. Cross-cutting Concerns

- 安全：默认不执行 shell、不访问网络；敏感字段在日志/错误中脱敏；`x-sensitive` 标记。`[INFERENCE]`
- 可观测性：稳定错误码、`--json-errors`、`--explain`（P1）、渲染报告。`[INFERENCE]`
- 性能：值树通常较小；模板文本流式读取与写出，避免一次性读入超大文件。`[INFERENCE]`
- 兼容性：语法预留 Phase 2 兼容模式与转换命令的扩展点。`[FACT]`（来源：ADR-002）
- 平台：Windows CRLF、路径分隔符、UTF-8 默认；二进制模板另行评估。`[TODO]` owner：技术负责人。
- 可测试性：为语法和类型系统建立 golden fixture；覆盖嵌套默认值、转义、数组/类型错误。`[INFERENCE]`

## 非目标（本部分）

- 不替代整体设计评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `03-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
