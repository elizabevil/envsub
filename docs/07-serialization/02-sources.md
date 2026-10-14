> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §4。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 4. 值来源与解码规则

**本章解决**：各值来源（文件、CLI、环境、schema 默认）的类型与整数范围规则，并给出每类来源的
可运行案例。CLI 的行为规则（值来源档位、优先级、交互）见 §6。

### 4.1 通用整数阶梯（N5）

所有整数来源必须走同一把尺子：**取能精确表示该值的最窄类型**——不是"类型限制"，而是
"最窄可精确表示"：`int64` 内（`2^63-1`，9223372036854775807）→ `int`；`uint64` 内
（`2^64-1`，18446744073709551615）→ `uint64`；超出 → 精确整数（任意精度十进制文本）。
**禁止**四舍五入，**禁止**降级为 `float64`（N4/N5）。

### 4.2 来源表与案例

**来源优先级（先命中者胜）**：`--set`/`--set-json` > YAML/JSON 值文件 > `.env` > 进程环境 > schema `default`
（`default` 为兜底）。完整规则见 [`04-cli.md` §6.2](04-cli.md)。`[FACT]`（来源：需求提出者确认，2026-10-10）

_列：来源 | 类型与范围规则 | 证据 / 测试 | 案例。_

| 来源 | 类型与范围规则 | 证据 / 测试 | 案例（现状） |
|------|----------------|-------------|--------------|
| JSON 值文件 | `jsonx.UnmarshalValue` → `json.Number` → `NormalizeNumbers` 逐级；重复成员名拒绝 | json.go:18-31；§6.1 C3 | `echo '{"id":18446744073709551616}' > v.json; envsub render -f v.json <(printf '${id}')` → `18446744073709551616` |
| `--set-json` | 与 JSON 值文件同一条解析路径 → 精确类型树 | set.go:108-113 | `envsub render --set-json 'x=18446744073709551616' <(printf '${x \| toJson}')` → `18446744073709551616` |
| YAML 值文件 | `yamlx.Decode` 节点遍历；`int`/`uint64`/精确整数；重复键拒绝；`<<` 合并 | yamlv3.go:16-22,70-105 | `printf 'x: 18446744073709551616\n' > v.yaml; envsub render -f v.yaml <(printf '${x}')` → 精确十进制 |
| `.env`（`--dotenv-file`） | 全部为字符串；按行扫描；重复键报错；双引号解码 `\n\r\t`。**格式由 flag 强制**，与文件名无关（`.env.local`/无扩展名均可）。写入**根命名空间**（层序：`.env` > `-f`） | dotenv.go:51-62,88-90,139-171；loader.go `DotenvLoader`；`06-15` §2.2 | `printf 'BIG=18446744073709551615\n' > .env; envsub render --dotenv-file .env <(printf '${BIG \| type int}')` → 精确十进制 |
| `--set` | **存原始字符串**；类型只由 schema 声明或模板 `\| type xxx` 决定 | set.go:114-118；§6.1 C1 | `envsub render --set 'x=8080' <(printf '${x} ${x \| toJson}')` → `8080 "8080"` |
| `--set` | 存原始字符串；类型由 schema 或 `\| type` 决定 | set.go:104-110；§6.1 | `envsub render --set 'x=8080' <(printf '${x \| toJson}')` → `"8080"` |
| schema `default` | **兜底**：仅当该 key 在根命名空间与环境命名空间都缺失时填入（`CloneValue` 原样写入，随后由 `Materialize` 按声明类型转换字符串） | defaults.go；materialize.go；§13.1 G21 | 见 §9.2 SC6 案例 |
| `${A:=字面量}` / `${A=字面量}` 写回 | 写回的是**渲染后的字符串**，不做类型转换（T5） | engine.go:267-276 | `envsub render <(printf 'a=${A:=8080} j=${A \| toJson}')` → `a=8080 j="8080"` |
| 数组下标赋值 | 禁止：赋值路径不接受 `a[0]`，读取路径支持 `${a[0]}` | set.go:130-132 vs path.go:115-148 | `envsub render --set 'a[0]=1' <(printf 'x')` → exit 2（**待定**） |

**来源边界**：`-f/--values-file` 只接受 YAML/JSON 值文件并合并进**根命名空间**；`.env` 只能通过
**`--dotenv-file`**提供，同样合并进**根命名空间**（层序高于 `-f`）——
把 `.env` 传给 `--values-file` 是 `KindUsage` exit 2（见 §4.2 表下案例与
`04-cli.md` §6.1）。反过来，`--dotenv-file` **按 flag 语义强制 dotenv**，文件名任意
（`.env`/`debug.env`/`.env.local`/无扩展名均可；fixture `env_file_nonstandard`）。`[FACT]`

可复制的来源案例脚本：

```bash
# JSON 文件：超 uint64 精确
echo '{"id":18446744073709551616}' > /tmp/v.json
./dist/envsub render -f /tmp/v.json <(printf '${id}\n')          # 18446744073709551616

# YAML 文件：超 uint64 精确（= 2^64，18446744073709551616）
printf 'x: 18446744073709551616\n' > /tmp/v.yaml
./dist/envsub render -f /tmp/v.yaml <(printf '${x}\n')           # 18446744073709551616

# .env + 类型物化（= 2^64-1，18446744073709551615）
printf 'BIG=18446744073709551615\n' > /tmp/v.env
./dist/envsub render --dotenv-file /tmp/v.env <(printf '${BIG | type int}\n')

# --set / --set-json 的字符串 vs 精确类型
./dist/envsub render --set 'x=8080'      <(printf '${x | toJson}\n')   # "8080"
./dist/envsub render --set-json 'x=8080' <(printf '${x | toJson}\n')   # 8080
```

> 哪些来源需要物化、为什么，见 [§17 物化模型](13-materialization.md)。
