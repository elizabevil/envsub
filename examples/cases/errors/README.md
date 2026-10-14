# cases/errors/

每个子目录一个失败路径：`input.tmpl` 是模板，`expect.exit` 是期望退出码（必填），
`expect.err` 是期望出现在 stderr 里的片段（子串比对）。可选 `args` 文件提供该案例
需要的额外参数（每行一个，`${CASE_DIR}` 展开为案例目录），用于共享旗标集表达不了的形状。

| 案例 | 触发方式 | 退出码 |
|------|----------|--------|
| `missing/` | 引用一个未设置的变量 | 6 |
| `syntax/` | 未闭合的 `${` | 3 |
| `type/` | `\| type bool` 转换失败 | 4 |
| `schema/` | 值不满足 `schema.yaml` | 4 |
| `io/` | `--set-file` 指向不存在的文件 | 5 |
| `usage/` | 非法 `--on-missing` 取值 | 2 |
| `outdated/` | `--check` 的目标存在但与本次渲染不同（`args` 给出 `-o current.txt`） | 7 |
| `check_missing/` | `--check` 的目标不存在（同样算过期） | 7 |

七个退出码（2–7）在这里全部有断言；`just auxiliary-check` 另外演示了正常路径下
"树过期"时的可读输出。
