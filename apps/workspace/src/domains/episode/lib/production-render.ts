import type { ProductionPlan, ProductionShot } from "@/domains/episode/lib/production";

export interface ProductionRenderPolicy {
	/** Provider/model single-request limit. Omit it when the route has no known limit. */
	maxSegmentDurationSeconds?: number;
}

export interface ProductionRenderSegment {
	id: string;
	shotId: string;
	shotOrder: number;
	segmentIndex: number;
	segmentCount: number;
	/** Offset inside the creative ProductionShot, not the global episode timeline. */
	startOffsetSeconds?: number;
	endOffsetSeconds?: number;
	durationSeconds?: number;
	prompt: string;
	shot: ProductionShot;
}

export interface ProductionRenderPlan {
	productionPlanId: string;
	segments: ProductionRenderSegment[];
}

/**
 * Adapts duration-agnostic production intent to one render route's technical limits.
 * It never mutates/truncates the source ProductionShot.
 */
export const segmentProductionPlanForRender = (
	plan: ProductionPlan,
	policy: ProductionRenderPolicy = {},
): ProductionRenderPlan => ({
	productionPlanId: plan.id,
	segments: plan.shots.flatMap((shot) => segmentProductionShotForRender(shot, policy)),
});

export const segmentProductionShotForRender = (
	shot: ProductionShot,
	policy: ProductionRenderPolicy = {},
): ProductionRenderSegment[] => {
	const duration = finitePositive(shot.timing.durationSeconds);
	const maxDuration = finitePositive(policy.maxSegmentDurationSeconds);

	if (!duration || !maxDuration || duration <= maxDuration) {
		return [createSegment(shot, 0, 1, duration ? 0 : undefined, duration, duration)];
	}

	const segmentCount = Math.ceil(duration / maxDuration);
	return Array.from({ length: segmentCount }, (_, index) => {
		const start = index * maxDuration;
		const end = Math.min(duration, start + maxDuration);
		return createSegment(shot, index, segmentCount, start, end, end - start);
	});
};

const createSegment = (
	shot: ProductionShot,
	segmentIndex: number,
	segmentCount: number,
	startOffsetSeconds?: number,
	endOffsetSeconds?: number,
	durationSeconds?: number,
): ProductionRenderSegment => ({
	id: `${shot.id}-render-${segmentIndex + 1}`,
	shotId: shot.id,
	shotOrder: shot.order,
	segmentIndex,
	segmentCount,
	startOffsetSeconds,
	endOffsetSeconds,
	durationSeconds,
	prompt: renderSegmentPrompt(
		shot,
		segmentIndex,
		segmentCount,
		startOffsetSeconds,
		endOffsetSeconds,
	),
	shot,
});

const renderSegmentPrompt = (
	shot: ProductionShot,
	segmentIndex: number,
	segmentCount: number,
	start?: number,
	end?: number,
) => {
	if (segmentCount <= 1) return shot.prompt;
	const range =
		start !== undefined && end !== undefined
			? `${formatSeconds(start)}-${formatSeconds(end)}秒`
			: "";
	return [
		shot.prompt,
		"",
		`[MediaGo Render Segment ${segmentIndex + 1}/${segmentCount}${range ? ` · ${range}` : ""}]`,
		"这是同一 ProductionShot 的技术分段，不是新的剧情镜头；保持角色、场景、构图、动作连续性和前后状态一致。",
	].join("\n");
};

const finitePositive = (value?: number) =>
	typeof value === "number" && Number.isFinite(value) && value > 0 ? value : undefined;

const formatSeconds = (value: number) =>
	Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/0+$/g, "").replace(/\.$/, "");
