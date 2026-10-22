---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# Extended 设计 · 06 开放问题与评审清单

> 本文是 `03-00-index.md` 的拆分部分。

## 9. Open Questions

所有未决项都必须有 owner 与解决时点。

- 默认 `--on-missing`：**已定 `error`**（`04-cli.md` C8；`empty`/`keep`/`zero` 为可选值）。`[FACT]`
- 更多 formatter：**已定白名单** `join`、`toJson`、`toYaml`、`quote`、`indent`（另有内联 `| type`）；
  `nindent` **未提供**（新增需按 REQ 立项）。见 `07-serialization/05-formatters-and-types.md`。`[FACT]`
- `int`/`float` 显式转换：**已定**——`| type int`/`| type float` 为显式转换，越界/丢精度报 KindType（exit 4）。
  见 `05-formatters-and-types.md`、`09-gaps` G2/G3。`[FACT]`
- 条件/循环的语法与验收标准：**仍待定**，属 P1 且当前不实现。`[TODO]` 见 `05-04` TASK-115；
  owner：需求提出者/技术负责人；触发条件：P1 条件/循环开工时。
- `--merge-env`（OS env 覆盖根值）：**已决为不提供**——进程环境只在 `@process`（`13-materialization.md` §12/§14），
  路径推导已移除（G14）。`[FACT]`
- 特殊字符 key 的写法：**已定**——非标识符 key 必须写成 quoted-string（`["a.b"]`、`["my-key"]`），
  由 `values.IsSimpleIdentifier`/`CanonicalPath` 决定何时需要引号；`["0"]` 恒为键、`[0]` 恒为下标（O3）。`[FACT]`
- schema 文件命名与搜索：**已定**——只按 `--schema` 指定的文件加载，无目录搜索；工具不引入自身配置文件
  （配置面 = CLI flag + 值文件 + schema）。`[FACT]`
- include 的白名单/循环检测/参数：**仍待定**，属 P1 且当前不实现。`[TODO]` 见 `02-05` REQ-TPL-04 与
  `05-04` TASK-114；owner：技术负责人；触发条件：include 开工时。

## 评审重点清单

- 已确认、可从 Open Questions 移除的项：
    - 内联 `| type` 已采用；如果提供，必须作为 pipeline 第一个操作符，且可以缺省。
    - schema 与内联取并集，schema 优先，交集冲突报错。
    - schema 直接复用 JSON Schema Draft 2020-12。
    - 结构化值使用 Helm-like pipeline。
    - 类型名使用 Go 基础类型 `string`/`bool`/`int`/`float`，内部映射 JSON Schema 标准类型。
    - 控制指令单独成行；include 功能 P1 实现。
    - 默认 required 字段不自动填零。
- 仍需确认的 `[ASSUMPTION]`：
    - 默认 `--on-missing=error`；`--on-missing=zero` 已确认加入，仅限非 required 字段。
    - 特殊 key 只允许 quoted-string。
    - 环境变量如何进入值树？**已决**：`.env` 顶层键写入根，进程环境只经 `${@process.NAME}` 读取，
      不做"路径 → 环境变量名"推导（见 `07-serialization/13-materialization.md` §13/§14）。
- 主要 `[INFERENCE]`：
    - `env` 命名空间与根命名空间。
    - `${path:-default}` 等操作符语义。
    - 多文件加载顺序。
- 仍待补的信息：
    - Open Questions 中两条 P1 项（条件/循环、include）——已在表中标注 owner 与触发条件。
    - Windows/CRLF/编码保真（`02-05` REQ-OUT-06 / REQ-DIS-04 未实现，需 Windows CI）。
- 替代方案重点：
    - 自研 schema DSL（拒绝）。
    - Jinja `{{ }}` 风格（拒绝）。
    - 自动文本拼接数组/对象（拒绝）。
    - `:=` 写回环境变量（拒绝）。

## 非目标（本部分）

- 不替代整体设计评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `03-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
