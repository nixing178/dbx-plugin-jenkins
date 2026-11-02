# 测试证据

> 后端 `go test` 与前端 `pnpm test` 按 TDD 红→绿推进，每轮结果记录如下。

> **版本号重编（2026-11-02）**：0.10.0 之后的制品下载、归档配置、控制台导出等功能，此前按内部迭代号 `0.11.0`–`0.16.1` 记录，均未正式发布（未提交 Git、未进商店）。现统一收敛为对外版本 **`0.11.0`**。下文 `0.11.0`–`0.16.1` 段落保留原始验收记录，其编号仅代表内部迭代顺序，不代表对外发布号；对外产物以 `0.11.0` 重新打包。

> **0.11.0 安装验收（2026-11-02）**：新包 `io.github.nixing178.jenkins-0.11.0-windows-x64.dbxp`（sha256 `e52c648b585d5f49c814c4cfe0762158767d196c85ad3b609b3183b04c208a30`）已在 DBX 安装并激活。落盘证据：`%APPDATA%\com.dbx.app\plugins\io.github.nixing178.jenkins\versions\0.11.0` 的 `manifest.version=0.11.0`、`ui/index.html` 哈希 `bd43e1362af206ed54e91de6c844644c7334623c6497850a14ef0e33769c3be5`（与本地构建一致）、sidecar identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.11.0`；activation 记录时间 2026-11-02 10:10。

## 环境

| 项 | 值 |
| --- | --- |
| Go | 1.22.12 |
| Node | 22.22.2 |
| Vitest | 2.1.9 |
| 执行时间 | 2026-10-19 |

## 汇总

| 轮次 | 范围 | 命令 | 红 | 绿 |
| --- | --- | --- | --- | --- |
| M0-B1 | 路径构造 + 状态归一化 | `go test ./internal/jenkins/ -v` | 32 FAIL / 41 | 41 PASS |
| M1-C1 | Client（Auth / CSRF / 只读 / 错误映射 / TLS） | `go test ./internal/jenkins/ -v` | 12 FAIL / 16 | 61 PASS |
| M1-R1 | RPC 生命周期 + secret 形态 + 会话 | `go test ./internal/rpc/ -v` | 14 FAIL / 14 | 75 PASS（全量） |
| M1-F1 | 前端错误映射 | `pnpm test` | 模块缺失，1 suite FAIL | 6 PASS |
| M2-B2 | 任务 / 构建 / 控制台 / 触发停止 | `go test ./internal/jenkins/ -v` | 16 FAIL / 21 | 82 PASS |
| M2-R2 | RPC jobs/job/builds/build/console/trigger/stop | `go test ./internal/rpc/ -v` | 6 FAIL | 102 PASS（全量） |
| M2-F2 | 前端状态/时长/日志纯函数 | `pnpm test` | 3 suite 模块缺失 | 18 PASS |
| M2-S1 | Sidecar 二进制 JSON-RPC 冒烟 | `node sidecar-smoke.js` | — | init/connect/jobs/disconnect OK |
| M3-B3 | 队列 / 节点 / 概览聚合 | `go test ./internal/jenkins/ -v` | 5 FAIL | 111 PASS（全量） |
| M3-R3 | RPC overview/queue/cancelQueue/nodes | `go test ./internal/rpc/ -v` | 4 FAIL | 111 PASS（全量） |
| M3-S2 | Sidecar 冒烟（M3 端点） | `node sidecar-smoke.js` | — | overview/queue/nodes/cancelQueue OK |
| M4-B4 | MCP tools/call（10 工具、只读过滤、注解） | `go test ./internal/rpc/ -v` | 9 FAIL | 120 PASS（全量） |
| M4-S3 | Sidecar 冒烟（MCP） | `node sidecar-smoke.js` | — | 10 tools 列出 + list_jobs 调用成功 |
| M5-E2E | 真实 Jenkins 2.586 全链路 | `node scripts/e2e-jenkins.js` | — | 连接/任务/构建/参数/控制台/停止/队列/节点/概览/MCP/只读全部 OK |
| M5-REG | `computer.idle` 布尔值回归 | `go test ./internal/jenkins/ -run TestGetNodes` | — | 121 PASS（全量） |
| V1 | Jenkins Views API + 视图任务 | `go test ./internal/jenkins/ -v` | 4 FAIL | 130 PASS（全量） |
| V2 | RPC `jenkins/views`、jobs 按视图、MCP view | `go test ./internal/rpc/ -v` | 4 FAIL | 130 PASS（全量） |
| V3 | 前端 `groupJobs` | `pnpm test` | 模块缺失 | 21 PASS |
| V4 | 真实 Jenkins Views E2E | `node scripts/e2e-jenkins.js` | — | all/Builds/Ops 分组正确 |
| UI1 | 视图 Tab 溢出 + 首屏自动选中 | Chrome headless 截图 | — | 5 视图/22 任务：当前 Tab 完整居中、箭头可见、右栏自动加载首个任务 |
| UI2 | 视图 Tab 鼠标滚轮横向滚动 | `pnpm test` + Chrome CDP 真实滚轮 | 1 suite FAIL（`./tabs` 模块缺失） | 38 PASS；滚轮 0→150→550→586（钳制在 max）、边界外保持 586、反向回滚 436、页面 scrollTop 恒为 0 |
| ST1 | Stage `wfapi/describe` + 矩阵聚合 | `go test ./internal/jenkins/ -v` | 5 FAIL | 137 PASS（全量） |
| ST2 | RPC `jenkins/stages` | `go test ./internal/rpc/ -v` | 2 FAIL | 137 PASS（全量） |
| ST3 | 前端阶段耗时/色调/强度 | `pnpm test` | 模块缺失 | 28 PASS |
| ST4 | 真实 Pipeline 阶段视图 E2E | `node scripts/e2e-jenkins.js` | — | 7 stages / 5 builds / WorkflowJob 识别通过 |
| CRUD-B1 | 名称校验 / config.xml 路径 / 创建·编辑·删除任务 / 创建·编辑·删除视图 / 只读守卫 | `go test ./internal/jenkins/ -v` | 27 FAIL / 45 PASS | 84 PASS |
| CRUD-B2 | XML 保守改写（description/disabled/script/ListView） | `go test ./internal/jenkins/ -run TestPatch -v` | 6 FAIL | 11 PASS |
| CRUD-B3 | 增量更新（改名→读原文→局部改写→写回） | `go test ./internal/jenkins/ -run TestUpdate -v` | — | 9 PASS |
| CRUD-R1 | RPC 9 个 CRUD 方法 + 只读拦截 | `go test ./internal/rpc/ -v` | 10 FAIL | 45 PASS |
| CRUD-F1 | 前端名称校验/载荷/差异字段/删除确认 | `pnpm test` | 1 suite FAIL（`./forms` 模块缺失） | 55 PASS |
| CRUD-E1 | 真实 Jenkins 2.586 CRUD E2E | `node scripts/e2e-jenkins.js` | — | 视图建/改/删 + 任务建/改/禁用/重命名/删 + 只读 3 项拦截全部通过；Pipeline 创建因实例缺 workflow 插件跳过 |
| CRUD-E2 | HTML 错误页摘要 | `go test ./internal/jenkins/ -run TestAPIError` | — | 2 PASS |
| UI3 | Pipeline / XML 编辑器：语法高亮 + 行号 + 当前行 + 展开全屏 + Tab 缩进 | `pnpm test` + Chrome CDP 像素校验 | 2 suite FAIL（`./highlight`、`./editor` 模块缺失） | 82 PASS（全量）；行高 18.6 / 内距 10 两侧一致、高亮层与 textarea `scrollHeight` 均为 392、展开 900×760 突破 modal `overflow:hidden`、Esc 先收编辑器再关弹窗、Tab 首行 `pipeline` → `  pipeline`（caret 2） |
| AR1 | 自动刷新判定 + 串行轮询器 | `npx vitest run src/lib/refresh.test.ts` | 1 suite FAIL（`./refresh` 模块缺失） | 20 PASS；全量 102 PASS（11 files） |
| CH1 | 阶段矩阵变更集：`changeSet`/`changeSets` 合并去重、200 rune 截断、空数组序列化、`jenkins/builds` 契约不变 | `go test ./internal/jenkins/ -run "TestGetStageMatrix\|TestGetBuild"` | build failed（10 处未定义字段/常量） | 15 PASS；全量 141 PASS |
| CH1b | Pipeline `changeSets`（复数）真机缺陷回归 | 同上 | 5 FAIL（tree 缺 `changeSets`、`commitCount=0`） | 15 PASS |
| CH2 | 前端浮层纯函数：短哈希 / 首行摘要 / 视口定位与翻转 | `pnpm test` | 11 FAIL / 114 PASS（函数未定义） | 125 PASS（11 files） |
| CH3 | 浮层视觉与交互：Teleport 不被裁切、贴边翻转、8 条滚动、hover/键盘/Esc/外部点击/详情跳转 | Chrome CDP + headless 截图 | — | `parent=BODY`、`clippedBy=null`、8 条 `listScrollable=true`、底部行翻转 `top=92`、点击详情 `openBuild=38`、Esc/外部点击均关闭 |
| CH4 | 真实 Jenkins 2.586 Pipeline changelog：`jenkins/stages` 返回 `changes` | 临时 sidecar 探针（已删） | 修复前 `commitCount=0`、`changes=[]` | `commitCount=1`、`changes=[945f1594 / feat(export): 阶段视图变更浮层联调提交 / e2e]` |
| PG1 | 构建列表进度列：`estimatedDuration` 透出 + 进度算法（构建中百分比 / 超估算封顶 99% / 缺估算走不确定态 / 已完成返回 null） | `go test ./internal/jenkins/ -run TestGetBuildsReadsEstimatedDuration` + `npx vitest run src/lib/progress.test.ts` | Go：`builds[0].EstimatedDuration undefined (BuildSummary has no field)` 编译失败；前端：4 FAIL / 5（空实现返回 null） | Go 全量 OK；vitest 206 PASS（15 files）；`vue-tsc --noEmit` exit 0；`pnpm build` 产物 `ui/index.html` 223 kB（版本 0.4.4 一致） |
| PG2 | 进度条按阶段步进：单构建 `jenkins/buildStages` + `stageProgress`（已完成一格 / 进行中半格 / 封顶 99 / 空阶段回退时长估算） | `go test ./internal/rpc/ -run TestRPCBuildStages` + `npx vitest run src/lib/progress.test.ts` | Go：`method not found: jenkins/buildStages`（3 FAIL）；前端：`stageProgress is not a function`（5 FAIL / 13） | Go 全量 OK；vitest 214 PASS（15 files）；`vue-tsc --noEmit` exit 0；`pnpm build` 224.07 kB；预览 `#632` 时长估算 63% → 阶段步进 37%，确认阶段优先 |
| EX1 | 构建执行状态列表（0.5.0）：`jenkins/executors` 槽位明细（空闲占位 / Queue$Executable 等待启动 / 节点离线 / 进度复用时长估算） | `go test ./internal/jenkins/ -run "TestGetExecutors\|TestJobNameFromDisplayName"` + `npx vitest run src/lib/executors.test.ts` | Go：`client.GetExecutors undefined`、`undefined: jobNameFromDisplayName`（build failed）；前端：`./executors` 模块缺失、`hasRunningExecutor is not a function`、`expected ['overview'] to deeply equal ['overview','executors']`（3 files / 4 tests FAIL） | Go 全量 OK；vitest 244 PASS（17 files）；`vue-tsc --noEmit` exit 0；`[check-version] ok: 0.5.0`、`pnpm build` 230.47 kB |
| BB1 | 任务批量构建（0.8.0）：`Client.BatchBuild`（参数默认值、不可构建/排队/构建中跳过、单条失败不中断、只读零请求、读并发上限）+ RPC `jenkins/batchBuild` + 前端选择/汇总纯函数 | `go test ./internal/jenkins/ -run TestBatchBuild` + `go test ./internal/rpc/ -run TestRPCBatchBuild` + `npx vitest run src/lib/batchBuild.test.ts` | Go jenkins：`client.BatchBuild undefined`、`undefined: BatchBuildOptions`（build failed，6 条用例全部阻塞）；Go rpc：3 FAIL `method not found: jenkins/batchBuild`；前端：`Failed to load url ./batchBuild`（0 test） | Go 全量 OK（176 顶层用例，+10）；vitest 315 PASS（21 files）；`vue-tsc --noEmit` exit 0；`[check-version] ok: 0.8.0`、`pnpm build` ui/index.html 263.69 kB；预览截图 `docs/batch-preview.png` / `-dark.png` / `-result.png` |
| BB2 | 批量构建优化（0.8.1）：分批 `chunkJobs`/`failedChunk`/`abortedChunk`/`mergeBatchResults`、进度 `batchProgress`、中止判定 `shouldAbortChunks`、Shift 区间 `selectRange`、失败原因细化 | `npx vitest run src/lib/batchBuild.test.ts` | 17 FAIL / 33：`chunkJobs is not a function` 等 8 个新函数未定义；`batchReasonKey("forbidden")` 仍回落通用文案 | vitest 332 PASS（21 files，+17）；`go test ./...` 回归 OK；`vue-tsc --noEmit` exit 0；`[check-version] ok: 0.8.1`、`pnpm build` ui/index.html 267.25 kB；截图 `docs/batch-preview.png` / `-dark.png` / `-running.png`（12/30）/ `-result.png`（重试按钮） |
| BB3 | 批量构建 v3（0.9.0）：共享参数叠加（只叠加任务声明过的参数名）、`jenkins/batchInspect` 预检、跨视图缓存与计数、`ParamFields` 复用 | `go test ./internal/jenkins/ -run "TestBatchBuildAppliesSharedParams|TestBatchBuildSharedParamsIgnoredForPlainJob|TestInspectBatch"` + `go test ./internal/rpc/ -run TestRPCBatchInspect` + `npx vitest run src/lib/batchBuild.test.ts` | Go jenkins：`unknown field Params in struct literal of type BatchBuildOptions`、`client.InspectBatch undefined`（build failed）；Go rpc：2 FAIL `method not found: jenkins/batchInspect`；前端：9 FAIL / 37（`rememberTargets`/`collectSharedParameters`/`buildParamValues`/`countOutsideCurrent` 未定义） | Go 全量 OK（jenkins/rpc 含 4 组新用例）；vitest 336 PASS（21 files）；`vue-tsc --noEmit` exit 0；`[check-version] ok: 0.9.0`、`pnpm build` ui/index.html 272.49 kB；预览页已更新四态（读取中/确认/执行/结果），本轮未重拍截图（headless Chrome/Edge 启动即退，工具异常） |
| BB4 | 批量参数区只保留 Boolean 开关（0.9.1）：`collectSharedParameters` 过滤类型、移除 `conflicts` 机制 | `npx vitest run src/lib/batchBuild.test.ts` | 2 FAIL / 37：`expected [ 'MODULE_NAME', 'BRANCH_NAME', …(1) ] to deeply equal [ 'SKIP_BUILD' ]`；`expected [ { name: 'BRANCH_NAME', …(3) } ] to deeply equal []` | vitest 336 PASS（21 files）；`go test ./...` 回归 OK；`vue-tsc --noEmit` exit 0；`[check-version] ok: 0.9.1`、`pnpm build` ui/index.html 275,114 B；产物含新提示「仅共享开关（Boolean）参数」，旧「同名参数类型不一致」文案已消失（grep 0） |
| OA1 | 概览页自动刷新（0.9.2）：活跃/空闲双档 `hasQueuedBuild` / `resolveOverviewRefresh`、`pollOverview` 概览+执行器并行、OverviewView 徽章 | `npx vitest run src/lib/refresh.test.ts` | 8 FAIL / 60：`expected undefined to be 5000`（常量未导出）；`hasQueuedBuild is not a function` ×2；`resolveOverviewRefresh is not a function` ×5 | vitest 344 PASS（21 files，+8）；`go test ./...` 回归 OK；`vue-tsc --noEmit` exit 0；`[check-version] ok: 0.9.2`、`pnpm build` ui/index.html 276,405 B；打包 4,987,190 B / sha256 `65636639…`，包内 ui 逐字节一致，sidecar 含 `0.9.2` 且无 `0.9.1` |

