---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# envsubst 兼容基线：GNU / a8m / drone

> 目的：解释“兼容基线”的含义，比较三个候选目标，并给出建议。  
> 来源：GNU envsubst 本地验证、a8m/envsubst README、drone/envsubst README。

## 1. 什么是“兼容基线”

“兼容基线”不是简单指支持 `$VAR`，而是选择某个实现作为以下行为的参照标准：

1. 模板表达式语义：`$VAR`、`${VAR}`、默认值、转义、字符串函数。
2. 未设置/空值行为：替换为空、保留原样、警告还是报错。
3. 变量范围：替换全部变量、白名单、忽略 `$1` 等。
4. CLI/退出行为：参数、输入输出、错误码、严格模式。
5. 输出保真：最后一行、换行、空值、二进制/大文件。

选择一个基线，意味着兼容性测试以它为准；其他实现的行为差异要么通过模式隔离，要么明确不兼容。`[INFERENCE]`

## 2. 三者定位

| 候选           | 定位                 | 形态                            | 对用户的意义                                           |
|----------------|----------------------|---------------------------------|--------------------------------------------------------|
| GNU envsubst   | gettext 官方标准工具 | C 程序，Linux/Unix 常预装       | 最大众的 `envsubst` 命令行为基线；模板简单、无高级语法 |
| a8m/envsubst   | 本项目的升级对象     | Go 库 + CLI，预编译多平台       | 现有 a8m 用户迁移；支持默认值和 `-no-*` 严格标志       |
| drone/envsubst | Bash 风格替换库      | Go 库，主要供 Flux/Drone 等集成 | GitOps/K8s 语义；支持更多 shell 参数展开               |

来源：GNU 文档与本地验证；https://github.com/a8m/envsubst；https://github.com/drone/envsubst。`[FACT]`

## 3. 能力对比

| 能力                                 | GNU envsubst                   | a8m/envsubst                                          | drone/envsubst     | 标注                |
|--------------------------------------|--------------------------------|-------------------------------------------------------|--------------------|---------------------|
| `$VAR` / `${VAR}`                    | 支持                           | 支持                                                  | 支持               | `[FACT]`            |
| `${VAR:-default}`                    | 不支持                         | 支持                                                  | 支持               | `[FACT]`            |
| `${VAR-default}`                     | 不支持                         | 支持                                                  | 支持               | `[FACT]`            |
| `${VAR:=default}` / `${VAR=default}` | 不支持                         | 支持                                                  | 支持               | `[FACT]`            |
| `${VAR+other}` / `${VAR:+other}`     | 不支持                         | 支持                                                  | README 列为不支持  | `[FACT]`            |
| `${VAR:?default}`                    | 不支持                         | 未在 README 中声明                                    | README 列为不支持  | `[TODO]`            |
| 大小写转换/offset/替换               | 不支持                         | 未在 README 中声明                                    | 支持               | `[FACT]`            |
| `$$var` -> `$var`                    | 不转义，保留 `$$`              | 支持                                                  | 未在 README 中声明 | `[FACT]` / `[TODO]` |
| `$1` 行为                            | 不替换                         | 默认替换；可用 `-no-digit` 禁止                       | README 未声明      | `[FACT]` / `[TODO]` |
| 未设置变量                           | 替换为空                       | 默认空；`-no-unset` 报错                              | README 未声明      | `[FACT]` / `[TODO]` |
| 空值变量                             | 替换为空                       | 默认空；`-no-empty` 报错                              | README 未声明      | `[FACT]` / `[TODO]` |
| 变量白名单                           | 支持位置参数 `envsubst '$VAR'` | 有受限 API；CLI 未强调                                | 未在 README 中声明 | `[FACT]` / `[TODO]` |
| CLI                                  | 有                             | 有 `-i`、`-o`、`-no-unset`、`-no-empty`、`-fail-fast` | 主要作为库         | `[FACT]`            |
| 跨平台静态单文件                     | 否，随 gettext 分发            | 可构建；历史发布覆盖不足                              | 无独立 CLI         | `[FACT]`            |

## 4. 关键冲突

