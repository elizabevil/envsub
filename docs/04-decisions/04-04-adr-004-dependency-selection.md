---
status: draft-ai
owner: [ 待指定 ]
version: 0.2
last-updated: 2026-10-08
---

# ADR-004：项目依赖库选型

## 1. 标题

ADR-004：项目依赖库选型

## 2. 状态

Accepted（已确认），来源：需求提出者确认其余推荐项，2026-09-29。`[FACT]`
2026-10-08 修订 JSON 一项：改为 `internal/jsonx` 双后端，默认标准库 `encoding/json/v2`，
`-tags=sonic` 保留 sonic（见第 8 节）。

## 3. 背景

- 技术选型已确认 v0/v1 使用 Go，目标为多平台静态单文件。`[FACT]`（来源：ADR-001）
- Schema 已确认复用 JSON Schema Draft 2020-12 + `santhosh-tekuri/jsonschema/v6` + 薄适配层。`[FACT]`（来源：ADR-003）
- 当前只实现 P0；P1/P2 保留为待办。`[FACT]`
- 依赖选型原则：纯 Go、无 CGO、多平台可编译、可静态链接、维护活跃、API 稳定、尽量少而精。`[INFERENCE]`

## 4. 建议选型

| 领域        | 推荐库/工具                                      | 备选                                | 理由                                                       |
|-------------|--------------------------------------------------|-------------------------------------|------------------------------------------------------------|
| CLI         | `github.com/spf13/cobra`（已确认）               | `alecthomas/kong`、标准 `flag`      | 成熟、子命令/help/completion 支持好，纯 Go                 |
| YAML 编解码 | `internal/yamlx`，**单一库 `go.yaml.in/yaml/v3`**（解码与 `toYaml` 编码；2026-10-08 二次修订） | `goccy/go-yaml`、`sigs.k8s.io/yaml` | 解码保留整数与引号字符串、查重、支持合并键；编码复用同一库，体积更小（见第 9、10 节） |
| JSON        | `internal/jsonx` 双后端（默认标准库 `encoding/json/v2`；`-tags=sonic` 切 sonic）（2026-10-08 修订） | 纯 sonic 或纯标准库 | 统一入口 + build tag；默认体积优先，需要吞吐时切 sonic |
| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6`       | `kaptinlin/jsonschema`              | 成熟、Draft 2020-12 覆盖好；已确认                         |
| `.env` 解析 | 自研最小解析器（已确认）                         | `github.com/joho/godotenv`          | 需要精确控制 JSON 数组规则、行号与错误信息                 |
| Glob / 批量 | `github.com/bmatcuk/doublestar/v4`               | 标准 `filepath.Glob`                | 支持 `**` 递归 glob，便于批量渲染                          |
| TOML（P1）  | `github.com/pelletier/go-toml/v2`                | `github.com/BurntSushi/toml`        | P1 再引入，避免 P0 膨胀                                    |
| Diff（P1）  | `github.com/hexops/gotextdiff`                   | `github.com/sergi/go-diff`          | P1 的 dry-run/diff/check 使用，P0 不引入                   |
| 测试断言    | 标准 `testing` + `github.com/google/go-cmp`      | `stretchr/testify`                  | 标准库为主，go-cmp 仅做结构 diff                           |
| 日志        | 标准库 `log/slog`                                | -                                   | 零依赖，结构化日志                                         |
| 构建发布    | GitHub Actions（已确认）；可选 GoReleaser action | 自研脚本                            | 多架构静态归档、checksums、发布自动化                      |
| 安全扫描    | `govulncheck`、`syft`、`cosign`                  | -                                   | 供应链基础；工具非运行时依赖                               |
| Lint        | `golangci-lint`                                  | -                                   | 统一静态检查；工具非运行时依赖                             |
| 测试        | 测试矩阵 + TDD + golden fixtures（已确认）       | 先实现后补测试                      | 规则已定义，测试先行可减少返工                             |

## 5. 被拒绝的方案

- **只用标准库 `flag` 做 CLI：拒绝。** P1/P2 会有更多子命令与 completion 需求，标准库后期维护成本更高。`[INFERENCE]`
- **使用 `gopkg.in/yaml.v3` 直接解码：拒绝。** 它更偏 YAML 原生类型，需额外处理 JSON 兼容性；`sigs.k8s.io/yaml` 更贴近 JSON
  Schema，且已确认采用。`[FACT]`
- **使用 `joho/godotenv` 解析 `.env`：本次拒绝。** 它对变量展开、行号、错误细节的控制不足；我们的 JSON 数组规则需要精确实现。
  `[INFERENCE]`
- **P0 引入 TOML / Diff 库：拒绝。** 当前只实现 P0，P1/P2 再引入可减少依赖和测试面。`[FACT]`
- **JSON 只使用标准库：拒绝。** 需求提出者确认使用 `github.com/bytedance/sonic`。`[FACT]`
  （2026-10-08 修订：默认构建改为标准库 `encoding/json/v2`，sonic 通过 `-tags=sonic` 保留，
  原因与实测见第 8 节。）

## 6. 后果

- 正面：核心依赖少而明确；CLI、YAML、Schema、测试分层清晰；P0 可以快速落地。`[INFERENCE]`
- 负面：Cobra 会增加一定二进制体积；自研 `.env` 解析器需要额外测试与维护。`[INFERENCE]`
- 现状：YAML 归一化边界由 `test/yamlx/` 的等价性/差异断言与基准钉住；`.env` 解析器由
  `internal/source` 单测与 fuzz 目标覆盖（`test/fuzz/`，4 个目标）；Cobra 升级策略**不作承诺**（见第 7 节）。
  `[FACT]`

## 7. 待确认问题

- **待办**：`sonic` 后端的多平台**运行时**兼容性（Windows/macOS/arm64 实机运行）尚未验证。
  交叉编译已在 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 通过（2026-10-08）。
  触发条件：有实机或 CI 矩阵 runner 时执行；owner：技术负责人。`[TODO]`
- 发布流程已落地（未引入 GoReleaser）。`[FACT]` owner：技术负责人。
  （2026-10-08：已用 GitHub Actions 落地，未引入 GoReleaser——`release.yml` 矩阵构建 6 个目标
  （2026-10-21 起加 windows/arm64），
  `supply-chain` job 产出 `SHA256SUMS` 与 SPDX SBOM，tag 推送创建 draft release；
  checksums/SBOM 由 `anchore/sbom-action` 与 `sha256sum` 负责，见 05-01「供应链基础」。）

## 8. 修订：JSON 改为双后端（2026-10-08）

### 8.1 动机

体积测量（`just size`）显示 8.62 MiB 的二进制中 sonic 家族占比最大：它连带
`golang-asm` 的全架构汇编器（x86/arm64/ppc64/s390x/wasm/riscv）、JIT 解码器、
native kernel 与 cpuid/x-arch。编译期开关实测无法替代依赖裁剪：

| 方案 | 体积 | 代价 |
|------|------|------|
| baseline | 8.62 MiB | - |
| `-gcflags=all=-l`（关内联） | 7.77 MiB | 大模板渲染 49 ms → 73 ms（慢 48%） |
| `-tags=noasm` | 8.55 MiB | sonic 走 Go 回退 |
| `CGO_ENABLED=0`、`GOEXPERIMENT=nodwarf5/jsonv2/simd` | 8.62 MiB | 无变化 |

### 8.2 决策

- 抽出统一 JSON 入口 `internal/jsonx`（`Unmarshal`、`UnmarshalValue`、`Marshal`、
  `MarshalIndent`），用 build tag 选择后端：默认标准库 `encoding/json/v2`，
  `-tags=sonic` 使用 `github.com/bytedance/sonic`。`[FACT]`
- 默认构建改为标准库后端（体积优先）；`just build-sonic`、`just size sonic` 提供
  sonic 变体。`[FACT]`

### 8.3 实现要点

- v2 对 `any` 默认把数字解成 `float64`（2^53 以上丢精度），且**没有 `UseNumber` 开关**（那是 v1
  `Decoder` 的方法）。标准库后端因此用 v2 自己的 `Unmarshal` 加其文档化的
  `WithUnmarshalers` 扩展点：用类型化 unmarshaler 把数字落成 `json.Number`，容器仍由 v2 解析
  （带上同一 hook 递归），**不自建解析层**；具体类型直接交给 v2（对具体整数本就精确）。`[FACT]`
- `json.Number` 在 v2 下仍按 JSON 数字输出（`8080`、`1.50` 原样保留）。`[FACT]`
- 标准库后端拒绝顶层值之后的尾随内容，与 v1/v2 `Unmarshal` 一致。`[FACT]`
- 被拒绝的备选：单遍 `jsontext` token 遍历（自建树）。它更快——同一负载 3.55 ms vs 12.04 ms
  （3.4×）——但属于"自行封装一层解析逻辑"，与"优先使用标准库 API"的规则冲突，故**不作生产
  实现**；该实现保留在 `test/jsonx/walker_test.go`，与生产实现做功能等价与性能对照，
  数据与复现命令见 [06-06 JSON 数字精度核查](../99-archive/06-review/06-06-json-number-precision.md)。`[FACT]`

### 8.4 实测（2026-10-08，本机）

| 指标 | 默认（stdlib v2） | `-tags=sonic` |
|------|------------------|---------------|
| 二进制 linux/amd64 | **4.92 MiB** | 8.43 MiB |
| 二进制 linux/arm64 | 4.75 MiB | 5.50 MiB |
| 启动（`version`，30 次均值） | **4.6 ms** | 9.5 ms |
| REF-04 模板峰值 RSS | **1.30× GNU** | 1.80–2.25× GNU |
| JSON 解码 364 KB 到 any（`test/jsonx`） | 12.04 ms（30.97 MB/s） | **2.45 ms**（快 4.9×） |
| JSON 编码 500 KB | 2.68 ms | **1.44 ms**（快 1.86×） |

两个后端的 18 个测试包（含 160 fixture、18 E2E、REF-04 性能门禁、`test/jsonx` 对照）均通过。`[FACT]`

体积的按包归因与可选优化（含与外部对照物 a8m/envsubst 的同口径比较）见
[06-07 二进制体积归因与优化选项](../99-archive/06-review/06-07-binary-size-attribution.md)；
复现命令 `just size-deps`。`[FACT]`

### 8.5 两后端的行为差异

统一入口 `internal/jsonx` 把"必须一致"的部分固定成契约（有测试），把"有意不同"的部分
显式记录（每个后端各有一条测试固化）。`[FACT]`

| 行为 | 默认（stdlib v2） | `-tags=sonic` | 处理 |
|------|------------------|---------------|------|
| 对象键顺序 | 排序（`Deterministic(true)`） | 排序（`SortMapKeys: true`） | 统一。两个后端的默认配置都不排序，`toJson` 输出会随 Go map 迭代顺序抖动——这是切换前就存在的缺陷，本次一并修掉 |
| nil 切片 / nil map | `{}` / `[]`（库默认） | `{}` / `[]`（sonic 归一化） | 统一为库默认，不再输出 `null`（§V1，2026-10-08 实现） |
| HTML 字符 `< > &` | 不转义 | 不转义 | 本来就一致（v1 会转义，两者都不转义） |
| 数字拼写 | `json.Number` 原样（`1.50`、`1e21`） | 同 | 统一 |
| 尾随内容 | 报错（标准库） | 报错（sonic 语法错误） | 一致，仅错误文本不同 |
| 形状不匹配 | 标准库文案（`cannot/unable to unmarshal ...`） | sonic 文案 | 有意不统一：诊断只保证"有错误 + 退出码"，不承诺解析错误文本 |
| **重复成员名** | **报错**（`duplicate object member name`） | 后者覆盖前者 | 有意保留，且已定为 **sonic 唯一例外**（序列化规则 Q3b/T9，2026-10-08 决定，见 docs/07-serialization/00-index.md）：与 `yaml.v3`（已拒绝重复键）一致；需放宽可用 `jsontext.AllowDuplicateNames(true)` |
| **非法 UTF-8** | **报错** | **报错**（后置校验；sonic 的 `ValidateString` 会把非法字节替换成 U+FFFD，属静默篡改，故不启用） | 已统一（§Q3a，2026-10-08 实现） |

对应的测试：契约类在 `internal/jsonx/jsonx_test.go`（两个后端都跑），
后端差异分别在 `jsonv2_test.go`（`//go:build !sonic`）与 `sonic_test.go`（`//go:build sonic`），
后两者各自演示了放宽选项；生产实现与 token 遍历备选的功能/性能对照在 `test/jsonx/`。

