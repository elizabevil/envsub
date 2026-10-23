---
status: active
owner: 技术负责人
version: 0.1
last-updated: 2026-10-22
---

# `--explain` 八项修复：通俗说明与最小案例

> 面向不熟悉内部实现的读者。每条只回答三件事：**原来哪里不对、为什么、修完长什么样**。
> 精确契约见 [`../07-serialization/13-materialization.md`](../07-serialization/13-materialization.md) §15
> 与 [`../04-decisions/04-05-adr-005-explain-origin-model.md`](../04-decisions/04-05-adr-005-explain-origin-model.md)。

## 0. 先理解报告在说什么

`--explain` 不输出值，只输出"这个变量是从哪来的"。它把每个变量的来源画成一条链：

```text
低优先级 ───────────────────────────────────────────────► 高优先级
schema default → 值文件 → .env → --set → 模板里的 ${K:=v} 写回
```

报告里只有两个字段和这条链有关：

- `origin`：最后胜出的那一笔来源；
- `shadowed`：被胜出者盖住的来源，最近的在前。

所以读报告的方法很简单：**`shadowed` 后面接 `origin`，就是完整的来源链**。
八条 bug 本质上都是同一句话：**报告里的链，和值树里真正发生的事对不上**。

> 下面的 JSON 为便于阅读省略了空字段和部分 `file`/`line`；真实报告的五字段恒在。

---

## E1：模板写回被漏掉

### 最小案例

`values.yaml`：

```yaml
K: ""
```

`input.tmpl`：

```text
first=${K:=written}
second=${K}
```

`K` 是空值，所以 `:=` 会真的把 `written` 写回值树；后面的 `${K}` 读到的也是 `written`。

```bash
envsub render --no-process-env -f values.yaml --explain input.tmpl
```

### 修复前

渲染结果是对的（`first=written`、`second=written`），但报告只写了：

```json
{"variable":"K","origin":{"kind":"values","source":"values.yaml"},"shadowed":[]}
```

看起来 `K` 一直来自值文件，完全看不到模板写过它。

### 修复后

```json
{"variable":"K",
 "origin":{"kind":"assign","method":"assign","source":"input.tmpl","line":1},
 "shadowed":[{"kind":"values","source":"values.yaml"}]}
```

最后胜出的是模板第 1 行的写回；原来的空值来源退到 `shadowed`。

### 一个容易误会的点

如果 `K: "x"`（非空），`:=` **不会**写回，报告只显示值文件是**对的**。
只有赋值真正发生时，才应该有 `assign` 帧。

---

## E2：报了一个"够不着"的 schema default

### 最小案例

`schema.json` 声明：

```json
{"type":"object","properties":{
  "app":{"type":"object","properties":{
    "port":{"type":"integer","default":8080}
  }}
}}
```

但值文件是空的，**没有 `app` 这个对象**。模板：

```text
${app.port}
```

```bash
envsub render --no-process-env --schema schema.json --on-missing=keep --explain input.tmpl
```

`--on-missing=keep` 让缺失值原样保留，这样进程不会先因 missing 退出。

### 修复前

渲染结果是 `${app.port}`（没填上，因为父对象 `app` 不存在），
报告却说：

```json
{"variable":"app.port","origin":{"kind":"schema","method":"default","source":"#/properties/app/properties/port/default"}}
```

**报告说来自 default，实际 default 根本没被应用。**

### 为什么不能简单改成"只报实际填了的 default"

假设 `.env` 提供了 `K`，schema 里也有 `K` 的 default。
这时 default 同样没被填（因为 `K` 已经有值），但它是真实存在、只是输给了 `.env` 的兜底层，
按 ADR-005 I6 必须留在 `shadowed` 里。

所以真正的判据不是"填没填"，而是：

> **这个 default 的路径，在值树里有没有一笔精确记录？**

- 有：说明该路径要么被真实来源提供（default 被覆盖），要么 default 自己被应用 → 可以进链；
- 没有：父对象都不存在，这个 default 够不着 → 不报。

### 修复后

没有 `app` 时：

```json
{"variable":"app.port","origin":{"kind":"missing","method":"missing"},"shadowed":[]}
```

有 `app`（例如值文件写了 `app.host`）时，default 会被应用，报告才写 `schema/default`。
被 `.env` 覆盖的 `K` 也仍然留在 `shadowed` 里。

### 进阶案例：父 default 是对象

