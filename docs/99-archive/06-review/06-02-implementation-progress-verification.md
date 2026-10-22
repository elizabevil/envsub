---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-02
---

# 实现进度核查报告（Round 2）

> 核查方式：只读检查代码 + 实际编译/测试 + 真实 CLI 探针，不修改任何实现。
> 核查基线：`docs/05-tasks/05-01-p0-task-breakdown.md`、`docs/05-tasks/05-02-p0-test-matrix.md`、
> `docs/03-design/03-07-code-and-repo-structure.md`、`docs/03-design/03-08-cli-design.md`。
> 结论摘要： **骨架与模板语法层（Sprint 0 全量 + TASK-005/006/007）真实可用；
> 文档中标记为完成的 TASK-020 以及 `docs/99-archive/00-raw/00-04` / `06-01` 里“下一步 TASK-008”之后的内容，
> 与代码现状不一致。P0 剩余 17 个任务中有 15 个尚未实现。**

## 1. 核查命令与结果

| 命令                              | 结果                                                                           |
|-----------------------------------|--------------------------------------------------------------------------------|
| `go build ./...`                  | 通过（需可写 `GOCACHE`，见 §6.1）                                              |
| `go vet ./...`                    | 通过                                                                           |
| `go test -count=1 ./...`          | 全部通过；`internal/source`、`internal/pipeline`、`internal/schema` 无测试     |
| `go build -o envsub ./cmd/envsub` | 通过                                                                           |
| CLI 探针（14 项，见 §3）          | 8 项符合文档，6 项与文档不符                                                   |
| `gofmt -l`                        | 2 个文件未格式化：`internal/source/dotenv.go`、`internal/source/json_array.go` |

代码规模：43 个 Go 文件、2685 行（含测试）。fixture：engine 67 个、E2E 4 个。

## 2. 任务完成度对照

图例：✅ 已实现并有测试 · 🟡 部分实现 · ❌ 未实现（仅占位/死代码）