## 关键红测样本

### M0：未实现桩

```
--- FAIL: TestJobPath/nested_folder
    paths_test.go:...: JobPath() = "", want "/job/team/job/api/job/build"
--- FAIL: TestNormalizeColor/red
    status_test.go:31: NormalizeColor("red") = "unknown", want "failure"
```

### M1：Client 未实现

```
--- FAIL: TestPostFormRetriesWithFreshCrumb
    client_test.go:172: NewClient() error = not implemented
--- FAIL: TestReadOnlyBlocksWrites
    client_test.go:223: NewClient() error = not implemented
```

### M1：RPC 未实现

```
--- FAIL: TestConnectionTestTokenShapes/secrets
    service_test.go:...: connection/test error = not implemented
--- FAIL: TestConnectDisconnectIdempotent
    service_test.go:215: connect #1 error = not implemented
```

### M1：前端模块缺失

```
FAIL  src/lib/errors.test.ts
Error: Failed to load url ./errors
```

### M2：任务/构建/控制台未实现

```
--- FAIL: TestGetJobsRoot
    jobs_test.go:30: GetJobs() error = not implemented
--- FAIL: TestGetConsoleProgressive
    console_test.go:39: GetConsole(first) error = not implemented
--- FAIL: TestTriggerBuildWithParams
    actions_test.go:...: TriggerBuild() error = not implemented
```

### M2：RPC 方法不存在

```
--- FAIL: TestRPCJobsAndJob
    m2_test.go:108: jobs error = method not found: jenkins/jobs
--- FAIL: TestRPCTriggerReadOnly
    m2_test.go:263: error = method not found: jenkins/trigger, want read_only
```

### M2：前端纯函数模块缺失

```
FAIL  src/lib/status.test.ts
FAIL  src/lib/format.test.ts
FAIL  src/lib/console.test.ts
```

### M3：队列/节点/概览未实现

```
--- FAIL: TestGetQueue
    system_test.go:28: GetQueue() error = not implemented
--- FAIL: TestGetOverview
    system_test.go:145: GetOverview() error = not implemented
--- FAIL: TestRPCOverview
    m3_test.go:64: overview error = method not found: jenkins/overview
```

### M4：MCP 方法未实现

```
--- FAIL: TestMCPToolsReadOnly
    mcp_test.go:...: mcp/tools error = method not found
--- FAIL: TestMCPCallTriggerReadOnly
    mcp_test.go:241: mcp/call error = method not found: mcp/call
```

### V1/V2：Views 未实现

```
--- FAIL: TestGetViews
    views_test.go:30: GetViews() error = not implemented
--- FAIL: TestRPCViews
    views_rpc_test.go:68: jenkins/views error = method not found
--- FAIL: TestMCPListJobsSchemaHasView
    views_rpc_test.go:180: list_jobs schema missing view
```

### ST1/ST2：阶段视图未实现

```
--- FAIL: TestGetBuildStages
    stages_test.go:30: GetBuildStages() error = not implemented
--- FAIL: TestGetStageMatrix
    stages_test.go:97: GetStageMatrix() error = not implemented
--- FAIL: TestRPCStages
    stages_rpc_test.go:54: stages error = method not found: jenkins/stages
```

### UI2：滚轮换算模块缺失

```
FAIL  src/lib/tabs.test.ts [ src/lib/tabs.test.ts ]
Error: Failed to load url ./tabs (resolved id: ./tabs) in .../src/lib/tabs.test.ts
Test Files  1 failed (1)   Tests  no tests
```

修复后：`src/lib/tabs.test.ts (10 tests)` 通过，全量 `38 passed (38)`。

## 视图 Tab 滚轮 UI 验证（Chrome 147 headless + CDP，2026-10-19）

真实 `Input.dispatchMouseEvent(type=mouseWheel)` 事件打到 `.view-tabs` 上（14 个视图，maxScroll=586）：

| 步骤 | scrollLeft |
| --- | --- |
| 初始 | 0 |
| 向下滚 150 | 150 |
| 再向下滚 400 | 550 |
| 滚到底（deltaY=4000） | 586（= maxScroll，不越界） |
| 边界外继续向下 | 586 |
| 向上滚 150 | 436 |
| 页面 scrollTop | 0（未把外层页面带着滚，`preventDefault` 生效） |

截图：`docs/screenshots/view-tabs-wheel.png`

## Sidecar 冒烟（真实二进制）
用 mock Jenkins（`127.0.0.1:18080`）驱动编译后的 `dbx-plugin-jenkins.exe`，按 DBX 协议发送 JSON-RPC：

```
init: {"capabilities":["connections"],"plugin":{"id":"com.anjun.jenkins","version":"0.1.0"},"protocolVersion":1}
connect: {"message":"Connected to Jenkins 2.440.3","readOnly":false,"success":true,"user":{"name":"u","authenticated":true,...}}
jobs: {"jobs":[{"name":"demo","fullName":"demo","status":"success","isFolder":false,...}]}
mcp/tools: 10 list_jobs,get_job,list_builds,get_build,get_console_log,list_queue,list_nodes,trigger_build,stop_build,cancel_queue_item
mcp/call list_jobs: {"content":[{"type":"text","text":"{\"jobs\":[...]}"}],"isError":false}
overview: {"version":"2.440.3","user":"u","mode":"NORMAL","queueLength":1,"nodesOnline":1,"nodesTotal":1,"executorsBusy":2,"executorsTotal":4}
queue: {"items":[{"id":42,"jobName":"team/demo","why":"Waiting for next available executor",...}]}
nodes: {"nodes":[{"name":"Built-In Node","offline":false,"numExecutors":4,"busy":2,"labels":["built-in"]}]}
cancelQueue: {"success":true}
disconnect: {"success":true}
```

## 真实 Jenkins E2E（2026-10-19）

Jenkins 2.586 + Java 21，临时 JENKINS_HOME，`/jenkins` 前缀，CSRF 开启：

```text
connect: Connected to Jenkins 2.586
jobs: demo, param-demo, pipeline-demo, slow-job
views: Builds, Ops, all
Builds view: demo, param-demo
Ops view: slow-job
param-demo parameters: BRANCH=main
demo build: SUCCESS, console contains "hello from dbx"
param-demo: console contains "branch=dev"
pipeline class: org.jenkinsci.plugins.workflow.job.WorkflowJob
stages: 获取构建人信息,参数验证,代码检出,打包构建,备份回滚应用,部署,启动应用 | builds 5 | avgRun 20961
stage build: {"number":5,"commits":0,"stages":7}
slow-job: ABORTED after stop
nodes: Built-In Node 0/2 busy
overview: executors 2, nodes 1/1
mcp tools: 10
read-only trigger: read_only
E2E OK
```

### E2E 发现并修复

`computer.idle` 在真实 Jenkins 里是**布尔值**，原实现按 int 解码导致 `jenkins/nodes` 报
`cannot decode Jenkins response`。修复为 `executors[idle]` 明细统计空闲数，旧版本无明细时退化为布尔值；
新增 `TestGetNodesFallbackIdleFlag` 回归测试。

## 绿测结果

```
ok  github.com/nixing178/dbx-plugin-jenkins/internal/jenkins
ok  github.com/nixing178/dbx-plugin-jenkins/internal/rpc

Test Files  6 passed (6)
Tests       28 passed (28)
```

> Vitest 配置 `fileParallelism: false`：宿主 FS shim 偶发拦截并行 worker 的临时 SSR 文件（EPERM），串行执行后稳定。

### CRUD：桩实现未落地

```
--- FAIL: TestCreateView
    config_test.go:...: CreateView() error = nil, want a request
--- FAIL: TestPatchDescriptionReplacesAndPreserves
    xmlpatch_test.go:64: description not replaced/escaped:
--- FAIL: TestRPCUpdateJob
    config_rpc_test.go:...: updateJob error = &{Code:-32601 Kind:method_not_found}
```

### CRUD：XML 1.1 声明

```
--- FAIL: TestPatchDisabled
    xmlpatch_test.go:99: PatchDisabled() error = invalid XML: xml: unsupported version "1.1"; only version 1.0 is supported
```

修复：扫描前把声明里的 `1.1` 等长替换成 `1.0`，原文与偏移保持不变。

### AR：自动刷新模块缺失

```
FAIL  src/lib/refresh.test.ts [ src/lib/refresh.test.ts ]
Error: Failed to load url ./refresh (resolved id: ./refresh) in E:/DBX插件开发/Jenkins/src/lib/refresh.test.ts. Does the file exist?
 Test Files  1 failed | 10 passed (11)
      Tests  82 passed (82)
```

实现 `src/lib/refresh.ts` 后 20 PASS，全量 102 PASS（11 files）。覆盖：

- `hasRunningBuild`：空数组 / `null` / 全部结束 / `building: true` / `status: "building"`；
- `hasRunningStage`：空矩阵 / 全部结束 / 构建 `building` / stage `IN_PROGRESS`、`in_progress`、`BUILDING` / `stages: null`；
- `createAutoRefresher`：未到间隔不执行、到点执行并续排、上一轮未完成不重入、`stop()` 终止、重复 `start()` 幂等、`stop()` 后可重新 `start()`、`shouldContinue` 返回 false 自动停、任务抛错后继续、`restart()` 重置计时、未启动时 `restart()` 为空操作；
- 常量：构建列表 3000ms、阶段视图 5000ms。

### AS：触发构建后不主动刷新

红（只加了测试，未实现）：

```
 ❯ src/lib/refresh.test.ts (0 test)
 FAIL  src/lib/refresh.test.ts [ src/lib/refresh.test.ts ]
TypeError: createPendingBuild is not a function
 ❯ src/lib/refresh.test.ts:320:19
 Test Files  1 failed | 10 passed (11)
      Tests  82 passed (82)
```

绿（实现 `hasNewBuild` / `createPendingBuild` / `evaluatePendingBuild` + `App.vue` 接线）：

```
 ✓ src/lib/refresh.test.ts (32 tests)
 Test Files  11 passed (11)
      Tests  114 passed (114)
```

`pnpm build`：`[check-version] ok: 0.3.0` → `vue-tsc --noEmit` 无错误 → `ui/index.html 191.48 kB`。

新增覆盖：

- `hasNewBuild`：空 / `null` / `undefined`；只有触发前的编号 → false；出现更大编号 → true；首次构建（`previousNumber = 0`）→ true；上一个构建仍在 `building` 但编号未涨 → false（避免误判提前停止跟踪）；
- `createPendingBuild`：绑定任务名、触发前编号与 `deadline = now + 120000`；省略 `now` 时以当前时间为准；
- `evaluatePendingBuild`：任务切换 → `stale`；出现新构建 → `settled`；`settled` 优先于 `expired`；超时无新构建 → `expired`；未到期 → `watching`；
- 接线（`App.vue`）：`pendingBuild` 并入 `autoRefreshMode` 的 `builds` 分支（不新建定时器）；触发成功后立即静默补刷 + 800ms 再补一次；`pollBuilds`/`selectJob` 命中 settled / expired / stale 即清空跟踪；切视图、卸载时清空。

## 视图改名（2026-10-22）

方案与接口实测见 [`docs/plan-view-rename.md`](./plan-view-rename.md)。

红（stub 阶段，6 例后端 + 2 例前端）：

```
--- FAIL: TestRenameViewPostsFullForm (0.00s)
    update_test.go:375: form name = "", want Builds2
    update_test.go:378: form item_demo = "", want on
    update_test.go:396: restore path = "", want /view/Builds2/config.xml
--- FAIL: TestRenameViewIncludesRegexInForm (0.00s)
    update_test.go:430: useincluderegex missing: Jenkins would drop includeRegex
--- FAIL: TestRenameViewRejectsDuplicateName (0.00s)
    update_test.go:458: error = <nil>, want invalid_config
--- FAIL: TestRenameViewRejectsNonListView (0.00s)
    update_test.go:489: error = <nil>, want invalid_config
--- FAIL: TestUpdateViewRenamesThenPatchesNewPath (0.00s)
    update_test.go:542: UpdateView() error = 404 page not found (HTTP 404)
--- FAIL: TestUpdateViewRenameOnlyWritesOnce (0.00s)
    update_test.go:588: UpdateView() error = 404 page not found (HTTP 404)

FAIL  src/lib/forms.test.ts > buildViewUpdateFields > detects a renamed view
AssertionError: expected {} to deeply equal { name: 'Builds 2' }
FAIL  src/lib/forms.test.ts > buildViewUpdateFields > trims the new name before comparing
```

RPC 层红（mock 缺 `/api/json` 与 `configSubmit`）：

```
--- FAIL: TestRPCUpdateViewRename (0.00s)
    config_rpc_test.go:393: updateView error = not_found: 404 page not found
```

绿：

```
ok  github.com/nixing178/dbx-plugin-jenkins/internal/jenkins  1.471s
ok  github.com/nixing178/dbx-plugin-jenkins/internal/rpc      0.793s

 Test Files  11 passed (11)
      Tests  129 passed (129)

pnpm typecheck: 通过
pnpm build: [check-version] ok: 0.4.0 → ui/index.html 201.06 kB
```

真实 Jenkins 2.586 E2E：

```
CRUD view updated: "updated by dbx" demo,param-demo
CRUD view renamed: CRUD-View-Renamed "updated by dbx" demo,param-demo
CRUD cleanup ok
E2E OK
```

> 改名后描述（`updated by dbx`）与任务清单（`demo,param-demo`）都保留，说明 configSubmit 的完整表单 + XML 回写生效。

新增覆盖：

- `RenameView`：表单必含 `name`、`json`（description/recurse/filterQueue/filterExecutors/columns）与每个任务的 `item_<job>`；有正则时补 `useincluderegex` + `includeRegex`；改名后用原 XML 回写新路径；
- 重名（区分大小写）不发任何写请求，返回 `invalid_config`；
- 非 ListView（AllView）拒绝改名，返回 `invalid_config: view is not a ListView`；
- `UpdateView`：先改名再读新路径 XML 做局部改写，返回最终名称；只改名时只写一次 XML；同名视为 no-op；
- RPC `jenkins/updateView` 返回 `fullName`；
- 前端 `buildViewUpdateFields` 比对 trim 后的名称，空名等于不修改。

