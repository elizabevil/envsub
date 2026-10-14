# templates/

**含占位符**的模板（`${...}`、`${@process.X}`、`$NAME`）。它们被多个配方复用，因此不放进案例目录。

每个模板的期望产物与它同目录、同前缀：

| 模板 | 期望产物 | 附加契约 |
|------|----------|----------|
| `scenarios-json.tmpl` | `scenarios-json.golden` | `scenarios-json.parse` = `json`：产物必须是合法 JSON |
| `scenarios-yaml.tmpl` | `scenarios-yaml.golden` | `scenarios-yaml.parse` = `yaml`：产物必须是合法 YAML |
| `precedence.tmpl` | `precedence.golden` | — |
| `realworld.tmpl` | `realworld.golden` | `realworld.parse` = `yaml`：产物必须是合法 YAML |
| `unused.tmpl` | `unused.golden` | 另外用 `--report=unused` 与 `unused.unused` 比对 |
| `stdin.tmpl` | `stdin.golden` | 经 stdin 传入 |

产物里的文件路径不同机器一致（全部相对仓库根），因此 golden 可提交。

`.parse` 是"产物必须是某格式"的**唯一来源**：配方用 `cat <name>.parse` 取格式，
`test/examples` 也读同一个文件（并强制每个声称格式的模板都有标记）。
