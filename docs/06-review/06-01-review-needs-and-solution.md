---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# 评审报告：需求定义与方案（Round 1）

## 1. 评审范围

- `docs/01-context/`
- `docs/02-product/`
- `docs/03-design/`
- `docs/04-decisions/`
- 关联参考：`docs/reference/`

## 2. PRD / 需求门禁

| 门禁                                | 结果     | 说明                                                                                                |
|-------------------------------------|----------|-----------------------------------------------------------------------------------------------------|
| 问题陈述有可度量证据                | 部分通过 | 有 GitHub/StackOverflow 证据与 510 条 issue 统计，但缺少目标产品自身的成功指标。`[TODO]`            |
| Non-Goals 至少 3 条                 | 通过     | Context、Product、Design、ADR 均有非目标。                                                          |
| 用户故事有 Given-When-Then 验收条件 | 未通过   | 当前只有 Context 与功能地图，没有 PRD/用户故事。`[TODO]`                                            |
| 异常流程与边界条件覆盖              | 部分通过 | 设计覆盖 unset/empty/required/schema 错误，但错误码、退出码、批量失败、权限失败等未系统化。`[TODO]` |
| 非功能需求有数值或 TODO             | 未通过   | 性能、体积、启动时间、内存、安全基线只有 `[TODO]`，无目标值。                                       |
| INFERENCE / ASSUMPTION 已人工审核   | 未通过   | 多条已确认决策仍留在 ASSUMPTION / INFO GAP 中，详见第 4 节。                                        |

## 3. Design Doc 门禁

| 门禁                               | 结果     | 说明                                                                  |
|------------------------------------|----------|-----------------------------------------------------------------------|
| Alternatives 至少 1 个被拒绝       | 部分通过 | ADR 有替代方案；但 `03-05` 的 Alternatives 已与当前决策冲突，需重写。 |
| Trade-offs 写明代价                | 通过     | `03-05` 第 7 节有 trade-offs。                                        |
| 长度符合规范                       | 通过     | 已拆分为多个 1–3 页级文档。                                           |
| Open Questions 有 owner 和解决时点 | 未通过   | 大多数只有 owner，没有解决时点。`[TODO]`                              |

## 4. 严重问题清单

### P0 问题

1. **`03-05` 的 Alternatives 与 ADR-003 冲突**
    - 位置：`docs/03-design/03-05-api-data-tradeoffs.md` 第 6.2 节。
    - 现状：写着“采用完整 JSON Schema”被拒绝，当前决策是“采用 JSON Schema 风格子集”。
    - 冲突：`docs/04-decisions/04-03-adr-003-json-schema-reuse.md` 已确认直接复用 JSON Schema Draft 2020-12。
    - 建议：把 6.2 改为“自研 schema DSL”被拒绝；增加“JSON Schema + 薄适配层”为当前方案。

2. **兼容性分期未清理，存在自相矛盾**
    - 位置：
        - `docs/01-context/01-04-assumptions-and-gaps.md` 第 5 节：仍写 `--compat=gnu|a8m|extended`。
        - `docs/02-product/02-01-feature-map.md` 第 3.1 节：仍写 `--compat=gnu|a8m|drone|extended`。
        - `docs/02-product/02-02-product-positioning.md` 第 3 节、第 5.1 节、第 9 节：仍指向兼容模式/零改动迁移。
    - 冲突：ADR-002 已确认一期只做 extended 原生模式，P2 才做兼容与转换。
    - 建议：所有兼容相关表述改为“P2 兼容与转换；一期无 `--compat`”。

3. **`03-05` 中旧语法残留**
    - 位置：`03-05` 第 6.3 节。
    - 现状：仍写 `:yaml` / `:text`。
    - 冲突：`03-02` 已改为 Helm-like pipeline `| toYaml` / `| toJson` / `| join`。
    - 建议：更新为 pipeline 语法，删除旧 `:yaml` / `:text`。

4. **Open Questions 与已确认决策不一致**
    - 位置：`docs/03-design/03-06-open-questions-and-review.md` 第 9 节。
    - 现状：仍问“是否采用内联 `| type`”“schema 是否用 JSON Schema 子集”等。
    - 冲突：内联 `| type` 已确认；JSON Schema Draft 2020-12 已确认。
    - 建议：把已确认项改写为“已确认”，只保留未决问题。