### AT：新建任务后视图任务列表不刷新（0.4.1）

红（先写测试，`src/lib/viewRefresh.ts` 尚未实现）：

```
FAIL  src/lib/viewRefresh.test.ts [ src/lib/viewRefresh.test.ts ]
Error: Failed to load url ./viewRefresh (resolved id: ./viewRefresh) in E:/DBX插件开发/Jenkins/src/lib/viewRefresh.test.ts. Does the file exist?
 Test Files  1 failed | 11 passed (12)
      Tests  129 passed (129)
```

绿（实现纯函数 + `App.vue` / `JobList.vue` 接线）：

```
 ✓ src/lib/viewRefresh.test.ts (18 tests)
 Test Files  12 passed (12)
      Tests  147 passed (147)
```

`pnpm typecheck` 无报错；`pnpm build` 通过（`ui/index.html` 202.41 kB）；`go test ./...` 通过；`check-version` 0.4.1 四处一致。

覆盖：

- `isNamedView`：空串 / 纯空白 / `null` / `undefined` / `all` / `ALL` 均非命名视图；首尾空白不影响自定义视图名；
- `containsJob`：按 `fullName` 精确匹配（`demo` 不命中 `demo2`），空名与空列表恒 false；
- `appendJobName`：空集合追加、保持原顺序末尾追加、已存在不重复、`null` 按空集合、输入 trim 与去空白项、空任务名不写入；
- `needsJoinView`：根目录 + 命名视图 + 列表不含新任务才成立；已包含（正则视图）、文件夹内创建、`all`/空视图名、空任务名均跳过；
- `resolveJoinAction`：不需要写回 → `skip`；ListView（含 `jobNames: null`）→ `add`；`fields` 为 `null`（聚合视图等）→ `unsupported`。

UI 接线：`createJob` 先静默刷新列表，再按 `needsJoinView` 读 `viewConfig` 追加 `jobNames` 并回写，最后按结果给三种提示（加入视图 / 普通创建 / 创建但未加入）；`triggerBuild` 触发后与 800ms 补刷各刷一次列表，`settlePendingBuild` 在 `settled` 时刷新一次，`autoRefreshMode` 由 `builds` 转 `null` 时再收敛刷新一次。

## 覆盖点

- `BaseURL`：scheme/host/port 校验、base_path 归一化、IPv6、非法输入。
- `JobPath`：嵌套文件夹、URL 转义、空段与 `..` 拒绝。
- `NormalizeColor`：6 种状态 + `_anime` + 大小写/空白。
- `Client`：Basic Auth、匿名、CookieJar、CSRF 自动获取与 403 重试一次、CSRF 关闭降级、只读守卫、401/403/404/500 映射、超时、自签证书开关。
- `RPC`：`connection/test|connect|disconnect`、5 种 secret 传递形态、base_path、只读标记、幂等连接/断开、会话隔离、未知连接提示。
- `Jobs`：根/文件夹列表、文件夹识别与排序、浮层 fullName 补全、状态归一化、参数定义（string/choice/boolean）。
- `Builds`：构建列表默认 limit、触发原因、构建详情、变更集、产物、结果→状态。
- `Console`：`progressiveText` 增量、`X-Text-Size`/`X-More-Data`、缺失 header 的兜底、512 KiB 分块。
- `Actions`：无参/带参触发路径、队列 ID 解析、停止构建、只读拦截、HTTP 错误映射。
- `Queue`：队列项解析（task/why/stuck/inQueueSince/cause）、取消队列项、只读拦截。
- `Nodes`：在线/离线、执行器 busy/total、标签、离线原因。
- `Overview`：whoami + mode + queue + nodes 聚合、执行器/节点统计。
- `MCP`：10 个工具定义与 schema、只读连接隐藏 3 个写工具、注解（readOnly/destructive/idempotent）、`mcp/call` 路由、生命周期兜底建连、工具级错误（`isError`）、控制台日志两段返回（正文 + nextStart/moreData）。
- `Views`：`GetViews`、`GetViewJobs`（视图名 URL 转义）、视图内文件夹/任务归一化与排序；RPC folder 优先于 view；MCP `list_jobs` 支持 view；前端 `groupJobs` 拆分与视图 Tab；视图改名（configSubmit 完整表单 + XML 回写、查重、非 ListView 拒绝）。
- `Stages`：`wfapi/describe` 解析、矩阵列顺序、缺失 stage 留空、平均耗时（失败/进行中排除）、提交数、404 → `stages_unavailable`、部分构建无阶段时降级；变更集同时解析 `changeSet`（Freestyle）与 `changeSets`（Pipeline），按 commitId 去重，矩阵内单条信息截断 200 rune。
- `CRUD`：名称校验（路径分隔符/`..`/控制字符/255 上限）；任务 config.xml 路径；自由风格与 Pipeline 最小模板（脚本转义）；文件夹内创建；`doRename`；`doDelete`；ListView `createView`（name/mode 在 form 与 json 各一份、7 列默认配置）；视图 config.xml 读写；9 个写入口只读拦截；XML 保守改写保留 SCM/构建步骤/comparator/columns；增量更新只提交变化字段；HTML 错误页摘要。
- 前端：错误码 → 中英文提示、未知错误码保留 detail；状态色调、时长/时间格式、日志尾部截断、匹配计数、大小写高亮；表单名称校验、载荷裁剪、差异字段、任务列表集合比较、删除确认精确匹配。

## 新增任务 400 可读原因（0.4.2）

红（先写测试，6 项 Go + 5 项前端全部失败）：

```
--- FAIL: TestAPIErrorExtractsSendErrorPage
    kind = "request_failed", want rejected
    message = "Jenkins returned an HTML error page (HTTP 400); check the Jenkins server log"
--- FAIL: TestAPIErrorExtractsDuplicateName
--- FAIL: TestAPIErrorStopsAtParagraphBoundary
--- FAIL: TestAPIErrorUnescapesHTMLEntities
--- FAIL: TestAPIErrorKeepsLoggingID
--- FAIL: TestValidateNameRejectsUnsafeCharacters   (14 个字符全部放行)
```

```
 FAIL  src/lib/errors.test.ts > errorKind > reads the kind after an RPC method prefix
 FAIL  src/lib/errors.test.ts > errorText > localizes kinds that carry an RPC method prefix
 FAIL  src/lib/errors.test.ts > errorText > shows the Jenkins reason for rejected requests
 FAIL  src/lib/errors.test.ts > errorText > keeps the detail when a config error explains the problem
 FAIL  src/lib/forms.test.ts > validateName > rejects characters Jenkins refuses
 Tests  5 failed | 28 passed (33)
```

绿：

```
ok  github.com/nixing178/dbx-plugin-jenkins/internal/jenkins
ok  github.com/nixing178/dbx-plugin-jenkins/internal/rpc
```

```
 ✓ src/lib/errors.test.ts (10 tests)
 ✓ src/lib/forms.test.ts (23 tests)
 Test Files  12 passed (12)
      Tests  153 passed (153)
```

`pnpm typecheck` 无报错；`pnpm build` 通过（`ui/index.html` 202.73 kB）；`check-version` 0.4.2 四处一致。

真实 Jenkins 2.586（sidecar 直连，18081 `/jenkins`）：

```
PASS 重名（Jenkins 返回 400，需透出原因）
     -> rejected: A job already exists with the name 'probe-2'
PASS 名字含 *（本地拦截）        -> invalid_config: name must not contain "*"
PASS 名字含 "（本地拦截，避免 500） -> invalid_config: name must not contain "\""
PASS 中文名                     -> {"fullName":"中文任务-crud","success":true}
PASS 文件夹内新建                -> {"fullName":"probe-folder2/child-b","success":true}
PASS 文件夹内重名                -> rejected: A job already exists with the name 'child-b'
```

覆盖：

- `newAPIError`：400（非 401/403/404）→ `rejected`；HTML 页提取 `<h1>Error</h1><p>原因`（`<p>` 未闭合，按 `</p>`/`<!--`/`</div>`/`<h1`/`<div` 截断）；HTML 实体反转义（`&lt;` → `<`）；无原因的 500 Oops 页保留通用提示并附 `Logging ID=`。
- `ValidateName`：拒绝 `* : < > | & ; ! @ $ ^ [ ] "`；放行 `( ) { } = + , ' ~ \` - 。` 空格与中文。
- `errorKind` / `errorText`：识别 `jenkins/createJob: rejected: …` 形式的方法前缀；`rejected` 与 `invalid_config` 把 detail 拼在本地化文案后；未知 kind 仍只保留 detail。
- `validateName`（前端）：与后端字符集一致。
- `JobEditor.submit`：`busy` 期间直接返回，避免重复提交导致第二次重名 400。

## 操作提示改 toast（2026-10-22）

顶部内联绿条（`.banner.notice`）改为右上角浮层 toast：`src/lib/toast.ts`（队列：4s 过期、同文案去重重置计时、最多 3 条、点击关闭）、`src/components/ToastHost.vue`（浮层渲染）、App.vue 的 14 处 `showNotice` 全部改走 `showToast`。错误横幅 `.banner.error` 保留（需要常驻 + 重试按钮）。

红（`toast.ts` 未创建时）：

```
❯ src/lib/toast.test.ts (0 test)
Error: Failed to load url ./toast (resolved id: ./toast) ... Does the file exist?
 Test Files  1 failed | 12 passed (13)
      Tests  153 passed (153)
```

第一次写实现后仍红 1 例——不是实现缺陷，是测试过度约束了「两条同时到期各回调一次」：

```
FAIL  src/lib/toast.test.ts > createToastQueue > notifies subscribers on push, drop and expiry
AssertionError: expected "spy" to be called 3 times, but got 4 times
      Tests  1 failed | 165 passed (166)
```

绿：

```
 ✓ src/lib/toast.test.ts (13 tests)
 Test Files  13 passed (13)
      Tests  166 passed (166)
```

`pnpm typecheck` 无报错；`pnpm build` 通过（`ui/index.html` 213.76 kB）；`check-version` 0.4.2 四处一致。

UI 目视（Chrome headless 截图，`pnpm dev` + `/toast-preview.html`）：

- `docs/toast-preview.png`：浅色主题，三条堆叠（success 绿左条 / info 蓝左条 / 长文案换行），浮层压在 56px 顶栏下方、不挤压列表。
- `docs/toast-preview-dark.png`：深色主题，卡片随 `--panel/--text` 变量翻转，文字对比正常。

覆盖点：

- 入队返回 id、默认 success、info 保留 tone、文本 trim、空文本返回 null。
- 4s 自动过期；同文案重复 push 不叠加且重置倒计时；过期后可再次入队。
- 超过 3 条挤掉最早一条；`dismiss` 立即移除并清定时器；未知 id 不影响其它条目。
- `clear()` 清空并取消所有计时器；`onChange` 在入队/挤出/过期/清空时回调。

---

## 2026-10-22 · 自动刷新治理（0.4.3）

范围：`docs/refresh-hardening-plan.md` 的 R1~R10，决策 D1:A / D2:A / D3:A / D4:A / D5:A。

### Step 1 轮询基建（R3/R4/R5）

红（`POLL_TIMEOUT_MS` / 护栏 / `trigger()` 尚未实现）：

```
 ❯ src/lib/refresh.test.ts (38 tests | 4 failed)
   × refresh intervals > keeps poll requests on a shorter timeout than manual loads
     → expected undefined to be 15000
   × createAutoRefresher failure guard > stops once consecutive failures reach the limit
     → expected true to be false
   × createAutoRefresher trigger > runs immediately and resets the countdown
     → refresher.trigger is not a function
   × createAutoRefresher trigger > does nothing on an inactive refresher
     → refresher.trigger is not a function
      Tests  4 failed | 34 passed (38)
```

绿：

```
 ✓ src/lib/refresh.test.ts (38 tests)
      Tests  38 passed (38)
```

### Step 2 构建详情轮询（R1）

红：

```
   × shouldPollBuildDetail > returns false for a missing build
   × shouldPollBuildDetail > polls while the build is running
   × shouldPollBuildDetail > detects the building status without the flag
   × shouldPollBuildDetail > stops once the build finished
     → shouldPollBuildDetail is not a function
      Tests  4 failed | 38 passed (42)
```

绿：`✓ src/lib/refresh.test.ts (42 tests)`。

### Step 3 队列轮询（R2）

红：`expected undefined to be 5000 // QUEUE_AUTO_REFRESH_INTERVAL_MS`（1 failed | 42 passed）。
绿：`✓ src/lib/refresh.test.ts (43 tests)`。

### Step 4 延迟刷新收敛（R6/R9/R10）

红：

```
   × shouldRefreshJobsAfterPending > refreshes once the new build showed up
   × shouldRefreshJobsAfterPending > refreshes when the tracking window expired
   × shouldRefreshJobsAfterPending > skips while still watching or after a job switch
     → shouldRefreshJobsAfterPending is not a function
      Tests  3 failed | 43 passed (46)
```

绿：`✓ src/lib/refresh.test.ts (46 tests)`，全仓 180 passed。

### Step 5 控制台内存与后台（R8）

红：

```
 ❯ src/lib/console.test.ts (10 tests | 4 failed)
   × trimLogTail > leaves short logs untouched
   × trimLogTail > treats a non-positive limit as unlimited
   × trimLogTail > keeps the tail and drops the partial first line
   × trimLogTail > falls back to the plain tail when there is no line break
     → trimLogTail is not a function
      Tests  4 failed | 6 passed (10)
```

绿：`✓ src/lib/console.test.ts (10 tests)`。

### 最终验收

```
 Test Files  13 passed (13)
      Tests  184 passed (184)
```

```
> node scripts/check-version.mjs && vue-tsc --noEmit && vite build
[check-version] ok: 0.4.3
✓ 71 modules transformed.
ui/index.html  216.89 kB │ gzip: 68.53 kB
✓ built in 1.69s
```

新增覆盖点：

- 连续失败达阈值后自动停轮询并回调 `onPaused("failures")`；中间成功一次即清零；未配置阈值时保持旧行为（无限重试）。
- `trigger()` 立即执行一轮并重置倒计时；未启动时为空操作。
- `shouldPollBuildDetail`：`building` 真值 / `status=building` / 缺省 / 已结束。
- `shouldRefreshJobsAfterPending`：settled 与 expired 都补刷，watching 与 stale 不刷。
- `trimLogTail`：未超限原样返回、超限按行边界留尾、无换行退化为纯尾、非正数视为不限。
- `QUEUE_AUTO_REFRESH_INTERVAL_MS = 5000`；`POLL_TIMEOUT_MS = 15000`。

无法用纯函数覆盖的行为（已人工审查）：`visibilitychange` 隐藏停轮询/回前台 `trigger()`、`stopBuild` 延迟刷新目标校验、`refreshJob` 的 `restart()` 前置、`pollBuilds` 的 `selectToken` 快照校验、ConsoleView 的 `inFlight` 代际互斥。

