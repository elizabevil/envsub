---
status: draft-ai
owner: [ 待指定 ]
version: 0.1
last-updated: 2026-09-29
---

# envsubst 升级版用户故事

> 来源：`02-03-prd.md` 与已确认上下文。  
> 当前只实现 P0 故事；P1/P2 故事保留为待办。

## US-001 基础变量替换

- **描述**：作为平台/DevOps 工程师，我希望用 `$VAR` / `${VAR}` 渲染文本，以便替换配置中的简单占位符。
- **验收条件**：
    - Given 模板包含 `$PORT` 且 PORT 存在，When 渲染，Then 输出替换后的文本。
    - Given 模板包含 `$1`，When 渲染，Then `$1` 按字面量输出。
- **优先级**：P0，基础能力。
- **依赖**：无。
- **估算**：S。

## US-002 默认值与存在性操作

- **描述**：作为平台/DevOps 工程师，我希望使用 `:-`、`-`、`:+`、`+`、`:?`、`?`，以便处理未设置、空值和必填变量。
- **验收条件**：
    - Given `${PORT:-8080}` 且 PORT 未设置或为空，When 渲染，Then 输出 `8080`。
    - Given `${URL:-http://host:8080}`，When 渲染，Then 默认值中的 `:`、`-` 按字面量处理。
    - Given `${PORT:?必须设置}` 且 PORT 缺失，When 渲染，Then 报错并返回非零退出码。
- **优先级**：P0。
- **依赖**：US-001。
- **估算**：M。

## US-003 转义与变量范围控制

- **描述**：作为容器镜像维护者，我希望用 `$$` 输出字面量 `$`，并只替换指定变量，以便避免误伤 NGINX/K8s 内置变量。
- **验收条件**：
    - Given 模板包含 `$$` 和 `$http_host`，When 渲染，Then `$$` 输出 `$` 且 `$http_host` 保持不变。
    - Given 使用变量白名单，When 渲染，Then 只替换白名单内变量。
- **优先级**：P0。
- **依赖**：US-001。
- **估算**：M。

## US-004 JSON Schema 类型校验

- **描述**：作为平台/DevOps 工程师，我希望渲染前按 schema 校验类型和约束，以便避免错误配置进入运行时。
- **验收条件**：
    - Given schema 声明 `port: integer`、minimum=1，When PORT=8080，Then 通过校验并渲染。
    - Given PORT=abc，When 校验，Then 报错并指出变量路径、期望类型和来源。
- **优先级**：P0。
- **依赖**：US-001。
- **估算**：L。

## US-005 内联 `| type`

- **描述**：作为平台/DevOps 工程师，我希望在模板中对少量变量快速声明类型，以便不写完整 schema 也能做局部校验。
- **验收条件**：
    - Given `${PORT | type int}` 且 PORT=8080，When 渲染，Then 通过并输出原值。
    - Given `${HOSTS | type array<string> | toJson}`，When 渲染，Then 通过并输出 JSON 数组。
    - Given `| type` 出现在 formatter 后面，When 解析，Then 直接报错。
    - Given 不写 `| type`，When 渲染，Then 仍可渲染，不强制要求内联类型。
- **优先级**：P0。
- **依赖**：US-004。
- **估算**：M。

## US-006 数组 / map 渲染 pipeline

- **描述**：作为平台/DevOps 工程师，我希望用 `join`、`toJson`、`toYaml`、`quote`、`indent` 渲染数组和对象，以便适配
  YAML/JSON/文本配置。
- **验收条件**：
    - Given `${hosts | toJson}`，When hosts 为数组，Then 输出 JSON 数组。
    - Given `${app | toYaml | indent 2}`，When app 为对象，Then 输出缩进后的 YAML。
    - Given `indent` 输入不是字符串，When 渲染，Then 报错提示先转换类型。
- **优先级**：P0。
- **依赖**：US-001。
- **估算**：M。

## US-007 多文件 values 覆盖

- **描述**：作为平台/DevOps 工程师，我希望按顺序加载多个 values 文件，以便用后加载文件覆盖先加载文件。
- **验收条件**：
    - Given 基础文件和环境文件都声明 `app.port`，When 按顺序加载，Then 环境文件覆盖基础文件。
    - Given 两个文件都声明数组，When 合并，Then 后加载数组整体替换前一个数组。
    - Given CLI `--set` 与 values 文件冲突，Then CLI `--set` 优先。
- **优先级**：P0。
- **依赖**：US-004。
- **估算**：M。

## US-008 安全 in-place 与批量渲染

- **描述**：作为 CI 用户，我希望安全原地写回和批量处理多文件，以便避免 `< file > file` 截断并减少脚本拼接。
- **验收条件**：
    - Given 对同一文件执行 in-place 渲染，When 成功，Then 临时文件原子替换，原文件不丢失。
    - Given 某个输出文件无写权限，When 批量渲染，Then 失败并报告具体文件。
- **优先级**：P0。
- **依赖**：US-001。
- **估算**：M。

## US-009 dry-run / diff / check

- **描述**：作为 CI 用户，我希望预览变更、输出 diff 或检查是否需要更新，以便接入流水线门禁。
- **验收条件**：
    - Given 渲染结果与原文件一致，When 执行 `--check`，Then 退出码 0。
    - Given 渲染结果会改变文件，When 执行 `--check`，Then 非零退出码并报告差异。
- **优先级**：P1。
- **依赖**：US-008。
- **估算**：M。

## US-010 include / 条件 / 循环

- **描述**：作为平台/DevOps 工程师，我希望复用模板片段并按条件/数组生成内容，以便处理复杂但仍受限的配置场景。
- **验收条件**：
    - Given 条件为真，When 渲染，Then 输出对应分支。
    - Given include 嵌套超过 3 层，When 渲染，Then 报错。
    - Given 对象循环，When 渲染，Then 按键排序输出，保证确定性。
- **优先级**：P1。
- **依赖**：US-001、US-007。
- **估算**：L。

## US-011 profiles / environments

- **描述**：作为平台/DevOps 工程师，我希望选择 dev/staging/prod 等环境，以便复用同一模板。
- **验收条件**：
    - Given profile=prod，When 渲染，Then 加载 prod 对应 values。
    - Given profile 不存在，When 渲染，Then 报错并列出可用 profile。
- **优先级**：P1。
- **依赖**：US-007。
- **估算**：M。

## US-012 GNU/a8m/drone 兼容与转换

- **描述**：作为从 envsubst/a8m 迁移的用户，我希望使用兼容模式或转换命令，以便降低迁移成本。
- **验收条件**：
    - Given `--compat=gnu`，When 渲染 `$1`，Then 按 GNU 行为保留 `$1`。
    - Given 使用 `migrate` 命令，When 处理支持模板，Then 输出 native 模板并报告无法转换项。
- **优先级**：P2。
- **依赖**：US-001 至 US-010 稳定。
- **估算**：L。

## 故事间依赖关系图（文字）

```text
US-001 基础替换
  ├── US-002 默认值/存在性
  ├── US-003 转义与范围
  ├── US-004 JSON Schema 类型校验
  │     ├── US-005 内联 | type
  │     └── US-007 多文件 values
  ├── US-006 数组/map pipeline
  ├── US-008 安全 in-place / 批量
  │     └── US-009 dry-run/diff/check
  └── US-010 include/条件/循环
        └── US-011 profiles/environments

US-012 兼容与转换依赖 US-001..US-010 稳定。
```

## 非目标（整体）

- 本用户故事列表不是任务拆分，不替代开发评估。
- 不包含具体技术实现方案。
- 不承诺 P1/P2 进入一期。