```json
{"properties":{"app":{
  "type":"object",
  "default":{"port":1},
  "properties":{"port":{"type":"integer","default":2}}
}}}
```

实际值是 `1`，来自 `app` 的 default；`port` 自己的 default `2` 没有生效。
修复前报告会指向 `port` 的 default，修复后指向 `app` 的 default。

---

## E3：不存在的子路径借用了父路径的来源

### 最小案例

`values.yaml`：

```yaml
app:
  port: 1
```

模板：

```text
${app.port.x}
```

`app.port` 是一个数字，`app.port.x` 根本不存在。

```bash
envsub render --no-process-env -f values.yaml --on-missing=keep --explain input.tmpl
```

### 修复前

报告说 `app.port.x` 来自 `values.yaml`——因为它找不到自己的来源时，借用了最近的父路径
`app.port` 的来源。

**父路径在哪，不等于子路径在哪。**

### 修复后

```json
{"variable":"app.port.x","origin":{"kind":"missing","method":"missing"},"shadowed":[]}
```

### 但有一个例外要保留

如果 schema 把一段字符串物化成对象：

```yaml
app: '{"port": 8080}'
```

物化后的 `app.port` **值确实存在**，只是它没有自己单独的来源记录，只有 `app` 有。
这时报告允许退回父来源，说它来自 `values.yaml`，否则就会把存在的值误报成 `missing`。

所以规则是：

1. 先找精确来源；
2. 精确来源没有，但**值存在** → 退回最近父来源；
3. 值不存在 → `missing`。

---

## E4：没被替换的变量，被报告成"有来源"

### 最小案例

```text
模板：${B}
命令：--set B=from-set --only-vars A --explain
```

`--only-vars A` 表示只有 `A` 参与替换，所以 `${B}` 会原样留在输出里，根本没有解析过 `B`。

### 修复前

报告仍然写：

```json
{"variable":"B","origin":{"kind":"set","method":"flag","source":"B"}}
```

### 修复后

报告里没有 `B`。规则与 `--report=unused` 一致：

> **没被替换的变量，不能算作"解析到了某个来源"。**

### 嵌套的情况也一起修了

模板 `${A:-${B}}`，命令 `--ignore-vars A`：

外层 `A` 被排除，整个 `${A:-${B}}` 会原样输出，`B` 也不会被求值。
修复后报告不再单独列出 `B`。

---

## E5：只出现在默认值里的变量完全没报

### 最小案例

```text
模板：${A:-${B}}
命令：--set B=from-set --explain
```

`A` 不存在，所以输出会采用默认词 `${B}`，而 `B` 来自 `--set`。

### 修复前

报告只有 `A`：

```json
{"variable":"A","origin":{"kind":"missing"}}
```

`B` 完全消失了。

### 为什么

枚举变量时只看了操作符**前面**的路径 `A`，没有把操作符**后面**的默认词当成一段小模板来扫描。
默认词本身也是模板，里面当然可以有 `${B}`。

### 修复后

```json
{"variable":"A","origin":{"kind":"missing"}}
{"variable":"B","origin":{"kind":"set","method":"flag","source":"B"}}
```

两个引用都能解释。

---

## E6：报告写失败，把真正的渲染错误顶掉了

### 最小案例

```text
模板：${MISSING}
命令：envsub render --explain input.tmpl 2>/dev/full
```

`/dev/full` 用来模拟"写报告时磁盘满"。

### 修复前

进程以 exit 5（I/O 错误）退出，错误信息是写报告失败；
真正的 `MISSING` 未定义（exit 6）反而看不到了。

### 修复后

进程以 exit 6（missing）退出，并同时给出两条信息：

- 渲染失败：`variable "MISSING" is not set`
- 报告失败：`write explain: ... disk full`

原则：**报告是诊断工具，不能盖住被诊断的错误。** 渲染错误放在前面，退出码和错误分类仍按渲染错误算。

---

## E7：`kind=set` 配 `method=assign`，抽屉对不上

### 最小案例

模板 `${K:=written}` 的写回帧原来是：

```json
{"kind":"set","method":"assign"}
```

但 `set` 这个抽屉只允许三种入口：`flag`（`--set`）、`json`（`--set-json`）、
`file`（`--set-file`）。`assign` 不属于它。

### 修复后

```json
{"kind":"assign","method":"assign"}
```

新增了 `assign` 这个层位。可以这样理解：

- `kind` = 值在哪一层（schema / values / envfile / set / assign / process / missing）；
- `method` = 它是怎么进这一层的；
- 每一层的 method 有固定允许集合，不能随便配。

