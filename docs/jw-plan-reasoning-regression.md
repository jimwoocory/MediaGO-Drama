# JW Drama 计划绑定与第三方推理选择回归

日期：2026-09-05。此前完成了跨轮绑定、推理选择和同轮执行同步源码修复。用户随后授权直接更新原版本；22:27 已完成桌面重构建及原位程序更新，原设置、测试记录和剧本数据保留。

## 已交付到原入口

- 原启动路径：`D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905/desktop/win-unpacked/JW Drama.exe`，实际 app.asar 版本已核对为 `0.1.0-beta.plan-sync.20260905`。
- 更新前确认程序已关闭，完整备份并核对 16,972 个文件。备份路径：`D:/openai/MediaGo-Builds/archive-20260905/before-plan-sync-inplace-01`。
- 仅更新 7 个程序文件（含 2 个本地 Electron 运行时附带文件）；更新后核对全部 101 个程序文件，原有 16,873 个数据文件、271,852,617 字节逐文件 SHA-256 完全一致。
- 桌面包中的前端与嵌入前端来自同一次 Electron 构建；三份 Go 程序来自当前源码。原桌面外壳与代理工具等 36 个保留项校验一致。ZIP 清单与程序清单一致且不含用户数据。
- 包内实际后端与代理的隔离测试通过：计划停更后继续写入七场测试文档、验证标题并结束，保存后的计划和工具进度正确，high 推理参数到达本地模拟上游。报告：`D:/openai/MediaGo-Builds/verification/jw-provider-validation/workspace-0tzytu/report.json`。
- 安装及数据校验证明：`D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905/latest-update.json`。原 ZIP 路径同步更新，SHA-256 为 `253efda63d71d617e7c701232de6452624b0a249f7b66531d8b4fa9ca1ad5407`。
- 保留基版 `JW-Drama-Providers-20260905` 未覆盖，未调用真实付费模型，未自动启动用户工作区。本次是用户授权的本地候选版更新；既有 Go lint/race 工具链门禁仍按收尾记录保留，未宣称正式发布验收全部完成。

以下未打包说明属于源码验证阶段的历史记录；当前交付状态以上述原位更新为准。

## 同轮执行与计划回报

实际记录显示：原生计划最后回报停在 9 步中的第 5 步，随后工具仍继续写入和校验，最终返回 end_turn。此前仅修复跨轮绑定、隐藏结束卡片不能解决此现象。

- 原生计划每次回报时，前后端都保存该轮工具状态检查点。后续工具开始、成功、失败会直接更新卡片上的执行信息，不需要再收到计划事件，也不依赖超时猜测。新的结构化计划回报会推进检查点。
- 卡片明确区分“计划已确认几步”和“计划回报后完成了几项操作”。工具成功不会自动折算为任意剧本步骤完成；原生 shell 命令并没有可靠的计划步骤关联。
- 绑定根运行结束、失败、取消或中断后，保留当前轮的静态计划与结束说明。未回报的步骤显示未确认；开始新轮后不会带入这张卡片。历史时间线和卡片共用终态步骤转换逻辑。
- 服务端计划投影同样保护终态、按轮次和条目身份更新，避免重新打开会话后恢复成运行中。工具检查点随消息保存和恢复。
- 固定运行指令补充原生计划维护约束：逐步核对工具结果后提交计划更新，最终答复前提交最后一次回报，不重做成功写入。该约束走现有原生注入或兼容内联路径，指令指纹随之更新；没有新增模型请求、收尾重试或付费运行。

边界：上述指令不能保证任意模型每次都遵守。模型漏报时，界面仍能同步真实工具执行和运行终态，但不会声称已自动核实每个剧本步骤，也不会补造旧记录中缺失的逐步完成证据。

本次验证日志使用仓库根目录 `.agent-logs/plan-reasoning/same-run-*` 前缀，下面原有计数保留为上一轮记录。

本次最终验证：前端全量 1455 项通过、0 失败、0 跳过；服务端 1575 条测试通过记录（含子测试）、0 失败、17 条既有环境条件跳过。前端 lint、format、TypeScript/Vite build，服务端 vet/build 及 diff-check 全部通过。独立验证程序为 `bin/mediago-plan-sync-verification.exe`；它不是桌面交付包。原有 Go lint/race 工具链门禁仍未完成，不能据此宣称发布门禁全部通过。

真实打包 ACP + 回环模拟接口验证报告：`D:/openai/MediaGo-Builds/verification/jw-provider-validation/workspace-xATqkw/report.json`。模拟上游发送一次 9 步计划（4 完成、1 进行中、4 待执行），随后实际创建七场标题文档并校验，最终正常结束。会话重新读取后仍有原计划检查点及两次成功工具结果，完成数没有被伪造；四次上游请求均携带 high 推理参数，没有增加模型请求。前端真实 store 回归覆盖这些事件如何驱动卡片以及结束后的静态状态。

复现：设置 `JW_SMOKE_SERVER` 为上述独立程序，`JW_SMOKE_PLAN_SYNC=1`、`JW_SMOKE_REASONING=high`，运行 `node scripts/verify-provider-tools.mjs`。脚本创建全新测试项目，仅对自己生成的两条固定测试命令选择单次权限确认，不修改会话级策略。首次权限等待超时报告 `workspace-v3E2DZ` 和首次误按总行数校验的报告 `workspace-3mwV7b` 保留；文档监听器会自动加入 frontmatter 和章节标识，最终验证据此准确检查七个场次标题。

