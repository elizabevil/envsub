> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §5。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 5. 模板操作符规则

**本章解决**：bash 风格操作符的语法、语义、写回、错误，以及存在性/空值判定，并给出每操作符案例。

### 5.1 已实现的操作符

_列：ID | 语法 | 语义 | 写回值树 | unset/空判定依据 | 错误与退出码 | 测试。_

| 编号 | 语法 | 语义 | 写回值树 | unset / 空判定依据 | 错误与退出码 | 测试 |
|------|------|------|----------|--------------------|--------------|------|
| O1 | `${A}` / `$A` | 直接取值 + `--on-missing`/`--on-empty` 策略 | 否 | 存在性 `ResolvePath`/`ResolveName`（`values/tree.go`）；空 `isEmptyValue`（`extended/engine.go`） | unset→6；空+`error`→6；`keep`→原样 | errors/missing_variable、policies/* |
| O2 | `${A:-word}` | `!exists \|\| empty` → word | 否 | `isEmptyValue`（`extended/engine.go`） | word 渲染错误透传；word 走管道 | defaults/default_or_empty_*、operators/pipeline_on_word |
| O3 | `${A-word}` | 仅 `!exists` → word（空值仍取值） | 否 | 仅存在性（`extended/engine.go`） | 同上 | defaults/default_unset* |
| O4 | `${A:=word}` | `!exists \|\| empty` → 渲染并 `Store` | **是**：存 word **原文**，不存管道结果（`extended/engine.go`） | `isEmptyValue` | `Store` 失败→3 | policies/assign_default、operators/pipeline_on_word |
| O5 | `${A=word}` | 仅 `!exists` 时赋值 | **是**（`extended/engine.go`） | 仅存在性 | 同上 | policies/assign_if_unset |
| O6 | `${A:+word}` | `exists && !empty` → word，word 走管道 | 否 | `isEmptyValue`（`extended/engine.go`） | 管道失败按类型错误（exit 4） | defaults/alternate_nonempty* |
| O7 | `${A+word}` | `exists` → word，word 走管道 | 否 | 仅存在性（`extended/engine.go`） | 同上 | defaults/alternate_if_set* |
| O8 | `${A:?word}` | `!exists \|\| empty` → 报错，word 为消息 | 否 | `isEmptyValue`（`extended/engine.go`） | KindMissing=6（`extended/engine.go`） | defaults/required_empty |
| O9 | `${A?word}` | 仅 `!exists` → 报错 | 否 | 仅存在性（`extended/engine.go`） | KindMissing=6 | defaults/required_unset |

### 5.1.1 word 与管道（O2–O7）

**规则**：运算符产出的 word 是**表达式的值**，因此表达式的管道作用于它，与直接取值走同一条路径。

| 语法 | 期望 |
|------|------|
| `${A:-fallback \| quote}` | `"fallback"`（管道作用于 word） |
| `${A:+alt \| toJson}` | `"alt"` |
| `${A:=word \| quote}` | 输出 `"word"`；**值树里存的是 `word`**（原文，不带引号），后续 `${A}` 得到 `word` |
| `${A:-word \| join ","}` | word 是字符串，`join` 要求数组 → 类型错误 exit 4 |
| `${INT:+word}`，schema 声明 `INT: integer` | word 是字符串且管道结果仍是字符串 → 类型错误 exit 4 |

`[FACT]` 依据：`extended/engine.go` 的 `pipelineWord`/`assignWord`；fixture
`operators/pipeline_on_word`（含 `:=` 写回原文的断言）与 `operators/pipeline_on_word_rejects_type`。

### 5.2 未实现的操作符（禁止依赖）

_列：语法 | 实际行为 | 证据 | 退出码。_

| 语法 | 实际行为 | 证据 | 退出码 |
|------|----------|------|--------|
| `${A#p}` `${A##p}` `${A%p}` `${A%%p}` `${A^}` `${A^^}` `${A,}` `${A,,}` `${A@Q}` `${A:offset:length}` | 非 operator rune → `invalid expression` | `extended/expression.go`,124 | 3 |
| `${!A}` | 路径解析先失败 → `expected identifier` | `values/path.go`；`extended/expression.go` | 3 |

### 5.3 存在性与空值判定

**本章节解决**：区分"路径是否存在"与"值是否为空"，并明确 `""` 判空是 **bash 兼容语义**。

判定分**两级**：

- **存在性**：路径/键是否有值；只看 `ResolvePath`/`ResolveName` 是否成功（`values/tree.go`）。
- **空值性**：值存在时内容是否为空；见 OV1/OV2（`extended/engine.go`）。

> **bash 兼容语义（必须理解，OV9）**：`null` 与 `""`、空集合一样判为**空**。因此带冒号的四个
> 操作符在 **unset 或空值**时生效，不带冒号的四个只在 **unset** 时生效：
> `${A:-d}`/`${A:+a}`/`${A:?e}`/`${A:=d}` 对"空值"也触发；`${A-d}`/`${A+a}`/`${A?e}`/`${A=d}`
> 只在 unset 时触发。`0`/`false`/精确整数/非空集合**不算空**。
> 这是**有意的兼容选择**，对齐 bash `:-`/`:+`/`:?`/`:=`，**不是**通用编程语言的真值判断。

_列：ID | 规则 | 证据。_

| 编号 | 规则 | 证据 |
|------|------|------|
| OV1 | **算空**：`nil`（含 typed nil）、`""`、空 `[]any`、空 `map[string]any` | `extended/engine.go` |
| OV2 | **不算空**：`0`、`false`、uint64/精确整数、非空集合 | `extended/engine.go` |
| OV3 | typed nil map/slice 判空，但编码为 `{}`/`[]`（V1） | `extended/engine.go` |
| OV4 | 存在性只看路径是否解析成功，与值内容无关 | `values/tree.go`；`values/path.go` |
| OV5 | 默认策略 `--on-missing=error`、`--on-empty=ok` | `cli/render.go` |
| OV6 | `:=`/`=` 是唯一写回值树的运算符；`${A:-x}` 不得影响后续引用 | `extended/engine.go`；policies/default_does_not_assign |
| OV7 | 含 `:=`/`=` 的模板并行渲染回退串行 | `extended/parallel.go` |
| OV8 | `NaN`/`±Inf` 不应出现在值树；出现即按非法值报错 exit 4（F8） | §3.10 F8 ✅ |
| OV9 | `""` 判空是 bash 兼容语义（有意选择） | 本节上方 callout；`extended/engine.go` |

下表为**默认策略**（`--on-missing=error`、`--on-empty=ok`）下的实际结果，已实测。
_列/图例：`err6`=KindMissing exit 6；`err4`=KindType exit 4；`(空)`=不输出；`写回`=同时把 `d`
以字符串写回值树。_

| 值 \ 表达式 | `${A}` | `${A:-d}` | `${A-d}` | `${A:+a}` | `${A+a}` | `${A:?e}` | `${A?e}` | `${A:=d}` | `${A=d}` |
|-------------|--------|-----------|----------|-----------|----------|-----------|----------|-----------|----------|
| 未设置 | `err6` | `d` | `d` | `(空)` | `(空)` | `err6` | `err6` | `d`（写回） | `d`（写回） |
| `null` | `(空)` | `d` | `(空)` | `(空)` | `a` | `err6` | `(空)` | `d`（写回） | `(空)` |
| `""` | `(空)` | `d` | `(空)` | `(空)` | `a` | `err6` | `(空)` | `d`（写回） | `(空)` |
| `0` | `0` | `0` | `0` | `a` | `a` | `0` | `0` | `0` | `0` |
| `false` | `false` | `false` | `false` | `a` | `a` | `false` | `false` | `false` | `false` |
| `[]` | `err4` | `d` | `err4` | `(空)` | `a` | `err6` | `err4` | `d`（写回） | `err4` |
| `{}` | `err4` | `d` | `err4` | `(空)` | `a` | `err6` | `err4` | `d`（写回） | `err4` |
| typed nil map | `err4` | `d` | `err4` | `(空)` | `a` | `err6` | `err4` | `d`（写回） | `err4` |
| typed nil slice | `err4` | `d` | `err4` | `(空)` | `a` | `err6` | `err4` | `d`（写回） | `err4` |
| 精确整数 | 精确十进制 | 精确十进制 | 精确十进制 | `a` | `a` | 精确十进制 | 精确十进制 | 精确十进制 | 精确十进制 |
| `NaN`/`±Inf`（非法） | `err4` | `err4` | `err4` | `err4` | `err4` | `err4` | `err4` | `err4` | `err4` |

**案例（操作符）**：每个操作符给未设置 / 空串 / 有值三类输入。
_列：操作符 | 输入 | 输出 | exit | 标注。_

| 操作符 | 输入 | 输出 | exit | 标注 |
|--------|------|------|------|------|
| O1 `${A}` | 未设置 / `A=""` / `A=0` | `variable "A" is not set` / 空 / `0` | 6/0/0 | 现状 |
| O2 `${A:-d}` | 未设置 / `A=""` / `A=0` | `d` / `d` / `0` | 0 | 现状 |
| O3 `${A-d}` | 未设置 / `A=""` / `A=0` | `d` / 空 / `0` | 0 | 现状 |
| O4 `${A:=d}` | 未设置 / `A=""` / `A=0` | `d`（写回） / `d`（写回） / `0` | 0 | 现状 |
| O5 `${A=d}` | 未设置 / `A=""` / `A=0` | `d`（写回） / 空 / `0` | 0 | 现状 |
| O6 `${A:+a}` | 未设置 / `A=""` / `A=false` | 空 / 空 / `a` | 0 | 现状 |
| O7 `${A+a}` | 未设置 / `A=""` / `A=null` | 空 / `a` / `a` | 0 | 现状 |
| O8 `${A:?e}` | 未设置 / `A=""` / `A=0` | `e` / `e` / `0` | 6/6/0 | 现状 |
| O9 `${A?e}` | 未设置 / `A=""` / `A=null` | `e` / 空 / 空 | 6/0/0 | 现状 |

```bash
# 三类输入的可复制案例
for v in '""' 0 false; do
  ./dist/envsub render --set-json "A=$v" <(printf 'p=${A} d=${A:-d} a=${A:+a}\n')
done
./dist/envsub render <(printf 'p=${A}\n')                       # 未设置 → exit 6
./dist/envsub render --set-json 'A=[]' <(printf '${A:-d}\n')     # 空数组 → d
./dist/envsub render --set-json 'A=[]' <(printf '${A}\n')        # exit 4（无 formatter）
```

