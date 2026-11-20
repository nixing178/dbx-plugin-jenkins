# 0.13.2 Active Choices 级联参数

## 问题

`test-buss-optional` 的 `BRANCH` 和 `BUILD_MODULES` 是 `org.biouno.unochoice.CascadeChoiceParameter`，不能从普通任务 JSON 的 `choices` 或静态 Groovy 字面量取得。旧版因此把两者回退为文本框，且模块无法随分支刷新。

## 修复

- `GetParameterOptions` 读取 Jenkins 构建参数表单，按 `data-proxy-name` 绑定同名 Stapler 代理，调用 `doUpdate` 和 `getChoicesForUI` 计算动态选项；不在本地执行 Groovy。
- 代理只接受表单生成的 `$stapler/bound/<UUID>` 路径，方法只允许 `doUpdate` / `getChoicesForUI`；复用客户端 Basic Auth、会话 Cookie 和 crumb。
- Jenkins 2.529 的参数表单 GET 可以返回带完整表单的 HTTP 405，仅该读取路径兼容此响应，其它 API 错误处理不变。
- `jenkins/parameterOptions` 将动态选项传给参数弹窗；分支变化后重新计算引用它的模块，过期响应被丢弃。
- 单选保留下拉框，多选保留复选框和逗号拼接提交格式；保留有效选择，清理失效值，支持 Jenkins 的 selected / disabled 标记。
- 普通 Choice、静态 Active Choices 与其它参数维持原行为。

## 验证

- `go test ./... -count=1`：通过，覆盖动态表单、级联引用参数、多选、禁用/选中项、静态参数不读表单、RPC 以及禁止触发构建请求。
- `go vet ./...`：通过。
- `pnpm test`：29 个测试文件、416 个用例通过。
- `pnpm typecheck`、`pnpm build`：通过，版本一致性检查为 0.13.2。
- `@dbx-app/plugin-cli@0.1.9 package .`：生成 `io.github.nixing178.jenkins-0.13.2-windows-x64.dbxp` 本地未签名包。
- 使用修复后的 Go 客户端对真实 Jenkins 做只读验证：`BRANCH` 返回 37 个选项；`origin/test` 和 `origin/dev` 均返回 `buss-mock`、`common-component`、`iot-codec`、`iot-connector`、`iot-log` 五个模块，且 `BUILD_MODULES` 正确识别为引用 `BRANCH` 的多选参数。
- 所有真实验证均未触发 Jenkins 构建；未以 `test-buss-optional` 代替允许的 `analytics-service` 做构建测试。
