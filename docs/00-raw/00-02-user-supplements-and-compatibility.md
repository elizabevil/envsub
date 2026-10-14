---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 原始材料 · 02 用户补充与兼容决策

> 本文是 `00-00-index.md` 的拆分部分。

## 7. 需求提出者补充痛点（2026-09-29）

- 明确补充以下功能/使用诉求：默认值、数组表示、转义格式、类型校验、文件配置、多文件配置。`[FACT]`（来源：需求提出者本轮补充）
- 上述诉求的场景、样例与优先级：**现状**已逐条落到需求台账与规则文档——六项诉求对应
  `02-05` 的 REQ-SRC/REQ-TYP/REQ-OUT 条目，语法与样例见 `07-serialization` 各章与 `examples/`。
  `[FACT]` 若需补充新的场景/验收标准，按 REQ 立项。
- 这些诉求与线上证据中的默认值、转义、配置来源、多文件覆盖等方向一致，但“数组表示”“类型校验”属于新的增强信号。`[INFERENCE]`
  依据：需求提出者补充项与另见线上 issue 的对应关系。

## 8. 决策讨论输入（2026-09-29）

- 需求提出者授权：缺口可由 AI 直接补充，后续再评审。`[FACT]`（来源：需求提出者本轮补充）
- 需求提出者要求：尽量兼容 envsubst；如果 envsubst 方案不合适，可直接换方案。`[FACT]`（来源：需求提出者本轮补充）
- 需求提出者询问：继续用 Go 开发还是切到 Rust；并希望参考 helmfile 类“渲染”工具。`[FACT]`（来源：需求提出者本轮补充）
- 已生成 ADR 草案：`docs/04-decisions/04-01-adr-001-language-and-render-model.md`，提出 v0/v1 继续 Go、核心改为层级值模型 +
  渲染引擎、envsubst 作为兼容模式。`[INFERENCE]`（依据：需求提出者目标、六项补充、线上证据）

## 9. 非目标与类型检查讨论（2026-09-29）

- 需求提出者接受非目标第 1、3、4 条：不做 Helmfile/Helm/Kustomize 发布编排替代品；首版不做 GUI/Web、远程配置中心和插件市场；默认不执行
  shell、不访问网络、不自动读取远程密钥。`[FACT]`（来源：需求提出者确认）
- 需求提出者追问“通用编程语言或模板引擎”的含义，以及当前能否支持类型检查。`[FACT]`（来源：需求提出者本轮补充）
- 当前项目状态：仍处于文档与 ADR 草案阶段，尚无代码实现；类型检查为规划能力。`[FACT]`（来源：
  `docs/04-decisions/04-01-adr-001-language-and-render-model.md`、`docs/02-product/02-01-feature-map.md`）
- 讨论结论：排除图灵完备通用语言，但保留受限配置渲染能力；类型检查支持值级/schema 级运行时校验，不承诺纯文本模板静态类型检查。
  `[INFERENCE]`（依据：需求提出者边界 + 纯文本模板缺少类型信息）

## 10. 补充市场信号：GitHub “envsubst alternative” 主题

- GitHub issue 搜索 `"envsubst" alternative in:title,body` 共 510 条结果；按 reactions 取前 50 条分析。`[FACT]`（来源：GitHub
  Search API，2026-09-29）
- `project-zot/zot#2521`：希望在配置文件中替换环境变量，避免把 OIDC client secret 明文写进配置。`[FACT]`
  （来源：https://github.com/project-zot/zot/issues/2521）
- `linuxserver/docker-swag#276`：希望在容器启动时通过 envsubst 把邮箱、Cloudflare token 等凭据注入 DNS 配置，避免硬编码进
  Git。`[FACT]`（来源：https://github.com/linuxserver/docker-swag/issues/276）
- `Yellow-Dog-Man/Resonite-Issues#6347`：自动化部署中需要从环境变量生成 `Config.json`，避免把密钥提交进仓库。`[FACT]`
  （来源：https://github.com/Yellow-Dog-Man/Resonite-Issues/issues/6347）
- `caronc/apprise#1156`：希望在配置文件中支持环境变量，用于包含 secret 的 URI。`[FACT]`
  （来源：https://github.com/caronc/apprise/issues/1156）
