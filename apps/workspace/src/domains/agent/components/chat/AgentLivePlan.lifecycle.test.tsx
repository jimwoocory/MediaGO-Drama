import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import {
	selectAgentLiveConversation,
	useAgentStore,
	type AgentMessage,
} from "@/domains/agent/stores";
import { createConversation } from "@/domains/agent/stores/conversation";
import { buildAgentTurnViewModels } from "../timeline/model";
import { PlanBlock } from "../timeline/PlanBlock";
import { AgentLivePlan, activePlanFromMessages } from "./AgentLivePlan";

const entries = [
	{ content: "已确认", status: "completed" },
	{ content: "执行中", status: "in_progress" },
	{ content: "待确认", status: "pending" },
];
const plan = (turnId?: string): AgentMessage => ({
	id: `plan-${turnId ?? "legacy"}`,
	turnId,
	kind: "plan",
	role: "assistant",
	content: "",
	metadata: { planEntries: entries },
});
const user = (turnId?: string): AgentMessage => ({
	id: `user-${turnId}`,
	turnId,
	role: "user",
	content: "继续",
});
const LivePlan = () => {
	const conversation = useAgentStore(selectAgentLiveConversation);
	return (
		<AgentLivePlan
			isRunning={Boolean(conversation)}
			runId={conversation?.runId ?? null}
			messages={conversation?.messages ?? []}
		/>
	);
};
afterEach(() => {
	cleanup();
	useAgentStore.getState().resetSession();
});

describe("plan run ownership regressions", () => {
	it("never revives an old plan in a new pending or bound run without a plan", () => {
		const store = useAgentStore.getState();
		store.startRun("第一轮");
		store.bindRootRun("run-1");
		store.setPlan(entries, "run-1");
		store.finishRun("run-1");
		store.startRun("第二轮");
		const pending = selectAgentLiveConversation(useAgentStore.getState())!;
		expect(activePlanFromMessages(pending.messages, pending.runId)).toBeNull();
		store.bindRootRun("run-2");
		render(<LivePlan />);
		expect(screen.queryByTestId("agent-live-plan")).not.toBeInTheDocument();
	});
	it("selects the current run even when a different run's replayed plan is last", () => {
		expect(
			activePlanFromMessages([user("run-2"), plan("run-2"), plan("run-1")], "run-2")?.entries,
		).toEqual(entries);
		expect(activePlanFromMessages([plan("run-1")], "run-2")).toBeNull();
		expect(activePlanFromMessages([plan("run-1")], null)).toBeNull();
	});
	it("bounds identityless legacy plans by the latest user and honors an empty update", () => {
		expect(activePlanFromMessages([user(), plan(), user()], "run-2")).toBeNull();
		expect(activePlanFromMessages([user("run-2"), plan()], "run-2")?.entries).toEqual(entries);
		expect(activePlanFromMessages([user("run-1"), plan()], "run-2")).toBeNull();
		expect(activePlanFromMessages([plan()], "run-2")).toBeNull();
		expect(
			activePlanFromMessages(
				[plan("run-2"), { ...plan("run-2"), metadata: { planEntries: [] } }],
				"run-2",
			),
		).toBeNull();
	});
	it.each(["completed", "failed", "cancelled", "interrupted", "paused"] as const)(
		"hides a %s owner even if another conversation is running",
		(status) => {
			useAgentStore.getState().hydrateAgentChatState([], [], {
				rootRunId: "run-1",
				running: true,
				conversations: {
					"run-1": createConversation("run-1", {
						status,
						messages: [user("run-1"), plan("run-1")],
					}),
					"run-2": createConversation("run-2", { status: "running" }),
				},
			});
			render(<LivePlan />);
			expect(screen.queryByTestId("agent-live-plan")).not.toBeInTheDocument();
		},
	);
	it("restores only the active run's plan and does not fall back from an empty root", () => {
		const store = useAgentStore.getState();
		store.hydrateAgentChatState([], [], {
			rootRunId: "run-2",
			running: true,
			conversations: {
				"run-2": createConversation("run-2", {
					status: "running",
					messages: [user("run-1"), plan("run-1"), user("run-2"), plan("run-2")],
				}),
			},
		});
		const view = render(<LivePlan />);
		expect(screen.getByTestId("agent-live-plan")).toBeInTheDocument();
		view.unmount();
		store.hydrateAgentChatState([], [], {
			rootRunId: "run-3",
			running: true,
			conversations: {
				...useAgentStore.getState().conversations,
				"run-3": createConversation("run-3", { status: "running" }),
			},
		});
		render(<LivePlan />);
		expect(screen.queryByTestId("agent-live-plan")).not.toBeInTheDocument();
	});
	it.each(["finishRun", "failRun", "cancelRun"] as const)(
		"does not let a late plan update undo %s",
		(action) => {
			const store = useAgentStore.getState();
			store.startRun("第一轮");
			store.bindRootRun("run-1");
			store.setPlan(entries, "run-1");
			if (action === "finishRun") store.finishRun("run-1");
			else store[action]("结束", "run-1");
			const status = useAgentStore.getState().conversations["run-1"].status;
			store.setPlan(entries, "run-1");
			expect(useAgentStore.getState().conversations["run-1"].status).toBe(status);
			expect(selectAgentLiveConversation(useAgentStore.getState())).toBeUndefined();
		},
	);
	it("updates the same turn without overwriting another turn during replay", () => {
		const store = useAgentStore.getState();
		store.startRun("请求");
		store.bindRootRun("run-2");
		store.setPlan(entries, "run-2", { turnId: "run-1" });
		store.setPlan(entries, "run-2", { turnId: "run-2" });
		store.setPlan([{ content: "更新", status: "completed" }], "run-2", { turnId: "run-1" });
		const plans = useAgentStore
			.getState()
			.conversations["run-2"].messages.filter((message) => message.kind === "plan");
		expect(plans).toHaveLength(2);
		expect(plans[0]).toMatchObject({
			turnId: "run-1",
			metadata: { planEntries: [{ content: "更新", status: "completed" }] },
		});
		expect(plans[1]).toMatchObject({ turnId: "run-2", metadata: { planEntries: entries } });
	});
	it.each(["succeeded", "failed", "cancelled", "interrupted"] as const)(
		"retains unconfirmed history without animation after %s",
		(outcome) => {
			const original = plan("run-1");
			const turns = buildAgentTurnViewModels([user("run-1"), original], {
				activeTurnId: "run-1",
				activeTurn: { lifecycle: "completed", outcome },
			});
			const historyEntries = turns[0].processItems[0].metadata?.planEntries;
			const view = render(<PlanBlock content="" entries={historyEntries} />);
			expect(screen.getAllByText("未确认")).toHaveLength(2);
			expect(view.container.querySelector(".animate-spin")).toBeNull();
			expect(view.container.querySelectorAll(".agent-plan-row-completed")).toHaveLength(1);
			expect(original.metadata?.planEntries).toEqual(entries);
		},
	);
});