5. **缺少 PRD / 用户故事 / 验收标准**
    - 位置：整个 `docs` 缺 PRD。
    - 影响：无法通过 PRD 门禁，也无法把设计映射到可测试用户故事。
    - 建议：基于 `02-01` 与 `02-02` 生成 PRD，并为 P0 场景补 Given-When-Then。

### P1 问题

6. **类型 / 语法表述不统一**
    - `02-01` 第 3.3 节写“string、number、bool、array、map、null”。
    - `03-02` 第 3.2 节写“string、bool、int、float、array<T>、object、null”。
    - 建议：统一为设计文档的 Go 风格类型别名 + JSON Schema 标准类型映射。

7. **`.env` JSON 数组规则状态不一致**
    - `01-02-facts.md` 写 `[FACT]`，但括注“具体规则待最终确认”。
    - `03-03` 写 `[ASSUMPTION]`。
    - 建议：统一为 `[ASSUMPTION]`，直到需求提出者逐条确认 6 条规则。

8. **`--unset=zero` 未决且 CLI 不一致**
    - `03-04` 有 `--unset=zero` 候选，但 CLI 草案只列 `error|empty|keep`。
    - 建议：先确认是否纳入 P0/P1；若纳入，更新 CLI 草案与错误码；若不纳入，标记为未来扩展。

9. **`--schema-unknown` 未进入 CLI 草案**
    - 已在 ADR-003 确认默认不报错、可选严格模式，但 `03-04` CLI 草案没有 `--schema-unknown`。
    - 建议：加入 `--schema-unknown=ignore|warn|error`，默认 `ignore`。

10. **远程 secret 的 P2 与 Non-Goal 关系不清**
    - `02-01` 第 4 节 P2 写“远程 secret 集成”。
    - `02-01` 第 5 节写“默认不访问网络、不自动读取远程密钥”。
    - 建议：明确为“默认关闭，显式启用，P2 仅预留设计”，避免与 Non-Goal 冲突。

### P2 问题

11. **Open Questions 无解决时点**
    - 位置：`03-06` 第 9 节。
    - 建议：每条补 `解决时点` 或 `目标版本`。

12. **include 细节仍是 P1 设计项**
    - 已确认 P1 实现，但路径白名单、循环检测、执行时指定文件方式尚未设计。
    - 可以接受，但需在 PRD/任务拆分前补最小设计。`[TODO]`

## 5. 修改顺序建议

1. 先修正 ADR-002 与 Context/Product/Design 中的兼容性矛盾。
2. 重写 `03-05` 的 Alternatives 与 Trade-offs，使其与 ADR-003 一致。
3. 清理 `03-06` 已决问题，只保留真正 Open Questions。
4. 生成 PRD，补 P0 场景的用户故事与 Given-When-Then。
5. 补非功能指标与错误码/退出码矩阵。
6. 再次运行评审门禁。

## 6. 总体结论

**需修改后再评审。**

当前方案主体已收敛，但兼容性分期、schema 方案、旧语法、Open Questions 与已确认决策之间存在多处冲突，且缺少 PRD 与可测试验收条件。

## 非目标（本部分）

- 本报告不直接修改需求或方案，只记录评审结论。`[FACT]`
- 不替代产品/技术负责人对每条问题的最终确认。`[ASSUMPTION]`
- 不展开具体实现代码与接口细节。`[INFERENCE]`

## 7. 修复状态（2026-09-29 更新）

以下已确认项已完成修复：

- 兼容性分期矛盾已修复：
    - `01-01`、`01-04`、`02-01`、`02-02` 已统一为“一期 extended、P2 兼容与转换”。
- `03-05` Alternatives 已修复：
    - 6.2 改为“自研 schema DSL 被拒绝；当前采用 JSON Schema Draft 2020-12 + 薄适配层”。
    - 6.3 改为 Helm-like pipeline，删除旧 `:yaml` / `:text`。
- `03-06` Open Questions 已重写：
    - 已确认项移出 Open Questions。
    - 每个未决项补充 owner 与解决时点。
