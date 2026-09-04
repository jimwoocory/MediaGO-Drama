# MediaGO-Drama

这是 [jimwoocory](https://github.com/jimwoocory) 的个人工作副本，不是 MediaGo 官方仓库。

上游开源项目是 [mediago-dev/mediago-drama](https://github.com/mediago-dev/mediago-drama)。本仓库从官方 `v0.1.8` 源码拉出一份干净基线，再把本地已经验证过的 **Agent 底座** 能力补回去，方便在 Windows 上继续改、继续跑。

如果你要官方产品、官方文档或官方发行版，请去上游，不要把这里当成官方主页。

## 这个副本做什么

本地跑一套「项目文档 + Agent + 生成工作区」：

- 小说 / 剧本 / 角色 / 场景 / 道具 / 分镜以本地文档保存
- Agent 在当前项目上下文里读文档、调 Skill、调 MCP、把结果写回项目
- 确认过的设定可以直接进入图片和视频生成，不必每次重拼 Prompt

本仓库当前基线：`65e0b8f`（`v0.1.8-11-g65e0b8f`），分支 `mediago-clean-baseline-20260903`。

## 相对上游补了什么

官方源码能编译，但本地工作副本额外恢复并核对了这些底座能力：

| 能力 | 说明 |
| --- | --- |
| Shared Skills | 剧本、角色、场景、道具、分镜等 Skill 可按任务装载 |
| MCP | Agent 通过 MCP 调文档工具和外部能力 |
| Workspace | 前端工作台能看到长任务、工具调用和文档改动 |
| Documents | 项目文档作为长期记忆和检查点，而不是一次性聊天记录 |
| 文件工具 | Agent 能读写项目内文件 |
| 兼容适配 | 设置页、第三方网关和 Codex 入口按本地干净默认值整理 |
| 防泄漏 | Agent 运行时隔离认证目录，避免把本机凭证带进错误上下文 |
| 循环保护 | ACP / Agent 循环有边界，避免会话上下文无限膨胀 |
| 可移植工作区 | 项目状态跟着本地目录走，不绑死某一台机器的临时路径 |
| 动态端口 | 桌面端走 sidecar 动态端口，不再写死固定 API 端口 |

这些是工作副本里实际要用的工程能力，不是官方宣传页上的产品口号。

## 仓库结构

```text
apps/workspace     前端工作台（Vite + Electron）
services/server    Go 服务端 / Agent / MCP / 文档 / 设置
packages/          共享 Go 与工具包
docs/              工程说明
scripts/           构建与辅助脚本
```

发布产物、安装包、`apps/workspace/release-media-*` 这类大体积构建结果 **不进 Git**。

## 本地运行

当前面向本地开发，不是给终端用户下载安装的发行通道。

| 工具 | 建议版本 |
| --- | --- |
| Node.js | 24 |
| pnpm | 11.9+ |
| Go | 1.25+ |
| go-task | 3.x |

```bash
pnpm install
```

开两个终端：

```bash
# 终端 1：本地服务端
pnpm dev:server
```

```bash
# 终端 2：Electron 桌面端
pnpm dev:desktop
```

改过服务端或 Agent MCP 代码后，先跑 `pnpm build:server` 再重启。

常用命令：

```bash
pnpm build:server     # 构建服务端与 Agent MCP
pnpm workspace:dev    # 只起前端工作台
pnpm dev:server       # 起本地服务端
pnpm dev:desktop      # 起 Electron 桌面端
pnpm check:go         # 检查 Go packages
```

Windows 上部分 Go 测试会碰到 SQLite 临时目录锁、`.exe` 后缀、以及 `HOME` / `USERPROFILE` 不一致。这是环境差异，不代表 Agent 底座没接上。

## 这个仓库不提供什么

- 不是 [mediago-dev/mediago-drama](https://github.com/mediago-dev/mediago-drama) 官方主页
- 不带 MediaGo 官方账户、商城、托管、支付、加密组件
- 不保证能连上官方在线服务
- 不把本仓库构建结果声明成官方 MediaGo 产品

## 许可

本仓库自有源码按 [Apache License 2.0](LICENSE) 提供。第三方依赖仍走各自原始许可证。

官方发行版里另外打包的专有组件和在线服务，不在本仓库授权范围内，边界见 [COMMERCIAL_FEATURES.md](COMMERCIAL_FEATURES.md)。

## 上游

- 官方源码：[mediago-dev/mediago-drama](https://github.com/mediago-dev/mediago-drama)
- 本仓库：[jimwoocory/MediaGO-Drama](https://github.com/jimwoocory/MediaGO-Drama)
