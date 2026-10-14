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

### 4.2 kind 是层位，method 是入口写法

`--set` / `--set-json` / `--set-file` 是**同一层的三种写法**（同一层位、同样覆盖值文件、互相顺序
固定），因此不拆成多个 kind，而是 `kind` 记层位、`method` 记入口写法。层位集合稳定，
新增语法只扩 `method`。

| kind | method | `source`（该 kind 下的地址） | `file` | `line` |
|------|--------|------------------------------|--------|--------|
| `schema` | `default` | JSON 指针，如 `#/properties/K/default` | schema 文件 | 0 |
| `values` | `file` | 文件路径（`.yaml`/`.yml`/`.json` 由扩展名区分） | 该文件 | 0 |
| `envfile` | `file` | 文件路径 | 该文件 | 有 |
| `set` | `flag` \| `json` \| `file` | 键名 | `--set-file` 时为被读文件，否则 `""` | 0 |
| `process` | `env` | 环境变量名 | `""` | 0 |
| `assign` | `assign` | 模板路径 | 模板文件 | 有 |
| `missing` | `missing` | `""` | `""` | 0 |

`set` 的三种 method 分别对应 `--set`（`flag`）、`--set-json`（`json`）、`--set-file`（`file`）；
`--set-file` 的被读文件必须填入 `file`（当前实现丢失了它）。

### 4.3 每帧结构统一，字段各回答一个问题

```json
{"kind":"values","method":"file","source":"examples/data/values-prod.yaml",
 "file":"examples/data/values-prod.yaml","line":0}
```

| 字段 | 回答的问题 | 是否恒在 |
|------|------------|----------|
| `kind` | 它在**哪一层**（闭集） | 是 |
| `method` | 它**怎么进来的**（该层的入口写法） | 是 |
| `source` | 它的**地址**：同类来源中唯一标识它（文件路径 / 环境变量名 / JSON 指针 / 键名 / 模板路径） | 是 |
| `file` | 它的**物理位置**所在文件；无文件来源为 `""` | 是 |
| `line` | 1 基行号；不可得为 `0` | 是 |

`source` 与 `file` 在文件类来源上取值相同，但**不是重复表达**：`source` 是"哪一层的哪个来源"，
`file`/`line` 是"去哪个文件的哪一行看"。反例是旧实现的
`{"source":"schema","kind":"schema"}`——那里 `source` 没有提供 `kind` 之外的任何信息。

**禁止缺省**：五字段全部输出，空值用 `""`/`0`。

### 4.4 行格式

```json
{"template":"examples/templates/scenarios-json.tmpl","variable":"string_type_root",
 "origin":{"kind":"values","method":"file","source":"examples/data/values-prod.yaml",
           "file":"examples/data/values-prod.yaml","line":0},
 "shadowed":[
   {"kind":"values","method":"file","source":"examples/data/values-base.yaml",
    "file":"examples/data/values-base.yaml","line":0},
   {"kind":"schema","method":"default","source":"#/properties/string_type_root/default",
    "file":"examples/data/schema.yaml","line":0}
 ]}
```

`--set-file` 的键：

```json
{"variable":"string_setfile_from_file",
 "origin":{"kind":"set","method":"file","source":"string_setfile_from_file",
           "file":"examples/secrets/tls.key","line":0},
 "shadowed":[]}
```

`shadowed` 恒输出，无被覆盖帧时为 `[]`；`origin` 恒为链尾。不输出值（值可能是密钥）。

### 4.5 不变式（可直接写成断言）

| # | 不变式 |
|---|--------|
| I1 | `kind` 属于闭集，`shadowed` 每帧同 |
| I2 | `method` 属于该 `kind` 的允许集合（`set` 三种，其余一种） |
| I3 | `source` 只表达地址：文件类等于 `file`，`process` 为环境变量名，`schema` 为指针，`set` 为键名 |
| I4 | 五个字段恒在，空值 `""`/`0`，不得 `omitempty` |
| I5 | `origin` 是链尾，`shadowed` 近者在前，二者不重复同一帧 |
| I6 | 被覆盖的 schema `default` 必须出现在链里 |
| I7 | 行信封只有 `template`、`variable`、`origin`、`shadowed`，且不输出值 |

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
| 产品 | `internal/render/explain.go` 重构为帧结构；`load` 观察者需上报 schema `default` 帧（当前 default 只报最终生效项）与 `--set-file` 的被读文件；`values` 的 kind 常量改为 kind+method 两段 |
| 契约 | `--explain` JSONL 字段不兼容变更（已决：不保留旧形状） |
| 测试 | 7 个 e2e `explain_*` fixture 重生成；补 `shadowed` **顺序**与**多层链**断言（`05-02` 登记的缺口）；补 I1–I5 |
| 文档 | `13-materialization.md` §15 重写；`06-11`/`06-13`/`06-14` 的 `[TODO]` 转正并更新字段表；`05-03` 待办 I 状态更新 |
| 展示 | `examples/tools/digest --sources` 按新结构打印，路径归一为**仓库根相对** |

## 7. 未决

- 值文件（`values`）是否补行号：需加载器记录位置，成本较高，暂记 `line: 0`；
- `schema` 帧的 `file`/`line` 取 schema 文件路径与 `0`（JSON 无行概念），若将来在 schema 内联
  记录键的行号再更新。
