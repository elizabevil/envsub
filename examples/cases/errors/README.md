# cases/errors/

每个子目录一个失败路径：`input.tmpl` 是模板，`expect.exit` 是期望退出码（必填），
`expect.err` 是期望出现在 stderr 里的片段（子串比对）。

| 案例 | 触发方式 | 退出码 |
|------|----------|--------|
| `missing/` | 引用一个未设置的变量 | 6 |
| `syntax/` | 未闭合的 `${` | 3 |
| `type/` | `\| type bool` 转换失败 | 4 |
| `schema/` | 值不满足 `schema.yaml` | 4 |
| `io/` | `--set-file` 指向不存在的文件 | 5 |
| `usage/` | 非法 `--on-missing` 取值 | 2 |

`--check` 检出的"输出过期"是第 7 个退出码，由 `just auxiliary-check` 覆盖（没有单独目录）。
