---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-13
---

# ADR-005：`--explain` 的来源模型（Accepted）

## 1. 标题

ADR-005：`--explain` 用「统一来源帧 + 完整层序链」表达值从哪来

## 2. 状态

Accepted，2026-10-13。**不考虑历史兼容**：现有 `--explain` 的 JSONL 字段直接替换，
不保留 `[]string` 形式的 `shadowed`，不提供 v2 开关。

## 3. 背景

`--explain` 要回答的是「这个值从哪来、如何来」。当前实现有三个结构性问题：

1. **`source` 与其它字段重复**：`{"source":"schema","kind":"schema"}`、`{"source":"process","kind":"process"}`，
   同一信息出现两次；对文件来源，`source` 又等价于 `file:line`。
2. **层序链表达不了**：`shadowed` 是 `[]string`，只记"最近被覆盖者在前"，粒度不统一
   （`app.env:4` 带行号、`values-prod.yaml` 不带），且**不含 schema `default`**——
   默认值被覆盖后完全消失，无法从报告看出"这里原本有个 default"。
3. **环境变量只记到 kind**：`@process.ENVSUB_X` 记录 `source:"process"`，丢失变量名。

## 4. 决策

### 4.1 一个值 = 一条层序链，链上每个元素是同构的「来源帧」

```
更低优先级 ─────────────────────────────────────────────► 更高优先级
schema default → yaml/json 值文件 → .env → --set/--set-json/--set-file → 模板写回 := / =
                                                                          ▲ 胜出者
```

- 记录**完整链**（含 `default`），不截断、不去重（同一文件深合并可为不同路径贡献多帧）；
- 报告给出现胜帧 `origin` 与它覆盖的帧 `shadowed`（**近者在前**）；
  于是 `shadowed + origin` 逆序即为完整链，`origin` 恒为链尾。

### 4.2 每帧结构统一，字段职责互不重复

```json
{"kind":"dotenv","source":"examples/data/app.env","file":"examples/data/app.env","line":4}
```

| 字段 | 含义 | 是否恒在 |
|------|------|----------|
| `kind` | 来源类别，闭集：`schema` `yaml` `json` `envfile` `set` `setfile` `process` `assign` `missing` | 是 |
| `source` | 该类别的**唯一地址**，不重复 `file`/`line`：文件类给路径；`process` 给**环境变量名**；`schema` 给**指针**；`set`/`setfile` 给键名；`assign` 给模板路径 | 是 |
| `file` | 该帧所在文件；无文件来源为空串 `""` | 是（无则空） |
| `line` | 1 基行号；不可得或对同文件所有键相同者为 `0` | 是（无则 0） |

**禁止缺省**：所有帧的四个字段都输出，空值用 `""`/`0`，消费方不需要判空分支。

### 4.3 行格式

```json
{"template":"examples/templates/scenarios-json.tmpl","variable":"string_type_root",
 "origin":{"kind":"yaml","source":"examples/data/values-prod.yaml","file":"examples/data/values-prod.yaml","line":0},
 "shadowed":[
   {"kind":"yaml","source":"examples/data/values-base.yaml","file":"examples/data/values-base.yaml","line":0},
   {"kind":"schema","source":"#/properties/string_type_root/default","file":"examples/data/schema.yaml","line":0}
 ]}
```

进程环境：

```json
{"template":"in.tmpl","variable":"@process.ENVSUB_STRING",
 "origin":{"kind":"process","source":"ENVSUB_STRING","file":"","line":0},
 "shadowed":[]}
```

约定：`template`/`variable` 仍是信封（一行一条）；`shadowed` **恒输出**，无被覆盖帧时为 `[]`。
不输出值（值可能是密钥）。

### 4.4 不变式（可直接写成断言）

| # | 不变式 |
|---|--------|
| I1 | `origin.kind` 属于闭集；`shadowed` 的每一帧同样属于闭集 |
| I2 | `origin` 是链尾（最高优先级），`shadowed` 近者在前，二者不重复同一帧 |
| I3 | `source` 不与 `file`/`line` 重复表达：文件类帧的 `source == file` |
| I4 | 任一帧的四个字段都存在（不得 `omitempty`） |
| I5 | 若某路径在 schema 中有 `default` 且被覆盖，链中必须出现该 `schema` 帧 |

## 5. 考虑的替代方案

| 方案 | 结论 |
|------|------|
| 保持 `[]string` 的 `shadowed`，只改展示层 | **未采用**：机器消费方仍拿不到 kind/行号/default，问题留在协议里 |
| `shadowed` 用 `[]object`，但顶层仍是 `source`/`file`/`line` 平铺 | **未采用**：顶层与帧两套形状，重复问题（`source` 与 `file:line` 等价）依旧存在 |
| 只输出胜出者，不输出链 | **未采用**：回答不了"如何来"；默认值被覆盖的场景无法解释 |
| 输出完整链为单个 `chain` 数组（含胜出者） | **未采用但记录**：语义更直接（`chain[last]` 即胜出者），代价是取胜出者需要索引；本方案用 `origin` + `shadowed` 表达同一信息且更利于流式打印 |

## 6. 影响

| 面 | 影响 |
|----|------|
| 产品 | `internal/render/explain.go` 重构记录结构；`load` 观察者需上报 schema `default` 帧（当前 default 不与来源关联）；`kind` 细分出 `envfile`/`setfile`/`assign` |
| 契约 | `--explain` JSONL 字段不兼容变更（已决：不保留旧形状） |
| 测试 | 7 个 e2e `explain_*` fixture 重生成；补 `shadowed` **顺序**与**多层链**断言（`05-02` 登记的缺口）；补 I1–I5 |
| 文档 | `13-materialization.md` §15 重写；`06-11`/`06-13`/`06-14` 的 `[TODO]` 转正并更新字段表；`05-03` 待办 I 状态更新 |
| 展示 | `examples/tools/digest --sources` 按新结构打印，路径归一为**仓库根相对** |

## 7. 未决

- `assign`（模板内写回）是否单列 kind，还是并入 `set`（待实现时定，需模板行号）；
- 值文件（yaml/json）是否补行号（需加载器记录位置，成本较高，暂记 `line: 0`）。