| 任务                                    | 文档标注 | 实测 | 依据                                                                                                                                 |
|-----------------------------------------|----------|------|--------------------------------------------------------------------------------------------------------------------------------------|
| TASK-001 工程骨架/CI                    | 完成     | 🟡   | 目录与 justfile 齐备，但 CI 用 `go-version: '1.22.x'` 而 `go.mod` 要求 `go 1.27.1`；`examples/`、`deploy/`、`scripts/` 仅 `.gitkeep` |
| TASK-027 Engine/Registry/Capabilities   | 完成     | ✅   | `internal/engine/engine.go`、显式注册 `extended.Register`                                                                            |
| TASK-002 CLI 骨架 + render              | 完成     | ✅   | `render`/`version`/`completion`/`help` 可用，未知 flag 退出码 2                                                                      |
| TASK-003 错误码与退出码                 | 完成     | ✅   | `internal/diag/errors.go`：0/1/2/3/4/5/6；`errors_test.go` 表驱动                                                                    |
| TASK-004 stdin/stdout/file 管线         | 完成     | ✅   | stdin、`-o` 文件输出均验证通过                                                                                                       |
| TASK-005 扫描器 `$NAME`/`${}`/`$$`/`$1` | 完成     | ✅   | `scanner.go` + `syntax/` 19 个 fixture                                                                                               |
| TASK-006 路径与环境命名空间             | 完成     | ✅   | `values/path.go`；`${app.port}`、`${env.PORT}`、`${app["my-key"]}`、`hosts[0]`                                                       |
| TASK-007 默认值/存在性操作符            | 完成     | ✅   | `:-` `-` `:+` `+` `:?` `?` `:=` `=` 全部实测通过                                                                                     |
| TASK-008 `.env` 解析器                  | 未标注   | 🟡   | `internal/source/dotenv.go` 已写（150 行）但**无任何引用、无测试**，且未 gofmt                                                       |
| TASK-009 YAML/JSON values 加载          | 未标注   | 🟡   | `source/yaml.go`、`source/json.go` 已写但无引用、无测试                                                                              |
| TASK-010 多文件合并与优先级             | 未标注   | ❌   | `values` 包只有单棵树，无合并逻辑                                                                                                    |
| TASK-011 Schema 加载/别名/unknown       | 未标注   | ❌   | `schema` 包只有 `Validate()`，别名与 unknown 策略均无；`--schema` 直接报未实现                                                       |
| TASK-012 schema `default` 应用          | 未标注   | ❌   | 无代码                                                                                                                               |
| TASK-013 字符串物化与类型转换           | 未标注   | 🟡   | `types/stringify.go` 仅输出；schema 驱动的转换无                                                                                     |
| TASK-014 jsonschema/v6 校验集成         | 未标注   | ❌   | `schema.Validate` 是死代码（仅被 `engine.Input` 字段类型引用）                                                                       |
| TASK-015 内联 `\| type`                 | 未标注   | ❌   | 无 pipeline 解析                                                                                                                     |
| TASK-016 错误来源路径/行列              | 未标注   | 🟡   | 错误含变量路径，但不含来源文件与行列                                                                                                 |
| TASK-017 pipeline formatter             | 未标注   | ❌   | `internal/pipeline` 只有注册表骨架，无 `join`/`toJson`/`toYaml`/`quote`/`indent`                                                     |
| TASK-018 安全 in-place 原子写回         | 未标注   | ❌   | `--in-place` 报未实现；`output` 只有 stdout/file writer                                                                              |
| TASK-019 批量/多文件渲染                | 未标注   | ❌   | `--output-dir`/`--recursive` 报未实现                                                                                                |
| TASK-020 `--unset`/`--empty` 策略       | 完成     | ❌   | 仅 `error` 生效；`empty`/`keep`/`zero` 报未实现；`--empty` **完全未接线**                                                            |
| TASK-021 P0 golden/E2E                  | 未标注   | 🟡   | 67+4 个 fixture 全绿，但缺 values/.env/schema/pipeline/输出类 case                                                                   |
| TASK-022 多架构静态构建                 | 未标注   | 🟡   | justfile 与 release.yml 已写，无产物校验                                                                                             |
| TASK-023 性能基准                       | 未标注   | ❌   | 无 benchmark                                                                                                                         |
| TASK-024 安全与供应链                   | 未标注   | ❌   | 无 govulncheck/SBOM/checksums 接入                                                                                                   |
| TASK-025 CLI 文档/迁移/示例             | 未标注   | ❌   | `examples/` 为空，README 仍写 “Early skeleton”                                                                                       |
| TASK-026 试用发布                       | 未标注   | ❌   | 未开始                                                                                                                               |

实际完成： **10 / 27**（TASK-001 计为部分）。Sprint 1 只完成语法与操作符三件，`TASK-008~010` 未落地。

## 3. 真实 CLI 探针结果（关键不符项）

| 探针                                    | 期望（文档）       | 实测                                                        |
|-----------------------------------------|--------------------|-------------------------------------------------------------|
| `APP_PORT=8080 render` 中 `${app.port}` | 渲染出 8080        | ❌ 不解析 env 回退，退出 6（仅单段名 `${PORT}` 会回退 env） |
| `render -f values.yaml`                 | 加载值文件         | ❌ `value files are not implemented yet`，退出 1            |
| `render --schema schema.json`           | 校验               | ❌ `schema files are not implemented yet`，退出 1           |
| `render --set app.port=1234`            | 类型化覆盖         | ❌ `CLI value overrides are not implemented yet`，退出 1    |
| `render -i tmpl`                        | 原地写回           | ❌ `--in-place is not implemented yet`，退出 1              |
| `render --unset=keep                    | empty              | zero`                                                       | 4 种策略 | ❌ 除 `error` 外全部报未实现 |
| `render --only A` / `--ignore A`        | 限定替换范围       | ❌ 被静默忽略，输出与不加 flag 完全一致                     |
| `render --empty=error`                  | 空值报错           | ❌ 被静默忽略，永不生效                                     |
| `render --schema-unknown=error`         | 严格模式           | ❌ 被静默忽略                                               |
| `render --unset=bogus`                  | 用法错误（退出 2） | ❌ `not implemented`，退出 1                                |
| `render --validate-only`                | 仅校验             | 🟡 仍执行渲染，缺变量即退出 6（与“只做校验”语义有出入）     |
| `${app.port:-9090}` 缺省                | 输出 9090          | ✅                                                          |
| `$$`、`$1` 字面量                       | 原样输出           | ✅                                                          |
| `--unset=error` 缺变量                  | 退出 6             | ✅                                                          |