- GNU 默认不做默认值展开；a8m/drone 做。若默认模式严格跟随 GNU，高级模板会报错或保持原样。`[FACT]`（来源：本地 GNU
  验证、a8m/drone README）
- a8m 默认替换 `$1`，GNU 不替换；如果默认跟随 a8m，会破坏部分 NGINX/脚本模板。`[FACT]`（来源：a8m issue #38、GNU 本地验证）
- drone 支持更多 bash 字符串函数，但不支持 `+`/`:+`；a8m 支持 `+`/`:+`，但缺少 drone 的部分函数。`[FACT]`（来源：两者 README）
- 三者 CLI/错误行为不同；一个默认行为无法同时满足。`[INFERENCE]`

## 5. 建议方案

建议采用“一个核心引擎 + 显式兼容模式”，而不是选一个基线覆盖全部：

- **默认 CLI 基线：GNU。**  
  当可执行名为 `envsubst` 或用户未指定模式时，优先保持 GNU 语义：`$VAR`/`${VAR}`、白名单、`$1` 不替换、默认值不展开、未设置变量为空。
  `[INFERENCE]`（依据：平台/DevOps 和 CI 用户优先级最高，最大兼容面是系统 envsubst）
- **迁移模式：`--compat=a8m`。**  
  支持 a8m 的默认值表达式、`$$` 转义和 `-no-digit`/`-no-unset`/`-no-empty`/`-fail-fast`；不刻意复现已知 bug，差异写入迁移文档。
  `[INFERENCE]`（依据：a8m 是本项目升级对象）
- **Bash/GitOps 模式：`--compat=drone`。**  
  支持 drone 的 shell 参数展开和字符串函数，用于 Flux/K8s/GitOps 类模板。`[INFERENCE]`（依据：drone 在 GitOps 生态的历史使用）
- **扩展模式：`--compat=extended`（或 `--compat=native`）。**  
  启用类型 schema、数组/map、多文件分层、profiles、dry-run/diff/check 等现代能力；默认值语法建议兼容 a8m 子集，但扩展能力不承诺与
  GNU byte 级一致。`[INFERENCE]`（依据：类型校验为 P0 的确认）
- **模式选择规则建议**：
    - `envsubst` 别名/同名调用：默认 `--compat=gnu`。`[ASSUMPTION]`
    - 新命令名（如 `render`/`xsubst`）调用：默认 `--compat=extended`。`[ASSUMPTION]`
    - 也可统一要求用户显式传 `--compat`，避免隐式行为差异。`[ASSUMPTION]`
      验证：由需求提出者确认默认命令与模式策略；owner：待指定。

## 6. 非目标（Non-Goals）

- 不再以“同时完全兼容三者所有行为”为目标。`[ASSUMPTION]` 验证：确认可接受差异文档；owner：产品/技术负责人。
- 不复制 a8m 已知 bug 作为兼容行为。`[ASSUMPTION]` 验证：确认迁移策略；owner：技术负责人。
- 不把 drone 全部 bash 语义纳入核心模型，超出部分由兼容模式隔离。`[ASSUMPTION]` 验证：确认 GitOps 兼容范围；owner：技术负责人。

## 7. 待补充

- GNU / a8m / drone 的完整行为测试矩阵与 fixture。`[TODO]` owner：技术负责人。
- `${VAR:?}`、`$1`、`$$`、drone 默认值等具体边界案例。`[TODO]` owner：技术负责人。
- 是否保留 `envsubst` 命令名，以及多模式默认值策略。`[TODO]` owner：需求提出者/产品负责人。

## 8. 分期决策（2026-09-29）

- 需求提出者确认采用方案 A：窄兼容 + 转换 + 原生扩展，但分期实施。
- 一期只实现 extended 原生模式，不实现 GNU/a8m/drone 兼容模式，也不实现 migrate/convert 命令。
- 二期再评估并实现兼容模式与自动转换命令。
- 一期 extended 可支持 `$VAR`、`${VAR}` 等基础写法，但不承诺逐字节兼容任一 envsubst 实现。
- 详见 `docs/04-decisions/04-02-adr-002-compatibility-and-migration.md`。