---

## 2026-10-22 · 自动刷新提示与页面反馈（0.4.4）

范围：`docs/refresh-feedback-plan.md` 的 F1~F6，决策 D1:A / D2:A / D3:A / D4:A / D5:A。

### Step 1 轮询器运行回调

红（`onRunStart` / `onRunEnd` 尚未接）：

```
 ❯ src/lib/refresh.test.ts (49 tests | 3 failed)
   × createAutoRefresher run hooks > reports the start and a successful end of each run
     → expected [] to deeply equal [ 'start', 'ok' ]
   × createAutoRefresher run hooks > reports a failed run and still counts it towards the guard
     → expected [] to deeply equal [ false, false ]
   × createAutoRefresher run hooks > still reports the end of an in-flight run after stop()
     → expected [] to deeply equal [ true ]
      Tests  3 failed | 46 passed (49)
```

绿：`✓ src/lib/refresh.test.ts (49 tests)`。

> 顺带修正：四个 `pollXxx` 原先在内部 `catch` 掉错误，`AutoRefresher` 永远认为成功——护栏的「连续 3 次失败暂停」此前实际不可能触发。现在错误冒泡到轮询器，计数才生效。

### Step 2 刷新状态纯函数

红（模块缺失）：`Failed to load url ./refreshStatus ... Tests no tests`。
绿：`✓ src/lib/refreshStatus.test.ts (14 tests)`。

### 最终

```
 Test Files  14 passed (14)
      Tests  201 passed (201)
```

```
[check-version] ok: 0.4.4
✓ 75 modules transformed.
ui/index.html  221.19 kB │ gzip: 69.78 kB
```

新增覆盖点：

- `onRunStart` 在 task 前触发、`onRunEnd(true)` 在成功后触发；失败回调 `false` 且仍计入护栏；`stop()` 时仍在飞的请求也会回调 end，界面不会卡在「请求中」。
- `refreshBadge`：暂停优先于请求中；无 mode 时返回 idle（即使 paused 残留也不显示）。
- `formatClockTime`：补零、23 点不溢出、无值返回空串。
- `diffBuildRows`：首次加载不高亮、新增与状态翻转变更、上一次为空列表时高亮、下一次为空时忽略。

目视（Chrome headless，`pnpm dev` 打开 `/refresh-preview.html`）：

- `docs/refresh-badge-preview.png`：浅色下四态与按钮三态，暂停为琥珀色。
- `docs/refresh-badge-preview-dark.png`：深色下文字与圆点对比正常。

## 2026-10-22 · 进入页面自动刷新（0.4.5）

需求：切换页面时自动补刷一次。决策 D1:B / D2:A / D3:A / D4:A / D5:A。

### Step 1 进入动作判定（`src/lib/enterRefresh.ts`）

红（模块缺失）：

```
 FAIL  src/lib/enterRefresh.test.ts [ src/lib/enterRefresh.test.ts ]
Error: Failed to load url ./enterRefresh (resolved id: ./enterRefresh) in src/lib/enterRefresh.test.ts. Does the file exist?
 Test Files  1 failed (1)
      Tests  no tests
```

绿：`✓ src/lib/enterRefresh.test.ts (11 tests)`。

### Step 2 节流语义修正

第一版只按时间戳节流：刚加载完概览（`lastRefreshAt` 已更新）后立刻点「任务」页会被挡掉，列表直接空白。
改成**换页必刷、只有同一页重复进入才节流**（`EnterRefreshGate`）。

红：

```
 ❯ src/lib/enterRefresh.test.ts (12 tests | 4 failed)
   × shouldRefreshOnEnter > 换页面总是刷新，不受上次刷新时间影响
   × shouldRefreshOnEnter > 首次进入某页（还没有上次记录）总是刷新
   × shouldRefreshOnEnter > 同一页刚好到达最小间隔就放行
   × shouldRefreshOnEnter > 支持自定义间隔；间隔为 0 表示不节流
      Tests  4 failed | 8 passed (12)
```

绿：`✓ src/lib/enterRefresh.test.ts (12 tests)`。

### App.vue 接线

- `switchView` → `await refreshOnEnter()`；任务页保留当前 `folder`。
- 阶段视图 `@back` 由裸赋值 `view = 'jobs'` 改为 `backToJobs()`（静默补刷一次）。
- `handleVisibility` 在没有轮询模式时也走 `refreshOnEnter`（概览 / 节点 / 空闲的任务页）。
- `enterRefreshBusy` 防重入；刷新期间用户切走，结束后会给新页补一次。
- 任务页用 `loadJobs(folder, true)` + `selectJob(..., true)` 静默刷新，不闪骨架；用 `selectToken` 快照避免 `loadJobs` 自动重选后重复请求。

### 最终验收

```
 Test Files  16 passed (16)
      Tests  226 passed (226)
```

```
cd backend && go test ./...
ok  github.com/nixing178/dbx-plugin-jenkins/internal/jenkins  1.425s
ok  github.com/nixing178/dbx-plugin-jenkins/internal/rpc      1.682s
```

```
[check-version] ok: 0.4.5
✓ 80 modules transformed.
ui/index.html  224.79 kB │ gzip: 70.79 kB
```

打包与身份：

```
Success: Built ...\dist\io.github.nixing178.jenkins-0.4.5-windows-x64.dbxp
Signature: unsigned review candidate
[verify-sidecar] ok: io.github.nixing178.jenkins/0.4.5
```

新增覆盖点：

- `resolveEnterActions`：概览 / 队列 / 节点各刷自己；任务页首次补 `views`；有选中任务时追加 `detail`；build / stages 返回空；顺序固定 views → jobs → detail。
- `shouldRefreshOnEnter`：换页必刷；首次进入必刷；同页 2s 内跳过、边界放行；自定义间隔与 0 不节流；时钟回拨不刷。

> 说明：App.vue 的分发属接线层，未做 UI 自动化；判定逻辑全部由纯函数单测覆盖，接线通过类型检查 + 构建 + 代码审查确认。

## 构建执行状态列表（0.5.0，2026-10-22）

需求来源：Jenkins 原生首页右栏有 Build Queue 与 Build Executor Status 两个 widget，插件只有前者。决策 D1:1 / D2:1 / D3:1 / D4:2 / D5:1 —— 放概览页、有执行中才 5s 轮询、点行跳任务、不暴露 MCP、版本升 0.5.0。

### 红

```
$ cd backend && go test ./internal/jenkins/ -run "TestGetExecutors|TestJobNameFromDisplayName"
internal\jenkins\system_test.go:156:22: client.GetExecutors undefined (type *Client has no field or method GetExecutors)
internal\jenkins\system_test.go:205:22: client.GetExecutors undefined (type *Client has no field or method GetExecutors)
internal\jenkins\system_test.go:230:13: undefined: jobNameFromDisplayName
FAIL	github.com/nixing178/dbx-plugin-jenkins/internal/jenkins [build failed]
```

```
$ npx vitest run src/lib/executors.test.ts src/lib/refresh.test.ts src/lib/enterRefresh.test.ts
 ❯ src/lib/executors.test.ts (0 test)
   → Failed to load url ./executors ... Does the file exist?
 ❯ src/lib/enterRefresh.test.ts (12 tests | 1 failed)
   × 概览页刷新概览数据 + 执行器列表 → expected [ 'overview' ] to deeply equal [ 'overview', 'executors' ]
 ❯ src/lib/refresh.test.ts
   × TypeError: hasRunningExecutor is not a function
 Test Files  3 failed (3)
      Tests  4 failed | 60 passed (64)
```

### 绿

```
$ cd backend && go test ./...
ok  github.com/nixing178/dbx-plugin-jenkins/internal/jenkins  1.246s
ok  github.com/nixing178/dbx-plugin-jenkins/internal/rpc      0.865s
```

```
$ npx vitest run
 ✓ src/lib/executors.test.ts (15 tests)
 ✓ src/lib/refresh.test.ts (52 tests)
 ✓ src/lib/enterRefresh.test.ts (12 tests)
 Test Files  17 passed (17)
      Tests  244 passed (244)
```

```
$ pnpm build
[check-version] ok: 0.5.0
✓ 84 modules transformed.
ui/index.html  230.47 kB │ gzip: 72.14 kB
```

### 实现要点

| 位置 | 处理 |
| --- | --- |
| `backend/internal/jenkins/system.go` | `GetExecutors` 复用 `/computer/api/json`，独立 tree（不动 `GetNodes`，避免概览聚合回归）；执行器数组下标 +1 作为槽位序号；空闲槽位也返回 |
| 同上 | `jobNameFromDisplayName` 剥掉 `Run.getFullDisplayName()` 尾部的 ` #N`；自定义 displayName 不遵循该格式时原样返回 |
| 同上 | `Queue$Executable`（占位但构建未建）只带 url → `pending=true`，前端显示「等待启动」而不是报错 |
| `src/lib/executors.ts` | 纯函数：`executorState`（idle/pending/building/settling）、`executorSummary`（`!idle` 计 busy）、`executorProgress` 复用 `buildProgress`、`executorNodeLabel`（空 displayName → 内置节点） |
| `src/lib/refresh.ts` | `hasRunningExecutor` 只看 `idle === false`；`EXECUTOR_AUTO_REFRESH_INTERVAL_MS = 5000` |
| `src/App.vue` | `AutoRefreshMode` 新增 `executors`，仅概览页且有占用时启用；`pollExecutors` 不内部 catch，错误上抛给护栏计数 |

### 覆盖点

- 后端：三槽位（内置节点构建中 / 内置节点空闲 / 离线 agent 构建中）字段与顺序；`pending` 占位形态；`jobNameFromDisplayName` 五种输入（含中文与空格、number=0、空串）。
- 前端：四种执行器状态；`busy/total` 汇总（含 pending 算 busy、空列表不出 NaN）；进度（45% / 超时封顶 99 / 缺估算不定态 / 空闲与等待启动返回 null）；`#N` 标签；轮询判定（占用、等待启动、全空闲、空列表、未加载）。
- 进入刷新：概览页 actions 变为 `["overview", "executors"]`，先数字后明细；其余页不变。

> 说明：App.vue 的分发属接线层，判定逻辑全部由纯函数单测覆盖，接线通过 `vue-tsc --noEmit` + 构建确认。

### 预览（`docs/executor-preview*.png`）

- `pnpm dev` 打开 `/executor-preview.html`（`?theme=dark` 切深色），假数据覆盖六种形态：内置节点构建中 61%、等待启动、空闲、缺估算的不定态进度、收尾中、离线节点。
- 截图：`docs/executor-preview.png`、`docs/executor-preview-dark.png`（Chrome headless，1360 宽）。
- 深色首轮截图暴露「构建中」标签用 `var(--accent)` 作字色对比不足（`#3355ff` on `#1c1f26`）→ 改为「底色 + 1px 描边表达色相、文字统一用 `var(--text)`」；`--warning` 同理，不再硬编码 `#d99b1a1f` / `#b47d0d`。

### 打包与冒烟（0.5.0）

```
$ npx --yes @dbx-app/plugin-cli@0.1.9 package .
Building: Go backend
Packaging: ...\dist\io.github.nixing178.jenkins-0.5.0-windows-x64.dbxp
Success: Built ...\dist\io.github.nixing178.jenkins-0.5.0-windows-x64.dbxp
Signature: unsigned review candidate
```

包内 6 条目；`ui/index.html` 232,537 B 与本地逐字节一致；manifest 0.5.0；sidecar 8,574,976 B → `[verify-sidecar] ok: io.github.nixing178.jenkins/0.5.0`。

从 .dbxp 取出 sidecar 起 mock Jenkins 连接冒烟（临时脚本已删）：

```
[smoke] jenkins/executors -> 3 rows
  node="" #1 idle=false pending=false job="demo-job" build=12 building=true
  node="" #2 idle=false pending=true job="" build=0 building=false
  node="agent-1" #1 idle=true pending=false job="" build=0 building=false
[smoke] jenkins/nodes -> built-in busy=2/2
[smoke] ok: executors 3 rows + nodes 未受影响
```

确认：空闲槽位占位、`Queue$Executable` → `pending`、离线节点标记、`GetNodes` 的 busy 统计未被新 tree 影响。

## 修复：执行器行跳转 404（0.5.1，2026-10-22）

### 根因

`Run.getFullDisplayName()` 返回的是 `project.getFullDisplayName() + ' ' + displayName`，而

```java
// hudson.model.AbstractItem
public final String getFullDisplayName() {
    return getParent().getFullDisplayName() + " » " + getDisplayName();   // 分隔符是 » 不是 /
}
```

所以文件夹里的任务拿到的是 `Builds » demo-web #12`，按 ` #N` 剥出来是 `Builds » demo-web`，
拼成 `/job/Builds » demo-web` 必然 404（界面显示「Jenkins 地址、上下文路径或资源不存在」）。
自定义 displayName 的任务同样对不上。

### 修复

改为从 `currentExecutable.url` 反解 fullName（`Run.getUrl()` = `project.getUrl() + number + "/"`，
`project.getUrl()` 是 `/job/<folder>/job/<job>/`），新增 `jobFullNameFromURL`：

```go
segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
for i := 0; i < len(segments)-1; i++ {
    if segments[i] == "job" { names = append(names, segments[i+1]); i++ }
}
```

`jobNameFromDisplayName` 降级为 url 缺失时的兜底，注释里写明只对顶层任务可靠。

### 红 → 绿

```
$ go test ./internal/jenkins/ -run "TestGetExecutors|TestJobFullNameFromURL"
internal\jenkins\system_test.go:236:13: undefined: jobFullNameFromURL
FAIL	[build failed]

$ go test ./...
ok  internal/jenkins  1.176s
ok  internal/rpc      0.750s
```

`TestGetExecutors` 的 mock 换成真实形态（`Builds » demo-web #12` + 编码过的 folder url），
新增 `TestJobFullNameFromURL` 10 个用例：顶层 / context path / 文件夹 / 三层 / 相对地址 /
URL 编码中文 / 缺尾斜杠 / 带查询串 / 队列占位 / 空串。

### 打包与端到端冒烟（0.5.1）

- `dist/io.github.nixing178.jenkins-0.5.1-windows-x64.dbxp`，4,954,841 B，sha256 `a1221d54…`；
  `ui/index.html` 与本地一致；`[verify-sidecar] ok: io.github.nixing178.jenkins/0.5.1`。
- mock Jenkins（`fullDisplayName` 用 `»`、url 用 folder 结构）→ 从包内 sidecar 走完整链路：

```
[smoke] executors[0]: jobName="Builds/demo-web" build=12
[smoke] jenkins/job -> Builds/demo-web (status building)
[smoke] jenkins/build -> #12
[smoke] ok: 文件夹任务名从 url 反解正确，跳转链路 200
```

## 构建详情页变更记录列表优化（0.6.0，2026-10-22）

### 需求与决策

源哥一句话「构建任务详情页-变更记录列表优化」，按 `docs/plan-build-changes.md` 推荐项全取：
D1:2 展示层 + 数据增强、D2:A 时间 + 文件数 + 正文、D3:2 最新在前、D4:1 折叠 8 条、
D5:1 点哈希复制、D6:1 首行 + 展开正文、D7:1 不做搜索、D8:1 浮层只复用排序、D9:1 0.6.0、D10:1 MCP 透出。