## 修复范围

- 悬浮计划读取当前绑定的根运行及其状态，不使用历史会话回退。按 `turnId` 选择计划；无身份的旧记录只能从最近一条用户消息之后选择。新一轮没有计划时不显示旧计划，空计划更新清空悬浮计划。
- 计划更新按轮次和条目身份匹配。同轮更新保留原条目身份，不覆盖其他轮次；晚到的计划更新不能把完成、失败、取消等终态改回运行中。
- 历史计划只保留明确确认的完成勾选。终态下残留的 pending/in_progress 投影为“未确认”，不再显示动画；原始消息中的计划状态不被改写。
- 用户在本任务中追加了“第三方接口全部模型都不能选择推理等级”的修复要求。第三方模型现有独立的“接口默认、无、最低、低、中、高、超高”选择，使用单独的值前缀和来源，避免沿用账户通道的保存值。
- 明确选择的等级通过隔离的运行配置、ACP 会话选项和兼容转发层传递。Chat Completions 接收 `reasoning_effort`，Responses 保留 `reasoning.effort`。默认模式不额外发送推理参数；默认与显式等级使用不同模型目录，避免覆盖运行中的配置。
- 打包 ACP 的 `supports_reasoning_summaries` 旧字段同时控制 effort 是否发出。仅在明确选择等级时启用此开关，摘要配置仍为 none。模拟接口验证覆盖了这个实际行为，避免只显示下拉框却没有传参。

没有修改供应商选择、写作 Skill、上下文限制或重试机制，没有修改用户文稿或重跑付费模型。

## 验证结果

- 前端全量：1451 项通过，0 失败、0 跳过。包含跨轮、同轮更新、恢复、重放、无计划新轮、空根会话、完成/失败/取消/暂停/中断、晚到事件和历史“未确认”显示。
- 前端 `pnpm lint`：0 警告、0 错误；`pnpm format`：671 个匹配文件通过；`pnpm build`：TypeScript 与 Vite 生产构建通过。构建保留已有大 bundle 提示。
- 服务端 `go test -json ./...`：1566 条测试通过记录（含子测试）、0 失败、17 条既有环境条件跳过。未添加 skip 或删除失败测试。
- 服务端 `go vet ./...`、`go build ./...` 通过；本轮 Go 文件 gofmt 检查与 `git diff --check` 通过。
- 真实打包 ACP + 本地模拟上游：自定义接口 high、统一接口 high、接口默认、显式 none、Responses low 均通过。Chat Completions 案例含连续五次工具往返请求，逐次断言推理参数。所有模拟上游均为回环地址，测试工作区与用户 data 隔离。

源码库 `.agent-logs/plan-reasoning/` 保留前端首次失败报告、最终 JSON 报告和服务端逐项 JSONL 报告。首次失败的旧“全部打勾”断言已改为验证真实完成数与未确认状态，随后全量复跑通过。

模拟接口报告位于 `D:/openai/MediaGo-Builds/verification/jw-provider-validation/`：

| 案例 | 报告 |
| --- | --- |
| 自定义接口 high | `workspace-z3Ht0X/report.json` |
| 统一接口 high | `workspace-bbjN6i/report.json` |
| 接口默认，不发送 effort | `workspace-KtsnFv/report.json` |
| 显式 none | `workspace-eb2Ui3/report.json` |
| Responses low | `workspace-qLmHuB/report.json` |

可复现方式：先在 `services/server` 构建独立测试程序，再在仓库根运行 `scripts/verify-provider-tools.mjs`。环境变量 `JW_SMOKE_SERVER` 指向测试程序，`JW_SMOKE_REASONING` 设为 high/default/none/low；`JW_SMOKE_UNIFIED=1` 测试统一接口，`JW_SMOKE_RESPONSES=1` 测试 Responses。脚本自动创建隔离工作区和回环上游。

推理等级是发给接口的请求参数，不能保证任意模型都支持所有等级。各接口有自己的支持与映射规则，例如 [DeepSeek 思考模式](https://api-docs.deepseek.com/guides/thinking_mode/) 与 [OpenRouter 推理参数](https://openrouter.ai/docs/guides/best-practices/reasoning-tokens)。本轮验证了参数传递，不声称真实供应商付费调用或所有模型能力已验收。

## 交付边界

当前源码包含本轮修复。`bin/mediago-plan-reasoning-verification.exe` 是隔离测试服务端，`apps/workspace/dist` 是前端构建产物，均不是新桌面交付包。

正在使用的 `D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905/desktop/win-unpacked` 和指定保留基版 `D:/openai/MediaGo-Builds/JW-Drama-Providers-20260905/win-unpacked` 均未覆盖，所有用户 data 原位保留。当前运行版仍不含本轮计划/推理修复，也不含上一轮最后的外部计费修复。

仓库原有未提交修改全部保留，未提交、推送、强制结束程序或原位升级。Go lint 工具版本不兼容及 race 缺少 C 工具链仍沿用原收尾记录中的独立未完成门禁，本任务没有扩展处理。
