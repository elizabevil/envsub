---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# envsubst 升级版产品定位

> 依据：需求提出者目标、六项补充痛点、线上 pain-point 调研、竞品调研与市场信号。  
> 本文用于收敛产品定位、目标用户和差异化边界；所有假设需人工确认。

## 1. 一句话定位

面向 DevOps/平台工程师与容器化应用开发者，提供一款 **单静态二进制、支持类型校验与多文件分层的 extended 受限配置渲染器**；P2
再提供 GNU/a8m/drone 兼容与转换。`[INFERENCE]`（依据：需求目标 + 六项补充 + 竞品空白；ADR-002）

英文可参考类别： **envsubst-compatible typed configuration renderer**。`[INFERENCE]`

## 2. 目标用户

- 已确认优先级： **平台/DevOps 工程师 > CI 用户 > 容器维护者**。`[FACT]`（来源：需求提出者确认，2026-09-29）
- **平台/DevOps 工程师**：需要在 CI/CD、K8s、NGINX、Systemd、应用配置间做多环境渲染。`[ASSUMPTION]` 验证：访谈 3–5
  名目标用户；owner：待指定。
- **容器镜像维护者/应用开发者**：需要在 entrypoint 中从环境变量或文件生成配置，且不想在镜像里安装 gettext/Python/Java。
  `[ASSUMPTION]` 验证：确认镜像基线与依赖约束；owner：待指定。
- **GitOps/CI 工程师**：需要 dry-run、diff、check、确定性输出与稳定退出码。`[ASSUMPTION]` 验证：确认 CI 工具与流水线形态；owner：待指定。
- **从 envsubst/envplate/a8m 迁移的用户**：P2 前需要手工迁移或并行运行；P2 提供兼容模式与转换命令。`[FACT]`（来源：ADR-002）

## 3. 核心 Jobs-to-be-done

- 当应用部署到多环境时，我希望从多层 values 渲染配置，避免复制多份配置文件。`[INFERENCE]`
- 当配置里包含密钥时，我希望从文件/stdin/环境变量注入，而不是硬编码进 Git。`[FACT]`（来源：
  `docs/00-raw/00-02-user-supplements-and-compatibility.md` 第 10 节多个 issue）
- 当模板同时包含 NGINX/K8s 的 `$http_host` 等内置变量时，我希望只替换我指定的变量，避免配置损坏。`[FACT]`
  （来源：docker-library/docs #496、StackOverflow Q39968344）
- 当值包含数组、数字、布尔时，我希望在渲染前校验类型，避免把错误配置带到运行时。`[INFERENCE]`（依据：需求提出者“数组表示/类型校验”与
  NixNG 类型化配置讨论）
- 当默认值为空、未设置或格式错误时，我希望得到明确错误，而不是静默空字符串。`[FACT]`（来源：a8m/envsubst #25/#61、StackOverflow
  Q67907177）
- 当 CI/GitOps 运行时，我希望单二进制、零运行时依赖、可 diff/check、确定性输出。`[INFERENCE]`（依据：线上容器/CI 痛点与目标约束）

## 4. 市场信号

- GitHub `"envsubst" alternative` issue 搜索有 510 条结果，说明 envsubst 替代与增强需求真实存在。`[FACT]`（来源：GitHub
  Search API，2026-09-29）
- 需求集中在：密钥注入、默认值、递归 `.env`、类型/约束、容器内零依赖、避免使用被放弃的项目。`[INFERENCE]`（依据：第 10 节市场信号）
- 简单替换工具已有多个替代品，仅做“更快/更小 envsubst”很难形成足够差异化。`[INFERENCE]`（依据：
  `docs/reference/REF-01-competitive-landscape.md`）
- 类型化配置和 schema 校验已有 CUE/Nickel/ytt/Helm values schema 等方案，但多偏语言/特定生态。`[FACT]`
  （来源：CUE、Nickel、ytt、Helm 官方文档与社区 issue）
- 市场上缺少“envsubst 兼容 + typed schema + 多文件分层 + 单静态二进制 + 批量/CI”这一组合。`[INFERENCE]`（依据：
  `docs/reference/REF-01-competitive-landscape.md`）

## 5. 差异化价值主张

1. **迁移路径明确**：P2 提供 GNU/a8m/drone 兼容模式与转换命令；一期先提供手工迁移指南与语法对照。`[FACT]`（来源：ADR-002）
2. **类型和多文件不是语法糖**：schema 校验、数组/map、多文件覆盖优先级解决真实生产配置问题。`[INFERENCE]`
3. **受限但足够**：支持默认值、转义、Helm-like pipeline；条件/循环/includes 后置 P1，但不做通用编程语言，降低安全与维护风险。
   `[FACT]`（来源：需求提出者确认，2026-09-29）
4. **生产原生**：安全 in-place、批量/目录、dry-run/diff/check、JSON 报告、稳定退出码。`[INFERENCE]`
5. **零依赖交付**：多架构静态单文件、scratch/distroless 容器、CI 直接下载。`[FACT]`（来源：需求提出者目标描述）
6. **可组合而非替代**：不替代 Helm/Helmfile/Kustomize/ytt，而是覆盖它们之间的“文本配置渲染”断层。`[INFERENCE]`

