---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-10-14
---

# ADR-006：批量输出名去掉模板后缀（Accepted）

## 1. 标题

ADR-006：`--output-dir` 的输出名用 `--strip-suffix EXT` **显式**去掉模板后缀

## 2. 状态

Accepted，2026-10-14。新增 CLI 选项，默认关闭；`REQ-OUT-11`（P2）由此闭环。

## 3. 背景

模板仓库习惯用 `*.tmpl` 命名：便于 `*.tmpl` glob 选取，也能一眼区分"模板"与"产物"。但
`--output-dir` 会**原样保留**模板文件名，产物因此带着模板后缀，下游（nginx/k8s/docker/GitOps、
`git`/`diff`/`kubectl apply -f`）需要额外改名。实测：

```
$ envsub render --output-dir out tpl/nginx.conf.tmpl tpl/service.yaml.tmpl
out/nginx.conf.tmpl      out/service.yaml.tmpl        # 后缀被保留
$ envsub render --check --output-dir out2 tpl/nginx.conf.tmpl
1 of 1 outputs are outdated: out2/nginx.conf.tmpl      # 诊断里也是这个名字，exit 7
```

用户当前只有两条出路，都有代价：模板不写 `.tmpl`（丢掉模板信号与 glob 能力），或渲染后
`for f in out/*.tmpl; do mv …`（多一步、易漏、CI 里要额外脚本）。

## 4. 决策

新增 `--strip-suffix EXT`（可重复，默认关）：

| 维度 | 决策 |
|------|------|
| 作用范围 | **仅 `--output-dir`**；`-o` 用显式路径、`-i` 写回原文件，均不受影响 |
| 匹配 | 对输出**文件名**做字面量、区分大小写的后缀匹配 |
| 多值 | **最长匹配优先**，与书写顺序无关（`.yaml.tmpl` 比 `.tmpl` 剥得多） |
| 未匹配 | 文件名原样保留 |
| 空值 | `--strip-suffix ''` → `KindUsage` exit 2 |
| 缺 `--output-dir` | `KindUsage` exit 2（不做静默忽略） |
| **撞名** | 两个模板解析到同一输出 → `KindUsage` exit 2，报出两个模板与目标，**不静默覆盖** |
| `--check` | 复用同一路径计算，诊断显示剥离后的目标名 |

契约与案例见 `07-serialization/04-cli.md` C18。

## 5. 备选与取舍

| 备选 | 做法 | 结论 |
|------|------|------|
| A 只文档化 | 说明"想要干净名字就别用 `.tmpl` 命名" | ❌ 让用户放弃模板命名惯例与 glob 能力，问题仍在 |
| **B 显式开关（选用）** | `--strip-suffix EXT`，默认关 | ✅ 显式、可预测、默认行为零变化 |
| C 自动剥离 | 见到 `.tmpl`/`.tpl` 自动去掉 | ❌ 隐式且静默改变既有行为；违背"显式 > 隐式"；可能剥掉用户真正想要的后缀 |

选择 B 时还有一个形态问题：**内置后缀清单**还是**用户给定 EXT**？选后者——不把
`.tmpl/.tpl/.gotmpl` 这类策略藏进实现，用户写什么就剥什么；重复给出多个后缀时用最长匹配
消除顺序依赖。

## 6. 后果

- 正面：产物名可控且可预期；默认行为完全不变；批量与 `--check` 命名一致；撞名从"静默覆盖"
  变成可诊断的用法错误。
- 代价：新增一个选项及其文档面；用户需要在批处理命令里写一次（示例见 `04-cli` C18）。
- 不改变值语义、不影响 `-o`/`-i`、不触碰既有 fixture 的默认命名。

## 7. 非目标

- 不自动推断"哪个后缀是模板后缀"，不做隐式剥离；
- 不改 `-o`/`-i` 的命名语义；
- 不重命名**已存在**的历史产物（只影响本次写入的目标路径）；
- 不引入通配符/正则替换（需要时按需求另立）。

## 8. 验证

| 层 | 证据 |
|----|------|
| 单测 | `internal/render/strip_suffix_test.go`：剥离与布局保留、最长匹配（两种书写顺序）、未匹配不动、撞名 KindUsage、空值/缺 `--output-dir` 的用法错误、`-o` 不受影响、`--check` 报剥离后的目标名 |
| E2E | fixture `strip_suffix`（`out/a`、`out/nested/b`）、`strip_suffix_collision`（exit 2 + `both map to`） |
