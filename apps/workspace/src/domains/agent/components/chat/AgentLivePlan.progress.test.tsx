import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import {
	selectAgentRootConversation,
	selectAgentLiveConversation,
	useAgentStore,
} from "@/domains/agent/stores";
import { AgentLivePlan } from "./AgentLivePlan";

const View = () => {
	const root = useAgentStore(selectAgentRootConversation);
	const live = useAgentStore(selectAgentLiveConversation);
	return (
		<AgentLivePlan
			isRunning={Boolean(live)}
			status={root?.status}
			runId={root?.runId ?? null}
			messages={root?.messages ?? []}
		/>
	);
};
const entries = Array.from({ length: 9 }, (_, index) => ({
	content: `剧本步骤 ${index + 1}`,
	status: index < 4 ? "completed" : index === 4 ? "in_progress" : "pending",
}));
const begin = () => {
	const store = useAgentStore.getState();
	store.startRun("写剧本");
	store.bindRootRun("script-run");
	store.setPlan(entries, "script-run");
	return store;
};
afterEach(() => {
	cleanup();
	useAgentStore.getState().resetSession();
});

describe("same-run execution and plan progress", () => {
	it("tracks later tool results even when the model stops reporting its nine-step plan", () => {
		const store = begin();
		const view = render(<View />);
		act(() => {
			store.upsertToolCallMessage("write-scene-seven", { status: "in_progress" }, "script-run");
		});
		expect(screen.getByRole("status")).toHaveTextContent("1 项进行中");
		act(() => {
			store.upsertToolCallMessage("write-scene-seven", { status: "completed" }, "script-run");
			store.upsertToolCallMessage("verify", { status: "completed" }, "script-run");
		});
		expect(screen.getByRole("status")).toHaveTextContent("上次计划回报后：已完成 2 项操作");
		expect(view.container.querySelectorAll(".agent-plan-row-completed")).toHaveLength(4);
		expect(view.container.querySelector(".animate-spin")).toBeNull();
		act(() => store.finishRun("script-run"));
		expect(screen.getByTestId("agent-live-plan")).toBeInTheDocument();
		expect(screen.getByRole("status")).toHaveTextContent("执行已结束；5 个步骤未收到完成回报");
		expect(view.container.querySelectorAll(".agent-plan-row-completed")).toHaveLength(4);
		expect(screen.getAllByText("未确认")).toHaveLength(5);
	});
	it("clears the activity checkpoint when a new native plan confirms the steps", () => {
		const store = begin();
		render(<View />);
		act(() => store.upsertToolCallMessage("write", { status: "completed" }, "script-run"));
		expect(screen.getByRole("status")).toHaveTextContent("已完成 1 项操作");
		act(() =>
			store.setPlan(
				entries.map((entry) => ({ ...entry, status: "completed" })),
				"script-run",
			),
		);
		expect(screen.queryByRole("status")).not.toBeInTheDocument();
		act(() => store.finishRun("script-run"));
		expect(screen.getByRole("button")).toHaveTextContent("计划已确认 9 / 9 步");
		expect(screen.queryByText("未确认")).not.toBeInTheDocument();
	});
	it.each(["failRun", "cancelRun"] as const)(
		"retains terminal state through hydration and a late plan after %s",
		(action) => {
			const store = begin();
			store.upsertToolCallMessage("write", { status: "failed" }, "script-run");
			store[action]("结束", "script-run");
			const saved = JSON.parse(JSON.stringify(useAgentStore.getState().conversations));
			store.resetSession();
			store.hydrateAgentChatState([], [], {
				rootRunId: "script-run",
				conversations: saved,
				running: false,
			});
			const view = render(<View />);
			expect(screen.getByTestId("agent-live-plan")).toBeInTheDocument();
			expect(view.container.querySelector(".animate-spin")).toBeNull();
			act(() => store.setPlan(entries, "script-run"));
			expect(screen.getByRole("status")).toHaveTextContent(
				action === "failRun" ? "执行失败" : "执行已取消",
			);
			act(() => store.startRun("新请求"));
			expect(screen.queryByTestId("agent-live-plan")).not.toBeInTheDocument();
		},
	);
});