## 4. 文档与实现的一致性问题

1. **`docs/99-archive/00-raw/00-04` §26–31 与 `docs/99-archive/06-review/06-01` §12–18 只记录到 TASK-007**，
   而 `05-01` 的进度行（第 12 行）却写着 “TASK-001…007 已完成”——从 TASK-008 起没有实现证据。
2. **`05-01` 第 12 行写“TASK-001、TASK-027、TASK-002…TASK-007 已完成”，按字面不含 TASK-020**；
   但 TASK-020 的验收标准（`error/empty/keep/zero` 四种策略）实测只实现 `error`。
   需要澄清该行是“仅列到 007”还是“Sprint 0+1 早期任务全部完成”——现状是
   `--unset` 非 `error` 分支与 `--empty` 都属于“CLI 已暴露但未生效”的接口空洞（silent no-op）。
3. **`docs/05-tasks/05-02` §3 声称 fixture 覆盖 `.env`/YAML/JSON/多文件/schema/pipeline/输出**，
   实际 fixture 只有 syntax、defaults、types、errors 四类，与 §2 的矩阵维度差距明显。
4. **`README.md` 的 “P0 scope” 列了 7 项，其中 5 项尚未实现**，对外部读者是过度承诺。
5. **`docs/03-design/03-08` §3 的全局参数 `--no-color`、`--json-errors` 未实现**，
   §4 的 `--set-file` 未实现（`--set-file` 未在 `constants.go` 中登记）。
6. `internal/source/*`、`internal/pipeline/*` 共 6 个文件是 **未被引用的死代码**；
   `schema.Validate` 同样未被调用。它们会让 `go test ./...` 显示“ok/no test files”，掩盖真实空白。

## 5. 建议的修复顺序

1. **先修文档口径**：把 `05-01` 的进度行改为“TASK-001～007 已完成；TASK-008 起未开始”，
   修正 `05-02` §3 的覆盖声明；README 的 P0 scope 标注为“目标”而非“现状”。
2. **堵住 silent no-op**：`--only`/`--ignore`/`--empty`/`--schema-unknown`/`--unset=<未支持值>`
   要么接线，要么像 `-f`/`--schema` 一样显式返回“未实现”，不要静默忽略。
3. **修 CI 与格式**：`ci.yml`/`release.yml` 的 Go 版本改为 `1.27.x`（或与 `go.mod` 对齐，
   并把 `go 1.27.1` 降为 CI 可得的版本）；对 `internal/source` 两个文件执行 `gofmt`。
4. **进入 TASK-008/009/010**：`.env` 与 YAML/JSON loader 已写好，只差接线（值文件 + `--env-file` + 合并优先级），
   这是解锁 TASK-011～021 的前置。
5. **补齐 fixture 维度**：在 `testdata/` 增加 values/env/merge 目录，保持“测试先行”约定。
6. **清理或落地死代码**：`internal/pipeline` 与 `internal/schema` 建议先删除骨架，
   待 TASK-011/017 真正实现时再引入，避免误判进度。

## 6. 环境备注

1. 共享工具链的 `GOCACHE` 落在只读卷上，裸跑 `go build ./...`/`go test ./...` 会报
   `read-only file system`。核查时用 `GOCACHE=$PWD/.tmp/gocache` 绕过。
   建议在 `justfile` 或 `CONTRIBUTING.md` 里给出可写 `GOCACHE` 的说明。
2. 仓库当前 **不是 git 仓库**（无 `.git`），`05-01`/`06-01` 里的“完成”标记没有版本历史可追溯。

## 非目标（本部分）

- 本报告只核查进度与一致性，不修改任何实现代码。`[FACT]`
- 不替代需求提出者对 TASK-020 等状态口径的最终确认。`[ASSUMPTION]`
- 不评估 P1/P2 设计。`[FACT]`
