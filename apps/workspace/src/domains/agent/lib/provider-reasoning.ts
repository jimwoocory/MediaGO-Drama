import type { AgentRuntimeSelectConfig } from "@/domains/agent/api/agent";

// Keep API choices distinct from saved account-channel effort values.
const providerReasoningConfig: AgentRuntimeSelectConfig = {
	configId: "reasoning_effort",
	source: "providerReasoning",
	currentValue: "provider:default",
	options: [
		{ name: "接口默认", value: "provider:default" },
		{ name: "无", value: "provider:none" },
		{ name: "最低", value: "provider:minimal" },
		{ name: "低", value: "provider:low" },
		{ name: "中", value: "provider:medium" },
		{ name: "高", value: "provider:high" },
		{ name: "超高", value: "provider:xhigh" },
	],
};

export const reasoningConfigForModel = (model: string, config?: AgentRuntimeSelectConfig) =>
	/^(api-|gateway-)[^:]+:/.test(model) ? providerReasoningConfig : config;
