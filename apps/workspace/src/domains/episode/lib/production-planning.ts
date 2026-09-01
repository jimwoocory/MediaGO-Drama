import type {
	ProductionPlanningPolicy,
	ProductionProfileDefinition,
} from "@/domains/episode/lib/production";

export interface ProductionShotCountGuidance {
	min?: number;
	target?: number;
	max?: number;
}

export interface ProductionPlanningDirective {
	profileId: string;
	profileLabel: string;
	profileVersion: number;
	targetDurationSeconds?: number;
	preferredShotDurationSeconds?: {
		min?: number;
		target?: number;
		max?: number;
	};
	shotCount?: ProductionShotCountGuidance;
	policy: ProductionPlanningPolicy;
	prompt: string;
}

/**
 * Resolves one Production Profile into deterministic planning guidance.
 * The directive is advisory input for storyboard planning; it never edits existing ProductionShot objects.
 */
export const resolveProductionPlanningDirective = (
	profile: ProductionProfileDefinition,
	targetDurationSeconds?: number,
): ProductionPlanningDirective => {
	const duration = finitePositive(targetDurationSeconds);
	const preferred = compactDurationPreference(profile.preferredShotDurationSeconds);
	const policy = compactPlanningPolicy(profile.planning);
	const shotCount = duration ? deriveShotCountGuidance(duration, preferred, policy) : undefined;

	const directive: ProductionPlanningDirective = {
		profileId: profile.id,
		profileLabel: profile.label,
		profileVersion: profile.version,
		targetDurationSeconds: duration,
		preferredShotDurationSeconds: preferred,
		shotCount,
		policy,
		prompt: "",
	};
	directive.prompt = buildProductionPlanningPrompt(directive);
	return directive;
};

export const deriveShotCountGuidance = (
	targetDurationSeconds: number,
	preferredShotDurationSeconds: ProductionProfileDefinition["preferredShotDurationSeconds"],
	policy: ProductionPlanningPolicy = {},
): ProductionShotCountGuidance | undefined => {
	const duration = finitePositive(targetDurationSeconds);
	if (!duration) return undefined;
	const minShotDuration = finitePositive(preferredShotDurationSeconds?.min);
	const targetShotDuration = finitePositive(preferredShotDurationSeconds?.target);
	const maxShotDuration = finitePositive(preferredShotDurationSeconds?.max);
	const density = finitePositive(policy.targetShotsPerMinute);

	const min = maxShotDuration ? Math.max(1, Math.ceil(duration / maxShotDuration)) : undefined;
	const max = minShotDuration ? Math.max(1, Math.floor(duration / minShotDuration)) : undefined;
	let target = density
		? Math.max(1, Math.round((duration / 60) * density))
		: targetShotDuration
			? Math.max(1, Math.round(duration / targetShotDuration))
			: undefined;

	if (target !== undefined && min !== undefined) target = Math.max(min, target);
	if (target !== undefined && max !== undefined) target = Math.min(max, target);

	if (min === undefined && target === undefined && max === undefined) return undefined;
	return compactObject({ min, target, max });
};

export const buildProductionPlanningPrompt = (directive: ProductionPlanningDirective) => {
	const lines = [
		"[MediaGo Production Planning Policy]",
		`制作模式：${directive.profileLabel} (${directive.profileId}) · v${directive.profileVersion}`,
		"本策略只用于规划镜头，不得把 Provider 单次生成时长限制写回 ProductionShot。",
	];
	if (directive.targetDurationSeconds) {
		lines.push(
			`项目目标时长：${formatNumber(directive.targetDurationSeconds)} 秒；这是编辑目标，不是硬上限。`,
		);
	}
	const preferred = directive.preferredShotDurationSeconds;
	if (preferred && Object.keys(preferred).length > 0) {
		const parts = [
			preferred.min ? `最短偏好 ${formatNumber(preferred.min)} 秒` : "",
			preferred.target ? `目标 ${formatNumber(preferred.target)} 秒` : "",
			preferred.max ? `最长偏好 ${formatNumber(preferred.max)} 秒` : "",
		].filter(Boolean);
		lines.push(`镜头自然时长偏好：${parts.join("，")}；这是偏好，不是镜头合法性限制。`);
	}
	if (directive.shotCount) {
		const parts = [
			directive.shotCount.min ? `至少约 ${directive.shotCount.min}` : "",
			directive.shotCount.target ? `建议约 ${directive.shotCount.target}` : "",
			directive.shotCount.max ? `至多约 ${directive.shotCount.max}` : "",
		].filter(Boolean);
		lines.push(`镜头数量指导：${parts.join("，")} 个；根据剧情完整性允许偏离。`);
	}
	appendPolicyLine(lines, "镜头密度", directive.policy.targetShotsPerMinute, " 镜头/分钟");
	appendWeightLine(lines, "节奏强度", directive.policy.pacingIntensity, "0 偏舒缓，1 偏快速");
	appendWeightLine(
		lines,
		"对白权重",
		directive.policy.dialogueWeight,
		"越高越优先用角色对白承载信息",
	);
	appendWeightLine(lines, "旁白权重", directive.policy.voiceoverWeight, "越高越允许旁白承载信息");
	appendWeightLine(
		lines,
		"连续性强度",
		directive.policy.continuityStrength,
		"越高越严格保持角色、场景、动作和光线状态连续",
	);
	appendWeightLine(
		lines,
		"资产复用倾向",
		directive.policy.assetReuseBias,
		"越高越优先复用已建立角色/场景/道具资产",
	);
	return lines.join("\n");
};

