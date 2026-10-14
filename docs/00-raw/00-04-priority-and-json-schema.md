---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 原始材料 · 04 优先级与 JSON Schema 决策

> 本文是 `00-00-index.md` 的拆分部分。

## 19. 功能优先级确认（2026-09-29）

- P0 实现核心功能。
- P1 实现 include 功能（以及受限条件/循环等 P1 生产增强）。
- P2 实现 GNU/a8m/drone 兼容模式与转换命令。
- include 执行时指定允许文件/目录；最大嵌套深度候选 3 层，循环检测、路径白名单与错误格式在 P1 细化。

## 20. JSON Schema 复用决策（2026-09-29）

- 采用方案：JSON Schema Draft 2020-12 + santhosh-tekuri/jsonschema/v6 + 薄适配层。
- schema 文件支持 JSON / YAML / YML，先归一化为 JSON 兼容数据树。
- 支持 Go 风格类型别名：int -> integer、float -> number、bool -> boolean、array<T> -> array + items。
- 未知非 `x-*` 关键字默认不报错，便于 schema 跨系统复用；可选 `--schema-unknown=warn|error` 严格模式。
- 远程 `$ref` 默认禁用；default 由薄适配层主动应用。
- 已记录至 `docs/04-decisions/04-03-adr-003-json-schema-reuse.md` 和
  `docs/reference/REF-03-json-schema-reuse-research.md`。

## 非目标（本部分）

- 不替代整体证据审阅与人工确认。`[ASSUMPTION]`
- 不展开本部分之外的输入，完整索引见 `00-00-index.md`。`[INFERENCE]`
- 不把本部分的 `[ASSUMPTION]` 当作最终承诺。`[ASSUMPTION]`

## 21. 技术选型、NFR 与 type 规则补充（2026-09-29）

- 技术选型确认：继续使用 Go，ADR-001 状态升级为 Accepted。
- 非功能需求确认：采用常规二进制优化即可，不额外追求非常规性能/体积指标。
- `| type` 规则确认：如果提供，必须作为 pipeline 第一个操作符；可以缺省。
- 已基于前期需求生成 PRD 与用户故事：
    - `docs/02-product/02-03-prd.md`
    - `docs/02-product/02-04-user-stories.md`

## 22. 性能、规则确认与范围收敛（2026-09-29）

- 性能指标目标值改为参考 GNU envsubst，基线见 `docs/reference/REF-04-gnu-envsubst-performance-baseline.md`。
- `.env` JSON 数组 6 条规则已确认，设计文档升级为 `[FACT]`。
- `--unset=zero` 已确认加入；仅对非 required 字段生效，required 不自动填零。
- P1/P2 保留为待办；当前只实现 P0。
- PRD 与用户故事已补充，并标注 P0/P1/P2：`docs/02-product/02-03-prd.md`、`docs/02-product/02-04-user-stories.md`。

## 23. 依赖库、测试与代码结构确认（2026-09-29）

- JSON 使用 `github.com/bytedance/sonic`。
- YAML 使用 `sigs.k8s.io/yaml`。
- 发布使用 GitHub Actions；提供 `justfile` 快捷编译。
- 测试采用测试先行：规则先落测试矩阵与 golden fixtures，再推动实现。
- 开始设计代码与仓库目录结构，并预留 `docs/`、`deploy/`、`examples/`、`testdata/`、`test/` 等扩展位置。
- 相关文档：
    - `docs/04-decisions/04-04-adr-004-dependency-selection.md`
    - `docs/03-design/03-07-code-and-repo-structure.md`
    - `docs/05-tasks/05-02-p0-test-matrix.md`

## 24. CLI 与兼容架构补充（2026-09-29）

- 确认 CLI 使用 `spf13/cobra`。
- CLI 功能独立设计文档：`docs/03-design/03-08-cli-design.md`。
- `.env` 使用自研最小解析器。
- P2 兼容模式通过 Engine 接口与 Registry 接入，不同实现分别为 extended / gnu / a8m / drone。
- 扩展与兼容架构：`docs/03-design/03-09-extension-and-compat-architecture.md`。
- 代码目录增加 `internal/engine`、`internal/source` 等边界。

## 25. Go 版本与代码规范确认（2026-09-29）

- `go.mod` 使用 Go 1.27.1。
- 严格遵循 Google Go Style Guide。
- 避免旧语法：`interface{}`、`io/ioutil`、`math/rand`、`panic` 普通错误、`init` 注册副作用等。
- 新增代码规范文档：`docs/03-design/03-10-go-code-standards.md`。
- 新增根目录 `CONTRIBUTING.md`。
- 代码骨架已改为显式 Engine 注册，避免 `init()` 副作用。

## 26. 代码结构基础与 TASK-006（2026-09-29）

- 常用字符串字面量必须提取为常量，已落实到 engine/cli/render/logging/values。
- 常用重复逻辑提取到 `internal/utils` 或领域包，例如 `utils.IsStdio`、`diag.NotImplemented`。
- TASK-006 完成：路径解析、env/根命名空间、简单变量替换。
- extended Engine 已支持 `$NAME`、`${app.port}`、`env.PORT` 和缺失变量退出码 6。
- 日志统一使用 `log/slog`。

## 27. TASK-007 完成（2026-09-29）

- 默认值/存在性操作已实现：`:-`、`-`、`:+`、`+`、`:?`、`?`、`:=`、`=`。
- 支持嵌套默认值、URL/负数字面量、空值语义。
- 缺失/必填错误 exit code 6。
- 下一步 TASK-008：`.env` 解析器与 JSON 数组规则。

## 28. 测试矩阵与 Fixture 落盘（2026-09-29）

- 已建立完整 fixture 矩阵：`internal/engine/extended/testdata/`。
- 覆盖 syntax、defaults、types、errors 四类，40+ 个 case。
- fixture 格式：`input.tmpl`、`env.yaml`、`values.yaml`、`meta.yaml`、`expect.out`、`expect.err`。
- 运行器：`internal/engine/extended/fixture_test.go`。
- 新增快捷命令：`just test-fixtures`。
- 测试不再在 Go 代码中实时生成模板和数据。

## 29. Fixture 扩充与 justfile 命令（2026-09-29）

- fixture 数量从 40+ 扩充到 67 个。
- 新增 fixtures-list、test-all、test-race 等 justfile 命令。
- `just test` 使用 `-count=1`，避免缓存掩盖问题。

## 30. 真实 CLI E2E 测试（2026-09-29）

- 新增 `../../test/testdata` 4 个真实 CLI case。
- E2E 执行逻辑改为 Go 测试：`test/e2e/e2e_test.go`；`test/justfile` 仅作为入口。
- 新增 `just test-e2e`、`just e2e-list`。
- `just test-all` 已包含单元测试、fixture 矩阵和 E2E。
- 覆盖环境变量替换、默认值、转义、缺失变量、语法错误和文件输出。

## 31. 包级 justfile（2026-09-29）

- 新增 `internal/engine/extended/justfile`，提供本包 test / test-fixtures / fixtures-list / fixtures-tree。
- 根 justfile 对应命令改为委托包级入口。
