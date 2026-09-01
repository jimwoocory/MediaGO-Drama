import { describe, expect, it } from "vitest";
import type { ProductionPlan, ProductionProfileDefinition } from "@/domains/episode/lib/production";
import {
	applyProductionProfileSelection,
	createProductionProfileRegistry,
	resolveProductionPlanPlanningDirective,
} from "@/domains/episode/lib/production-profiles";

describe("production profile registry", () => {
	it("supports six built-in-style profiles without coupling them to Shot Schema", () => {
		const profiles = Array.from({ length: 6 }, (_, index) => profile(`mode-${index + 1}`));
		const registry = createProductionProfileRegistry(profiles);
		expect(registry.list()).toHaveLength(6);
		expect(registry.has("MODE-6")).toBe(true);
	});

	it("rejects duplicate stable ids", () => {
		expect(() => createProductionProfileRegistry([profile("mode-a"), profile("MODE-A")])).toThrow(
			/duplicate production profile id/,
		);
	});

	it("applies project selection without mutating shots", () => {
		const registry = createProductionProfileRegistry([profile("mode-a")]);
		const plan = productionPlan();
		const selected = applyProductionProfileSelection(
			plan,
			{ profileId: "MODE-A", targetDurationSeconds: 900 },
			registry,
		);

		expect(selected.profileId).toBe("mode-a");
		expect(selected.targetDurationSeconds).toBe(900);
		expect(selected.shots).toBe(plan.shots);
		expect(selected.shots[0]?.timing.durationSeconds).toBe(120);
	});

	it("does not persist an unknown profile id", () => {
		const selected = applyProductionProfileSelection(
			productionPlan(),
			{ profileId: "not-registered", targetDurationSeconds: 1200 },
			createProductionProfileRegistry([]),
		);
		expect(selected.profileId).toBeUndefined();
		expect(selected.targetDurationSeconds).toBe(1200);
	});

	it("resolves a ProductionPlan profile into planning guidance", () => {
		const registry = createProductionProfileRegistry([
			{
				...profile("mode-a"),
				preferredShotDurationSeconds: { min: 4, target: 6, max: 10 },
				planning: { targetShotsPerMinute: 12, continuityStrength: 0.9 },
			},
		]);
		const plan = { ...productionPlan(), profileId: "mode-a", targetDurationSeconds: 120 };
		const directive = resolveProductionPlanPlanningDirective(plan, registry);

		expect(directive?.profileId).toBe("mode-a");
		expect(directive?.shotCount).toEqual({ min: 12, target: 24, max: 30 });
		expect(directive?.prompt).toContain("连续性强度：0.9");
		expect(plan.shots[0]?.timing.durationSeconds).toBe(120);
	});

	it("rejects invalid planning rules during registry creation", () => {
		expect(() =>
			createProductionProfileRegistry([
				{
					...profile("bad-mode"),
					planning: { pacingIntensity: 1.5 },
				},
			]),
		).toThrow(/invalid production profile bad-mode/);
	});
});

const profile = (id: string): ProductionProfileDefinition => ({
	id,
	label: id,
	description: "test profile",
	version: 1,
});

const productionPlan = (): ProductionPlan => ({
	schemaVersion: "production-plan.v1",
	id: "plan-a",
	title: "制作计划",
	shots: [
		{
			schemaVersion: "production-shot.v1",
			id: "shot-a",
			title: "长镜头",
			order: 0,
			timing: { durationSeconds: 120 },
			visual: { description: "连续动作" },
			audio: {},
			prompt: "连续动作",
		},
	],
});