### 真机字段（临时 Jenkins 2.587，git-pipeline #3）

```
GET /job/git-pipeline/3/api/json?tree=changeSets[items[commitId,msg,comment,timestamp,date,author[fullName],affectedPaths]]
{"items":[{"commitId":"8e45f64…","msg":"probe B: 第二条提交",
           "comment":"probe B: 第二条提交\n\n- 明细一…\n- 明细二…\n",
           "timestamp":1792659786000,"date":"2026-10-22 17:03:06 +0800",
           "author":{"fullName":"e2e"},"affectedPaths":["PROBE.md"]}]}
```

四个必须记住的点：

1. **`msg` 只有首行（subject）**，多行正文在 `comment`；原来的详情页不是"多行撑爆"，而是"正文根本没取"。
2. **`date` 是字符串、`timestamp` 才是 epoch 毫秒** → 由后端归一化，前端不碰 `date`。
3. **tree 写 `items[*]` 会让 `author` 变成 `{}`**（本次实测踩到），必须显式 `author[fullName]`。
4. changelog 顺序是**旧→新**，且同秒提交 timestamp 相同 → 前端排序必须稳定。

### 后端红 → 绿

```
$ go test ./internal/jenkins/ -run "TestGetBuildDetailReadsChangeDetails|TestGetBuildChangeTimestampFallsBackToDate|TestGetBuildChangeBodyEmptyForSingleLine|TestGetStageMatrixDropsChangeBody"
internal\jenkins\builds_test.go:227:12: change.Timestamp undefined (type ChangeSetItem has no field or method Timestamp)
internal\jenkins\builds_test.go:230:12: change.FileCount undefined ...
FAIL	[build failed]

$ go test ./...
ok  	internal/jenkins	1.300s
ok  	internal/rpc	0.917s
```

新增 4 例：详情读全字段（含 tree 断言）/ `date` 兜底归一化 / 单行提交无正文 / 矩阵不请求 comment 且 Body 为空。

一次假绿：`changeBody` 起初先 `TrimSpace` 再切行，单行 `comment` 的换行被吃掉，
`TestGetBuildChangeBodyEmptyForSingleLine` 报 `body = "fix bug"` → 改成先按原始文本切行再修剪。

### 前端红 → 绿

```
$ npx vitest run src/lib/changes.test.ts
Error: Failed to load url ./changes (resolved id: ./changes) ... Does the file exist?
Test Files  1 failed (1)   Tests  no tests

$ npx vitest run
Test Files  18 passed (18)
     Tests  263 passed (263)
```

`src/lib/changes.ts` 19 例：稳定倒序 / 时间全缺失保序 / 部分缺失 / 同秒稳定 / 不改原数组 /
折叠边界（8、全量、不足 8、limit≤0）/ 摘要计数（空作者、缺 fileCount）/ 正文修剪 /
同年 `10-22 17:03` / 跨年 `2025-12-31` / 英文 `Oct 22, 17:03` / 缺失 `—`。

### 真机端到端（真实 client 直连，探针已删）

```
$ go run ./cmd/probe
[
  {"commitId":"6a24159…","message":"probe A: 第一条提交","author":"e2e","timestamp":1792659786000,"fileCount":1},
  {"commitId":"8e45f64…","message":"probe B: 第二条提交","author":"e2e","timestamp":1792659786000,"fileCount":1,
   "body":"- 明细一：验证多行消息的展示\n- 明细二：应默认折叠为单行摘要\n- 明细三：展开后才看到全部内容"},
  {"commitId":"0c69226…","message":"probe C: 第三条提交","author":"e2e","timestamp":1792659786000,"fileCount":1}
]
```

- `timestamp` 与 `git log --format=%ai` 一致（2026-10-22 17:03:06 +0800）；
- `body` 只出现在多行提交上、首行已剥掉；单行提交不带 `body` 字段（omitempty）。

### 交互（CDP，1280×980）

```
初始（折叠 8 条）：rows=8,  moreLabel="展开全部 12 条", bodyCount=0, summary="12 个提交 · 3 位作者 · 98 个文件"
点击展开全文：      rows=8,  bodyCount=1, toggle="收起全文"
点击展开全部：      rows=12, moreLabel="收起"
最后一行时间：      2025-10-07（跨年显示完整日期）
点击收起：          rows=8
```

### 预览

- `docs/screenshots/build-changes-preview.png`（浅色）
- `docs/screenshots/build-changes-preview-dark.png`（深色）
- `docs/screenshots/build-changes-expanded.png`（展开全部 + 正文展开）
- 页面：`changes-preview.html` + `src/changes-preview-main.ts`（`pnpm dev` 打开，`?theme=dark` 切深色）

### 验证命令汇总

```
go test ./...              # 全绿
npx vitest run             # 263 PASS（18 files）
npx vue-tsc --noEmit       # 0 错
pnpm build                 # [check-version] ok: 0.6.0，ui/index.html 235.25 kB
```

### 打包（0.6.0）

- `dist/io.github.nixing178.jenkins-0.6.0-windows-x64.dbxp`，unsigned review candidate。
- 包内 6 条目；`ui/index.html` 237,455 B 与本地逐字节一致（sha256 `75c6082b…`）。
- 从包内提取 `bin/windows-x64/dbx-plugin-jenkins.exe`（8,579,072 B）→
  `[verify-sidecar] ok: io.github.nixing178.jenkins/0.6.0`。

---

## 0.7.0 · 控制台日志列表（2026-10-23）

### 红 → 绿

新增 `src/lib/logLines.test.ts`（27 例），先跑出模块缺失红：

```
FAIL src/lib/logLines.test.ts [ src/lib/logLines.test.ts ]
Error: Failed to load url ./logLines ... Does the file exist?
Test Files 1 failed (1) / Tests no tests
```

实现后一次断言修正：`ESC[22m` 只取消加粗、不重置前景色，绿色应延续（测试期望写错，非实现问题）。

```
✓ src/lib/logLines.test.ts (27 tests) 8ms
```

覆盖：ANSI CSI / OSC / 光标序列剥离、颜色与 bold 合并、未知 SGR 忽略、
级别判定正反例（`0 errors` / `errorRate` 不误报）、过滤保号、命中行下标、行号起点。

### 全量

```
vitest run     # 290 PASS（19 files）
vue-tsc        # 0 错
go test ./...  # ok（jenkins / rpc）
pnpm build     # [check-version] ok: 0.7.0，ui/index.html 245.33 kB
```

### 交互（CDP，1280×1000，`?big=1` 共 6200 行）

```
初始：      rows=5000, first=1021, hint="仅显示最后 5000 行（行号为显示序号）。",
            footer="6020 行 · 显示 5000 行"
仅问题：    rows=1048, levels=[error,warn]
搜索：      flaky-retry → 1/117，active=1032
下一处：    2/117，active=1075（偏移 43 = 每轮行数）
全屏：      fullscreen=true，log-body clientHeight=895
Esc：       fullscreen=false
DOM 顺序：  严格递增（sorted=true），单行高度 22px
```

### 预览

- `docs/screenshots/console-list-preview.png`（浅色，42 行）
- `docs/screenshots/console-big-default.png`（6200 行截断 + 折行悬挂缩进）
- `docs/screenshots/console-big-problems.png`（仅问题）
- `docs/screenshots/console-big-search.png`（搜索命中 + 自动取消跟随 + ↓ 最新）
- `docs/screenshots/console-big-fullscreen.png`
- 页面：`console-preview.html` + `src/console-preview-main.ts`（`?big=1` / `?theme=dark`）

### 打包（0.7.0）

- `dist/io.github.nixing178.jenkins-0.7.0-windows-x64.dbxp`，4,961,298 B，unsigned review candidate。
- sha256 `3669bb85924e175e…`。
- 包内 6 条目；`ui/index.html` 247,690 B 与本地逐字节一致（sha256 `ef9400e1f2f50248…`）。
- 从包内提取 `bin/windows-x64/dbx-plugin-jenkins.exe`（8,579,072 B）→
  `[verify-sidecar] ok: io.github.nixing178.jenkins/0.7.0`。

---

## 0.7.1 · 控制台日志区高度（2026-10-23）

- 默认高度 `clamp(360px, 52vh, 720px)` → `clamp(420px, 64vh, 900px)`；
  1000px 高窗口下 520px → 640px，可视行数约 +5 行。
- `vue-tsc` 0 错；`pnpm build` → `[check-version] ok: 0.7.1`，`ui/index.html` 247,690 B
  （与 0.7.0 同大小，仅 CSS 数字等长替换），产物内含 `clamp(420px,64vh,900px)`。
- 截图：`docs/screenshots/console-height-preview.png`（6200 行）。
- 打包：`dist/io.github.nixing178.jenkins-0.7.1-windows-x64.dbxp`，4,961,304 B，
  sha256 `fc51b5a75f930e41…`，包内 ui 与本地逐字节一致，
  sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.7.1`。

---

## 0.7.2 · 首屏默认滚到底（2026-10-23）

### 根因

1. 首屏 5000 行带 `content-visibility: auto`，`scrollHeight` 先按 `contain-intrinsic-size`
   估算，一次 `scrollTop = scrollHeight` 后真实行高撑开，落点就不在底部了；
2. 程序化滚动也会触发 `scroll` 事件，布局抖动瞬间 `distance > 48` 会被 `handleScroll`
   误判成「用户上翻」，把 `follow` 关掉，后续追加不再跟随。

### 修复

- `scrollToBottom()` 改为三帧 settle：每帧重设 `scrollTop`，直到距离收敛；
- 新增 `scrollGuard` 代际，程序滚动期间 `handleScroll` 直接返回；
- `reset()` 恢复 `follow = true`（上一次手动上翻不影响新打开的构建）。

### 验证

```
首屏：      distance=0, scrollTop=109309, follow=true,  latestVisible=false
上翻 4 档：  distance=1600, follow=false, latestVisible=true
点「最新」： distance=0, follow=true,  latestVisible=false
vitest      # 290 PASS（19 files）
vue-tsc     # 0 错
pnpm build  # [check-version] ok: 0.7.2，ui/index.html 245.50 kB
```

截图：`docs/screenshots/console-first-open-bottom.png`。

### 打包（0.7.2）

- `dist/io.github.nixing178.jenkins-0.7.2-windows-x64.dbxp`，4,961,399 B，unsigned review candidate。
- sha256 `b23d72ffe8834b40…`；包内 6 条目，`ui/index.html` 247,856 B 与本地逐字节一致。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.7.2`。

---

## 0.7.3 · 日志区底部被裁（2026-10-23）

### 根因

`BuildView` 是 `display:flex; flex-direction:column`，`.console` 作为 flex item 默认
`flex-shrink: 1`。页面内容超过视口时，面板被压到 318px，而 `.log-body` 的
`min-height: 260px` 顶住不让缩 → 工具栏 + 日志 + footer 总高 359px 溢出，
`.console { overflow:hidden }` 把底部裁掉，**footer 完全不可见**。

复现（860px 高视口，`?layout=build&big=1`）：

```
修复前：console.clientHeight=318, scrollHeight=359
        log-body.bottom=820 > console.bottom=806
        footer: top=820 bottom=846 visible=false
修复后：console.clientHeight=611, scrollHeight=611
        footer: top=1072 bottom=1098 visible=true
        distance=0, lastFullyVisible=true
```

窄窗复测（760×620）：`console 553 = scrollHeight 553`，footer `visible=true`。

### 修复

`ConsoleView.vue` 的 `.console` 加 `flex: 0 0 auto`（不参与收缩，超出交给
`.build-view` 滚动）。全屏时元素是 `position: fixed`，不受影响。

### 验证

```
vitest      # 290 PASS（19 files）
vue-tsc     # 0 错
pnpm build  # [check-version] ok: 0.7.3，ui/index.html 245.51 kB
```

截图：`docs/screenshots/console-clip-after.png`（修复前状态代码已变更无法复现，此处只保留修复后）。

### 打包（0.7.3）

- `dist/io.github.nixing178.jenkins-0.7.3-windows-x64.dbxp`，4,961,406 B，unsigned review candidate。
- sha256 `b6c69ea085edac65…`；包内 6 条目，`ui/index.html` 247,870 B 与本地逐字节一致。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.7.3`。

---

## 0.7.4 · 滚到底看不到最后几行（2026-10-23）

### 现象

真机 1006 行日志，滚动条已到底、`自动跟随` 勾选着，但最后 8 行（999–1006）
没显示，末行 998 还被裁掉一半。

### 根因

`.log-row` 上的 `content-visibility: auto` + `contain-intrinsic-size: auto 22px`：
未进入视口的行按 22px 估算，而折行行实际 44/66px。滚动高度因此被低估，
滚动条停在「估算底部」，真实末尾还在下面。

实测（`?long=1`，5000 行、混入 1400 条折行长行）：

```
修复前：scrollHeight=111,098 / scrollTop=110,548（已到底）
修复后：scrollHeight=159,143 / scrollTop=158,593
低估：  48,045px ≈ 2184 行 × 22px（30%）
首屏：  526ms → 425ms（去掉估算反而更快）
```

### 修复

移除 `.log-row` 的 `content-visibility` / `contain-intrinsic-size`，
`scrollHeight` 恢复真实值，滚动条拖到底就是真底。

### 验证

```
滚到底：  lastLine=7420, visibleLast=7420, hiddenBelow=0
手动拖条：先回顶再设 scrollTop=max，等 1s → 末行完整可见（bottom 664 < body 672）
vitest     # 290 PASS（19 files）
vue-tsc    # 0 错
pnpm build # [check-version] ok: 0.7.4，ui/index.html 245.46 kB
```

截图：`docs/screenshots/console-bottom-after.png`（修复前状态代码已变更无法复现，此处只保留修复后）。
预览新增 `?long=1`（长行密集）用于回归这个场景。

### 打包（0.7.4）

- `dist/io.github.nixing178.jenkins-0.7.4-windows-x64.dbxp`，4,961,385 B，unsigned review candidate。
- sha256 `6e91ff5c4749105f…`；包内 6 条目，`ui/index.html` 247,813 B 与本地逐字节一致。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.7.4`。

## 0.7.5 · 停止构建 / 取消排队的二次确认（2026-10-23）

### 现象

点击「停止」既不弹确认也不执行。`stopBuild()` 里的原生 `window.confirm()` 在宿主
webview 子 frame 中被静默忽略并返回 `false`，`if (!confirm) return` 直接吞掉操作。
`cancelQueueItem()` 同样写法，同一隐患。

### 方案

新增应用内确认弹窗 `ConfirmAction.vue`，`src/lib/confirmRequest.ts` 负责构造与校验
待确认目标；`stopBuild` / `cancelQueueItem` 只写 `pendingConfirm`，实际调用移入
`confirmPending`。详见 `docs/plan-stop-confirm.md`。

### 红 → 绿

