# JW Drama ACP 分段写入修复与隔离验收

后续合并记录：已按用户指定的 `JW-Drama-Providers-20260905/win-unpacked` 基版生成 `D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905` 合并候选包，重新通过四个真实 ACP 本地模拟端点场景。当前包与限制见该目录的 `合并验收说明.md`、`package-verification.json`；下文独立二进制哈希是此前的验收快照，不是合并包哈希。原运行版仍未替换，全仓门禁未全部通过。

日期：2026-09-05。状态：源码修复和定向验收通过；尚未替换运行版，也未宣布整个仓库满足发布门槛。

## 修复内容

1. 撤回此前人为加入的 ACP 调用计数限制：总调用/回合上限、同文件写入次数、重复参数次数，以及第二次原生上下文压缩强制取消。工具通知不是执行前的权限边界，相同输入也不等于没有进展。保留原生 ACP、用户取消、权限请求、进程退出处理与真正的上游失败传播。
2. 修复后台章节同步改变原生文件工具写入路径的问题。原来的 reconcileProjectSectionsUnlocked 调用普通 saveUnlocked 时，会按 frontmatter 的 title 重命名 Markdown。验证中 script.md 的第一批因此被搬到 JW long-task fixture.md，后续追加重新创建 script.md。现仅后台章节锚点同步保存时保留已有文件名，显式文档修改仍保留原有标题投影/改名行为。
3. 不修改原写作 Skill、TOOLS.md、Provider API 配置、上下文窗口或 Provider 重试参数。不引入 Hermes 内核，不替换 Codex ACP。
4. 清理测试基础设施：NO_PROXY 测试按 Windows 环境变量大小写不敏感及现有合并行为断言；文档测试显式关闭各自 SQLite 数据库，解决 Windows 临时目录删除时的文件锁。没有为这两项改动生产代理或数据库实现。

删除的 acp_tool_loop_guard.go 和对应旧测试是有意撤回错误限制，均为已跟踪文件，可通过 Git 历史恢复；未删除用户文稿。

## 证据与验收范围

新增 acp_long_task_test.go：7 种 60 次工具调用序列，包括重复读取、相同 shell 输入、同文件分批写入、Skill 重载、用户交互等待、跨 5 次压缩通知继续运行。取消、429/503 错误及正常压缩信息另有测试。

新增 native_writer_path_test.go：根目录、子目录、中文空格文件名三种场景各追加并同步 60 批。逐批核验文件路径、文档 ID、标题、分类、单文档数量以及全部正文；修复前在第一批出现路径变化，修复后通过。

真实运行链路：独立 Go 服务 → 发布包内 codex-acp → Codex 原生工具和真实 MCP load_skill → 本机脚本化模型端点。所有配置、文稿、审批记录和 Codex home 均在独立临时工作区；仅逐次批准 fixture 的文件操作，不改变用户权限策略、不使用真实 API 密钥、不调用付费模型。

| 场景 | 模型请求 | 已完成工具 | 文稿批次 | 原生压缩 | 终态 |
| --- | ---: | ---: | ---: | ---: | --- |
| success | 57 | 56 | 54 | 0 | completed |
| compaction | 60 | 56 | 54 | 3 | completed |
| 429 | 15 | 14 | 12 | 0 | failed |
| cancel | 15 | 14 | 12 | 0 | cancelled |

正文核验仅忽略应用合法增加的 frontmatter ID/版本及 section-id 注释；每批正文和顺序仍精确比对。每场景验证只有一份文稿，并检查终态后无新模型请求和文稿变动。成功标记仅检查助手消息，不匹配用户提示文本。429 场景共返回 1 次模拟 429，未出现虚假的最终成功消息。

四份通过的报告位于：
- D:/openai/MediaGo-Builds/verification/jw-acp-longtask-validation/success-AoZ5X8/report.json
- D:/openai/MediaGo-Builds/verification/jw-acp-longtask-validation/compaction-XYbSUG/report.json
- D:/openai/MediaGo-Builds/verification/jw-acp-longtask-validation/429-PXtF9u/report.json
- D:/openai/MediaGo-Builds/verification/jw-acp-longtask-validation/cancel-2k94Hy/report.json

前面的失败/取消测试报告也保留在相同根目录，未掩盖或删除。

## 复跑

在 services/server 下：

```powershell
go test ./internal/service/acp ./internal/service/document -count=1
go build ./...
go vet ./...
go build -tags workspace_dist -o D:/openai/MediaGo-Builds/verification/jw-acp-longtask-validation/mediago-server.exe ./cmd/mediago-server
```

在仓库根目录下：

```powershell
node scripts/verify-acp-long-task.mjs --server D:/openai/MediaGo-Builds/verification/jw-acp-longtask-validation/mediago-server.exe --agent-dir D:/openai/MediaGo-Builds/JW-Drama-Providers-20260905/win-unpacked/resources/agents --scenario success
```

分别用 compaction、429、cancel 替换 success。旧 verify-provider-tools.mjs 的无限 loop/compaction 强停验证入口已明确停用，其他 API 验证入口保留。

独立验证二进制 SHA-256：
BF5247241DE77B7DED80BAA68301996B73E32704AC45A95D6D6C019412DA5135

## 当时的验收限制（历史快照）

下列失败记录保留原始验收背景；2026-09-05 后续已修复普通测试失败和格式问题，当前状态以 [仓库收尾清单](jw-repository-closeout.md) 为准。race 与 Go lint 仍未通过。

- ACP 与 document 两个完整测试包通过；整个服务端 go build ./...、go vet ./... 通过。
- 全服务端 `go test ./... -count=1` 未通过：多个其他测试包在 Windows 清理时仍持有 SQLite 文件锁；此外观察到 `TestBillingPriceOverlayCoversAvailableRoutes` 报 `videoapi.compatible, speechapi.compatible` 缺少价格覆盖。全量输出还包含其他跨模块失败，未将其笼统认定为全部由环境引起，也未在本次任务中改动这些模块。ACP 与 document 在全量运行中再次通过，不能用定向通过替代全量质量门槛。
- 当前环境 CGO_ENABLED=0 且 PATH 未提供 C 编译器，go test -race 明确报 requires cgo；race 尚未通过。task 与 golangci-lint 未在 PATH 提供，未运行完整 task check。仓库其他原有文件还存在 gofmt 差异，没有为本任务批量重排。
- 本测试证明真实 ACP 协议、权限、文件写入、MCP 加载、原生压缩和失败/取消处理可继续工作，不证明真实模型在多次压缩后一定保持创作质量，也不证明真实 Provider 永远不会返回 429。没有实现自动检查点换会话或供应商额度扩容。
- 已和任务“接入统一多媒体模型”同步修改范围与通过证据。API 任务结束后需共同重建、联调、再部署；当前独立二进制是验证快照，未覆盖运行中的 JW Drama。
