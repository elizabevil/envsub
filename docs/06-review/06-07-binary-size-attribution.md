---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-08
---

# 06-07 二进制体积归因与优化选项（Round 6）

## 1. 结论摘要

- 与外部对照物 `a8m/envsubst v1.4.3` 的差距**不是编译技巧**，而是功能集：对照物只有模板展开，
  没有 YAML/JSON 值文件、没有 JSON Schema、没有管道 formatter、没有 cobra。`[FACT]`
- 同口径（都 strip）：对照物 2.54 MB → **1.66 MB**；我们 **5.16 MB**（`-trimpath -ldflags "-s -w"`，
  默认标准库 JSON 后端）；go1.27 的 hello world 同参数 = **1.22 MB**（运行时与标准库地板）。`[FACT]`
- 我们比对照物多的 3.5 MB 中，可归因的部分：schema 栈 502 KB、YAML 栈 194 KB、
  JSON 栈 273 KB、cobra+pflag 83 KB、自有代码 307 KB（.text 符号口径）；
  元数据（`.gopclntab`/`.go.type`/`.rodata` 等）随代码量放大，乘数约 **1.7×**。`[FACT]`
- **要到 2.5 MB 只能做「最小构建」**：放弃 schema 校验 + YAML 来源 + cobra（并关闭内联）。
  保持全部功能的现实下限约 4.5 MB（去 schema 栈）或 4.8 MB（去 YAML 栈）。`[INFERENCE]`

## 2. 同口径对比

| 项 | 对照 a8m/envsubst v1.4.3 | envsub（本仓） |
|----|--------------------------|----------------|
| 文件大小 | 2.54 MB（**未 strip**） | 5.16 MB（已 strip） |
| strip 后 | 1.66 MB | 5.16 MB（本就是 strip 产物） |
| Go 版本 / CGO | go1.24.0 / CGO_ENABLED=1 | go1.27.1 / CGO_ENABLED=0 |
| 功能集 | 仅模板展开（shell 风格参数展开） | + YAML/JSON 值文件、JSON Schema 校验、管道 formatter、cobra CLI、批量/原地输出 |
| hello world 同参数 | 1.22 MB（同版本地板） | 1.22 MB |

## 3. 构成（section 口径，`size -A -d`）

| section | envsub | 对照（strip 后） |
|---------|--------|------------------|
| `.text` | 2,422,225 | 704,593 |
| `.gopclntab` | 1,849,678 | 543,248 |
| `.noptrdata` | 115,650 | 26,178 |
| `.rodata` | 237,954 | 343,351 |
| `.data` | 53,202 | 21,682 |
| `.go.type` | 461,352 | -（对照为 `.typelink` 2,100） |

`.gopclntab`（1.9 MB）是 panic/回溯所需的函数元数据，随函数数量增长，**无法 strip**；
这也是"删依赖"能同时缩小 `.text` 与元数据、收益被放大到约 1.7× 的原因。`[FACT]`

## 4. 按包归因（`just size-deps`，.text 已命名符号）

```text
    701.8 KB  runtime                 （地板）
    330.9 KB  go:func                 （编译器生成的函数值/包装）
    272.8 KB  encoding/json*          （v1 json.Number + v2 + jsontext）
    160.0 KB  github.com/elizabevil/envsub
    154.6 KB  github.com/santhosh-tekuri/jsonschema
    137.4 KB  golang.org/x/text
     89.1 KB  slices
     86.2 KB  reflect
     74.3 KB  math/big
     69.3 KB  time
     61.4 KB  regexp/syntax
     40.2 KB  fmt
     35.5 KB  regexp
     22.6 KB  text/template
     16.2 KB  net/url
--- feature stacks ---
schema stack               502.1 KB
yaml stack                 194.0 KB
json stack                 272.8 KB
cobra + pflag               82.8 KB
our code                   306.8 KB
file size              5.16 MB
```

已命名符号合计 2.87 MB，文件 5.16 MB → 元数据乘数 **1.80**。`[FACT]`