export const validateProductionPlanningProfile = (
	profile: ProductionProfileDefinition,
): string[] => {
	const errors: string[] = [];
	const preferred = profile.preferredShotDurationSeconds;
	for (const [name, value] of Object.entries(preferred ?? {})) {
		if (value !== undefined && !finitePositive(value)) errors.push(`${name} must be > 0`);
	}
	if (
		preferred?.min !== undefined &&
		preferred?.max !== undefined &&
		preferred.min > preferred.max
	) {
		errors.push("preferred shot duration min must be <= max");
	}
	if (
		preferred?.target !== undefined &&
		preferred?.min !== undefined &&
		preferred.target < preferred.min
	) {
		errors.push("preferred shot duration target must be >= min");
	}
	if (
		preferred?.target !== undefined &&
		preferred?.max !== undefined &&
		preferred.target > preferred.max
	) {
		errors.push("preferred shot duration target must be <= max");
	}
	const policy = profile.planning;
	if (policy?.targetShotsPerMinute !== undefined && !finitePositive(policy.targetShotsPerMinute)) {
		errors.push("targetShotsPerMinute must be > 0");
	}
	for (const key of weightKeys) {
		const value = policy?.[key];
		if (value !== undefined && !isUnitInterval(value))
			errors.push(`${key} must be between 0 and 1`);
	}
	return errors;
};

const weightKeys = [
	"pacingIntensity",
	"dialogueWeight",
	"voiceoverWeight",
	"continuityStrength",
	"assetReuseBias",
] as const;

const compactDurationPreference = (
	value?: ProductionProfileDefinition["preferredShotDurationSeconds"],
) => {
	if (!value) return undefined;
	const result = compactObject({
		min: finitePositive(value.min),
		target: finitePositive(value.target),
		max: finitePositive(value.max),
	});
	return Object.keys(result).length > 0 ? result : undefined;
};

const compactPlanningPolicy = (value?: ProductionPlanningPolicy): ProductionPlanningPolicy => {
	if (!value) return {};
	return compactObject({
		targetShotsPerMinute: finitePositive(value.targetShotsPerMinute),
		pacingIntensity: unitInterval(value.pacingIntensity),
		dialogueWeight: unitInterval(value.dialogueWeight),
		voiceoverWeight: unitInterval(value.voiceoverWeight),
		continuityStrength: unitInterval(value.continuityStrength),
		assetReuseBias: unitInterval(value.assetReuseBias),
	});
};

const appendPolicyLine = (lines: string[], label: string, value?: number, suffix = "") => {
	if (value === undefined) return;
	lines.push(`${label}：${formatNumber(value)}${suffix}。`);
};

const appendWeightLine = (
	lines: string[],
	label: string,
	value: number | undefined,
	explanation: string,
) => {
	if (value === undefined) return;
	lines.push(`${label}：${formatNumber(value)}（${explanation}）。`);
};

const compactObject = <T extends object>(value: T): T =>
	Object.fromEntries(Object.entries(value).filter(([, item]) => item !== undefined)) as T;

const finitePositive = (value?: number) =>
	typeof value === "number" && Number.isFinite(value) && value > 0 ? value : undefined;

const isUnitInterval = (value: number) => Number.isFinite(value) && value >= 0 && value <= 1;

const unitInterval = (value?: number) =>
	typeof value === "number" && isUnitInterval(value) ? value : undefined;

const formatNumber = (value: number) =>
	Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/0+$/g, "").replace(/\.$/, "");
