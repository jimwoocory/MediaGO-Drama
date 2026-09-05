import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { GenerationRoute, GenerationVersion } from "@/domains/generation/api/generation";
import { GenerationModelRoutePicker } from "./GenerationModelRoutePicker";

afterEach(cleanup);
it.each([
	["image", "aihubmix", "统一接口", "Vendor/Image:2"],
	["audio", "aihubmix", "统一接口", "Vendor/Voice:2"],
	["video", "aihubmix", "统一接口", "Vendor/Video:2"],
	["image", "codex-image", "Codex · ChatGPT 订阅", "Codex 生图（ChatGPT 订阅）"],
] as const)("selects discovered %s model from %s", (kind, provider, providerName, label) => {
	const route: GenerationRoute = {
		id: `dynamic-${kind}-${provider}`,
		familyId: `dynamic-${kind}`,
		versionId: `version-${kind}`,
		kind,
		provider,
		label: providerName,
		model: label,
		adapter: "test",
		docUrl: "",
		configured: true,
		status: "available",
		async: kind === "video",
		supportsReferenceUrls: false,
		params: [],
	};
	const version: GenerationVersion = {
		id: route.versionId,
		familyId: route.familyId,
		kind,
		label,
		canonicalModel: label,
		capabilities: { async: route.async, supportsReferenceUrls: false },
	};
	const onSelect = vi.fn();
	render(
		<GenerationModelRoutePicker
			versions={[version]}
			routes={[route]}
			selectedRoute={route}
			selectedVersion={version}
			onSelect={onSelect}
		/>,
	);
	fireEvent.click(screen.getByRole("button", { name: "模型版本和供应商" }));
	expect(screen.getByRole("button", { name: label })).toBeTruthy();
	fireEvent.click(screen.getByRole("button", { name: providerName }));
	expect(onSelect).toHaveBeenCalledWith(version.id, route.id);
});