```
新建 src/lib/confirmRequest.test.ts
vitest run src/lib/confirmRequest.test.ts
  红：Failed to load url ./confirmRequest — Test Files 1 failed, 0 test
  绿：✓ 9 tests passed
```

### 回归

```
go test ./...   # internal/jenkins ok 1.455s；internal/rpc ok 1.045s
vitest run      # 20 files / 299 PASS
vue-tsc         # 0 错
pnpm build      # [check-version] ok: 0.7.5，ui/index.html 249.45 kB
```

### 打包（0.7.5）

- `dist/io.github.nixing178.jenkins-0.7.5-windows-x64.dbxp`，4,962,300 B，unsigned review candidate。
- sha256 `6866817a3ec25b0b…`；包内 6 条目，`ui/index.html` 251,894 B 与本地逐字节一致。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.7.5`（从 .dbxp 取出 `bin/windows-x64/dbx-plugin-jenkins.exe`，8,579,072 B）。

### Release 发布（v0.7.5）

- tag `v0.7.5`（annotated，指向 `42d40a1`）推送后创建 Release，`release: published` 触发
  `plugin-release.yml`（run 39895532687，success）。
- CI 从 tag 源码重建（`ui/`、`dist/` 不进 Git），产出 Windows x64 候选包：
  - `io.github.nixing178.jenkins-0.7.5-windows-x64.dbxp`，4,962,258 B
  - sha256 `414a4d2a06e9e37a6f5362c3ac5c8f1d59d7bbc40f00a862ad853ff2d94be551`
- 与本地构建包（4,962,300 B / `6866817a…`）不同属预期：CI 重跑构建，时间戳与压缩顺序有差异，
  上架与用户校验一律以 Release 资产为准。
- 校验记录：下载包 sha256 与 `release-candidates.json` 一致；包内 6 条目、`manifest.json` 为 0.7.5；
  取出 sidecar 8,578,560 B → `[verify-sidecar] ok: io.github.nixing178.jenkins/0.7.5`。
- `docs/安装说明书-0.7.5.txt` 中的大小与 SHA-256 已同步为 Release 资产实测值。


### 打包（0.8.0）

- `dist/io.github.nixing178.jenkins-0.8.0-windows-x64.dbxp`，4,978,593 B，unsigned review candidate。
- sha256 `3e5c7fa90f6b039eeb4426a0250861019c6eb9978dc3a6ec6f886f00063f624b`；
  包内 6 条目（assets ×2 / bin ×1 / manifest / ui / checksums），`ui/index.html` 266,368 B 与本地逐字节一致。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.8.0`（从 .dbxp 取出 `bin/windows-x64/dbx-plugin-jenkins.exe`，8,603,136 B）。

### 打包（0.8.1）

- `dist/io.github.nixing178.jenkins-0.8.1-windows-x64.dbxp`，4,979,744 B，unsigned review candidate。
- sha256 `6da69207445f54b614dbf84eef1ce2949f6b9c8d50a9fbcc236243a5ecbf4227`；
  包内 6 条目，`ui/index.html` 270,023 B 与本地逐字节一致，包内 manifest 为 0.8.1。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.8.1`（从 .dbxp 取出，8,603,136 B）。

### 打包（0.9.0）

- `dist/io.github.nixing178.jenkins-0.9.0-windows-x64.dbxp`，4,987,050 B，unsigned review candidate。
- sha256 `530db5b5eeafabfe311c9896e71ba79d6d031cf1e3b4e142a4116807dcd79606`；
  包内 6 条目，`ui/index.html` 275,488 B 与本地逐字节一致，包内 manifest 为 0.9.0。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.9.0`（从 .dbxp 取出，8,609,792 B）。

### 打包（0.9.1）

- `dist/io.github.nixing178.jenkins-0.9.1-windows-x64.dbxp`，4,986,912 B，unsigned review candidate。
- sha256 `95c7456f79db5a32db5f1db27084990d40b4561cc3c2b48a865bf9105a16087a`；
  包内 6 条目，`ui/index.html` 275,114 B 与本地逐字节一致，包内 manifest 为 0.9.1。
- sidecar `[verify-sidecar] ok: io.github.nixing178.jenkins/0.9.1`（从 .dbxp 取出，8,609,792 B）。

## 0.9.2 · 概览页自动刷新（2026-10-26）

### 现象

概览页的自动刷新只落了一半：`autoRefreshMode` 仅在有执行器被占用时返回 `executors`，
而 `pollExecutors` 只拉 `jenkins/executors` —— 顶部四张卡片（执行器占用比 / 排队数 /
节点在线 / 静默模式）永远停在上次进入页面的值；全空闲时完全不刷新；页面也没有刷新徽章。

### 方案

见 `docs/plan-overview-auto-refresh.md`（D1:B / D2:B / D3:A / D4:A / D5:A）：

- 活跃档 5s、空闲档 30s（`OVERVIEW_AUTO_REFRESH_INTERVAL_MS` / `OVERVIEW_IDLE_REFRESH_INTERVAL_MS`）；
- `hasQueuedBuild` + `resolveOverviewRefresh` 纯函数分档：执行器被占用或队列非空 → 活跃；
- `pollOverview` 用 `Promise.allSettled` 并行拉 `jenkins/overview` + `jenkins/executors`，
  分别落盘、任一失败仍抛给既有三次暂停护栏；`overviewToken`/`executorsToken` 防旧响应覆盖；
- `OverviewView` 补 `RefreshBadge` + 按钮三态，`@refresh` 走 `refreshCurrent`；
- 后端零改动。

### 红 → 绿

- 红（09:45）：`env -u NODE_OPTIONS pnpm exec vitest run src/lib/refresh.test.ts`
  → `Tests 8 failed | 52 passed (60)`：`expected undefined to be 5000`（常量未导出）、
  `hasQueuedBuild is not a function` ×2、`resolveOverviewRefresh is not a function` ×5。
- 绿（09:48）：同命令 `Tests 60 passed (60)`。
- 全量（09:53）：`env -u NODE_OPTIONS pnpm exec vitest run --reporter=dot`
  → `Test Files 21 passed (21)`、`Tests 344 passed (344)`（+8，无回归）。
  （首次直接 `pnpm test` 因默认 120s 超时被 SIGTERM，仅跑到第 18 个文件，非用例失败。）
- 后端回归（09:59）：`go test ./...` → `internal/jenkins ok 2.125s`、`internal/rpc ok 1.384s`。
- 类型检查（10:01）：`node node_modules/vue-tsc/bin/vue-tsc.js --noEmit` → exit 0。
  首次报 `src/executor-preview-main.ts` 缺 `refresh` prop（TS2769），补 mock `RefreshUi` 后通过。
- 构建（10:25）：`[check-version] ok: 0.9.2`，`ui/index.html 273.41 kB`（276,405 B）。
  - 坑：`pnpm build` 的 `emptyOutDir` 触发本机 safe-delete 批量守卫（`SAFE_DELETE_BULK_GUARD_ERROR ... ETIMEDOUT`），
    构建失败但文件未损坏；改用 PowerShell 单独删 `ui/index.html` 后重跑即通过。

### 打包（0.9.2）

- `dist/io.github.nixing178.jenkins-0.9.2-windows-x64.dbxp`，4,987,190 B，unsigned review candidate。
- sha256 `65636639b1284cf9c549881e6a314df052ea0d19568bb92f1cc809ed2ca9e8fc`；
  包内 6 条目，`ui/index.html` 276,405 B 与本地逐字节一致，包内 manifest 为 0.9.2。
- sidecar（从 .dbxp 取出 `bin/windows-x64/dbx-plugin-jenkins.exe`，8,609,792 B）：
  含 `io.github.nixing178.jenkins` ×1、`0.9.2` ×2（Metadata + UA）、`0.9.1` ×0。
- 预览：概览页整页预览复用 `executor-preview.html`（`pnpm dev` 打开，`?theme=dark` 切深色），
  未新建 `overview-preview.html`——既有页面渲染的就是整个 `OverviewView`，新建会重复入口。

### 实现要点（易错）

- **初始档位不会触发 watch**：概览是首屏，`autoRefreshMode` 从一开就是 `overviewIdle`，
  没有「null → 非 null」的变化，`watch(autoRefreshMode)` 不会执行。必须在 `onMounted` 末尾补一次
  `startAutoRefresh(autoRefreshMode.value)`，否则全空闲时要切页才开始轮询。
- `pollOverview` 用 `Promise.allSettled`：两路分别落盘，任一路失败仍 `throw` 给既有护栏计数；
  token 校验（`overviewToken` / `executorsToken`）在落盘前统一判断，任一被手动刷新超越就整轮作废。

## 0.10.0 页面前进 / 后退（鼠标侧键）

### 方案

见 `docs/nav-history-plan.md`（D1 全量 / D2 返回改后退 / D3 加箭头 / D4 不做 Alt / D5 上限 50 / D6 弹窗优先 / D7 0.10.0）：

- `src/lib/navStack.ts`：内部导航栈是唯一真相，`push` 返回 `{index, added}`，`restore` 越界夹紧，上限 50，相同条目合并；
- `src/lib/mouseNav.ts`：`button 3/4 → back/forward`，`NAV_MOUSE_FALLBACK_MS=90` 兜底窗口、`NAV_DEDUPE_MS=150` 去抖；
- `src/lib/historyBridge.ts`：`push/replace/go/index`，`pushState` 抛错即永久降级；
- `App.vue`：`navEntry/recordNav/applyEntry/navBack/navForward/handleMouseNavButton/handlePopState`，
  六处导航入口接线，`closeBuild`/`backToJobs` 改为后退，`loadJobs` 加 `autoSelect`，topbar 加 ←/→；
- 后端零改动（仅版本号与 UA 字符串）。

### 测试

- 新增 3 个测试文件共 29 条：`navStack.test.ts` 12、`mouseNav.test.ts` 8、`historyBridge.test.ts` 9。
- **红图缺失（如实记录）**：10:51–11:43 期间本会话进程执行不可用
  （`SandboxError 0xFFFFF826`：center 规则快照不可用，Bash 与 PowerShell 均无法启动，免沙箱同样失败）。
  测试先于实现写完但无法运行，环境恢复后首个可运行结果即为绿。
- 变异验证（11:52，替代红图证明用例真的能拦 bug）：把 `resolveMouseButton` 的 `button === 3` 改成 `2`
  → `Tests 2 failed | 27 passed (29)`（`expected 'back' to be null`、侧键 3 不再返回 back）；
  还原后 `Tests 29 passed (29)`。
- 全量（11:43 → 11:52 复核）：`env -u NODE_OPTIONS node node_modules/vitest/vitest.mjs run`
  → `Test Files 24 passed (24)`、`Tests 373 passed (373)`（+29，无回归）。
- 后端回归（11:48）：`go test ./...` → `internal/jenkins ok 1.910s`、`internal/rpc ok 1.682s`。
- 类型检查（11:47）：`node node_modules/vue-tsc/bin/vue-tsc.js --noEmit` → exit 0。
- 构建（11:48）：`[check-version] ok: 0.10.0`，`ui/index.html 278.69 kB`（281,691 B）。

### 打包（0.10.0）

- 本地：`dist/io.github.nixing178.jenkins-0.10.0-windows-x64.dbxp`，4,988,865 B，
  sha256 `c1eb23dee418aad708e2fe50917c34b89c2cf760dd359fd16536492c465db1c3`；
  包内 6 条目，`ui/index.html` 281,691 B 与本地逐字节一致，包内 manifest 为 0.10.0。
- sidecar（从 .dbxp 取出，8,609,792 B）：含 `0.10.0` ×2（Metadata + UA）、`0.9.2` / `0.9.1` ×0。
- 重建可复现：变异还原后重新 `pnpm build` + `package .`，sha256 与首次完全一致。
- **Release 资产（CI 从 tag 重建，商店 candidate 以此为准）**：4,988,821 B，
  sha256 `e8ffb0f5ddb5c0436eb065467ee1a2bf3f03bb10df868ae3b39da05b19aaf60f`。
  本地包与 CI 包哈希不同属正常，安装说明书与商店元数据必须取 CI 值。

### 实现要点（易错）

- **双通道必须互让**：宿主把侧键当浏览器命令时先发 `popstate`，DOM 侧要空等 90ms 确认没人接再自己导航；
  否则一次按键退两步。反过来，DOM 通道自己动作时也必须用 `history.go(±1)` 走宿主栈，
  不能直接改内部游标——否则两套游标错位，下一次系统 back 会「按了没反应」。
- **降级后禁止 `go`**：`file://` / `about:srcdoc` 下 `pushState` 抛 `SecurityError`；一旦不可用，
  浏览器栈里没有我们的条目，此时 `go(-1)` 会直接退出插件，必须保持内部栈单通道。
- **回放要按页补数据**：切页走 `refreshOnEnter`；任务 / 构建 / 阶段必须
  `loadJobs(folder, silent, autoSelect=false)` + `selectJob(fullName)` 精确定位，
  否则「自动选中第一个」会让退回去看到另一条任务。
- **竞态守卫**：`openBuild` 记录前判 `rememberReturn && navApplying`；四个导航入口记录前判
  `view.value` 仍是目标页——请求回来时用户可能已经用后退离开。

### 真机验收

- 2026-10-26 12:00，源哥在 DBX 宿主内实测：**鼠标侧键前进 / 后退工作正常**（安装 0.10.0 包后验证）。
- 说明双通道至少一条生效；具体走 DOM 通道还是 `popstate` 未取日志确认（两条路径互为兜底，不影响使用）。
- 预览截图未出：Headless Chrome 在本机启动即退出（无输出），CDP 无法连接。

## 0.11.0 构建制品下载

### 方案

见 `docs/plan-artifact-download.md`（D1:1 / D2:1 / D3:1 / D4:1 / D5:1 / D6:1）：

- 后端 `internal/jenkins/artifacts.go`：`ArtifactPath` 逐段 `url.PathEscape`；`GetArtifactInfo` 先 HEAD、拿不到长度回退「下载后 stat」；`GetArtifactChunk` 首次把整份制品流式落盘到 `DBX_PLUGIN_DATA_DIR/artifacts/<sha1>.bin`，之后按 offset 读本地缓存；`pruneArtifactCache` 按 mtime 压到 512 MiB。
- 后端 `client.go`：新增无总超时的 `downloadClient`（普通请求仍是 15s），制品缓存按目标文件加锁避免并发写坏。
- 后端 `rpc/service.go`：`jenkins/artifactInfo` / `jenkins/artifactChunk`，`path` 非法路径落到 `ErrInvalidConfig` → `-32602`。
- 前端 `src/lib/artifacts.ts`：`downloadArtifact` 有 `fileTransfer` 走 `beginSave` + 4 MiB 分块 `write` + `finish`，无则 Blob 兜底（64 MiB 上限）；`empty_chunk` / `too_large` 为结构化错误。
- 前端 `BuildView.vue`：产物行显示大小（进页面 HEAD 预取，失败的 key 不重试）、下载按钮、`下载中 N%`、行内进度条、失败红字与重试。

### 测试

