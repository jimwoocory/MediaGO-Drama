import type React from "react";
import type { APIKeyProvider } from "@/domains/settings/api/settings";
import { Badge } from "@/shared/components/ui/badge";
import { cn } from "@/shared/lib/utils";

type Capability = "Agent" | "文本" | "图片" | "音频" | "视频" | "自定义 Base URL";

export type ProviderCapabilityTarget = "row" | "codex-access";

interface ProviderSpec {
	id: string;
	label: string;
	credential: string;
	capabilities: Capability[];
	badge?: string;
	target?: ProviderCapabilityTarget;
}

export const providerRowElementID = (providerID: string) => `provider-row-${providerID}`;

const providerSpecs: ProviderSpec[] = [
	{
		id: "codex",
		label: "Codex",
		credential: "ChatGPT OAuth",
		capabilities: ["Agent", "图片"],
		target: "codex-access",
	},
	{ id: "openai", label: "OpenAI", credential: "API Key", capabilities: ["Agent", "文本", "图片"] },
	{ id: "google", label: "Google Gemini", credential: "API Key", capabilities: ["文本", "图片"] },
	{
		id: "minimax",
		label: "MiniMax 国内",
		credential: "API Key",
		capabilities: ["Agent", "文本", "音频"],
	},
	{ id: "deepseek", label: "DeepSeek", credential: "API Key", capabilities: ["Agent", "文本"] },
	{ id: "volcengine", label: "Volcengine", credential: "API Key", capabilities: ["图片", "视频"] },
	{ id: "aliyun", label: "阿里云百炼", credential: "API Key", capabilities: ["图片", "视频"] },
	{
		id: "mediago",
		label: "MediaGo",
		credential: "API Key",
		capabilities: ["Agent", "文本", "图片", "视频"],
		badge: "后台备用",
	},
	{
		id: "dmx",
		label: "DMX",
		credential: "API Key",
		capabilities: ["Agent", "文本", "图片", "视频"],
	},
	{
		id: "aihubmix",
		label: "第三方",
		credential: "API Key",
		capabilities: ["Agent", "文本", "自定义 Base URL"],
	},
	{
		id: "speechapi",
		label: "第三方 Speech API",
		credential: "API Key",
		capabilities: ["音频", "自定义 Base URL"],
	},
	{
		id: "openrouter",
		label: "OpenRouter",
		credential: "API Key",
		capabilities: ["Agent", "文本", "图片", "视频"],
	},
	{ id: "jimeng", label: "即梦", credential: "OAuth/CLI", capabilities: ["图片", "视频"] },
	{ id: "libtv", label: "LibTV", credential: "OAuth/CLI", capabilities: ["图片", "视频"] },
	{ id: "xiaoyunque", label: "小云雀", credential: "Access Key/CLI", capabilities: ["视频"] },
];

export const ProviderCapabilityMatrix: React.FC<{
	providers: APIKeyProvider[];
	onSelectProvider?: (providerID: string, target: ProviderCapabilityTarget) => void;
}> = ({ providers, onSelectProvider }) => {
	const providerByID = new Map(providers.map((provider) => [provider.id, provider]));
	return (
		<section className="pb-8 pt-1">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h3 className="text-sm font-semibold text-foreground">模型提供方与能力</h3>
					<p className="mt-1 text-xs leading-5 text-muted-foreground">
						同一模型可以有多个提供方；Agent、图片、音频和视频只显示该 Provider 已真实接入的能力。
					</p>
				</div>
				<Badge variant="secondary" className="rounded-full">
					统一 Provider 层
				</Badge>
			</div>
			<div className="mt-4 grid gap-2.5 md:grid-cols-2">
				{providerSpecs.map((spec) => {
					const provider = providerByID.get(spec.id);
					const configured = Boolean(provider?.configured);
					const target = spec.target ?? "row";
					return (
						<button
							key={spec.id}
							type="button"
							onClick={() => onSelectProvider?.(spec.id, target)}
							className="rounded-lg border border-border bg-ide-list-hover/30 px-3 py-2.5 text-left transition-colors hover:bg-ide-list-hover/55"
						>
							<div className="flex items-center justify-between gap-3">
								<div className="flex min-w-0 items-center gap-2">
									<span className="truncate text-sm font-semibold text-foreground">
										{spec.label}
									</span>
									{spec.badge ? (
										<span className="text-[11px] font-medium text-muted-foreground">
											{spec.badge}
										</span>
									) : null}
								</div>
								<span className="shrink-0 text-[10px] text-muted-foreground">
									{spec.credential}
								</span>
							</div>
							<div className="mt-2 flex flex-wrap gap-1.5">
								{spec.capabilities.map((capability) => (
									<span
										key={capability}
										className={cn(
											"rounded-md bg-background px-2 py-0.5 text-[10px] text-muted-foreground",
										)}
									>
										{capability}
									</span>
								))}
							</div>
							{configured ? <span className="sr-only">凭据已配置</span> : null}
						</button>
					);
				})}
			</div>
		</section>
	);
};
