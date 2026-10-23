---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 代码与仓库目录结构设计

## 1. 目标

- 支持 P0 核心功能，并为 P1/P2 留出扩展位置。`[FACT]`
- 保持 CLI、解析器、类型系统、Schema、渲染、输出分层清晰。`[INFERENCE]`
- 支持后续加入说明文件、部署说明、示例、打包与发布资产。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 2. 模块路径

- 模块路径已确认：`github.com/elizabevil/envsub`。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 3. 顶层目录结构

```text
.
├── cmd/
│   └── envsub/
│       └── main.go              # 仅做进程启动与依赖注入
├── internal/
│   ├── app/                     # 应用装配、配置、生命周期
│   ├── cli/                     # Cobra 命令定义、参数解析
│   ├── engine/                  # Engine 与流式/路径能力接口
│   │   └── extended/
│   │       ├── justfile         # 本包快捷测试入口
│   │       ├── testdata/        # 本包专属 fixture 数据
│   │       ├── scanner.go
│   │       ├── expression.go
│   │       ├── fixture_test.go
│   │       └── engine.go
│   ├── source/                  # .env / YAML / JSON 值来源加载器（远程来源见 02-05 REQ-SRC-11）
│   ├── values/                  # 值树、合并、优先级
│   ├── schema/                  # JSON Schema 加载、别名、薄适配层
│   ├── types/                   # 类型模型、字符串物化、类型转换
│   ├── pipeline/                # join / toJson / toYaml / quote / indent
│   ├── load/                    # 值装载（来源→树→schema 类型化→校验）
│   ├── render/                  # 渲染编排
│   ├── output/                  # stdout/file/in-place/批量写回
│   ├── diag/                    # 错误码、错误报告、脱敏
│   └── version/                 # 版本与 build info
├── test/
│   ├── justfile                 # test/ 目录统一测试入口（非数据）
│   ├── e2e/
│   │   ├── e2e_test.go          # E2E Go 测试（非数据）
│   │   └── testdata/            # E2E 数据（纯数据）
├── examples/                    # 可运行示例模板、values、schema
├── deploy/
│   ├── docker/                  # Dockerfile、scratch/distroless 示例
│   ├── systemd/                 # systemd 示例
│   ├── kubernetes/              # K8s ConfigMap/Job 示例（非编排）
│   └── package/                 # Homebrew/Scoop/Winget 等打包脚本
├── docs/                        # 现有文档体系
├── scripts/                     # 开发、测试、发布辅助脚本
├── .github/
│   └── workflows/
│       ├── ci.yml               # 测试、lint、govulncheck
│       └── release.yml          # GitHub Actions 发布
├── justfile                     # 快捷编译/测试命令
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

## 4. 包职责

- `cmd/envsub`：只负责调用 `internal/app`，不写业务逻辑。`[INFERENCE]`
- `internal/cli`：Cobra 命令、flag、参数解析与退出码映射。命令树只把 flag 装配成 `render.Request`，执行由调用方注入的 `cli.Runner` 完成（`internal/app` 传入 `render.Run`）。`[FACT]`
- `internal/engine`：定义 `Engine`、`StreamEngine`、`StreamReaderEngine`、`PathEngine`、`PathFilterEngine` 接口；`extended` 实现承载 `$NAME`、`${path}`、`$$`、`$1`、默认值操作符。装配由 `internal/app` 注入一个引擎，流水线不按能力标记分支。`[INFERENCE]`
- `internal/source`：`.env`、YAML、JSON 加载器；CLI `--set*` 由 source 层输入。`[INFERENCE]`
- `internal/values`：值树、多文件合并、CLI `--set*` 优先级；**路径文法的唯一归属**
  （`ParsePath`/`CanonicalPath`，以及供其它包复用的 `AppendSegment`/`JoinIndex`/
  `ParentPath`/`IsBelow` 与 `PathSeparator`）。地址的身份是**规范拼写**
  （`CanonicalPath`，单射且可回读），元数据按段结构存放（`RecordMeta`/`MetaAt`/
  `MetaFor`/`WalkMeta`）；其它包不得自建分隔符、拼接路径或自造地址字符串。`[FACT]`
- `internal/schema`：JSON Schema 加载、别名归一化、unknown 策略、`santhosh-tekuri/jsonschema/v6` 集成。`[INFERENCE]`
- `internal/types`：类型模型、env string 物化、零值规则。`[INFERENCE]`
- `internal/pipeline`：固定 formatter 白名单。`[INFERENCE]`
- `internal/load`：值装载。按定稿层序读 `source`、应用 `--set*`、跑 schema 默认值/物化/校验，产出 `values.Tree`；
  通过 `load.Observer` 上报每个来源（`--explain` 是其中一个观察者），因此装载层不知道报告长什么样。`[FACT]`（`06-review/06-17` D5）
- `internal/render`：渲染编排。模板发现、输出目标选择、渲染循环与 `--explain` 报告；值装载后不再由它负责。`[FACT]`
  模式决策（写回/输出目录/`--check`/`--validate-only`）与引擎能力协商集中在一个 `plan`（`plan.go`）里，
  由循环消费，循环本身不含模式分支。`[FACT]`
- **来源元数据只有一个类型**：`values.Meta`（File/Line/Kind/Method/Source）。值文件、`--set*`、
  schema 默认值与模板赋值都把它记在值树上，`--explain` 的帧由它派生（`metaOrigin`），
  不再各自手工拼字段。**层序**（同一路径被多层覆盖时谁先谁后）仍由 `load` 在装载过程中按序报告：
  值树只保留**当前赢家**的 meta，不存谱系，这是当前的固有限制。`[FACT]`
- `internal/output`：安全 in-place、批量输出、`--check` 过期检测（已实现）、`--diff`（P1 预留）。`[FACT]`
- `internal/diag`：错误码、错误上下文、敏感字段脱敏。`[INFERENCE]`
- 一期不提供公共库 API；如果后续需要，再暴露稳定接口；一期 CLI 优先。`[ASSUMPTION]`

## 5. 依赖选型对接

- JSON：标准库 `encoding/json/v2`（默认），`-tags=sonic` 切 `github.com/bytedance/sonic`。`[FACT]`（2026-10-08 更新，见 ADR-004 第 8 节）
- YAML：`go.yaml.in/yaml/v3`（解码与编码同一库；2026-10-08 起，见 ADR-004 第 10 节）。`[FACT]`
- JSON Schema：`github.com/santhosh-tekuri/jsonschema/v6`。`[FACT]`
- CLI：`github.com/spf13/cobra`。`[FACT]`
- `.env`：自研最小解析器。`[FACT]`
- 发布：GitHub Actions。`[FACT]`（来源：需求提出者确认，2026-09-29）
- 快捷编译：`justfile`。`[FACT]`（来源：需求提出者确认，2026-09-29）

## 6. 目录扩展原则

- `docs/` 继续承载需求、设计、ADR、任务与部署说明。`[FACT]`
- `deploy/` 只放部署示例与打包资产，不放核心代码。`[INFERENCE]`
- `examples/` 必须可运行，用于用户学习与 E2E 冒烟。`[INFERENCE]`
- 测试数据遵循 Go 约定放到各 package 自己的 `testdata/`；`testdata` 只放数据，不放 runner/脚本。`[FACT]`
- 包内可提供同级 `justfile` 作为快捷测试入口，只运行本包测试。`[FACT]`
- `test/justfile` 只放 `test/` 目录的统一入口；E2E 执行逻辑写在 `test/e2e/e2e_test.go`，不单独维护 shell runner。`[FACT]`
- `examples/justfile` 面向**用户可运行示例**，依赖根 justfile 产出的 `dist/envsub`，**不参与门禁**：
  `just check` 只通过 `test/justfile` 调用 `test/` 下的包级测试，并只对 `examples/justfile` 做格式校验。
  示例本身由 `test/examples` 守护（规则族覆盖、旗标名与 `internal/cli` 一致）与 `test/e2e` 的
  `examples_tree` golden 覆盖；示例本身是 `data/` + `templates/*.golden` + 24 条 justfile 配方，
  见 `05-05`。`[FACT]`
- `examples/justfile::ensure-binary` 按**源文件是否比二进制新**决定重建，而不是只判断存在：
  旧二进制会静默拒绝已更名的旗标，示例会以 `unknown flag` 失败。`[FACT]`
- P1/P2 新能力新增 `internal/include`、`internal/expr`、`internal/compat` 等包，不破坏现有包边界。`[INFERENCE]`

## 7. 非目标（Non-Goals）

- 本结构不承诺未来所有包名保持稳定；P1/P2 可以新增包。`[ASSUMPTION]`
- 一期不承诺任何公共库 API。`[ASSUMPTION]`
- `deploy/` 不承担 K8s/Helm 编排替代职责。`[FACT]`
