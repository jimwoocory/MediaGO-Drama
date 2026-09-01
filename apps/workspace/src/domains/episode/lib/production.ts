import type {
	StoryboardLaneSource,
	StoryboardShotSummary,
} from "@/domains/episode/lib/storyboard-shots";
import { readStoryboardLaneSources } from "@/domains/episode/lib/storyboard-shots";
import type { MarkdownDocument } from "@/domains/documents/stores";
import { parseTimeline } from "@/lib/markdown/video";

export const productionShotSchemaVersion = "production-shot.v1" as const;
export const productionPlanSchemaVersion = "production-plan.v1" as const;

export type ProductionShotSchemaVersion = typeof productionShotSchemaVersion;
export type ProductionPlanSchemaVersion = typeof productionPlanSchemaVersion;

export interface ProductionShotTiming {
	/** Human-readable source timing, for example `4.00-7.50秒`. */
	label?: string;
	/** Natural shot duration. Core production planning intentionally has no maximum duration. */
	durationSeconds?: number;
	/** Optional absolute placement in the production timeline. */
	startSeconds?: number;
	endSeconds?: number;
}

export interface ProductionShotVisual {
	/** Primary visible action or visual description. */
	description: string;
	shotSize?: string;
	perspective?: string;
	cameraMove?: string;
	angle?: string;
	composition?: string;
	lighting?: string;
	color?: string;
	continuity?: string;
}

export interface ProductionShotAudio {
	dialogue?: string;
	voiceover?: string;
	ambience?: string;
	soundEffects?: string;
	music?: string;
}

export interface ProductionShotReference {
	id?: string;
	kind?: "character" | "scene" | "prop" | "image" | "video" | "audio" | "other";
	label?: string;
	assetId?: string;
	documentId?: string;
}

export interface ProductionShotSource {
	documentId?: string;
	blockId?: string;
	headingLevel?: number;
	headingOccurrence?: number;
	markdown?: string;
}

/**
 * Stable production-layer shot contract.
 *
 * This schema describes creative intent and timeline placement. Provider/model generation
 * limits (for example a 15-second video-call cap) must be handled by render segmentation,
 * never by truncating or rejecting a ProductionShot here.
 */
export interface ProductionShot {
	schemaVersion: ProductionShotSchemaVersion;
	id: string;
	title: string;
	order: number;
	timing: ProductionShotTiming;
	visual: ProductionShotVisual;
	audio: ProductionShotAudio;
	prompt: string;
	negativePrompt?: string;
	references?: ProductionShotReference[];
	source?: ProductionShotSource;
}

/**
 * Production profiles are planning strategies layered on top of the stable shot schema.
 * The registry is deliberately separate so legacy/specialized modes can be restored without
 * changing persisted shot data or imposing a fixed project duration.
 */
export interface ProductionProfileDefinition {
	id: string;
	label: string;
	description: string;
	version: number;
	/** Optional preference only. It is not a validation limit. */
	preferredShotDurationSeconds?: {
		min?: number;
		target?: number;
		max?: number;
	};
	planning?: ProductionPlanningPolicy;
	metadata?: Record<string, unknown>;
}

/**
 * Planning-only knobs used by Production Profiles.
 * All normalized weights use the inclusive [0, 1] range and never mutate ProductionShot data.
 */
export interface ProductionPlanningPolicy {
	/** Desired editorial density. Used to derive a suggested shot count when target duration is known. */
	targetShotsPerMinute?: number;
	/** 0 = deliberate/slow, 1 = intense/fast. */
	pacingIntensity?: number;
	/** Relative preference for spoken character dialogue. */
	dialogueWeight?: number;
	/** Relative preference for narration/voiceover. */
	voiceoverWeight?: number;
	/** Strength of cross-shot character/scene/action continuity requirements. */
	continuityStrength?: number;
	/** 0 = prefer fresh assets, 1 = prefer reusing established assets when suitable. */
	assetReuseBias?: number;
}

export interface ProductionPlan {
	schemaVersion: ProductionPlanSchemaVersion;
	id: string;
	title: string;
	profileId?: string;
	/** Optional editorial target. No global maximum is imposed. */
	targetDurationSeconds?: number;
	shots: ProductionShot[];
}

export type ProductionItemKind =
	| "character"
	| "scene"
	| "shot"
	| "asset"
	| "dialogue"
	| "voiceover"
	| "music"
	| "note";

export interface ProductionItem {
	id: string;
	kind: ProductionItemKind;
	title: string;
	summary: string;
	content: string;
	source: string;
	/** Present for storyboard-derived items; legacy board consumers can ignore it. */
	productionShot?: ProductionShot;
}

