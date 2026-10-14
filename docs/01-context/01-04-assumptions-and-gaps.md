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

- 六项默认假设：已逐条落在 §5（已确认者标注来源，仍待确认的以 `[ASSUMPTION]` 保留）；输入输出示例见 `examples/`，优先级见 `02-05`。`[FACT]`
- TOML 与自身配置格式：TOML 定为 **P1 候选、未实现**（`02-05` REQ-SRC-06，需补 ADR-004 决策）；
  工具自身不引入配置文件，配置面只有 CLI flag + 值文件 + schema。`[FACT]`
- 数组与类型语义：见 `07-serialization/01-types.md`（类型与取值范围）、`03-03-values-merge-and-schema.md`
  与 `07-serialization/05-formatters-and-types.md`（`| type` 转换与错误）。`[FACT]`
- 多文件配置与深合并：层序见 `04-cli.md` §6.2，合并规则见 `02-sources.md` §4.2，
  `default` 兜底见 G21；进程环境不入根、只在 `@process`。`[FACT]`
- P2 兼容边界：基线见 `reference/REF-02-envsubst-compatibility-baseline.md`；
  对应任务 `05-04` TASK-201/TASK-202（未开工，需先冻结兼容矩阵）。`[FACT]`
- 错误/退出码与写回策略：退出码见 `07-serialization/08-errors.md`；严格模式为
  `--validate-only` 与 `--on-missing=error`；批量写回为 `--output-dir`/`--in-place`
  （`--in-place` 原子替换）；**dry-run 仍是 P1 候选**（`05-03` §13 的 REQ-OUT-03）；
  输出权限保真见 `05-04` TASK-109。`[FACT]`
- 平台矩阵与发布：`release.yml` 矩阵构建 5 个目标（linux/darwin/windows × amd64/arm64 组合），
  纯 Go、无 CGO（`04-04` 依赖选型原则）；验收指标见 `reference/REF-04`。`[FACT]`
- 内联 `| type` 细节：见 `07-serialization/03-operators.md` §5 与
  `07-serialization/05-formatters-and-types.md`（含错误码与 case）。`[FACT]`
- 二进制优化与流水线：优化取舍见 `04-04` §8（含尺寸归因 `06-07`），
  发布流水线见 `.github/workflows/release.yml`，性能目标见 `reference/REF-04`。`[FACT]`

## 非目标（本部分）

- 不替代整体上下文评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `01-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