### 8.6 后果

- 正面：默认二进制缩小 41%，启动与峰值内存同时改善；JSON 后端成为可替换的实现细节；
  解码路径只使用标准库 API（`Unmarshal` + `WithUnmarshalers`），没有自建解析层。`[FACT]`
- 负面：默认 JSON 吞吐降低约 4.9×（相对 sonic；配置尺寸负载的绝对值仍在毫秒级）。
  这是"优先标准库 API / 少自建代码"的自觉取舍，数据见 06-06；需要吞吐时用 `-tags=sonic`。`[INFERENCE]`
- 备注：sonic 的 `compat.go` 构建约束为 `go1.28 || !amd64 || ...`，即升级 Go 1.28 后
  sonic 后端自身也会退化为标准库实现。`[FACT]`

## 9. 编解码层布局（2026-10-08）

格式相关的库选择集中到两个包，每个库一个文件并配同名测试，测试输入一律落盘
`testdata/`：

| 包 | 文件 | 库 | 职责 |
|----|------|----|------|
| `internal/jsonx` | `jsonv2.go`（默认，`Unmarshal` + `WithUnmarshalers`）、`sonic.go`（`-tags=sonic`） | `encoding/json/v2`、`github.com/bytedance/sonic` | JSON 编解码（见第 8 节） |
| `internal/yamlx` | `yamlv3.go`（解码）、`marshal.go`（编码） | `go.yaml.in/yaml/v3` | YAML 解码与 `toYaml` 编码，单一库（见第 10 节） |
| `internal/codec` | `exact.go` | 无（叶子包） | 超出 `uint64` 的精确整数类型 `ExactInt`，被 `values`/`types`/`jsonx`/`yamlx`/`schema` 复用（§T2） |

