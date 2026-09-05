# JW Drama 仓库收尾清单

更新：2026-09-05。用户授权直接更新原版本后，22:27 已将计划同步及第三方推理修复构建覆盖到原候选程序入口。更新时应用已关闭，全部 16,873 个原数据文件 SHA-256 校验不变；旧程序和数据已完整备份。详见 [最新交付记录](jw-plan-reasoning-regression.md)。尚未满足全部正式发布门禁，未提交 Git。以下较早的未打包说明作为历史记录保留。

追加：计划与运行绑定、历史步骤“未确认”显示，以及用户追加的第三方推理等级选择已完成源码修复。最新前端全量 1451 项通过，服务端 1566 条通过记录（含子测试）、0 失败、17 条既有跳过；详见 [本轮回归记录](jw-plan-reasoning-regression.md)。当前桌面运行版尚未包含这些修复，原 data 和保留基版均未改动。

## 唯一目录约定

| 路径 | 用途 |
| --- | --- |
| `D:/openai/MediaGo-Drama` | 唯一工作源码库，包含未提交的整合与收尾修复 |
| `D:/openai/MediaGo-Builds/JW-Drama-Providers-20260905/win-unpacked` | 用户指定基版；程序和原有 data 原位保留 |
| `D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905/desktop` | 基版加统一多媒体、ACP 长任务与文件边界修复的候选包 |
| `D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905/_build` | 对应该候选包的构建中间文件，不是第二份交付版本 |
| `D:/openai/MediaGo-Builds/verification` | 验证记录和测试工作区 |
| `D:/openai/MediaGo-Builds/archive-20260905` | 旧版、旧 release 和一次性脚本，可恢复归档 |

本轮没有删除账号、项目或文稿。旧版和三个根目录补丁脚本采用归档移动；脚本逐个核验 SHA-256。完整移动映射见 MediaGo-Builds/README.md。

## 本轮修复与验证

- 七个 Go 模块：core、instructions、jianyingdraft、mcp、tools、vendor、server 的 `go build ./...`、`go vet ./...`、`go test ./...` 均退出 0。
- 服务端 JSON 报告：1547 条测试通过记录（含子测试）、0 失败、17 条现有环境条件跳过。跳过不计作通过；未新增 skip 或删除失败测试。
- 前端：1431 项测试通过，0 失败、0 跳过；oxlint、TypeScript/Vite 生产构建、668 个匹配文件的格式检查通过。
- Go 全仓 `gofmt -l packages services` 无输出；`git diff --check` 通过。
- Windows 测试显式释放自己创建的 SQLite 连接、后台写入器和 CLI 子进程；不改生产数据库实现来回避测试失败。
- CLI 登录及 FFmpeg 夹具改用跨平台测试子进程；提示词快照仅兼容 Git 检出的 CRLF，未更新写作 Skill 或替换 golden 正文。
- 修正默认模型/制作模式断言，补全实际 HTTP 路由的 Swagger 注释；更新面板测试等待能力数据加载完成，保留所有用户可见断言。
- 通用第三方音频、视频路由使用外部计费，不再给未知供应商虚构固定金额；覆盖检查和“不生成虚假费用”回归测试均保留。
- 547 个 Go、600 个前端文件的格式差异逐个核实均仅为换行。未重排组件或改变 React Hook、请求、事件和渲染逻辑；`.gitattributes` 固定 Go 与前端已有 LF 约定。
- 刷新了 1156 个内容哈希与 Git 索引完全相同文件的元数据；刷新前后暂存内容均为空。没有用 reset、clean、批量提交或删除未提交修改制造“干净”状态。

## 证据

统一根目录：`D:/openai/MediaGo-Builds/verification/jw-repository-audit-20260905`。

- `packages-*-build-final.log`、`packages-*-vet-final.log`、`packages-*-tests-final.log` 与 `services-server-*-final.log`：七模块结果。
- `server-tests-final.jsonl`：服务端逐项结果；含跳过原因。
- `workspace-tests-cleanup-final.json`：前端最终结果；首次复跑的异步竞态已修复，最终完整复跑通过。
- `workspace-lint-cleanup-final.log`、`workspace-build-cleanup-final.log`、`workspace-format-cleanup-final.log`、`diff-check-final.log`：前端与补丁门禁。
- `server-golangci-final.log`、`server-race-cleanup-final.log`：未通过的工具链门禁，不计作成功。

## 仅剩的发布门禁与交付边界

1. **Go lint 未通过**：仓库固定 golangci-lint v1.64.8 无法读取本机 Go 1.27 的 export data version 4。需要确定兼容的 Go/lint 版本组合后再跑；不通过关闭检查或改业务代码掩盖工具错误。
2. **race 未通过**：实际命令返回 `-race requires cgo`，本机 `CGO_ENABLED=0` 且未找到 C 编译器。需补齐兼容 C 工具链再执行，不把普通测试等同于 race 验证。
3. **未重打包本轮最后修复**：当前运行候选包仍为此前已经校验的构建，最后的外部计费修复尚只在源码中。正在运行的候选包和其中新增的 data 均不覆盖。重新交付前应在应用正常退出后保留数据、归档旧程序、同源构建并重跑对应包的验证。

当前 ZIP SHA-256：`ba10ca8e356bc2d2e5eb7c99b1852c60ca7c866ee06081906ea9e33743a7c5e9`。99 个程序文件、36 个保留基版文件校验通过，候选程序内容未因目录整理发生改变。

Git 工作区仍有真实、未提交的整合和测试修复；这不等于已完成版本封板。未推送、未部署原位升级，也不声称真实供应商的额度、429 或付费生成已经全部验收。

## 2026-09-05 同轮计划同步补充

同轮计划停更后，卡片现在继续接收实际工具进度，并保留明确的运行结束状态与未确认步骤；固定运行指令补充逐步回报原生计划的要求。前端 1455 项、服务端 1575 条测试通过记录及真实 ACP 隔离七场文档流程验证通过。完整证据和模型漏报边界见 [计划与推理回归记录](jw-plan-reasoning-regression.md)。独立验证程序为 `bin/mediago-plan-sync-verification.exe`，当前运行候选桌面包尚未替换。