export interface ProductionBoard {
	characters: ProductionItem[];
	scenes: ProductionItem[];
	shots: ProductionItem[];
	assets: ProductionItem[];
	dialogue: ProductionItem[];
	voiceover: ProductionItem[];
	music: ProductionItem[];
	notes: ProductionItem[];
}

interface MarkdownSection {
	level: number;
	title: string;
	content: string;
}

export interface StoryboardProductionShotContext {
	documentId?: string;
	blockId?: string;
	headingLevel?: number;
	headingOccurrence?: number;
	markdown?: string;
	order?: number;
	id?: string;
}

export const storyboardShotToProductionShot = (
	shot: StoryboardShotSummary,
	context: StoryboardProductionShotContext = {},
): ProductionShot => {
	const fields = readProductionFields(context.markdown ?? shot.prompt);
	const durationSeconds = finiteNonNegative(
		shot.durationSeconds ?? parseDurationSeconds(fields.get("时长") ?? fields.get("时间")),
	);
	const startSeconds = finiteNonNegative(
		parseNumber(fields.get("开始") ?? fields.get("开始时间") ?? fields.get("start")),
	);
	const endSeconds = finiteNonNegative(
		parseNumber(fields.get("结束") ?? fields.get("结束时间") ?? fields.get("end")),
	);
	const order =
		Number.isInteger(context.order) && (context.order ?? 0) >= 0 ? (context.order ?? 0) : 0;
	const visualDescription = firstNonEmpty(
		shot.text,
		field(fields, "动作", "画面", "描述", "视觉", "visual"),
		shot.prompt,
	);

	return compactProductionShot({
		schemaVersion: productionShotSchemaVersion,
		id: context.id?.trim() || productionShotID(shot.title, order, context.blockId),
		title: shot.title,
		order,
		timing: {
			label: firstNonEmpty(shot.durationLabel, field(fields, "时长", "时间")) || undefined,
			durationSeconds,
			startSeconds,
			endSeconds,
		},
		visual: {
			description: visualDescription,
			shotSize: firstNonEmpty(shot.shotSize, field(fields, "景别", "shot size")) || undefined,
			perspective:
				firstNonEmpty(shot.perspective, field(fields, "视角", "perspective")) || undefined,
			cameraMove: firstNonEmpty(shot.cameraMove, field(fields, "运镜", "camera move")) || undefined,
			angle: field(fields, "机位", "角度", "camera angle") || undefined,
			composition: field(fields, "构图", "composition") || undefined,
			lighting: field(fields, "光影", "灯光", "lighting") || undefined,
			color: field(fields, "色调", "调色", "color") || undefined,
			continuity: field(fields, "连续性", "衔接", "continuity") || undefined,
		},
		audio: {
			dialogue: field(fields, "台词", "对白", "dialogue") || undefined,
			voiceover: field(fields, "旁白", "voiceover", "vo") || undefined,
			ambience: field(fields, "环境音", "ambience", "ambient") || undefined,
			soundEffects: field(fields, "音效", "sfx", "sound effects") || undefined,
			music: field(fields, "音乐", "配乐", "music") || undefined,
		},
		prompt: shot.prompt,
		negativePrompt: field(fields, "负向提示词", "negative prompt") || undefined,
		source: {
			documentId: context.documentId,
			blockId: context.blockId,
			headingLevel: context.headingLevel,
			headingOccurrence: context.headingOccurrence,
			markdown: context.markdown,
		},
	});
};

export const storyboardLaneToProductionShots = (
	lane: StoryboardLaneSource,
	documentId?: string,
	startingOrder = 0,
): ProductionShot[] =>
	lane.shots.map((shot, index) =>
		storyboardShotToProductionShot(shot, {
			documentId,
			blockId: lane.blockId,
			headingLevel: lane.headingLevel,
			headingOccurrence: lane.headingOccurrence,
			markdown: lane.markdown,
			order: startingOrder + index,
			id: `${lane.id}-shot-${index + 1}`,
		}),
	);

export const storyboardLanesToProductionPlan = (
	lanes: StoryboardLaneSource[],
	options: {
		id?: string;
		title?: string;
		documentId?: string;
		profileId?: string;
		targetDurationSeconds?: number;
	} = {},
): ProductionPlan => {
	let order = 0;
	const shots = lanes.flatMap((lane) => {
		const converted = storyboardLaneToProductionShots(lane, options.documentId, order);
		order += converted.length;
		return converted;
	});
	return {
		schemaVersion: productionPlanSchemaVersion,
		id: options.id?.trim() || "production-plan",
		title: options.title?.trim() || "制作计划",
		profileId: options.profileId?.trim() || undefined,
		targetDurationSeconds: finiteNonNegative(options.targetDurationSeconds),
		shots,
	};
};

