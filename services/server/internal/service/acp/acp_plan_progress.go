package acp

// The native plan is a separate stream from tool execution. Keep its reporting
// contract in fixed instructions so both native injection and inline delivery
// receive it, without adding another prompt or retrying completed work.
const acpPlanProgressInstructions = `## 执行计划进度同步
如果本轮使用了原生计划工具（例如 update_plan），必须持续维护同一份计划：
- 每个计划步骤实际完成并核实工具结果后，先通过原生计划工具将该步骤标记 completed，再开始下一步；正在处理的步骤标记 in_progress。
- 文件已写入、工具执行成功或正文中的进度描述不会自动更新计划。不要只在文字里说完成；需要提交结构化计划更新。
- 最终答复前核对全部步骤并提交最后一次计划更新。只确认有结果依据的步骤；失败、取消或尚未验证的步骤不得标记 completed，说明剩余工作。
- 不要为了补计划重做已经成功的文件写入或生成操作，也不要为了计划另外创建任务。无需多步骤计划的简单请求保持原有处理方式。`
