import { describe, expect, it } from "vitest";
import { reconcilePlanEntriesWithCompletedDocumentWrites } from "./plan-progress";

describe("reconcilePlanEntriesWithCompletedDocumentWrites", () => {
	it("completes only the plan entry with an exactly matching written document title", () => {
		const entries = [
			{ content: "第 09 集 曹万顺的反扑", status: "in_progress" },
			{ content: "第 10 集 夜半火光", status: "pending" },
		];
		const result = reconcilePlanEntriesWithCompletedDocumentWrites(
			entries,
			[
				{
					id: "write-nine",
					turnId: "run-1",
					role: "assistant",
					content: "",
					kind: "tool",
					metadata: {
						status: "completed",
						inputJson: {
							command:
								"@'\n---\ntitle: 第 09 集 曹万顺的反扑\n---\n'@ | Out-File -FilePath '第 09 集 曹万顺的反扑.md'",
						},
					},
				},
				{
					id: "read-ten",
					turnId: "run-1",
					role: "assistant",
					content: "title: 第 10 集 夜半火光",
					kind: "tool",
					metadata: { status: "completed", canonicalToolName: "read_file" },
				},
			],
			"run-1",
		);

		expect(result.map((entry) => entry.status)).toEqual(["completed", "pending"]);
	});

	it("does not use completed writes from another run", () => {
		const result = reconcilePlanEntriesWithCompletedDocumentWrites(
			[{ content: "第 09 集 曹万顺的反扑", status: "pending" }],
			[
				{
					id: "other-run-write",
					turnId: "run-2",
					role: "assistant",
					content: "title: 第 09 集 曹万顺的反扑 | Out-File",
					kind: "tool",
					metadata: { status: "completed" },
				},
			],
			"run-1",
		);
		expect(result[0]?.status).toBe("pending");
	});
});