测试与 fixture：

- `internal/jsonx/testdata/`：`numbers`、`nested`、`big_integers`、`array`、
  `duplicate_members`、`invalid_utf8`（原始字节）、`trailing`、`invalid/*`、
  `marshal/*`（含缩进 golden）。`jsonx_test.go` 是两后端共享契约；`jsonv2_test.go`、
  `sonic_test.go` 分别断言两后端有意的差异。`[FACT]`
- `test/jsonx/`：被拒绝的 token 遍历备选实现（`walker_test.go`）与生产实现的功能等价、
  大整数精度、性能对照（`compare_test.go`），数据落在 06-06。`[FACT]`
- `internal/yamlx/testdata/decode/*.yaml`、`testdata/encode/*.yaml`（含 golden）；
  `yamlv3_test.go`、`marshal_test.go` 分别覆盖解码与编码。`[FACT]`
- `test/yamlx/`：被否决的 `sigs.k8s.io/yaml` 编码器（`sigs_test.go`）与生产编码器的功能等价、
  已知风格差异与性能对照（`compare_test.go`）；该库仅为测试依赖，不进二进制。`[FACT]`

集中过程中修掉两个 YAML 缺陷：

1. **重复映射键**：yaml.Node 模式不查重（只有解码进 map 才查），原来 YAML values
   文件的重复键会静默后者覆盖。现在 `yamlx.Decode` 显式拒绝，与 JSON(v2)、
   `.env` 加载器一致。`[FACT]`