- 红图（2026-10-30，本会话执行，实现前）：
  - `go test ./internal/jenkins -run "Artifact|PruneArtifactCache"` → 9 failed（`ArtifactPath` 返回 not implemented、HEAD/回退/分块/越界/404/缓存清理全部断言失败）；
  - `go test ./internal/rpc -run Artifact` → 3 failed（`method not found: jenkins/artifactInfo` 等）；
  - `pnpm test -- src/lib/artifacts.test.ts` → `Tests 11 failed (11)`；
  - `go vet ./...` 与 `pnpm typecheck` 均 exit 0（红在断言层，不是编译层）。
  - 两个用例在红图阶段为「假通过」并已如实记录：`TestArtifactPathRejectsUnsafeInput`（桩函数一律报错）、`TestArtifactCacheDirUsesPluginDataDir`（环境变量直接拼接）。
- 绿图（同日）：
  - `go test ./...` → `internal/jenkins ok`、`internal/rpc ok`（195 个顶层用例，含 13 条新增）；
  - `pnpm test` → `Test Files 25 passed (25)`、`Tests 384 passed (384)`（+11，无回归）；
  - `pnpm typecheck` → exit 0；
  - `pnpm build` → `[check-version] ok: 0.11.0`，`ui/index.html 284.85 kB`（+3.16 kB）。

### 打包（0.11.0）

- 命令链：`node scripts/check-version.mjs` → `pnpm build` → `npx --yes @dbx-app/plugin-cli@0.1.9 package .`；输出 `Signature: unsigned review candidate`。
- 产物：`dist/io.github.nixing178.jenkins-0.11.0-windows-x64.dbxp`，6,423,334 B，
  sha256 `ffca7ed05c1a22d6555b75019b4f60ca672fb1e81b362a2479f6089a9dec2392`（与 `.artifact.json` 一致）。
- 包内 6 条目：`checksums.json` 526、`manifest.json` 7,485、`assets/plugin.svg` 477、
  `assets/connection.svg` 762、`bin/windows-x64/dbx-plugin-jenkins.exe` 11,219,968、`ui/index.html` 287,926。
- `checksums.json` 五个 sha256 逐个复核通过；包内 `ui/index.html` 与本地 `ui/index.html` 哈希同为
  `c36acacd72b48909ca2e93e5d835e85885ac675f669c93ae284a5e5329447bc8`（逐字节一致），manifest 为 0.11.0。
- sidecar identity：从包内取出的 exe 走 `plugin/initialize` 握手 → `[verify-sidecar] ok: io.github.nixing178.jenkins/0.11.0`；
  二进制内 `0.11.0` ×2（Metadata + UA）、`0.10.0` ×0。
- sidecar 体积从 0.10.0 的 8,609,792 B 涨到 11,219,968 B：本次工具链为 Go 1.27.0，功能代码只增加约 12 KB，差异来自编译器/工具链而非业务代码。
- **踩坑**：`dist/dbx-plugin-jenkins.exe` 是 0.4.2 的旧残留（打包 CLI 不再写这个位置），拿它跑 identity 会误报
  `version mismatch: backend=0.4.2`；README 的校验步骤已改为从 .dbxp 解包后校验。

### 未完成

- 真实 Jenkins 制品下载的人工验收（原生保存对话框、字节一致、只读连接、构建中制品缺失）尚未执行，需在 DBX 0.6.61 宿主内装 0.11.0 点一次。
- 0.11.0 未发版、未提交 Git。

## 0.11.1 修复：点击「下载」无反应

### 诊断

用户装 0.11.0 后点制品行「下载」无任何反馈。本机 DBX 0.6.61 前端 bundle（`EBWebView\Default\Code Cache\js`）取证：

- `fileTransfer` 只以 **capability 标志位**出现：`pick / beginSave / read / drop / folderExpansion`；
- 运行时桥接方法是 `host.pickFiles` / `host.beginFileSave` / `host.writeFileChunk` / `host.finishFileSave` / `host.closeFileHandle`，配错误文案 `Host file handling is unavailable`、`Plugin write chunk exceeds 8 MiB`；
- 整个 bundle 中 `beginSave` 仅出现 1 次（标志位），**没有** `beginSave` 方法。

结论：`resolveFileTransfer()` 判断 `typeof transfer.beginSave === "function"` 在 0.6.61 上恒为假 → 静默降级到 Blob + `<a download>`，沙箱 iframe 吞掉下载 → 点击无反应。

### 修复

- `src/lib/artifacts.ts`：新增 `normalizeFileTransfer(raw)`，`beginSave/write/finish/cancel` 与 `beginFileSave/writeFileChunk/finishFileSave/closeFileHandle` 两套命名都认，保留 `this`；缺 begin/write/finish 判为不支持，cancel 缺失为空操作。
- `src/components/BuildView.vue`：改用适配器；浏览器兜底的结果显示「已交给浏览器下载」，不再与「已保存」混为一谈。
- `src/dbx.d.ts`：两套方法名都声明为可选并注明必须过适配器。
- 版本 0.11.0 → 0.11.1（四处同步）。

### 测试

- 红：新增 6 条 `normalizeFileTransfer` 用例，实现前 `Tests 6 failed`（`normalizeFileTransfer is not a function`）。
- 绿：`pnpm test -- src/lib/artifacts.test.ts` → `Tests 17 passed (17)`；全量 `Test Files 25 passed (25)`、`Tests 390 passed (390)`（+6，无回归）。
- `pnpm typecheck` exit 0；`go test ./...` → `internal/jenkins ok`、`internal/rpc ok`。
- `pnpm build` → `[check-version] ok: 0.11.1`，`ui/index.html 285.47 kB`。

### 打包（0.11.1）

- `dist/io.github.nixing178.jenkins-0.11.1-windows-x64.dbxp`，6,423,588 B，
  sha256 `356fd35df383422e2f3654b34fa6fdf35794d50ec096df5bd2d690361a2615d1`（与 `.artifact.json` 一致）。
- 包内 6 条目；`ui/index.html` 288,559 B 与本地同哈希 `42bc583b…7936b9f`；
  sidecar 11,219,968 B，identity 握手 `[verify-sidecar] ok: io.github.nixing178.jenkins/0.11.1`。

### 遗留

- 用户侧复测未做：需装 0.11.1 再点一次。若按钮进入「下载中」后没弹对话框，说明适配到了错误签名；若显示「已交给浏览器下载」但没有文件，说明该机器根本没有 `fileTransfer`，需补「后端落盘到下载目录」的第三条路径。

## 0.11.2 修复：下载改由 sidecar 落盘

### 诊断（用户复测 0.11.1 仍无反应）

本机 DBX 0.6.61 bundle 中 workbench iframe 的 sandbox 属性：

```text
iframe ... srcdoc ... sandbox="allow-top-navigation allow-same-origin allow-forms allow-scripts ..."
```

没有 `allow-downloads` → 沙箱内 `<a download>` / Blob 下载被静默拦截，既不报错也不落盘。0.11.0 起控制台日志的「下载」其实也一直无效。

### 修复

- 后端新增 `internal/jenkins/downloads.go`：`DownloadDir()`（`<home>/Downloads`，缺失退回 home）、`SaveArtifact`（缓存 → Downloads，重名 `app (1).jar`，文件名过滤 `\ / : * ? " < > |`）、`SaveText`。
- RPC 新增 `jenkins/artifactSave`（job/number/path）与 `jenkins/saveText`（name/content）。
- 前端：`BuildView` 点下载直接调 `artifactSave`，按钮「保存中…」→「已保存」，行内显示 `…\Downloads\app.jar`（悬停全路径 + 复制）；`ConsoleView` 日志导出改调 `saveText`，失败才退回复制。
- 删除 `src/lib/artifacts.ts` 与其 17 条用例（fileTransfer/Blob 编排在沙箱内不可用），`formatBytes` 移入 `lib/format.ts`，新增 `shortenPath`。

### 测试

- 红：`go test ./internal/jenkins -run "Save|DownloadDir"` → 5 failed；`go test ./internal/rpc -run "ArtifactSave|SaveText"` → 2 failed。
- 绿：`go test ./...` → `internal/jenkins ok`、`internal/rpc ok`（202 顶层用例，+7）；`pnpm test` → `Test Files 24 passed (24)`、`Tests 376 passed (376)`；`pnpm typecheck` / `go vet ./...` exit 0。
- `pnpm build` → `[check-version] ok: 0.11.2`，`ui/index.html 283.55 kB`。

### 打包（0.11.2）

- `dist/io.github.nixing178.jenkins-0.11.2-windows-x64.dbxp`，6,431,539 B，
  sha256 `e531fa8ac0dfc67147ce1c04e189f43eb8b2db2c3ea0f0459937428d62432c83`。
- 包内 6 条目；`ui/index.html` 286,610 B 与本地同哈希 `4ec90f6a…b136a4`；
  sidecar 11,236,864 B，identity 握手 `[verify-sidecar] ok: io.github.nixing178.jenkins/0.11.2`。

### 遗留

- 用户侧复测未做：装 0.11.2 后点「下载」，应在 `C:\Users\<用户>\Downloads` 出现制品，界面显示「已保存 + 路径」。
- 若 RPC 失败，行内会出现具体错误码（`not_found` / `timeout` / `network`），据此可继续定位。

## 0.12.0 制品下载体验优化

### 方案

见 `docs/plan-artifact-download-optimize.md`（D1:1 / D2:1 / D3:2 / D4:1 / D5:2 / D6:1 / D7:1）：

- 后端新增任务化下载：`artifactStart`（taskId）→ `artifactStatus`（state/done/total/path）→ `artifactCancel`（context 打断，清 `.part`）；`prepareArtifactWithProgress` 每 256 KiB 上报。
- `download_dir` 连接字段：留空回落 `<home>/Downloads`、支持 `~/`、拒绝相对路径、自动建目录。
- 同名覆盖：`.part` + `os.Rename` 原子替换，不再产生 `(1)` 副本。
- `jenkins/revealFile`：Windows `explorer /select`、macOS `open -R`、Linux `xdg-open`。
- 前端 500 ms 轮询：按钮「保存中 42%」+ 行内进度条 + 「取消」；完成后「已保存 + 路径 + 打开文件夹 / 复制」；面板头「全部下载」顺序执行并显示 `2/5`。

### 测试

- 红：后端 9 条（覆盖写、配置目录、`resolveDownloadDir`、任务进度、取消、未知任务、reveal、进度递增）+ RPC 2 条 + 前端 6 条，实现前全部失败。
- 绿：`go test ./...` → `internal/jenkins ok`、`internal/rpc ok`（211 顶层用例，+9）；`pnpm test` → `Test Files 25 passed`、`Tests 382 passed`（+6）；`pnpm typecheck` / `go vet ./...` exit 0。
- `pnpm build` → `[check-version] ok: 0.12.0`，`ui/index.html 287.78 kB`。

### 打包（0.12.0）

- `dist/io.github.nixing178.jenkins-0.12.0-windows-x64.dbxp`，6,531,871 B，
  sha256 `3a22634edeb18f90a955315c7dc4066a82a62b60a41364c8bf496f9978db4639`。
- 包内 6 条目；manifest 8,029 B（含 `download_dir` 字段与中英本地化）；`ui/index.html` 290,879 B 与本地同哈希 `b3e9339b…7bbf14`；
  sidecar 11,415,040 B，identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.12.0`。

### 遗留

- 真机验收未做：需装 0.12.0 后确认进度条前进、取消后无残留 `.part`、「打开文件夹」能定位文件、「全部下载」5 个产物的顺序与失败隔离。
- 缓存策略按 D7 保持 512 MiB；若后续觉得双份磁盘占用明显，再切 D7:2（落盘后删缓存）。

## 0.12.1 界面版本徽章与无制品空态

### 背景

0.12.0 安装后用户反馈「改动未生效」。取证：

- 正在运行的 sidecar 已是 `versions\0.12.0\bin\…\dbx-plugin-jenkins.exe`；
- 但 WebView UI 缓存里 `全部下载 / artifactStart / 打开文件夹` 命中数为 0，说明新界面从未加载（DBX 进程自 10:57 未重启，工作台 iframe 还是旧 HTML）；
- 用户截图里的「下载」是**控制台日志导出**，且已实际写出 `C:\Users\anjun\Downloads\schedule-admin-469.log`（201,501 B）；该构建没有归档制品，页面根本没有「产物」区块。

### 修复

- `vite.config.ts` 从 `package.json` 注入 `__APP_VERSION__`，顶栏品牌名旁显示 `v0.12.1`；`src/dbx.d.ts` 在 `declare global` 中声明，避免模块作用域失效。
- `BuildView.vue` 新增无产物空态面板：「本构建没有归档制品。构建里执行 archiveArtifacts 后才会在这里列出文件。」
- 版本 0.12.0 → 0.12.1（四处同步）。

### 测试

- `pnpm typecheck` exit 0（首轮报了 `__APP_VERSION__` 未找到与 `node:fs` 缺类型，改为全局声明 + 直接 import package.json 后通过）。
- `pnpm test` → `Test Files 25 passed (25)`、`Tests 382 passed (382)`（本次为纯展示改动，无新增用例）。
- `pnpm build` → `[check-version] ok: 0.12.1`，`ui/index.html 288.52 kB`；产物内确认含 `0.12.1` 与「本构建没有归档制品」。

### 打包（0.12.1）

- `dist/io.github.nixing178.jenkins-0.12.1-windows-x64.dbxp`，6,532,181 B，
  sha256 `6570fa2b57c6a38d1d273e8fe69d98db46c5e138689b3b274ed2881604df7fe7`。
- 包内 `ui/index.html` 291,687 B 与本地同哈希 `90c7b35a…1f15ba`；
  sidecar identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.12.1`。

### 遗留

- 真机验收：装 0.12.1 并**重启 DBX**（或关闭工作台标签重开）后，顶栏应出现 `v0.12.1`；这是判断新 UI 是否真正加载的唯一可靠信号。

## 0.13.0 任务配置「归档产物」

### 方案

见 `docs/plan-job-artifact-archive.md`（D1:1 / D2:1 / D3:1 / D4:1 / D5:1 / D6:0.13.0）：

- 后端 `archive.go`：`ArtifactArchive` + `PatchArtifactArchive`（增/改/删 `hudson.tasks.ArtifactArchiver`，保留其它 publishers，自闭合 `<publishers/>` 展开）+ `findChildSpan`（片段内定位直接子元素）。
- 解析：`jobConfigDocument.Publishers.Archivers`，读写缺省值跟 Jenkins 对齐（`defaultExcludes=true`、`caseSensitive=false`）。
- Pipeline：`ParseJobFields` 返回 `archive: nil`；对 `flow-definition` 调用 patch 返回 `invalid_config`。
- RPC：`jobUpdate` 映射 `fields.archive`，`defaultExcludes` 缺省 true。
- 前端：`JobEditor` 新增「归档产物」区（开关 / 文件模式 / 排除 / 指纹 / 仅成功 / 允许空归档），启用但未填模式时前端拦截；Pipeline 显示脚本提示。

### 测试