export const productionPlanDurationSeconds = (plan: ProductionPlan) =>
	plan.shots.reduce((total, shot) => total + (shot.timing.durationSeconds ?? 0), 0);

/** Restored legacy production board, augmented with the stable ProductionShot contract. */
export const extractProductionBoard = (document: MarkdownDocument | null): ProductionBoard => {
	const board = createEmptyProductionBoard();
	if (!document) return board;

	const sections = readSections(document.content);
	for (const [index, section] of sections.entries()) {
		const kind = classifySection(section);
		if (!kind) continue;
		getProductionBoardList(board, kind).push({
			id: `${kind}-${index}-${slugify(section.title)}`,
			kind,
			title: cleanProductionTitle(section.title, kind),
			summary: summarizeSection(section.content),
			content: section.content,
			source: `H${section.level} ${section.title}`,
		});
	}

	const storyboardLanes =
		document.category === "storyboard"
			? readStoryboardLaneSources(document.content, { documentId: document.id })
			: [];
	const storyboardShots = storyboardLanesToProductionPlan(storyboardLanes, {
		id: `${document.id}-production-plan`,
		title: `${document.title || "分镜"}制作计划`,
		documentId: document.id,
	}).shots;
	if (storyboardShots.length > 0) {
		// Prefer the richer current storyboard representation over duplicate heading-only shot items.
		board.shots = board.shots.filter((item) => !isStoryboardSectionItem(item));
		for (const shot of storyboardShots) {
			board.shots.push({
				id: `shot-${shot.id}`,
				kind: "shot",
				title: shot.title,
				summary: shot.visual.description || shot.prompt,
				content: shot.source?.markdown || shot.prompt,
				source: "Storyboard",
				productionShot: shot,
			});
		}
	}

	// Preserve support for the older fenced video timeline format when a document has no H2 storyboard lanes.
	if (storyboardShots.length === 0) {
		for (const segment of parseTimeline(document.content)) {
			const durationSeconds = Math.max(0, segment.end - segment.start);
			board.shots.push({
				id: `shot-video-${segment.id}`,
				kind: "shot",
				title: segment.title,
				summary: segment.visual || segment.audio || "来自源文档的视频块。",
				content: `\`\`\`video\nstart: ${segment.start}\nend: ${segment.end}\nvisual: ${segment.visual}\naudio: ${segment.audio}\n\`\`\``,
				source: "视频块",
				productionShot: compactProductionShot({
					schemaVersion: productionShotSchemaVersion,
					id: `video-${segment.id}`,
					title: segment.title,
					order: board.shots.length,
					timing: {
						startSeconds: segment.start,
						endSeconds: segment.end,
						durationSeconds,
					},
					visual: { description: segment.visual || segment.title },
					audio: { ambience: segment.audio || undefined },
					prompt: segment.visual || segment.title,
					source: { documentId: document.id },
				}),
			});
		}
	}

	return board;
};

export const getProductionItemCount = (board: ProductionBoard) =>
	board.characters.length +
	board.scenes.length +
	board.shots.length +
	board.assets.length +
	board.dialogue.length +
	board.voiceover.length +
	board.music.length +
	board.notes.length;

const createEmptyProductionBoard = (): ProductionBoard => ({
	characters: [],
	scenes: [],
	shots: [],
	assets: [],
	dialogue: [],
	voiceover: [],
	music: [],
	notes: [],
});

const getProductionBoardList = (board: ProductionBoard, kind: ProductionItemKind) => {
	if (kind === "character") return board.characters;
	if (kind === "scene") return board.scenes;
	if (kind === "shot") return board.shots;
	if (kind === "asset") return board.assets;
	if (kind === "dialogue") return board.dialogue;
	if (kind === "voiceover") return board.voiceover;
	if (kind === "music") return board.music;
	return board.notes;
};

