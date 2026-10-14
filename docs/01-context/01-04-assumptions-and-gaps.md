---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 上下文摘要 · 04 待确认假设与信息缺口

> 本文是 `01-00-index.md` 的拆分部分。

## 5. 待确认假设

- 一期只实现 extended 原生模式；GNU/a8m/drone 兼容与转换后置 P2。`[FACT]`（来源：
  `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`）
- v0/v1 继续使用 Go，并采用层级值模型 + 渲染引擎。`[FACT]`（来源：需求提出者确认，2026-09-29；详见 ADR-001）
- “默认值”：值层默认值 + 模板层默认值。`[ASSUMPTION]` 验证：需求提出者确认语法；owner：待指定。
- “数组表示”：数组作为一等值类型，来源支持 YAML/JSON；`.env` 使用 JSON 字符串表达，输出由 pipeline formatter 决定。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- “转义格式”：扩展模式建议 `$$` 表示字面量 `$`，兼容模式按对应实现处理。`[ASSUMPTION]` 验证：确认默认转义符及兼容要求；owner：待指定。
- “类型校验”：已确认 P0；支持值级/schema 级运行时校验；来自 env/.env 的字符串可按 schema 解析/校验；不承诺对任意纯文本模板做静态类型检查。
  `[FACT]`（来源：需求提出者确认 P0，2026-09-29）具体类型集合、强制转换策略、失败行为与报错格式仍为 `[ASSUMPTION]`
  待确认；owner：待指定。
- “文件配置/多文件配置”：已确认支持 `.env`、YAML、JSON values 文件；后加载覆盖先加载，数组默认替换而非追加，CLI `--set` 优先级最高。
  `[FACT]`（来源：需求提出者确认，2026-09-29）
- “环境变量/多文件”：是否忽略 shell 环境、是否深合并、数组追加规则仍需确认。`[ASSUMPTION]` 验证：需求提出者/技术负责人给出规则；owner：待指定。

## 6. 信息缺口

- 确认或修正 AI 补充的六项默认假设；补充具体输入输出示例与优先级。`[TODO]` owner：需求提出者。
- TOML 是否纳入 P0/P1，以及工具自身配置格式。`[TODO]` owner：需求提出者/技术负责人。
- 数组与类型的语义：如何定义、嵌套、引用和输出？`[TODO]` owner：技术负责人。
- 多文件配置的环境隔离与深合并例外规则。`[TODO]` owner：技术负责人。
- P2 的 GNU envsubst、a8m/envsubst、drone/envsubst 兼容边界。`[TODO]` owner：技术负责人。
- 错误/退出码、严格模式、dry-run、批量写回和权限策略。`[TODO]` owner：技术负责人。
- 平台矩阵、静态编译约束、发布方式、安全与验收标准。`[TODO]` owner：产品/技术负责人。
- 内联 `| type` 的类型转换细节。`[TODO]` owner：需求提出者/技术负责人。
- 常规二进制优化的具体命令与发布流水线细节；性能目标参考 GNU envsubst。`[TODO]` owner：技术负责人。

## 非目标（本部分）

- 不替代整体上下文评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `01-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
