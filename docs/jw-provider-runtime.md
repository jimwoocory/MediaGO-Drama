# JW Drama Provider 与模型分离

JW Drama 保留 Codex ACP 内核。供应商身份决定认证、API 地址和协议；模型 ID 是不拆分、不改写大小写的上游标识。工作台 Agent/文本模型选项进入 Codex ACP，继续使用相同的业务 MCP、终端/文件工具、技能和权限选择。图片、音频、视频生成路由见 [统一多媒体说明](jw-unified-media.md)，不声称所有媒体请求都经 ACP。

运行时模型值使用 `chatgpt:<model>`、`api-<profileId>:<model>` 或兼容旧凭据的 `gateway-<providerId>:<model>`。API 响应同时给出 `providerId`、`providerLabel`、`modelId`，UI 不依赖模型名称猜测供应商。旧 OpenCode 模型值的解析仅保留为历史兼容。

ChatGPT OAuth 使用桌面应用原有的独立账号目录。第三方 API 使用按配置生成的独立 Codex home；本地 relay 路径绑定供应商，切换默认供应商不会改变其他正在运行会话的上游。真实 API Key 留在设置存储中，Codex home 只包含本地 bridge token。技能引用共享安装位置，并保留用户的启停规则。

第三方设置页的“默认供应商”只决定新对话初始选择。已配置的各供应商同时出现在模型菜单中。`/models` 失败时仍保留该供应商手工填写的模型 ID；模型列表存在不等于渠道推理或工具调用可用。

Codex 0.144 的业务 MCP 以 Responses `namespace` 工具组提供。Chat Completions 适配器展开工具组，保留各自身份，转回 Responses 时恢复 `namespace`、函数名和调用 ID；续接工具结果与强制工具选择也使用同一映射。超过 Chat Completions 名称长度的工具使用稳定哈希别名。

## 本轮验证

2026-09-05 修复补充：统一入口只列出 `gateway-aihubmix`，旧 `gateway-openai-compatible` 会话仍可使用同一凭据；不再添加未配置的默认模型。第三方使用独立的启动期 `model_catalog_json`，采用文本/工具兼容配置，32K 是应用保守上下文预算而非上游最大能力声明。未验证支持的推理档位不在第三方界面显示，后端也不提交遗留 OAuth 推理选择。上游 4xx/5xx 保持原响应，额外记录状态和安全错误代码，不自动更换供应商或模型。配置弹窗保留结构化接口错误的具体原因。

模型目录按 [OpenAI 官方配置参考](https://learn.chatgpt.com/docs/config-file/config-reference) 的启动期覆盖机制接入，并通过包内 Codex ACP 实测校验格式；没有通过过滤警告来掩盖目录缺失。

### 2026-09-05 上下文与运行终止修复

仍统一使用 Codex ACP，不引入第二套 Hermes 摘要器。第三方启动时读取当前网关 `/models` 中精确模型 ID 的 `context_length` / `context_window`，两者并存取较小有效值；未声明时才回退到 32K 应用预算，不根据 DeepSeek 等名称推测渠道上限。模型目录、启动配置和会话配置使用同一窗口，配置变化隔离到新的运行目录。压缩阈值取窗口的 80%，同时在有效 95% 窗口内预留输出空间。

工具安全保护现在取消正在执行的 ACP prompt；SDK 发送取消通知，500ms 内不响应则关闭该运行的进程。不再把安全保护错误转换成 `end_turn`，也不再追加模型收尾请求。原生 `exceeded retry limit, last status: ...` 消息即使伴随 `end_turn` 也按失败处理，429 显示中文提示；不把普通讨论 HTTP 429 的文字当作运行失败。

后续长任务修复已撤回第二次压缩强制停止和工具调用计数限制。原生压缩、同文件分批写入和重复加载 Skill 不再仅因次数而失败；用户取消、权限边界和真实上游失败仍保留。没有实现无限上下文或无损记忆，也不会绕过供应商限流。详见 [长任务验收](jw-acp-long-task-regression.md)。

真实包内 Codex ACP + 本地模拟上游验证保留 429 失败传播、上下文窗口声明、技能、终端和业务 MCP。旧 `loop` / `compaction` 强停测试入口已停用，替换为 `verify-acp-long-task.mjs` 的正常长写入、跨压缩继续、429 和用户取消四种场景。测试不使用用户 API Key，也不重跑用户剧本。

- 供应商独立路由、OAuth/API 环境分离、不透明模型 ID、技能禁用规则和 namespace 双向转换回归通过。
- 工作台模型选择器与第三方设置面板共 40 项测试通过；前端 lint、受改文件格式、生产构建通过。
- 受影响 Go 静态分析与编译通过；独立 Speech API / Video API 测试通过。
- `node scripts/verify-provider-tools.mjs` 使用本地模拟上游和真实 Codex ACP：执行终端命令、读取测试技能文件、调用文档 MCP `get_project_config`，并调用生成 MCP 验证未确认请求在提交前被拒绝。
- 此验证不调用付费上游，不宣称本轮已验证 Tokease / AIHubMix / OpenRouter 的真实远端渠道、OAuth 推理或付费音视频生成。

2026-09-05 收尾已修复上述普通测试失败；七个 Go 模块的构建、vet 和普通测试通过。格式换行已统一。race 与旧 Go lint 的工具链限制仍未通过，当前结果以 [仓库收尾清单](jw-repository-closeout.md) 为准。

要验证打包后的真实二进制，可设置 `JW_SMOKE_SERVER` 和 `JW_SMOKE_AGENT_DIR` 指向包内 server 与 agents 目录后运行同一脚本。报告位于 `D:/openai/MediaGo-Builds/verification/jw-provider-validation/workspace-*/report.json`。