---

## E8：stderr 混了多种格式，CI 没法直接解析

### 最小案例

渲染失败时，修复前的 stderr 长这样：

```text
{"template":"input.tmpl","variable":"MISSING","origin":{"kind":"missing"}}
variable "MISSING" is not set
```

第一行是 explain JSON，第二行是错误文案；如果开了 warn 日志，还会多出 slog 文本。
CI 想按行解析，只能猜哪一行是报告、哪一行是错误，`grep '{'` 也会把错误 JSON 一起抓进来。

### 修复后：统一信封 + 特化载荷

新增 `--jsonl`；`--explain` / `--report=unused` 会自动开启它。开启后 stderr 每行都有同一组
信封字段，差异只在 `data`：

```json
{"schema":1,"type":"explain","time":"2026-10-22T23:43:37.041Z","seq":1,"data":{"template":"input.tmpl","variable":"MISSING","origin":{"kind":"missing"},"shadowed":[]}}
{"schema":1,"type":"error","time":"2026-10-22T23:43:37.041Z","seq":2,"data":{"kind":"missing","exitCode":6,"message":"variable \"MISSING\" is not set"}}
```

| `type` | `data` 内容 |
|---|---|
| `explain` | `template`/`variable`/`origin`/`shadowed` |
| `unused` | `path` |
| `error` | `kind`、`exitCode`、`message`；批量失败一条错误一行 |
| `log` | `level`、`message`、`attrs` |

时间只有一个来源：`internal/timefmt` 统一生成 UTC、固定毫秒、`Z`；`seq` 解决同毫秒排序。
文本日志和 `version.buildDate` 也走同一个格式化函数，不再各写一套。

CI 可以直接过滤：

```bash
envsub render --explain -f values.yaml input.tmpl > out.txt 2> events.jsonl
jq -c 'select(.type=="error")' events.jsonl
jq -c 'select(.type=="explain" and .data.variable=="app.port")' events.jsonl
```

### 边界

- stdout 仍然是纯渲染结果，`--jsonl` 只影响 stderr。
- 退出码不变，仍按 2/3/4/5/6/7 分类；`data.exitCode` 与进程退出码一致。
- `--jsonl` 优先于 `--error-format`；`--error-format=json` 是机器模式的兼容别名，也输出同一信封。
- 想保留人类可读的错误文案，可以显式加 `--jsonl=false`（这时 `--explain` 的记录仍是事件信封）。

---

## 一张表记住八条规则

| # | 一句话规则 |
|---|---|
| E1 | 模板最后写回的那一笔，必须出现在链尾 |
| E2 | 够不着的 default 不是来源；被覆盖但够得着的 default 仍要保留 |
| E3 | 值不存在时，子路径不能借父路径的来源；值存在时才允许回退 |
| E4 | 没被替换的变量，不报告来源 |
| E5 | 默认词也是模板，里面的引用也要枚举 |
| E6 | 报告失败不能吞掉渲染错误 |
| E7 | `kind` 和 `method` 必须来自同一套允许集合 |
| E8 | stderr 用 `--jsonl` 统一成“信封 + `data`”的 JSONL 事件流，时间只有一个来源 |

## 可观察的行为变化

1. `--explain` 的赋值帧从 `kind=set` 改为 `kind=assign`；
2. 父对象不存在的 schema default 不再出现在报告里；
3. `--report=unused` 也会排除被 `--only-vars`/`--ignore-vars` 排除的引用（含嵌套引用）；
4. `--explain`/`--report=unused` 自动开启 `--jsonl`：stderr 每行统一为
   `schema`/`type`/`time`/`seq`/`data` 信封，载荷按类型特化；时间统一为 UTC 固定毫秒；
   `--jsonl=false` 可退回人类可读诊断；`--jsonl` 优先于 `--error-format`。

## 自己验证

```bash
go test ./...
golangci-lint run ./...
python3 tools/docs/checklinks.py
```

对应的实现与回归测试见：

- `internal/render/explain.go`、`internal/load/load.go`、`internal/schema/defaults.go`
- `internal/engine/extended/explain.go`、`internal/render/render.go`
- `internal/render/explain_test.go`、`internal/render/report_error_test.go`、`internal/render/unused_internal_test.go`
- `internal/load/load_test.go`、`internal/engine/extended/explain_test.go`
- `internal/app/jsonl_test.go`、`internal/logging/logging_test.go`
- `test/e2e/e2e_test.go`
