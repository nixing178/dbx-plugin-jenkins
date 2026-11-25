# Active Choices 首次级联默认值修复（0.13.2）

## 问题与证据

`dev-dmo-oss-dev` 首次打开带参数构建时，`BUILD_MODULES` 显示“克隆仓库失败，请检查分支名称”，切换 `REPOSITORY` 后恢复。

真实 Jenkins 只读诊断确认：

- `REPOSITORY` 是普通 `ChoiceParameterDefinition`，合法默认值为 `zcf-data-platform-jobs`。
- `BRANCH` 是引用 `REPOSITORY` 的 `CascadeChoiceParameter`，JSON 默认值为“请先选择代码仓库”。
- `BUILD_MODULES` 同时引用 `REPOSITORY` 和 `BRANCH`，JSON 默认值为“请先选择代码仓库和分支”。
- 首次模块代理收到 `REPOSITORY=zcf-data-platform-jobs__LESEP__BRANCH=请先选择代码仓库` 时，Jenkins 原始响应正是“克隆仓库失败，请检查分支名称”。
- 同一表单、同一会话下改用本轮计算出的 `BRANCH=origin/master` 后，模块列表正常返回。

因此根因不是 Git 仓库故障，而是插件在下游求值前没有采用刚计算出的上游默认值；60 秒选项缓存会继续保留首轮错误结果。

## 修复

- 后端在级联求值前归一化上游当前值：有效用户值优先，其次是 Jenkins selected/default 值，最后才是首个启用项。
- 本轮刚计算出的有效分支立即用于后续模块 `doUpdate`，不再把 JSON 中的占位默认值传给 Jenkins。
- 尚未求值的动态参数保留用户输入；只有返回空选项后才清空无效值，避免缓存命中时错误覆盖用户已选分支。
- 普通 Choice 也会校正无效默认值；普通文本、布尔参数、合法空选项和有效用户选择不变。
- 前端比较请求前后的引用值，归一化发生变化时自动补算下游，并限制补算轮次，防止异常脚本造成循环。

## 验证

- 新增后端真实场景 fixture，断言模块请求必须包含 `REPOSITORY=repo-a__LESEP__BRANCH=origin/master`，不得包含“请先选择”占位值；覆盖普通仓库默认值、缓存隔离及并发分支不串值。
- 新增前端归一化及引用变化检测测试；前端共 30 个测试文件、425 个用例通过。
- `go test ./... -count=1`、`go vet ./...`、`pnpm typecheck`、`pnpm build` 均通过。
- 使用修复后的 Go 客户端对 `dev-dmo-oss-dev` 做真实首次只读加载：默认仓库保持 `zcf-data-platform-jobs`，分支归一化为 `origin/master`，模块首次返回正常列表，不再出现克隆失败提示。
- 所有诊断和验证均未触发 Jenkins 构建；未安装到本机 DBX。