### 推荐定位语

> 给生产配置用的 envsubst 超集：兼容原有模板，又提供类型校验、多文件分层和 CI/GitOps 友好的单二进制渲染。`[INFERENCE]`

### 不建议的定位

- 另一个更快的 envsubst。`[INFERENCE]`
- Helm/ytt 的替代品。`[INFERENCE]`
- 通用模板引擎或可编程配置语言。`[INFERENCE]`（依据：需求提出者已接受默认不执行 shell/网络，且不采用图灵完备语言）

## 6. 竞品对标与切入策略

- 对标 `renvsubst`：它覆盖默认值、转义、严格模式；我们需强调 schema/数组/多文件分层/批量。`[FACT]`
  （来源：https://github.com/containeroo/renvsubst）
- 对标 `rsubst`：它覆盖 env 变量、条件、循环、strict；我们需强调 envsubst 兼容模式、类型 schema、多文件 values。`[FACT]`
  （来源：https://github.com/jtdowney/rsubst）
- 对标 `ytt`：它覆盖结构化 YAML、数据值、自定义校验、overlays；我们不与 Starlark 语言正面竞争，而是聚焦文本模板与 envsubst 迁移。
  `[FACT]`（来源：https://github.com/carvel-dev/ytt）
- 对标 `Helm/Helmfile`：它们覆盖多 values、schema、环境覆盖；我们聚焦非 K8s 专属、轻量、批量文本渲染。`[FACT]`（来源：Helm/Helmfile
  官方页面）
- 对标 `gomplate/render`：它们覆盖 datasource、多变量文件、函数；我们限制模板能力但强化类型校验、envsubst 兼容与安全默认值。
  `[FACT]`（来源：https://github.com/hairyhenderson/gomplate、https://github.com/sgreben/render）

## 7. 建议 MVP 定位边界

- **P0**：默认值/转义/作用域、类型校验/数组、内联 `| type`、JSON Schema Draft 2020-12 schema
  文件（enum/范围/pattern/长度基础约束）、env/.env/YAML/JSON、多文件覆盖、Helm-like pipeline（`join`/`toJson`/`toYaml`/
  `quote`/`indent`）、安全 in-place、批量单文件、静态多架构发布。`[FACT]`（来源：需求提出者确认复用 JSON Schema，2026-09-29）
- **P1（待办，当前不实现）**：include/partials 功能、受限条件/循环（控制指令单独成行，object
  遍历按键排序）、profiles/environments、dry-run/diff/check、JSON 报告、确定性输出、容器镜像、conformance 测试、迁移文档。
  `[FACT]`（来源：需求提出者确认 P1 实现 include，2026-09-29）
- **P2（待办，当前不实现）**：GNU/a8m/drone 兼容模式与 `migrate`/`convert` 转换命令，以及更多内置函数、变量来源解释、watch、插件/datasource、远程
  secret 集成。`[FACT]`（来源：需求提出者确认 P2 实现兼容其它，2026-09-29）
- 多文件覆盖规则已确认：后加载覆盖先加载，数组默认替换而非追加，CLI `--set` 优先级最高。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 兼容策略已确认：一期只实现 extended 原生模式；二期再考虑 GNU/a8m/drone 兼容与转换命令。`[FACT]`（来源：需求提出者确认，2026-09-29；详见
  `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`）
- 一期仍提供手工迁移指南与语法对照，降低迁移成本。`[INFERENCE]`
- 非功能基线：采用常规 Go 二进制优化（`CGO_ENABLED=0`、`-ldflags="-s -w"`、多架构静态编译）；不额外追求非常规性能指标。`[FACT]`
  （来源：需求提出者确认，2026-09-29）
- **首版建议不做**：通用编程语言、完整 Helm/Jinja、K8s 编排、GUI、远程配置中心、默认网络/命令执行。`[INFERENCE]`

## 8. 非目标（Non-Goals）

- 不做 Helmfile/Helm/Kustomize 的发布编排替代品。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 首版不做 GUI/Web 界面、远程配置中心、插件市场。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 默认不执行 shell 命令、不访问网络、不自动读取远程密钥。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 不做图灵完备的通用编程语言或完整 Helm/Jinja 模板引擎。`[ASSUMPTION]` 待确认“受限模板能力”边界；owner：待指定。

## 9. 待验证风险与问题

- 一期无 drop-in 兼容，目标用户是否接受手工迁移或并行运行？`[TODO]` 需用户访谈验证；owner：产品负责人。
- 多文件合并语义与 Helmfile 差异是否足够好懂？`[TODO]` owner：技术负责人。
- 与 `renvsubst`/`rsubst` 等轻量工具的差异，用户是否愿意为类型/多文件能力切换？`[TODO]` 需用户访谈验证；owner：产品负责人。
- 性能、体积、启动时间、安全基线是否需要硬指标？`[TODO]` owner：技术负责人。
- 是否需要密钥管理内置能力，还是只做文件/stdin 注入？`[TODO]` owner：产品/安全负责人。