- `02-01` 类型列表已统一为 `string`/`bool`/`int`/`float`/`array<T>`/`object`/`null`。
- `.env` JSON 数组规则状态已统一为 `[ASSUMPTION]`，待确认。
- `03-04` CLI 已补 `--schema-unknown=ignore|warn|error`，并标注 `--unset=zero` 仍待确认。
- 远程 secret P2 与 Non-Goal 关系已澄清为“默认关闭、显式启用、P2 预留”。
- `02-02` 风险清单中的过时项已清理。

后续已补：

- PRD：`docs/02-product/02-03-prd.md`。
- 用户故事与 Given-When-Then：`docs/02-product/02-04-user-stories.md`。
- ADR-001 已更新为 Accepted，Go 技术选型已确认。
- 非功能需求：采用常规二进制优化即可，已记录；具体数值指标仍保留 `[TODO]`。
- `| type` 规则已确认：如果提供必须为 pipeline 第一个操作符，可以缺省。
- Open Questions 中的未决项仍需逐条确认。`[TODO]`

## 8. 后续确认项更新（2026-09-29）

- `.env` JSON 数组 6 条规则已由需求提出者确认，设计文档中已从 `[ASSUMPTION]` 升级为 `[FACT]`。
- `--unset=zero` 已确认加入 CLI，仅对非 required 字段生效。
- P1/P2 保留为待办，当前只实现 P0。
- 性能目标已改为参考 GNU envsubst：
    - 10MB / 200k 变量模板不超过 GNU envsubst 基线 5 倍。
    - 空输入启动不超过 100ms。
    - 峰值内存不超过 GNU envsubst 基线 5 倍。
    - 基线记录见 `docs/reference/REF-04-gnu-envsubst-performance-baseline.md`。

## 9. 任务与依赖选型补充（2026-09-29）

- 已建立 P0 任务拆分：`docs/05-tasks/05-01-p0-task-breakdown.md`。
- 已建立 P0 测试矩阵：`docs/05-tasks/05-02-p0-test-matrix.md`。
- 已建立代码与仓库目录结构设计：`docs/03-design/03-07-code-and-repo-structure.md`。
- 依赖选型已收敛：
    - JSON：`github.com/bytedance/sonic`。
    - YAML：`sigs.k8s.io/yaml`。
    - JSON Schema：`santhosh-tekuri/jsonschema/v6`。
    - 发布：GitHub Actions。
    - 快捷编译：`justfile`。
- 仍需确认：
    - CLI 使用 Cobra 还是 Kong。`[TODO]`
    - `.env` 使用自研解析器还是 `joho/godotenv`。`[TODO]`
    - `sonic` 在 Windows/macOS/arm64 的构建与兼容性验证。`[TODO]`

## 10. CLI 与兼容架构补充（2026-09-29）

- ADR-004 已升级为 Accepted。
- CLI 最终使用 `spf13/cobra`，并新增独立设计文档：
    - `docs/03-design/03-08-cli-design.md`
- `.env` 最终使用自研最小解析器。
- 新增扩展与兼容架构文档：
    - `docs/03-design/03-09-extension-and-compat-architecture.md`
- P2 兼容模式将通过 Engine 接口 + Registry 接入，当前只注册 `extended`。
- 任务拆分新增 `TASK-027`：定义 Engine 接口与 extended 注册。
- 仍需验证 `sonic` 在 Windows/macOS/arm64 上的构建兼容性。`[TODO]`

## 11. 代码规范与骨架（2026-09-29）

- 已确认 Go 1.27.1。
- 已新增 Go 代码开发规范：
    - `docs/03-design/03-10-go-code-standards.md`
- 已新增根目录 `CONTRIBUTING.md`。
- 已搭建 Go 代码骨架，并通过 `just build`、`go test ./...`。
- Engine 注册已改为显式函数，避免 `init()` 副作用。
- 后续新增代码必须通过 `gofmt`、`goimports`、`go vet`、`golangci-lint`、`govulncheck`。

## 12. TASK-003 ~ TASK-005 实现进度（2026-09-29）