2. **合并键 `<<: *anchor` 未实现**：node 遍历把它当普通键名，合并值丢失。现在按
   yaml.v3 语义合并（显式键优先、靠前来源优先），锚点/别名/合并均有测试。`[FACT]`

编号保真度：YAML 走 yaml.v3，超过 int64 的整数变成 `uint64`，超过 `uint64`
退化为 `float64`（yaml.v3 的既有行为）；JSON 侧由 `json.Number` 保精度。values 层
（`internal/values`、`internal/types`）已统一为：int64 内 → `int`，uint64 内 →
`uint64`，更大 → 保留原始 `json.Number` 文本，**任何情况都不再降级为 `float64`**
（2026-10-08 修复；见 `internal/values/number_test.go`、`internal/types/number_test.go`
与 e2e `set_json_big_int`，示例见 `examples/data/values-extra.json`）。`[FACT]`

## 10. 修订：YAML 编码改用 yaml.v3（2026-10-08）

### 10.1 背景

`internal/yamlx` 原本一个方向一个库：解码 `go.yaml.in/yaml/v3`（保整数、查重、合并键），
编码 `sigs.k8s.io/yaml`（内部是 `encoding/json` + `go.yaml.in/yaml/v2` 的 JSON 往返，输出
K8s 风格）。前者不可替换：sigs 的解码会把 2^53 以上整数 float 化并丢失重复键检查。`[FACT]`

