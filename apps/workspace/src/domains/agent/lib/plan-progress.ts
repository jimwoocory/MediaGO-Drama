import type { AgentACPPlanEntry, AgentMessage } from "../stores/types";
import { findLastIndex } from "../stores/conversation";

/** A plan report checkpoints tool states, so later execution stays visible without guessing steps. */
export const capturePlanToolStates = (messages: readonly AgentMessage[], turnId: string) => {
	const lastUserIndex = findLastIndex(messages, (message) => message.role === "user");
	const states: Record<string, string> = {};
	for (const [index, message] of messages.entries()) {
		if (message.kind !== "tool") continue;
		if (
			message.turnId
				? message.turnId !== turnId
				: index <= lastUserIndex ||
					(messages[lastUserIndex]?.turnId && messages[lastUserIndex].turnId !== turnId)
		)
			continue;
		states[message.metadata?.toolCallId ?? message.itemId ?? message.id] =
			message.metadata?.status ?? "pending";
	}
	return states;
};

export const planExecutionProgress = (
	messages: readonly AgentMessage[],
	turnId: string,
	checkpoint?: Record<string, string>,
) => {
	const states = capturePlanToolStates(messages, turnId);
	const changed = Object.entries(states).filter(
		([id, status]) => !checkpoint || checkpoint[id] !== status,
	);
	return {
		completed: changed.filter(([, status]) => status === "completed").length,
		failed: changed.filter(([, status]) => status === "failed").length,
		running: changed.filter(([, status]) => status === "pending" || status === "in_progress")
			.length,
		hasCheckpoint: checkpoint !== undefined,
	};
};

export const settlePlanEntries = (entries: AgentACPPlanEntry[]) =>
	entries.map((entry) =>
		entry.status === "pending" || entry.status === "in_progress"
			? { ...entry, status: "unconfirmed" }
			: entry,
	);