## 5. 优化选项（按收益/风险排序）

| 选项 | .text 收益 | 折合文件（×1.7） | 代价 | 结论 |
|------|-----------|------------------|------|------|
| 移除 schema 栈（jsonschema/v6 + x/text + regexp + math/big + text/template + net/url） | 502 KB | **≈0.85 MB** | 放弃 Draft 2020-12 校验、默认值注入与 unknown-keyword 策略（ADR-003 的核心决策） | 需产品决策；可做 `-tags=noschema` 变体 |
| 移除 YAML 栈（现仅 yaml.v3） | 194 KB | ≈0.35 MB | 放弃 YAML 值文件与 `toYaml` | 需产品决策 |
| `encoding/json/v2` → v1（`Decoder.UseNumber`） | ≈120 KB | ≈0.20 MB | JSON 解码 2.3× 慢（实测 4.73 → 10.9 ms/364KB） | 不划算 |
| cobra → 标准库 `flag` | 83 KB | ≈0.14 MB | 失去 help/completion 与 P1 子命令便利 | 收益小，不建议 |
| `-gcflags=all=-l`（关闭内联） | - | **-0.41 MB（-7.7%）** | 大模板渲染 49 ms → 73 ms（+48%） | 已否决 |
| `-buildid=`、`GOEXPERIMENT=nodwarf5/jsonv2/simd`、`-tags=noasm` | 0 | ≈0 | 无 | 已验证无效 |
| UPX 压缩 | - | 约 -50% | 非编译期产物、启动变慢、杀软误报、不再是普通 ELF | 不建议 |

组合上限：去 schema + 去 YAML + 去 cobra + 关内联 ≈ **3.4 MB**；**2.5 MB 只有"最小构建"
（不含 schema/YAML/cobra）才可能达到**——此时地板 1.22 MB + 自有代码 + JSON 栈 + 元数据
≈ 2.3–2.6 MB。`[INFERENCE]`

## 5.1 与序列化规范的关系

体积优化过程中发现的"同一数值在 JSON 与 YAML 下类型不同（超出 uint64 时 YAML 输出带引号字符串）"
属于规范问题而非体积问题，规范与分析见
[序列化规则](../07-serialization/00-index.md)。`[FACT]`

## 6. 建议

1. **若目标是为用户提供 2.5 MB 级轻量二进制**：新增 `-tags=minimal` 变体（保留 `.env`/JSON 值
   来源、模板语法与管道 formatter；去掉 schema 校验、YAML 与 cobra），release 同时产出
   `standard` 与 `minimal` 两个变体，并在 README 说明能力差异。`[INFERENCE]`
2. **若必须保持全部功能**：接受 ~5 MB；可选的单点收益是自研最小校验器（替换 jsonschema/v6，
   省 ≈0.85 MB，但要自己维护 JSON Schema 子集与 conformance 测试，ADR-003 已选择复用现成库）。
3. **不建议**：关闭内联（性能换体积）、UPX（启动与信任成本）、为省 0.14 MB 去掉 cobra。`[INFERENCE]`

## 7. 复现

```bash
just size          # 本地构建 + 总体积与最大 section
just size-deps     # 按包与按功能栈的 .text 归因（本文件第 4 节）
just size sonic    # 对照：-tags=sonic 的体积（约 8.6 MB）
```

外部对照物 `a8m/envsubst v1.4.3` 不在本仓，如需复核：

```bash
go version -m <envsubst-linux-amd64>     # path/mod/GOOS/GOARCH 与 CGO 标志
cp <binary> /tmp/ref && strip --strip-all /tmp/ref && ls -l /tmp/ref
```

## 非目标（本部分）

- 本文不决定是否放弃 schema 校验或 YAML 支持；那是产品决策，需 owner 确认。`[TODO]`
- 不评估运行期内存与启动（见 REF-04 与 `just test-perf`）。`[FACT]`
- 不承诺与对照物相同的体积，两者功能集不同。`[FACT]`
