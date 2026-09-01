import { describe, expect, it } from "vitest";
import type { ProductionPlan, ProductionShot } from "@/domains/episode/lib/production";
import {
	segmentProductionPlanForRender,
	segmentProductionShotForRender,
} from "@/domains/episode/lib/production-render";

describe("production render segmentation", () => {
	it("keeps an unconstrained production shot intact", () => {
		const shot = createShot(120);
		const segments = segmentProductionShotForRender(shot);

		expect(segments).toHaveLength(1);
		expect(segments[0]).toEqual(
			expect.objectContaining({
				shotId: shot.id,
				durationSeconds: 120,
				segmentCount: 1,
				prompt: shot.prompt,
			}),
		);
		expect(shot.timing.durationSeconds).toBe(120);
	});

	it("segments a 120-second creative shot for a 15-second render route without truncation", () => {
		const shot = createShot(120);
		const segments = segmentProductionShotForRender(shot, { maxSegmentDurationSeconds: 15 });

		expect(segments).toHaveLength(8);
		expect(segments.map((segment) => segment.durationSeconds)).toEqual(Array(8).fill(15));
		expect(segments[0]).toEqual(
			expect.objectContaining({ startOffsetSeconds: 0, endOffsetSeconds: 15, segmentIndex: 0 }),
		);
		expect(segments[7]).toEqual(
			expect.objectContaining({ startOffsetSeconds: 105, endOffsetSeconds: 120, segmentIndex: 7 }),
		);
		expect(segments.reduce((sum, segment) => sum + (segment.durationSeconds ?? 0), 0)).toBe(120);
		expect(segments[0]?.prompt).toContain("1/8");
		expect(segments[0]?.prompt).toContain("不是新的剧情镜头");
		expect(shot.timing.durationSeconds).toBe(120);
	});

	it("keeps the final remainder instead of stretching it to the provider maximum", () => {
		const segments = segmentProductionShotForRender(createShot(17), {
			maxSegmentDurationSeconds: 15,
		});

		expect(segments.map((segment) => segment.durationSeconds)).toEqual([15, 2]);
	});

	it("can apply one route policy across a long-form production plan", () => {
		const plan: ProductionPlan = {
			schemaVersion: "production-plan.v1",
			id: "long-form",
			title: "长篇制作",
			targetDurationSeconds: 900,
			shots: [createShot(40, "shot-a", 0), createShot(20, "shot-b", 1)],
		};
		const renderPlan = segmentProductionPlanForRender(plan, { maxSegmentDurationSeconds: 15 });

		expect(renderPlan.segments.map((segment) => segment.durationSeconds)).toEqual([
			15, 15, 10, 15, 5,
		]);
		expect(renderPlan.productionPlanId).toBe("long-form");
		expect(plan.targetDurationSeconds).toBe(900);
	});
});

const createShot = (durationSeconds: number, id = "shot-long", order = 0): ProductionShot => ({
	schemaVersion: "production-shot.v1",
	id,
	title: "连续长镜头",
	order,
	timing: { durationSeconds },
	visual: { description: "人物完成一段连续调度。" },
	audio: {},
	prompt: "人物完成一段连续调度。",
});
