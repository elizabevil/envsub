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

- 默认 `--on-missing` 是 `error` 还是 `empty`？`[TODO]` owner：需求提出者；解决时点：PRD 前。
- 是否需要 `quote`、`nindent` 等更多 formatter？`[TODO]` owner：需求提出者；解决时点：P0 实现前。
- `int` 与 `float` 是否允许显式转换，还是严格禁止隐式/显式转换。`[TODO]` owner：技术负责人；解决时点：类型系统实现前。
- 条件/循环的语法细节与 P1 验收标准（待办，当前不实现）。`[TODO]` owner：需求提出者/技术负责人；解决时点：P1 设计前。
- 是否支持 `--merge-env`，让 OS env 覆盖根值？`[TODO]` owner：技术负责人；解决时点：P0 实现前。
- 带 `-`、`.`、`/`、空格的 key 是否只允许 quoted-string？`[TODO]` owner：需求提出者/技术负责人；解决时点：语法实现前。
- schema 文件命名、目录搜索顺序与工具自身配置格式。`[TODO]` owner：技术负责人；解决时点：CLI 设计前。
- include 的路径白名单、循环检测、执行时指定文件的参数设计（待办，当前不实现）。`[TODO]` owner：技术负责人；解决时点：P1 实现前。

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
- 主要 `[TODO]`：
    - 上表 Open Questions 全部项。
    - Go 接口签名。
    - Windows/二进制模板与性能边界。
- 替代方案重点：
    - 自研 schema DSL（拒绝）。
    - Jinja `{{ }}` 风格（拒绝）。
    - 自动文本拼接数组/对象（拒绝）。
    - `:=` 写回环境变量（拒绝）。

## 非目标（本部分）

- 不替代整体设计评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `03-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
