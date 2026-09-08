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

/**
 * Native plans are often broader than one tool call. For document-producing
 * plans, however, a completed write with the exact document title is concrete
 * evidence that the matching step is done, even if the model delays its next
 * update_plan notification.
 */
export const reconcilePlanEntriesWithCompletedDocumentWrites = (
	entries: readonly AgentACPPlanEntry[],
	messages: readonly AgentMessage[],
	turnId: string,
) => {
	const writtenTitles = new Set(
		messages
			.filter((message) => isCompletedToolInTurn(message, turnId))
			.flatMap(documentTitlesFromCompletedWrite)
			.map(normalizeDocumentTitle)
			.filter(Boolean),
	);
	if (!writtenTitles.size) return [...entries];

	return entries.map((entry) => {
		if (entry.status === "completed") return entry;
		const title = normalizeDocumentTitle(entry.content);
		return title && writtenTitles.has(title) ? { ...entry, status: "completed" } : entry;
	});
};

const isCompletedToolInTurn = (message: AgentMessage, turnId: string) =>
	message.kind === "tool" &&
	message.metadata?.status === "completed" &&
	(message.turnId ? message.turnId === turnId : true);

const documentTitlesFromCompletedWrite = (message: AgentMessage) => {
	const text = toolMessageText(message);
	if (!isDocumentWrite(text, message)) return [];
	return [
		...captureAll(text, /^\s*title:\s*(.+?)\s*$/gim),
		...captureAll(text, /(?:-FilePath|-Path)\s+['"]([^'"]+)\.(?:md|markdown)['"]/gi).map(
			(path) => path.split(/[\\/]/).at(-1) ?? path,
		),
	];
};

const isDocumentWrite = (text: string, message: AgentMessage) => {
	const toolName = [message.metadata?.canonicalToolName, message.metadata?.toolName, message.title]
		.filter((value): value is string => typeof value === "string")
		.join(" ")
		.toLowerCase();
	return (
		/(?:out-file|set-content|add-content|writealltext|-filepath\s+['"][^'"]+\.(?:md|markdown)|create_document|update_document|write_file|write_text_file)/i.test(
			text,
		) || /(?:write|edit|document)/.test(toolName)
	);
};

const toolMessageText = (message: AgentMessage) =>
	[
		message.content,
		message.title,
		message.metadata?.inputArgs,
		stringify(message.metadata?.inputJson),
		stringify(message.metadata?.outputJson),
		...(message.metadata?.outputBlocks ?? []).map((block) =>
			[block.text, block.path, block.oldText, block.newText].filter(Boolean).join("\n"),
		),
	]
		.filter(Boolean)
		.join("\n");

const stringify = (value: unknown) => {
	if (typeof value === "string") return value;
	if (value === undefined || value === null) return "";
	try {
		return JSON.stringify(value);
	} catch {
		return "";
	}
};

const captureAll = (value: string, pattern: RegExp) =>
	Array.from(value.matchAll(pattern), (match) => match[1]);

const normalizeDocumentTitle = (value: string) =>
	value
		.trim()
		.replace(/\.(?:md|markdown)$/i, "")
		.replace(/[“”"']/g, "")
		.replace(/\s+/g, " ")
		.toLocaleLowerCase("zh-CN");

export const settlePlanEntries = (entries: AgentACPPlanEntry[]) =>
	entries.map((entry) =>
		entry.status === "pending" || entry.status === "in_progress"
			? { ...entry, status: "unconfirmed" }
			: entry,
	);
