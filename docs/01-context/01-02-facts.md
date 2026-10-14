---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 上下文摘要 · 02 已明确的事实

> 本文是 `01-00-index.md` 的拆分部分。

## 3. 已明确的事实

- 工具定位为 `a8m/envsubst` 升级版，支持 amd64/arm64、多平台单体二进制、尽量静态编译。`[FACT]`（来源：需求提出者目标描述）
- 用户明确需要：默认值、数组表示、转义格式、类型校验、文件配置、多文件配置。`[FACT]`（来源：需求提出者补充）
- 需求提出者授权 AI 补充缺口，并允许重新选择方案；同时要求参考 helmfile 类渲染工具。`[FACT]`（来源：需求提出者本轮补充）
- 已确认用户优先级：平台/DevOps > CI 用户 > 容器维护者。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 已确认类型校验为 P0。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 已确认多文件覆盖规则：后加载覆盖先加载，数组默认替换，CLI `--set` 最高。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 兼容策略已确认：一期只实现 extended 原生模式；二期再实现 GNU/a8m/drone 兼容与转换命令。`[FACT]`（来源：需求提出者确认，2026-09-29；详见
  `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`）
- 已确认支持 Helm-like pipeline formatter，包含 `join`/`toJson`/`toYaml`/`quote`/`indent`；条件/循环后置
  P1；控制指令单独成行；object 遍历按键排序。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 功能优先级已确认：P0 实现核心功能；P1 实现 include；P2 实现 GNU/a8m/drone 兼容/转换。`[FACT]`（来源：需求提出者确认，2026-09-29）
- include/partials 在 P1 实现；执行时指定允许文件/目录，最大嵌套深度候选 3 层，循环检测、白名单与错误格式 P1 细化。`[FACT]`
  （来源：需求提出者确认，2026-09-29；细节仍为 `[ASSUMPTION]`）
- `--unset=zero` 已确认加入：按 schema 类型填充 Go 零值；required 字段不自动填零，直接报错，安全大于约定。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- 已确认采用内联 `| type`，如 `${PORT | type int}`；schema 与内联取配置并集，值必须同时满足；schema 的
  required/default/约束优先级高于内联；交集冲突直接报错。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 技术选型已确认：v0/v1 继续使用 Go，`go.mod` 使用 Go 1.27.1。`[FACT]`（来源：需求提出者确认，2026-09-29；详见 ADR-001）
- 代码规范已确认：严格遵循 Google Go Style Guide；规范见 `docs/03-design/03-10-go-code-standards.md`。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- 依赖选型已确认：JSON 使用 `github.com/bytedance/sonic`，YAML 使用 `sigs.k8s.io/yaml`。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 发布方式已确认：GitHub Actions；提供 `justfile` 快捷编译。`[FACT]`（来源：需求提出者确认，2026-09-29）
- CLI 设计独立成文：`docs/03-design/03-08-cli-design.md`；使用 Cobra。`[FACT]`（来源：需求提出者确认，2026-09-29）
- P2 兼容模式通过 Engine 接口与 Registry 接入，详见 `docs/03-design/03-09-extension-and-compat-architecture.md`。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- 测试策略已确认：测试先行，先建立 P0 测试矩阵与 golden fixtures。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 非功能基线已确认：采用常规二进制优化即可。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 性能目标参考 GNU envsubst：同基准下同量级；具体基线见 `docs/reference/REF-04-gnu-envsubst-performance-baseline.md`。
  `[FACT]`（来源：需求提出者确认，2026-09-29）
- `| type` 规则已确认：如果提供，必须作为 pipeline 第一个操作符；可以缺省。`[FACT]`（来源：需求提出者确认，2026-09-29）
- schema 方案已确认：直接复用 JSON Schema Draft 2020-12，校验库 santhosh-tekuri/jsonschema/v6，外加薄适配层；支持 JSON/YAML
  schema 文件。`[FACT]`（来源：需求提出者确认，2026-09-29；详见 ADR-003）
- 支持内部 Go 风格类型别名：`int` -> `integer`、`float` -> `number`、`bool` -> `boolean`、`array<T>` -> `array + items`。
  `[FACT]`（来源：需求提出者确认）
- 未知非 `x-*` 关键字默认不报错，以支持 schema 跨系统复用；可选 `--schema-unknown=warn|error` 严格模式。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- `int` 与 `float` 视为不同类型，`0` 与 `0.0` 也视为不同类型；但零值填充两者均支持，零值冲突/歧义时以 `0` 为准。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- 内联 `| type` 与 schema 冲突优先级：schema 与内联取并集，schema 优先，交集冲突报错；如果提供 `| type`，必须作为 pipeline
  第一个操作符，且可以缺省。`[FACT]`（来源：需求提出者确认，2026-09-29）
- `.env` 允许 JSON 数组，且已确认规则：只有 schema 声明为 array/object 时才做 JSON 解析，要求单行合法 JSON；未声明 schema
  时不自动推断。`[FACT]`（来源：需求提出者逐条确认，2026-09-29）
- macOS/CI/Alpine 等环境常无 envsubst，需额外安装 gettext。`[FACT]`（来源：StackOverflow Q23620827、Q64112268；a8m/envsubst
  #64）
- 无简单转义会导致 NGINX `$http_host` 等被替换为空；`$1` 也会被误替换。`[FACT]`（来源：docker-library/docs #496；a8m/envsubst
  #13/#27/#38）
- GNU envsubst 不支持 `${VAR:-default}`；a8m 在默认值、嵌套变量、`${VAR:+}`、`${FOO:=}` 等场景有 bug 或回归。`[FACT]`
  （来源：StackOverflow Q50230361；a8m/envsubst #8/#30/#41/#53/#61/#71）
- `envsubst < file > file` 会截断文件；缺少 in-place、env 文件、多文件批量能力。`[FACT]`（来源：StackOverflow
  Q35078753/Q47081507/Q65705619；a8m/envsubst #4/#23）
- 未设置变量会被静默替换为空；最后一行无换行时会丢失。`[FACT]`（来源：StackOverflow Q67907177；a8m/envsubst #6/#25）

## 非目标（本部分）

- 不替代整体上下文评审与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的主题，完整索引见 `01-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
