> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §11。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 11. 错误目录与退出码

**本章解决**：把 `diag.Kind` 映射到退出码与典型文案，供评测与用户对照。触发点全部可用
`file:line` 定位。

_列：kind | 退出码 | 触发条件 | 文案示例 | 证据。_

| kind | 退出码 | 触发条件 | 文案示例 | 证据 |
|------|--------|----------|----------|------|
| `internal` | 1 | 未分类内部错误（engine 未注册、批量渲染部分失败） | `engine "extended" is not registered` / `N of M templates failed: ...` | diag/errors.go:132-134；render.go:133-136 |
| `usage` | 2 | 非法 flag 值、赋值语法错误、未提供/未知类型名、非法值文件扩展名 | `invalid --on-missing value "bogus": want error, empty, keep or zero` | diag/errors.go:123-124；render.go:460-470 |
| `template` | 3 | 模板语法：未闭合、空表达式、未知 formatter、type 位置错误、非支持操作符、indent 参数 | `unclosed variable expression` / `unknown formatter %q` | diag/errors.go:125-126；scanner.go:257；expression.go:175 |
| `type` | 4 | 类型断言/物化/编码失败 | `value "1.5" is not a valid int` / `array value requires a formatter` | diag/errors.go:127-128；convert.go:121；stringify.go:49-50 |
| `schema` | 4 | schema 归一化/编译/校验/未知关键字失败 | `schema validation failed: at /port: ...` | diag/errors.go:127-128；jsonschema.go:32,48 |
| `io` | 5 | 文件读写失败 | `read template %s: %w` | diag/errors.go:129-130；render.go:653 |
| `missing` | 6 | 变量未设置（`--on-missing=error`、O8/O9） | `variable "A" is not set` | diag/errors.go:131-132；engine.go:541 |
| `policy` | 6 | 空值被拒 / `zero` 缺类型 | `variable "A" is empty` / `--on-missing=zero requires a schema type or an inline \| type` | diag/errors.go:131-132；engine.go:550,452 |

**案例（错误目录）**

| 命令 | 输出 / exit | 标注 |
|------|-------------|------|
| `render missing-file.tmpl` | IO 错误，exit 5 | 现状 |
| `render <(printf '${A')` | `unclosed variable expression`，exit 3 | 现状 |
| `--set 'x=1.5'` + `${x \| type int}` | `value "1.5" is not a valid int`，exit 4 | 现状 |
| schema `required: [port]` + 缺 port | exit 4 | 现状 |
| `--on-empty=keep` | exit 2 | 现状 |

