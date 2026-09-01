import { describe, expect, it } from "vitest";
import {
	loadProductionProfileManifest,
	productionProfileManifestSchemaVersion,
} from "@/domains/episode/lib/production-profile-manifest";
import { builtinProductionProfileManifest } from "@/domains/episode/lib/production-profile-manifest";

describe("production profile manifest", () => {
	it("loads six verified-style entries as data without changing their stable ids", () => {
		const profiles = loadProductionProfileManifest({
			schemaVersion: productionProfileManifestSchemaVersion,
			profiles: Array.from({ length: 6 }, (_, index) => ({
				id: `mode-${index + 1}`,
				label: `模式 ${index + 1}`,
				description: "fixture",
				version: 1,
			})),
		});
		expect(profiles).toHaveLength(6);
		expect(profiles.map((profile) => profile.id)).toEqual([
			"mode-1",
			"mode-2",
			"mode-3",
			"mode-4",
			"mode-5",
			"mode-6",
		]);
	});

	it("exposes the restored six built-in production modes", () => {
		const profiles = loadProductionProfileManifest(builtinProductionProfileManifest);
		expect(profiles.map((profile) => profile.id)).toEqual([
			"animation",
			"live-action",
			"comic-drama",
			"short-drama",
			"cinematic",
			"explainer",
		]);
		expect(profiles.find((profile) => profile.id === "animation")?.label).toBe("动画");
	});
});
