import { describe, expect, it } from "vitest";
import type { MarkdownDocument } from "@/domains/documents/stores";
import { readStoryboardLaneSources } from "@/domains/episode/lib/storyboard-shots";
import {
	extractProductionBoard,
	getProductionItemCount,
	productionPlanDurationSeconds,
	productionPlanSchemaVersion,
	productionShotSchemaVersion,
	storyboardLanesToProductionPlan,
	storyboardShotToProductionShot,
} from "@/domains/episode/lib/production";

describe("ProductionShot schema", () => {
	it("adapts the existing storyboard representation without losing source identity", () => {
		const lanes = readStoryboardLaneSources(
			[
				"# 第一集分镜",
				"",
				"<!-- section-id: section_reel_opening -->",
				"## 开场",
				"",
				"**时长**：8秒",
				"**景别**：中景",
				"**视角**：平视",
				"**运镜**：缓慢推近",
				"**动作**：江辰抬头看向教学楼。",
				"**光影**：清晨冷色逆光",
				"**台词**：江辰：又是新的一天。",
				"**环境音**：远处上课铃声",
				"**负向提示词**：字幕，水印",
			].join("\n"),
			{ documentId: "storyboard-episode-1" },
		);

		const plan = storyboardLanesToProductionPlan(lanes, {
			id: "episode-1-plan",
			title: "第一集制作计划",
			documentId: "storyboard-episode-1",
		});

		expect(plan.schemaVersion).toBe(productionPlanSchemaVersion);
		expect(plan.shots).toHaveLength(1);
		expect(plan.shots[0]).toEqual(
			expect.objectContaining({
				schemaVersion: productionShotSchemaVersion,
				id: expect.stringContaining("shot-1"),
				order: 0,
				title: "开场",
				timing: expect.objectContaining({ durationSeconds: 8, label: "8秒" }),
				visual: expect.objectContaining({
					description: "江辰抬头看向教学楼。",
					shotSize: "中景",
					perspective: "平视",
					cameraMove: "缓慢推近",
					lighting: "清晨冷色逆光",
				}),
				audio: expect.objectContaining({
					dialogue: "江辰：又是新的一天。",
					ambience: "远处上课铃声",
				}),
				negativePrompt: "字幕，水印",
				source: expect.objectContaining({
					documentId: "storyboard-episode-1",
					blockId: "section_reel_opening",
				}),
			}),
		);
	});

	it("does not impose a 60-second or provider-specific duration cap", () => {
		const shot = storyboardShotToProductionShot({
			title: "长镜头",
			text: "人物穿过整条街区，完成连续调度。",
			prompt: "时长：120秒\n动作：人物穿过整条街区，完成连续调度。",
			durationLabel: "120秒",
			durationSeconds: 120,
		});

		expect(shot.timing.durationSeconds).toBe(120);
		expect(shot.schemaVersion).toBe("production-shot.v1");
	});

	it("allows shots without a duration until editorial timing is decided", () => {
		const shot = storyboardShotToProductionShot({
			title: "情绪停顿",
			text: "人物沉默，看向窗外。",
			prompt: "人物沉默，看向窗外。",
		});

		expect(shot.timing.durationSeconds).toBeUndefined();
		expect(shot.visual.description).toBe("人物沉默，看向窗外。");
	});

	it("accepts long-form production targets independently from shot duration", () => {
		const lanes = readStoryboardLaneSources(
			[
				"# 长篇分镜",
				"",
				"## 第一场",
				"时长：120秒",
				"动作：第一场连续表演。",
				"",
				"## 第二场",
				"时长：180秒",
				"动作：第二场连续表演。",
			].join("\n"),
		);
		const plan = storyboardLanesToProductionPlan(lanes, {
			targetDurationSeconds: 900,
		});

		expect(plan.targetDurationSeconds).toBe(900);
		expect(productionPlanDurationSeconds(plan)).toBe(300);
	});

	it("restores the legacy production board categories", () => {
		const document = productionDocument(
			[
				"# 制作资料",
				"",
				"## 角色：江辰",
				"18岁，清瘦。",
				"",
				"## 场景：教学楼天台",
				"清晨，冷色光。",
				"",
				"## 台词：江辰",
				"又是新的一天。",
				"",
				"## 音乐建议：开场",
				"低沉氛围。",
			].join("\n"),
		);

		const board = extractProductionBoard(document);
		expect(board.characters).toHaveLength(1);
		expect(board.scenes).toHaveLength(1);
		expect(board.dialogue).toHaveLength(1);
		expect(board.music).toHaveLength(1);
		expect(getProductionItemCount(board)).toBe(4);
	});

	it("puts current storyboard shots on the legacy board with ProductionShot v1 attached", () => {
		const document = productionDocument(
			[
				"# 分镜脚本",
				"",
				"<!-- section-id: section_reel_01 -->",
				"## 镜头 01",
				"时长：12秒",
				"动作：江辰推门进入教室。",
			].join("\n"),
			"storyboard-doc",
			"storyboard",
		);

		const board = extractProductionBoard(document);
		expect(board.shots).toHaveLength(1);
		expect(board.shots[0]?.productionShot).toEqual(
			expect.objectContaining({
				schemaVersion: "production-shot.v1",
				timing: expect.objectContaining({ durationSeconds: 12 }),
			}),
		);
	});
});

const productionDocument = (
	content: string,
	id = "production-doc",
	category?: MarkdownDocument["category"],
): MarkdownDocument => ({
	id,
	title: "制作文档",
	content,
	category,
	parentId: null,
	sortOrder: 0,
	version: 1,
	updatedAt: "2026-08-31T00:00:00Z",
	isDirty: false,
	comments: [],
	workbenchDraft: null,
});