const readSections = (markdown: string): MarkdownSection[] => {
	const sections: MarkdownSection[] = [];
	let current: MarkdownSection | null = null;
	for (const line of markdown.split("\n")) {
		const match = /^(#{1,4})\s+(.+)$/.exec(line);
		if (match?.[1] && match[2]) {
			if (current) sections.push({ ...current, content: current.content.trim() });
			current = { level: match[1].length, title: match[2].trim(), content: "" };
			continue;
		}
		if (current) current.content += `${line}\n`;
	}
	if (current) sections.push({ ...current, content: current.content.trim() });
	return sections;
};

const classifySection = (section: MarkdownSection): ProductionItemKind | null => {
	const source = `${section.title}\n${section.content}`.toLowerCase();
	if (includesAny(source, ["角色", "character", "人物"])) return "character";
	if (includesAny(source, ["场景", "scene", "空间"])) return "scene";
	if (includesAny(source, ["分镜", "镜头", "shot", "storyboard"])) return "shot";
	if (includesAny(source, ["素材", "道具", "asset", "制作需求", "美术"])) return "asset";
	if (includesAny(source, ["台词", "对白", "dialogue"])) return "dialogue";
	if (includesAny(source, ["旁白", "voiceover", "vo"])) return "voiceover";
	if (includesAny(source, ["音乐", "音效", "music", "sound"])) return "music";
	if (includesAny(source, ["剪辑备注", "备注", "note"])) return "note";
	return null;
};

const cleanProductionTitle = (title: string, kind: ProductionItemKind) => {
	const labels: Record<ProductionItemKind, string[]> = {
		character: ["角色", "角色设定", "人物", "character"],
		scene: ["场景", "场景设定", "scene"],
		shot: ["分镜", "镜头", "shot", "storyboard"],
		asset: ["素材需求", "素材", "道具", "asset"],
		dialogue: ["台词", "对白", "dialogue"],
		voiceover: ["旁白", "voiceover", "vo"],
		music: ["音乐建议", "音乐", "音效", "music"],
		note: ["剪辑备注", "备注", "note"],
	};
	const pattern = new RegExp(`^(${labels[kind].join("|")})\\s*[｜:：-]?\\s*`, "i");
	return title.replace(pattern, "").trim() || title;
};

const summarizeSection = (content: string) => {
	const withoutCode = content.replace(/```[\s\S]*?```/g, "").trim();
	const lines = withoutCode
		.split("\n")
		.map((line) => line.replace(/^[-*]\s*/, "").trim())
		.filter(Boolean);
	return lines.slice(0, 3).join(" ") || "从文档中提取的结构化项目。";
};

const includesAny = (value: string, keywords: string[]) =>
	keywords.some((keyword) => value.includes(keyword.toLowerCase()));

const isStoryboardSectionItem = (item: ProductionItem) =>
	item.kind === "shot" && /(?:分镜|镜头|shot|storyboard)/iu.test(`${item.title}\n${item.source}`);

const compactProductionShot = (shot: ProductionShot): ProductionShot => ({
	...shot,
	timing: compactObject(shot.timing),
	visual: compactObject(shot.visual),
	audio: compactObject(shot.audio),
	source: shot.source ? compactObject(shot.source) : undefined,
});

const compactObject = <T extends object>(value: T): T =>
	Object.fromEntries(
		Object.entries(value).filter(([, item]) => item !== undefined && item !== ""),
	) as T;

const readProductionFields = (markdown: string) => {
	const result = new Map<string, string>();
	for (const rawLine of markdown.split("\n")) {
		const line = rawLine
			.replace(/^\s*[-*]\s+/, "")
			.replace(/^#{1,6}\s+/, "")
			.replace(/\*\*/g, "")
			.replace(/`/g, "")
			.trim();
		if (!line) continue;
		const match = /^([^:：]{1,24})[:：]\s*(.+)$/u.exec(line);
		if (!match?.[1] || !match[2]) continue;
		result.set(normalizeFieldName(match[1]), match[2].trim());
	}
	return result;
};

const field = (fields: Map<string, string>, ...names: string[]) => {
	for (const name of names) {
		const value = fields.get(normalizeFieldName(name));
		if (value) return value;
	}
	return "";
};

const normalizeFieldName = (value: string) => value.trim().toLowerCase().replace(/\s+/g, " ");

const parseDurationSeconds = (value?: string) => {
	if (!value) return undefined;
	const range = /(\d+(?:\.\d+)?)\s*(?:-|~|至|—|–)\s*(\d+(?:\.\d+)?)/u.exec(value);
	if (range?.[1] && range[2]) {
		const start = Number(range[1]);
		const end = Number(range[2]);
		if (Number.isFinite(start) && Number.isFinite(end) && end >= start) return end - start;
	}
	return parseNumber(value);
};

const parseNumber = (value?: string) => {
	if (!value) return undefined;
	const matched = /-?\d+(?:\.\d+)?/.exec(value)?.[0];
	if (!matched) return undefined;
	const parsed = Number(matched);
	return Number.isFinite(parsed) ? parsed : undefined;
};

const finiteNonNegative = (value?: number) =>
	typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : undefined;

const productionShotID = (title: string, order: number, blockId?: string) =>
	`${blockId?.trim() || slugify(title)}-shot-${order + 1}`;

const slugify = (value: string) =>
	value
		.toLowerCase()
		.replace(/[^a-z0-9\u4e00-\u9fa5]+/gu, "-")
		.replace(/^-|-$/g, "") || "shot";

const firstNonEmpty = (...values: Array<string | undefined>) => {
	for (const value of values) {
		if (value?.trim()) return value.trim();
	}
	return "";
};