- TASK-003：错误码与退出码模型已实现，并有表驱动测试。
- TASK-004：stdin/stdout/file 输入输出管线已实现，并有 writer/render 测试。
- TASK-005：extended 模板扫描器已实现，支持 `$NAME`、`${path}`、`$$`、`$1` 字面量、基础嵌套括号。
- 新增 `internal/logging`，统一使用 `log/slog`。
- CLI 增加 `--log-level=debug|info|warn|error`。
- 验证：`go test ./...`、`just build`、stdout/file 管线、退出码 2/3/5 均通过。

## 13. TASK-006 与代码结构基础（2026-09-29）

- TASK-006 已实现：
    - `internal/values/path.go`：路径解析与解析。
    - 支持 `app.port`、`hosts[0]`、`app["my-key"]`。
    - 支持 `env.PORT` 与简单标识符 env fallback。
    - extended Engine 现在能替换简单变量与 env 变量。
- 代码结构基础已补齐：
    - 常用字符串字面量提取为 `const`。
    - 新增 `internal/utils`：`StdioPath`、`IsStdio`、`CloneBytes`。
    - 新增 `diag.NotImplemented` 统一未实现错误。
    - 新增 `internal/logging`，使用 `log/slog`。
- 验证：
    - `go test ./...` 通过。
    - `just build` 通过。
    - `NAME=world envsub render` 输出正确。
    - 缺失变量 exit 6。

## 14. TASK-007 完成（2026-09-29）

- 已实现受限 POSIX shell 默认值/存在性操作：
    - `${path:-default}`
    - `${path-default}`
    - `${path:+alternate}`
    - `${path+alternate}`
    - `${path:?message}`
    - `${path?message}`
    - `${path:=default}`
    - `${path=default}`
- 支持嵌套默认值、默认词中的 `:`、`-` 字面量。
- 缺失/必填错误返回 exit code 6。
- 新增 `parseExpression`、`renderVariable`、`requiredError`、`isEmptyValue`。
- 测试覆盖操作符、空值、嵌套、特殊字符、别名。

## 15. 基础环境变量 + 模板渲染验证（2026-09-29）

- 已新增集成测试：`internal/render` 中使用真实 `extended` Engine 渲染环境变量。
- 已验证：
    - 文件模板 + `$NAME` 渲染成功。
    - stdin 模板 + `$NAME` / `${NAME}` 渲染成功。
    - 缺失变量 exit 6。
    - 默认值操作符 exit 0。
- `.env` 功能尚未引入；建议基础环境变量路径稳定后再进入 TASK-008。

## 16. 测试矩阵落盘（2026-09-29）

- 已建立预先定义的 fixture 目录：`internal/engine/extended/testdata/`。
- 新增 fixture 运行器：`internal/engine/extended/fixture_test.go`。
- 测试矩阵覆盖 syntax、defaults、types、errors 四类，共 67 个 fixture。
- 模板与数据均由文件定义，不再在 Go 测试函数中实时生成。
- 新增快捷命令：`just test-fixtures`、`just fixtures-list`、`just test-all`、`just test-race`。

## 17. 真实 CLI E2E 测试（2026-09-29）

- 新增 `../../test/testdata`，共 4 个真实 CLI case。
- E2E 执行逻辑已改为 Go 测试：`test/e2e/e2e_test.go`；`test/justfile` 仅作为入口。
- 每个 case 使用：
    - 真实 `envsub render` 二进制
    - 预定义 `input.tmpl`
    - 预定义 env 文件
    - `expect.out` / `exit` / `expect.err`
- 新增命令：
    - `just test-e2e`
    - `just e2e-list`
    - `just test-all` 已包含 e2e
- 覆盖：
    - env + `$NAME`
    - env + `${NAME}`
    - 默认值 + 空值
    - `$$` / `$1`
    - 多环境变量
    - 缺失变量 exit 6
    - 模板语法错误 exit 3
    - `-o` 输出文件

## 18. 包级 justfile（2026-09-29）

- 新增 `internal/engine/extended/justfile`，只运行本包测试。
- 根 justfile 的 `test-fixtures`、`fixtures-list`、`fixtures-tree` 已改为委托该包级入口。
