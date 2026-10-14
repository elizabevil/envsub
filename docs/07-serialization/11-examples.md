> 本文件是[序列化规则](00-index.md)的一部分，覆盖 §15。
> 规则编号全局唯一；跨篇引用一律保持 §x.y 形式（见索引的章节映射表）。

## 15. 复现命令

**本章解决**：提供可复制的验证片段，对应各章规则。

```bash
just build            # 或 just build-sonic

# 整数与空值编码（§3.4-§3.6、§10）
./dist/envsub render --set-json 'x=18446744073709551615' <(printf '${x | toJson}\n')  # = 2^64-1
./dist/envsub render --set-json 'x=123456789012345678901234567890' <(printf '${x | toYaml}\n')
for v in null '{}' '[]'; do ./dist/envsub render --set-json "x=$v" <(printf '${x | toJson}\n'); done

# 浮点 / number（§3.7、§3.10、§3.11）
./dist/envsub render --set-json 'x=1e309' <(printf '${x} ${x | toJson} ${x | toYaml}\n')  # 未声明：保留 1e309（FB2 已实现）
./dist/envsub render --set-json 'x=1.0e30' <(printf '${x}\n')
./dist/envsub render --set 'x=123456789012345678901234567890' <(printf '${x | type float}
')  # exit 4：float64 无法精确表示
for v in 1.0 1.50 1e3; do ./dist/envsub render --set-json "x=$v" <(printf '${x | toJson} ${x | toYaml}\n'); done

# 非有限值（F8，已实现：解码/物化/编码三路径 exit 4）
printf 'x: .nan\ny: .inf\n' > /tmp/nan.yaml
./dist/envsub render -f /tmp/nan.yaml <(printf '${x}\n')
./dist/envsub render -f /tmp/nan.yaml <(printf '${x | toYaml}\n')
./dist/envsub render -f /tmp/nan.yaml <(printf '${x | toJson}\n')   # exit 4（F8 已实现）

# CLI 值来源与优先级（§6.1、§6.2；--set-string 已删除）
./dist/envsub render --set 'x=8080'        <(printf '${x} ${x | toJson}\n')
./dist/envsub render --set 'x=8080' <(printf '${x} ${x | toJson}\n')   # --set 存字符串
./dist/envsub render --set-json 'x=8080'   <(printf '${x} ${x | toJson}\n')
./dist/envsub render --set 'x=1' --set 'x=2' --set-json 'x=3' <(printf '${x}\n')  # 3

# 空值与 bash 语义（§5.3）
for v in '""' null 0 false '[]' '{}'; do
  ./dist/envsub render --set-json "A=$v" <(printf 'p=${A} d=${A:-d} a=${A:+a}\n')
done

# 策略开关（§6.4）
./dist/envsub render --on-empty=error --set-json 'A=""' <(printf '${A}\n')   # exit 6
./dist/envsub render --on-missing=zero <(printf '${A}\n')                    # exit 6
./dist/envsub render --on-empty=keep <(printf 'x\n')                         # exit 2（G19）

# 类型名（§8）：uint 不提供
./dist/envsub render --set 'x=1' <(printf '${x | type uint}\n')       # exit 2
./dist/envsub render --set-json 'x=""'   <(printf '${x | type null}\n')      # G18：exit 4（已实现；G9 非法 JSON 统一 exit 4）

# schema（§9）
printf 'type: object\nproperties:\n  port: {type: integer, default: 8080}\n' > /tmp/s.yaml
./dist/envsub render --schema /tmp/s.yaml <(printf '${port}\n')              # 8080

# 后端/编码器对照
go test -v ./test/encoding/ && go test -v -tags=sonic ./test/encoding/
go test -v ./test/jsonx/ && go test -v ./test/yamlx/
just test && just test-e2e
```

