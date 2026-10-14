> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §10、§12。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 10. 输出形态规则

**本章解决**：编码输出的确定性、引号、转义、缩进、换行与字面量保留。

_列：ID | 规则 | 证据。_

| 编号 | 规则 | 证据 |
|------|------|------|
| D1 | 映射键在所有编码器排序输出，不依赖 Go map 迭代顺序 | jsonv2.go:132-134；sonic.go:20-23；rules_test.go:187-205 |
| D2 | `toJson` 无尾换行；`toYaml` 去掉尾部换行 | pipeline.go:92-106 |
| D3 | YAML 缩进 2 空格；映射键排序；顶层序列 `- a`，映射下嵌套序列跟随 yaml.v3 | yamlx/marshal.go:16,23-34；test/yamlx:119-139 |
| Q1 | 数字样字符串与歧义字符串必须加引号，保证往返类型不变 | rules_test.go:88-89 |
| Q2 | JSON 不转义 HTML：`<` `>` `&` 原样输出 | 实测 `"<a>&b"` 原样 |
| Q3a | 非法 UTF-8 在编解码两侧都拒绝，exit 4（sonic 需后置校验） | jsonv2_test.go:224-241；sonic.go:26-43,47-70 |
| Q3b | 重复成员名：默认后端与 YAML 侧拒绝；`sonic` 为例外（T9） | jsonv2_test.go:204-211；yamlv3.go:88-90 |
| Q4 | 空字符串输出 `""`，不得写成 `null` 或省略（V4） | rules_test.go:88 |
| Q5 | 缺失路径不产生输出，由 `--on-missing` 决定（V3） | engine.go:279-304 |
| Q6 | 未声明小数字面量保留拼写：`1.0`/`1.50`/`1e3` 原样输出，不得丢小数点（F6） | §3.11 ✅ |
| Q7 | `NaN`/`±Inf` 在编码侧也必须报错 exit 4（F8） | §3.10 F8 ✅ |

**案例（输出形态）**

| 规则 | 命令 | 输出 | 标注 |
|------|------|------|------|
| D1 | `--set-json 'x={"z":1,"a":2,"m":3}'` + `\| toJson` | `{"a":2,"m":3,"z":1}` | 现状 |
| D1 | 同上 + `\| toYaml` | `a: 2\nm: 3\nz: 1`（键排序） | 现状 |
| Q1 | `--set 'x=8080'` + `\| toJson` | `"8080"` | 现状 |
| Q2 | `--set-json 'x="<a>&b"'` + `\| toJson` | `"<a>&b"` | 现状 |
| Q3a | JSON 含非法 UTF-8 | exit 4 | 现状 |
| Q3b | JSON 重复成员名 / YAML 重复键 | exit 4 | 现状 |
| D2 | `${x \| toJson}` 与 `${x \| toYaml}` 尾部 | 均无尾换行 | 现状 |
| D3 | `--set-json 'x={"hosts":["a","b"]}'` + `\| toYaml` | `hosts:\n  - a\n  - b` | 现状 |
| Q6 | `--set-json 'x=1.0'` + `\| toJson` | 现状 `1.0` | 现状 |
| Q7 | `go test -run TestFloatRules ./test/encoding/` | 两编码器均拒绝 NaN/+Inf | 现状 |

## 12. 后端等价与已知例外

**本章解决**：默认后端、`sonic` 与 yaml.v3 的行为对照与有意例外。

_列：项 | 标准库 v2 | sonic | yaml.v3 | 规则/状态。_

| 项 | 标准库 `encoding/json/v2`（默认） | `sonic`（`-tags=sonic`） | `go.yaml.in/yaml/v3` | 规则 / 状态 |
|----|-----------------------------------|--------------------------|----------------------|-------------|
| 键排序 | `Deterministic(true)` 排序 | `SortMapKeys` 排序 | 原生排序 | D1 等价 ✅ |
| typed nil map/slice | 库默认 `{}`/`[]` | 归一化为 `{}`/`[]` | 库默认 `{}`/`[]` | V1 等价 ✅ |
| 非法 UTF-8 | 编解码原生拒绝 | 编解码双向后置校验拒绝 | 未显式检查 `[推断]` | Q3a ✅ |
| 重复成员名 | 拒绝 | **last-wins（T9 唯一例外）** | 拒绝 | 已决例外 ✅ |
| 整数精确性 | `json.Number` 全链路 | 同（`UseNumber`） | 节点遍历保留整数 | N1 ✅ |
| 非有限浮点 | `toJson` 报错 | `toJson` 报错 | `toYaml` 报错（不再输出 `.nan`/`.inf`） | F8 ✅ |
| 深度上限 L1 | 约 10000 层，干净报错、不 panic | 同库族 `[推断]` | 约 10000 层 | 采用库行为，`TestDepthLimitIsACleanError` 固化 ✅ |

**案例（后端等价）**

| 命令 | 输出 | 标注 |
|------|------|------|
| `go test -tags=sonic ./test/encoding/` | 与默认后端一致（除重复成员名） | 现状 |
| JSON 文件 `{"a":1,"a":2}` | 默认后端 exit 4；`-tags=sonic` last-wins（G1） | 已决例外 |

### 12.3 Go 1.27 里 `encoding/json`（v1）与 `v2` 的关系

实测（Go 1.27.1，`GOEXPERIMENT` 为空即 jsonv2 默认开启）：

| 观测 | 结果 |
|------|------|
| `go list -f '{{join .GoFiles "\n"}}' encoding/json` | `v2_decode.go v2_encode.go v2_indent.go v2_inject.go v2_options.go v2_scanner.go v2_stream.go`（旧的 `decode.go`/`encode.go`/`scanner.go` 被构建标签排除） |
| `go list -f '{{join .Deps "\n"}}' encoding/json` | 含 `encoding/json/jsontext` 与 `encoding/json/v2` |

结论：**v1 = v1 语义 + v2 引擎**（词法/解析/反射遍历由 v2 提供，`v2_options.go` 把 v1 的既有行为映射上去）。
但它**不是** v2 语义，二者默认值仍不同（实测）：

| 行为 | v1 | v2 |
|------|----|----|
| 重复成员名 | 允许（last-wins） | 拒绝 |
| Marshal 非法 UTF-8 | 替换为 U+FFFD | 报错 |
| map 键序 | 排序 | 保持原样（需 `Deterministic(true)`） |
| `1e999999` 解成 float64 | 报错（文案 `cannot unmarshal number …`） | 报错（文案 `cannot unmarshal JSON number … value out of range`） |

对本项目的三点影响：

1. 任何 import `encoding/json`（我们为 `json.Number` 类型大量使用）都会把 `encoding/json/v2` 与
   `jsontext` 带入依赖图——因此"使用 v2 家族"没有额外体积成本（默认与 `-tags=sonic` 构建均含 v2）。
2. `jsonx` 必须显式传 `Deterministic(true)`：v2 默认不排序 map 键，而规则 D1 要求确定键序。
3. **不得借用 v1 的宽松行为**：重复成员名与非法 UTF-8 在规则 Q3a/Q3b 下必须拒绝，因此所有路径都要
   走显式配置的 v2（默认后端）或做过归一化的 sonic，不能因为"底层是 v2"就假定语义一致。

纯文法校验（如 `codec.validJSONNumber`）使用 `jsontext`：这是两代共享的 tokenizer，结果与 v1/v2
各自的默认值无关。
