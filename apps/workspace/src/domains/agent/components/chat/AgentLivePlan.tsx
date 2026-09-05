import { ChevronDown, ChevronUp, LoaderCircle } from "lucide-react";
import type React from "react";
import { useId, useMemo, useState } from "react";
import type {
	AgentACPPlanEntry,
	AgentConversationStatus,
	AgentMessage,
} from "@/domains/agent/stores";
import { findLastIndex, isTerminalConversationStatus } from "@/domains/agent/stores/conversation";
import { planExecutionProgress, settlePlanEntries } from "@/domains/agent/lib/plan-progress";
import { cn } from "@/shared/lib/utils";
import { PlanBlock } from "../timeline/PlanBlock";

export interface ActiveAgentPlan {
	entries: AgentACPPlanEntry[];
	currentStep: number;
	toolStates?: Record<string, string>;
}

interface AgentLivePlanProps {
	isRunning: boolean;
	status?: AgentConversationStatus;
	runId: string | null;
	messages: AgentMessage[];
	className?: string;
}

/** Selects the bound run's latest plan; legacy records stay within the latest user turn. */
export const activePlanFromMessages = (
	messages: readonly AgentMessage[],
	runId: string | null,
): ActiveAgentPlan | null => {
	if (!runId) return null;
	const lastUserIndex = findLastIndex(messages, (message) => message.role === "user");
	const userTurnId = messages[lastUserIndex]?.turnId;
	for (let index = messages.length - 1; index >= 0; index -= 1) {
		const message = messages[index];
		const entries = message?.kind === "plan" ? message.metadata?.planEntries : undefined;
		if (!entries) continue;
		if (message.turnId) {
			if (message.turnId !== runId) continue;
		} else if (
			lastUserIndex < 0 ||
			index <= lastUserIndex ||
			(userTurnId && userTurnId !== runId)
		) {
			continue;
		}
		if (!entries.length) return null;

		const currentIndex = planCurrentEntryIndex(entries);
		return { entries, currentStep: currentIndex + 1, toolStates: message.metadata?.planToolStates };
	}
	return null;
};

/** Shows the active plan above the composer while an agent run is live. */
export const AgentLivePlan: React.FC<AgentLivePlanProps> = ({
	isRunning,
	status,
	runId,
	messages,
	className,
}) => {
	const plan = useMemo(() => activePlanFromMessages(messages, runId), [messages, runId]);
	const [expanded, setExpanded] = useState(true);
	const reactId = useId();
	const regionId = `agent-live-plan-${reactId}`;

	const terminal = status !== undefined && isTerminalConversationStatus(status);
	if ((!isRunning && !terminal) || !plan || !runId) return null;
	const entries = terminal ? settlePlanEntries(plan.entries) : plan.entries;
	const execution = planExecutionProgress(messages, runId, plan.toolStates);
	const hasLaterActivity = execution.completed + execution.failed + execution.running > 0;
	const unconfirmed = entries.filter((entry) => entry.status === "unconfirmed").length;
	const completed = entries.filter((entry) => entry.status === "completed").length;
	const terminalLabel =
		status === "completed"
			? "执行已结束"
			: status === "failed"
				? "执行失败"
				: status === "cancelled"
					? "执行已取消"
					: status === "paused"
						? "执行已暂停"
						: "执行已中断";

	const progressLabel = terminal
		? `${terminalLabel}，计划已确认 ${completed} / ${entries.length} 步`
		: hasLaterActivity
			? `执行继续，计划已确认 ${completed} / ${entries.length} 步`
			: `第 ${plan.currentStep} / ${plan.entries.length} 步`;
	const buttonLabel = `${expanded ? "收起" : "展开"}执行计划，${progressLabel}`;
	const CurrentIcon =
		!terminal && !hasLaterActivity && plan.entries[plan.currentStep - 1]?.status === "in_progress"
			? LoaderCircle
			: null;

	return (
		<aside
			className={cn("agent-live-plan px-4", className)}
			data-testid="agent-live-plan"
			aria-live="polite"
		>
			<div className="agent-live-plan-stack mx-auto flex w-full max-w-xl flex-col items-center">
				{expanded ? (
					<div
						id={regionId}
						role="region"
						aria-label="执行计划"
						className="agent-live-plan-card w-full"
					>
						{terminal ? (
							<p className="mb-2 text-caption text-muted-foreground" role="status">
								{terminalLabel}
								{unconfirmed ? `；${unconfirmed} 个步骤未收到完成回报。` : "。"}
							</p>
						) : hasLaterActivity ? (
							<p className="mb-2 text-caption text-muted-foreground" role="status">
								{execution.hasCheckpoint ? "上次计划回报后" : "本轮执行"}：已完成{" "}
								{execution.completed} 项操作
								{execution.running ? `，${execution.running} 项进行中` : ""}
								{execution.failed ? `，${execution.failed} 项失败` : ""}。计划步骤等待模型回报。
							</p>
						) : null}
						<PlanBlock
							content=""
							entries={entries}
							animateProgress={!hasLaterActivity && !terminal}
						/>
					</div>
				) : null}
				<button
					type="button"
					className="agent-live-plan-toggle"
					aria-controls={regionId}
					aria-expanded={expanded}
					aria-label={buttonLabel}
					onClick={() => setExpanded((value) => !value)}
				>
					{CurrentIcon ? (
						<CurrentIcon className="size-3.5 motion-safe:animate-spin" aria-hidden="true" />
					) : (
						<span className="agent-live-plan-dot" aria-hidden="true" />
					)}
					<span>{progressLabel}</span>
					{expanded ? (
						<ChevronDown className="size-3.5" aria-hidden="true" />
					) : (
						<ChevronUp className="size-3.5" aria-hidden="true" />
					)}
				</button>
			</div>
		</aside>
	);
};

const planCurrentEntryIndex = (entries: readonly AgentACPPlanEntry[]) => {
	const inProgressIndex = entries.findIndex((entry) => entry.status === "in_progress");
	if (inProgressIndex >= 0) return inProgressIndex;

	const failedIndex = entries.findIndex((entry) => entry.status === "failed");
	if (failedIndex >= 0) return failedIndex;

	const pendingIndex = entries.findIndex((entry) => entry.status === "pending");
	if (pendingIndex >= 0) return pendingIndex;

	return Math.max(0, entries.length - 1);
};
