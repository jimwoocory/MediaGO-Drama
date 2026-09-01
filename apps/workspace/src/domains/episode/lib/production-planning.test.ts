import { describe, expect, it } from "vitest";
import type { ProductionProfileDefinition } from "@/domains/episode/lib/production";
import {
	deriveShotCountGuidance,
	resolveProductionPlanningDirective,
	validateProductionPlanningProfile,
} from "@/domains/episode/lib/production-planning";

describe("production planning policy", () => {
	it("derives shot-count guidance from target duration and profile preferences", () => {
		const guidance = deriveShotCountGuidance(
			120,
			{ min: 4, target: 6, max: 10 },
			{ targetShotsPerMinute: 12 },
		);
		expect(guidance).toEqual({ min: 12, target: 24, max: 30 });
	});

	it("keeps long-form duration as a planning target rather than a schema limit", () => {
		const directive = resolveProductionPlanningDirective(profile(), 900);
		expect(directive.targetDurationSeconds).toBe(900);
		expect(directive.shotCount).toEqual({ min: 90, target: 180, max: 225 });
		expect(directive.prompt).toContain("项目目标时长：900 秒");
		expect(directive.prompt).toContain("不是硬上限");
		expect(directive.prompt).toContain("不得把 Provider 单次生成时长限制写回 ProductionShot");
	});

	it("emits all planning dimensions as stable instructions", () => {
		const directive = resolveProductionPlanningDirective(profile(), 60);
		expect(directive.prompt).toContain("镜头密度：12 镜头/分钟");
		expect(directive.prompt).toContain("节奏强度：0.8");
		expect(directive.prompt).toContain("对白权重：0.7");
		expect(directive.prompt).toContain("旁白权重：0.2");
		expect(directive.prompt).toContain("连续性强度：0.9");
		expect(directive.prompt).toContain("资产复用倾向：0.75");
	});

	it("validates invalid duration and normalized weight ranges", () => {
		const errors = validateProductionPlanningProfile({
			...profile(),
			preferredShotDurationSeconds: { min: 10, target: 4, max: 5 },
			planning: {
				targetShotsPerMinute: 0,
				pacingIntensity: 1.2,
				dialogueWeight: -0.1,
			},
		});
		expect(errors).toContain("preferred shot duration min must be <= max");
		expect(errors).toContain("preferred shot duration target must be >= min");
		expect(errors).toContain("targetShotsPerMinute must be > 0");
		expect(errors).toContain("pacingIntensity must be between 0 and 1");
		expect(errors).toContain("dialogueWeight must be between 0 and 1");
	});
});

const profile = (): ProductionProfileDefinition => ({
	id: "verified-mode",
	label: "已验证模式",
	description: "fixture",
	version: 1,
	preferredShotDurationSeconds: { min: 4, target: 6, max: 10 },
	planning: {
		targetShotsPerMinute: 12,
		pacingIntensity: 0.8,
		dialogueWeight: 0.7,
		voiceoverWeight: 0.2,
		continuityStrength: 0.9,
		assetReuseBias: 0.75,
	},
});