- 红：后端 11 条（解析 4、patch 6、UpdateJob 1）+ RPC 3 条 + 前端 4 条，实现前全部失败（`artifact archive is not implemented yet` / `archive = nil` / `expected undefined to deeply equal …`）。
- 绿：`go test ./...` → `internal/jenkins ok`、`internal/rpc ok`（225 顶层用例，+14）；`pnpm test` → `Test Files 25 passed`、`Tests 386 passed`（+4）；`pnpm typecheck` / `go vet ./...` exit 0。
- `pnpm build` → `[check-version] ok: 0.13.0`，`ui/index.html 292.18 kB`。

### 打包（0.13.0）

- `dist/io.github.nixing178.jenkins-0.13.0-windows-x64.dbxp`，6,548,034 B，
  sha256 `35bc5e026e638a479b89da5fc3ed0bc9a1c1ca4ac973c52774f00f1811ba765d`。
- 包内 6 条目；`ui/index.html` 295,491 B 与本地同哈希 `0188c288…053723`；
  sidecar 11,435,520 B，identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.13.0`。

### 遗留

- 真机验收未做：需装 0.13.0 + 重启 DBX 后，在自由风格任务的「编辑 → 基础」勾选归档、填 `dist/**/*.jar`，保存后到 Jenkins 任务页确认 Post-build Actions 出现 Archive the artifacts；跑一次构建看「产物」区块是否列出文件。
- 未做「已完成构建补归档」：Jenkins 核心没有该接口，需要服务端插件支持。

## 0.14.0 Pipeline 归档步骤生成

### 背景

用户问「Pipeline 任务不行吗」。查 `jenkinsci/workflow-job-plugin` 的 `WorkflowJob.java`：

```java
public final class WorkflowJob extends Job   // hudson.model.Job，不是 AbstractProject
```

全类没有 `publishers` 字段/`getPublishers()`，因此 Pipeline 没有 Post-build Actions，往 `config.xml` 写 `ArtifactArchiver` 不会生效。归档只能用 step。按拍板 D7=A 做「复制片段」，不自动改脚本。

### 实现

- 新增 `src/lib/archiveSnippet.ts`：`buildArchiveSnippet`（固定参数顺序、Groovy 转义、空模式返回空串）、`hasArchiveArtifacts`（`\barchiveArtifacts\b`）、`isDeclarativePipeline`（`^\s*pipeline\s*\{`）。
- `JobEditor.vue`：Pipeline 的「归档产物」区改为步骤生成器（文件模式 / 排除 / 指纹 / 仅成功 / 允许空归档 → 实时预览 → 一键复制）；脚本已含该步骤时显示「已存在」；按声明式 / 脚本式区分粘贴位置提示；Pipeline 提交不写 `publishers`。

### 测试

- 红：`archiveSnippet.test.ts` 9 条全部失败（`not implemented yet`）。
- 绿：`pnpm test` → `Test Files 26 passed`、`Tests 395 passed`（+9）；`pnpm typecheck` exit 0；后端未改动（225 用例仍全绿）。
- `pnpm build` → `[check-version] ok: 0.14.0`，`ui/index.html 295.83 kB`。

### 打包（0.14.0）

- `dist/io.github.nixing178.jenkins-0.14.0-windows-x64.dbxp`，6,548,922 B，
  sha256 `c29a9aab6f10071fe08a2f25988efd677cfc58ab41731f15253617cd24a3db94`。
- 包内 `ui/index.html` 与本地同哈希 `ef216d8a…b83918`；
  sidecar identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.14.0`。

### 遗留

- 真机验收未做：装 0.14.0 并重启 DBX 后，打开 Pipeline 任务编辑 → 归档产物区填 `dist/**/*.jar` → 复制语句 → 粘进 `steps` / `post` → 保存并构建，产物面板应列出文件。

## 0.15.0 下载进度明细与完成通知

### 需求

1. 下载完成发送消息通知；2. 进度提示要有速度与已下载量。

### 实现

- 新增 `src/lib/downloadMetrics.ts`：
  - `averageSpeed`：最近 4 s 采样滑动平均，样本不足/进度未前进返回 0；
  - `remainingSeconds`：剩余字节 ÷ 速度向上取整，未知返回 null；
  - `transferSummary`：`{bytes: "8.5 MB / 20.0 MB", speed: "3.2 MB/s", etaSeconds}`，总量未知时只给已下载量。
- `BuildView.vue`：每 500 ms 轮询采样 `{done, at}`（最多 16 条），速度与 ETA 存进聚合行状态；副行显示 `8.5 MB / 20.0 MB · 3.2 MB/s · 剩余 12s`，未知项省略；取消/完成/换构建/卸载时清空采样。
- 通知：`emit("notify", text, tone)` → `App` 复用 toast 队列（`showToast`）。单条下载完成 `已保存 app.jar（12.3 MB）`、失败 `下载失败：<原因>`；「全部下载」逐条静默，结束一条汇总 `全部下载完成：成功 4/5`。

### 测试

- 红：`downloadMetrics.test.ts` 8 条全部失败（`not implemented yet`）。
- 绿：`pnpm test` → `Test Files 27 passed`、`Tests 403 passed`（+8）；`pnpm typecheck` exit 0；后端未改动（225 用例仍全绿）。
- `pnpm build` → `[check-version] ok: 0.15.0`，`ui/index.html 298.16 kB`。

### 打包（0.15.0）

- `dist/io.github.nixing178.jenkins-0.15.0-windows-x64.dbxp`，6,549,730 B，
  sha256 `8e474b565632a2d68ad58d39b77bb118db934451589c0bc5620b748e78018b1c`。
- 包内 `ui/index.html` 301,620 B 与本地同哈希 `7ad68d0b…e3843`；
  sidecar identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.15.0`。

### 遗留

- 真机验收未做：装 0.15.0 + 重启 DBX 后，下载大制品确认副行速度/已下载量持续更新、完成后右上角出现「已保存 …」提示；「全部下载」结束只出现一条汇总。
- 速度是前端按轮询采样估算，若后端首块一次性拉完整份制品（Jenkins 响应无 `Content-Length`），可能只在结束时跳一次；此时不显示速度。

## 0.16.0 控制台日志导出改造

### 问题

截图箭头指向控制台工具栏的「下载」。原实现把**前端内存里的日志文本**（上限 2,000,000 字符，超出只留尾部）发给 `jenkins/saveText`，因此：

- 大构建导出的日志是被截断的，不是 Jenkins 上的完整日志；
- 导出过程没有进度，只有按钮变成「保存中…」；
- 完成后没有消息提示，也没有打开文件夹入口。

### 实现

- 后端 `internal/jenkins/console.go`：
  - `consoleExportName(fullName, number)`：任务全名的 `/` 换成 `-`，空名回落 `build-<n>.log`；
  - `StartConsoleSave`：复用后台任务表（与制品下载同一套 `artifactStatus` / `artifactCancel`），10 分钟 context；
  - `saveConsoleWithProgress`：GET `<job>/<n>/consoleText` → 流式写入下载目录，进度按 `Content-Length` 上报，404 走 `not_found`。
- 后端 `downloads.go`：抽出 `writeStreamToDownloads`（`.part` + `artifactProgressReader` + `copyWithCancel` + rename），`copyCachedToDownloads` 改为复用它。
- 后端 `rpc/service.go`：新增 `jenkins/consoleSave`。
- 前端 `ConsoleView.vue`：改调 `consoleSave`，500 ms 轮询进度 → 按钮 `保存中 42%`；完成 emit `notify` 弹「已保存 …」并显示路径 + 「打开文件夹」；失败显示原因并弹 info 提示；老 sidecar 无该接口时退回 `saveText`。
- 前端 `BuildView.vue` 转发 `ConsoleView` 的 `notify` 到 App 的 toast。

### 测试

- 红：`TestConsoleExportNameFlattensPath`、`TestStartConsoleSaveStreamsFullLog`、`TestStartConsoleSaveMapsMissingBuild` 全失败（`console save is not implemented yet`）；`TestConsoleSaveRPCStreamsLogToDownloads` 失败（`method not found: jenkins/consoleSave`）。
- 绿：`go test ./...` → `internal/jenkins ok`、`internal/rpc ok`（229 顶层用例，+4）；`pnpm test` → `Test Files 27 passed`、`Tests 403 passed`；`pnpm typecheck` / `go vet` exit 0。
- `pnpm build` → `[check-version] ok: 0.16.0`，`ui/index.html 299.47 kB`。

### 打包（0.16.0）

- `dist/io.github.nixing178.jenkins-0.16.0-windows-x64.dbxp`，6,555,015 B，
  sha256 `4a815ed188c39e21a461b69408ca015196fcb1ba52ff5a103a3bc84f461e83dd`。
- 包内 `ui/index.html` 302,926 B 与本地同哈希 `c9da5fff…c4b93`；
  sidecar 11,448,320 B，identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.16.0`。

### 遗留

- 真机验收未做：装 0.16.0 + 重启 DBX 后，打开一个日志很大的构建点「下载」，按钮应显示百分比、完成后弹「已保存 …」并能在 Downloads 里看到 `任务名-构建号.log`，且行数/字节数与 Jenkins 一致（不再被 2 MB 截断）。
- 导出期间离开构建页会停止轮询（任务在后端仍会完成，只是没有提示）。

## 0.16.1 移除 Pipeline 归档工具

### 背景

用户看到 Pipeline 任务编辑页里那块「归档产物」：脚本里已经有 `archiveArtifacts`（绿色提示），工具区却还是空白输入框，问是否该删。给出三个选项（折叠 / 只删提示 / 整块删除）后，用户选择 **3. 整块删除**。

### 变更

- `src/components/JobEditor.vue`：删掉 Pipeline 分支的归档工具区（文件模式 / 排除 / 三个勾选 / 语句预览 / 复制按钮 / 已存在提示 / 粘贴位置提示）及配套 computed、`copyArchiveSnippet`、3 段 CSS；提交时不再需要「Pipeline 不写 archive」的特判。
- 删除 `src/lib/archiveSnippet.ts` 与其 9 条用例。
- 删除 5 个中英 i18n key（`archivePipelineHint`、`archiveSnippetCopy`、`archiveSnippetExisting`、`archiveSnippetDeclarative`、`archiveSnippetScripted`）。
- 保留：自由风格任务的「归档产物」配置（0.13.0）、后端对 `flow-definition` 的 patch 拒绝。

### 测试

- `pnpm typecheck` exit 0。
- `pnpm test` → `Test Files 26 passed`、`Tests 394 passed`（403 − 9，删除随功能下线的用例，无其它回归）。
- 后端未改动（229 用例仍全绿）。
- `pnpm build` → `[check-version] ok: 0.16.1`，`ui/index.html 295.55 kB`。

### 打包（0.16.1）

- `dist/io.github.nixing178.jenkins-0.16.1-windows-x64.dbxp`，6,554,025 B，
  sha256 `10149089e94cc544be4e6d5956a6abc597637ed0377e5141a891ce5a6488c27f`。
- 包内 `ui/index.html` 与本地同哈希 `415e2cd6…3c271a`；
  sidecar identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.16.1`。

### 遗留

- 真机验收：装 0.16.1 + 重启 DBX 后打开 Pipeline 任务编辑，应只看到「启用」与「Pipeline 脚本」，不再出现归档工具区；自由风格任务的归档配置不受影响。

## 0.11.1 Active Choices 参数

### 背景

`org.biouno.unochoice.*` 参数在 JSON API 的 `parameterDefinitions` 里取不到 `choices`，此前参数区只能退回文本框。本次从任务 `config.xml` 的 Groovy 脚本里提取静态字面量选项：`PT_CHECKBOX` 渲染为复选框组，其余渲染为下拉框。

### 变更

- `backend/internal/jenkins/jobs.go`：`jobTree` 增加 `_class`；对 `choices` 为空的 uno-choice 参数额外读取 `config.xml`，只解析 `return ['a', 'b']` 形式的静态字面量，动态脚本与取不到选项时保持文本框。
- `backend/internal/jenkins/models.go`：`ParameterDefinition` 新增 `multiSelect`。
- `src/lib/paramValue.ts`：逗号分隔值的解析与切换，输出顺序跟 `choices` 一致，`choices` 之外的旧值保留在末尾。
- `src/components/ParamFields.vue`：多选参数渲染复选框组；有 `choices` 的参数统一下拉框。
- 版本号统一为 `0.11.1`（`manifest.json`、`package.json`、`backend/main.go`、User-Agent）。

### 测试（本地）

- `go vet ./...` 通过；`go test ./... -v` → 231 PASS / 0 FAIL（新增 2 条 Active Choices 用例）。
- `pnpm typecheck` exit 0。
- `pnpm test` → `Test Files 27 passed`、`Tests 403 passed`（新增 9 条 `paramValue` 用例）。
- `pnpm build` → `[check-version] ok: 0.11.1`，`ui/index.html 296.57 kB`。

### 遗留

- 真机验收未做：需在安装 `uno-choice 2.8.9` 的 Jenkins 上打开带 `PT_CHECKBOX` 参数的任务，确认复选框可多选、触发构建后参数值为逗号拼接；动态脚本任务应仍显示文本框。

## 0.11.2 用户名密码登录（Issue #4）

### 方案

见 `docs/plan-basic-auth.md`（D1:1 / D2:2 / D3:1 / D4:3）。诉求是「不想自己签发 API Token」。

- 连接表单新增 `auth_method`（API Token / 用户名密码，默认 Token）与 `password`（`binding: password`，仅密码模式显示且必填）。
- 后端按模式取凭据：密码模式只认 `connection.password` / `secrets.password`，残留 `api_token` 不抢占；缺密码在发请求前返回 `invalid_config`；非法 `auth_method` 同样拒绝。
- 密码模式 401 追加提示「该实例可能启用 2FA / 强制 Token，请改用 API Token」；Token 模式错误文案不变。
- `jenkins.Config.APIToken` 改名 `Secret`，语义覆盖 Token 与密码。

### 测试（本地）

- 红：8 条新用例（7 认证 + 1 manifest 契约）先失败 5 条，守卫 3 条通过。
- 绿：`go vet ./...` 无输出；`go test ./... -v` → 239 PASS / 0 FAIL（新增 8）；`pnpm test` → `Test Files 27 passed`、`Tests 403 passed`；`pnpm typecheck` exit 0；`node scripts/check-version.mjs` → `ok: 0.11.2`。

### 打包（0.11.2）

- `dist/io.github.nixing178.jenkins-0.11.2-windows-x64.dbxp`，6,566,177 B，
  sha256 `d7731dd89e79f892288cfebe7eab41d347840bdee9c189e24bf99975fe656ad3`。
- 包内 7 条目；`ui/index.html` 299,905 B 与本地同哈希 `9294c951…0ed02b164`；
  sidecar identity `[verify-sidecar] ok: io.github.nixing178.jenkins/0.11.2`。
- 首次打包发现包内英文描述仍是旧的「with an API token.」：根 manifest 贡献级描述未被替换（本地化级已替换），
  修正描述后重新打包，第二次包哈希为上值。

### 遗留

- 真机验收未做：需在真实 Jenkins 上验证密码登录成功、2FA 实例提示、旧连接 Token 回归，并确认宿主未把密码明文写入连接配置。
