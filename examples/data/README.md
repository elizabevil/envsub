# data/

共享数据集：**不含占位符**的输入，被所有配方复用（`-f` / `--dotenv-file` / `--schema` / `--set-file`）。

| 文件 | 层 | 提供 |
|------|----|------|
| `values-base.yaml` | L3 值文件 | 7 种类型的基础值、算子与 schema 用例的键 |
| `values-prod.yaml` | L3 覆盖 | 同键覆盖、map 深合并、数组替换 |
| `values-extra.json` | L3 JSON | 精确整数、JSON 字面量拼写 |
| `app.env` | L2 `.env` | 进根、高于值文件 |
| `extra.env` | L2 `.env` | 第二个 `.env`：后写胜 |
| `schema.yaml` | schema | A 类关键字全覆盖（默认值/物化/约束/组合/结构） |
| `schema-invalid-data.yaml` | 失败用例 | 触发 schema 校验失败（exit 4） |
| `invalid-syntax.yaml` | 失败用例 | 触发解析失败（exit 2） |

命名规则见 `examples/README.md`：`<类型>_<来源>_<效果>`。