### 10.2 决策

编码同样改用 `go.yaml.in/yaml/v3`，全仓只保留一个 YAML 库。`[FACT]`

- 数字：`json.Number` 编码前转成 int64 / uint64 / float64（超出 uint64 保留精确文本），
  不会被 yaml.v3 当作字符串加引号。`[FACT]`
- 类型化 nil map/slice 先转成 untyped nil，输出 `null`，与 `toJson` 的
  `FormatNilMapAsNull`/`FormatNilSliceAsNull` 语义一致。`[FACT]`
- `SetIndent(2)`，与值文件风格一致。`[FACT]`

### 10.3 唯一可见差异

yaml.v3 把序列缩进到键下方，sigs 保持父级缩进：

```yaml
# yaml.v3          # sigs.k8s.io/yaml
hosts:             hosts:
  - a              - a
  - b              - b
```

两者语义相同：`test/yamlx` 用 20 组语料（含 nil、类型化 nil、数字、嵌套、序列、多行、
Unicode）逐一编码后再解码比对，树完全相等。`[FACT]`

### 10.4 收益与代价

| 项 | 值 |
|----|----|
| 二进制 | 5.35 MB → **5.16 MB**（−184 KB，−3.6%）；`just size` 5.10 → **4.92 MiB** |
| 依赖 | 移除 `sigs.k8s.io/yaml` 与 `go.yaml.in/yaml/v2`；二进制依赖数为 0（该库仅作测试依赖保留） |
| 编码性能 | 同负载 yaml.v3 **21.99 ms** vs sigs 23.94 ms（yaml.v3 略快） |
| 代价 | 序列缩进风格变化；更新 2 个 golden（`internal/yamlx/testdata/encode/mapping.want.yaml`、`test/e2e/testdata/values_schema/expect.out`）与 1 条断言 |

### 10.5 备选与验证

- 备选（否决）：保留 sigs 编码。多带一个 yaml.v2 与 JSON 往返路径，换来的只是序列缩进风格。`[FACT]`
- 备选（否决）：只用 sigs。解码会 float 化大整数、丢失重复键检查，与"整数保持整数"契约冲突。`[FACT]`
- 对照实验落盘在 `test/yamlx/`：功能等价 + 已知差异断言 + 计时报告 + 两个基准；
  复现 `go test -v ./test/yamlx/`、`go test -bench=. ./test/yamlx/`。`[FACT]`
- 超出 uint64 的整数在两种格式下的取直（JSON 裸数字、YAML 当前为带引号字符串）以及
  `nil`/空集合语义：规则已定案（精确整数 N1；typed nil 遵循库默认 `{}`/`[]`），规范性文本见
  [序列化规则](../07-serialization/00-index.md)（方案选型记录见其附录 A）。
  实现见 [序列化规则第 14 节](../07-serialization/00-index.md)（两个命名空间、根/`@process` 读取规则），
  测试见 `13-materialization.md` §14 与 e2e fixture。`[FACT]`

## 非目标（Non-Goals）

- 本 ADR 不定义功能需求。`[ASSUMPTION]`
- 不在 P0 引入 P1/P2 依赖。`[FACT]`
- 不承诺依赖版本升级策略；若将来需要承诺，另开 ADR。`[FACT]`
