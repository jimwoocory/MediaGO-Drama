import { describe, expect, it } from "vitest";
import {
	buildRuntimeConfigSelection,
	normalizeRuntimeConfigValue,
} from "../components/chat/AgentRuntimeConfigControls";
import { reasoningConfigForModel } from "./provider-reasoning";

describe("API reasoning selection", () => {
	it.each(["api-tokease:vendor/model:variant", "gateway-aihubmix:DeepSeek-V3.2"])(
		"offers independent levels for %s and ignores stale OAuth effort",
		(model) => {
			const config = reasoningConfigForModel(model);
			expect(normalizeRuntimeConfigValue(config, "high")).toBe("provider:default");
			expect(normalizeRuntimeConfigValue(config, "provider:high")).toBe("provider:high");
			expect(buildRuntimeConfigSelection(config, "provider:high")).toEqual({
				configId: "reasoning_effort",
				source: "providerReasoning",
				value: "provider:high",
			});
		},
	);
	it("preserves account and legacy ACP choices", () => {
		const config = { configId: "reasoning_effort", options: [{ name: "high", value: "high" }] };
		expect(reasoningConfigForModel("chatgpt:gpt-5.5", config)).toBe(config);
		expect(reasoningConfigForModel("opencode/model", config)).toBe(config);
		expect(normalizeRuntimeConfigValue(config, "provider:high")).toBe("high");
	});
});