- `containers/podman-compose#1384`：希望 `.env` 文件支持递归变量替换和默认值，类似 Docker Compose 能力。`[FACT]`
  （来源：https://github.com/containers/podman-compose/issues/1384）
- `openclaw/gogcli#599`：希望 JSON 凭据支持 `${VAR}` 和 `${VAR:-fallback}`，减少调用方 shell 拼接。`[FACT]`
  （来源：https://github.com/openclaw/gogcli/issues/599）
- `nix-community/NixNG#4`：讨论用 Nickel 等类型化 schema 方案替代 Go template/Django/EEx 等模板式配置，以便约束模板可访问的数据。
  `[FACT]`（来源：https://github.com/nix-community/NixNG/issues/4）
- `openresty/docker-openresty#38`、`bitnami/containers#34957`：官方镜像缺少 envsubst，用户要求在镜像中内置或提供替代方案。
  `[FACT]`
  （来源：https://github.com/openresty/docker-openresty/issues/38、https://github.com/bitnami/containers/issues/34957）
- `kreuzwerker/envplate#36`：用户询问被放弃的项目是否有已知替代品，并提到 a8m/envsubst。`[FACT]`
  （来源：https://github.com/kreuzwerker/envplate/issues/36）
- `drone/envsubst#33`：讨论替代实现，并提到 a8m/envsubst 支持 fail-on-unset。`[FACT]`
  （来源：https://github.com/drone/envsubst/pull/33）
- `netbirdio/helms#18`：容器 sidecar 因缺少 `envsubst` 启动失败，说明容器内二进制可用性是真实部署问题。`[FACT]`
  （来源：https://github.com/netbirdio/helms/issues/18）
- 这些信号表明需求不只是“替换字符串”，还包含密钥注入、默认值、递归 `.env`、类型/约束、容器内零依赖与避免项目被放弃。
  `[INFERENCE]`（依据：上述 issue 主题与正文）

## 11. 用户优先级、P0 与兼容基线讨论（2026-09-29）

- 需求提出者确认目标用户优先级：平台/DevOps 工程师 > CI 用户 > 容器维护者。`[FACT]`（来源：需求提出者确认）
- 需求提出者确认：类型校验为 P0。`[FACT]`（来源：需求提出者确认）
- 需求提出者同意多文件覆盖规则：后加载覆盖先加载，数组默认替换而非追加，CLI `--set` 优先级最高。`[FACT]`（来源：需求提出者确认）
- GNU envsubst 0.23.1 本地行为：`$1` 不替换、`$$` 不转义为 `$`、`${VAR:-default}` 等默认值语法不展开、变量白名单只替换指定变量。
  `[FACT]`（来源：本地 GNU envsubst 0.23.1 验证）
- a8m/envsubst README 声明：支持 `${var-DEFAULT}`、`${var:-DEFAULT}`、`${var=DEFAULT}`、`${var:=DEFAULT}`、`${var+OTHER}`、
  `${var:+OTHER}`，支持 `$$var` 转义；CLI 提供 `-no-digit`、`-no-unset`、`-no-empty`、`-fail-fast`、`-i`、`-o`。`[FACT]`
  （来源：https://github.com/a8m/envsubst）
- drone/envsubst README 声明：主要作为 Go 库；支持 `${var}`、大小写转换、offset、字符串截取、前后缀裁剪、默认值表达式、替换表达式；README
  将 `${var-default}`、`${var+default}`、`${var:?default}`、`${var:+default}` 列为不支持。`[FACT]`
  （来源：https://github.com/drone/envsubst）
- 三者不是同一个兼容目标：GNU 简单、a8m 面向原有 CLI/升级用户、drone 面向 shell-like 库语义与 GitOps（Flux 曾使用）。
  `[INFERENCE]`（依据：上述 README 与 Flux 相关 issue）

## 12. 兼容策略决策（2026-09-29）

- 需求提出者选择方案 A：窄兼容 + 转换 + 原生扩展。
- 一期只支持 extended 本应用原生模式；不做 GNU/a8m/drone 完整兼容模式，不做 migrate/convert 命令。
- 二期再考虑兼容模式与自动转换命令。
- 一期仍提供手工迁移指南与语法对照，降低后续迁移成本。
- 已记录至 `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`。

## 非目标（本部分）

- 不替代整体证据审阅与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的输入，完整索引见 `00-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`
