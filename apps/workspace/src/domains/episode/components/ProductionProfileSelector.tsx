import type React from "react";
import type { ProductionProfileDefinition } from "@/domains/episode/lib/production";

interface ProductionProfileSelectorProps {
	profiles: readonly ProductionProfileDefinition[];
	value?: string;
	disabled?: boolean;
	onChange: (profileId: string) => void;
}

export const ProductionProfileSelector: React.FC<ProductionProfileSelectorProps> = ({
	profiles,
	value,
	disabled = false,
	onChange,
}) => {
	const hasProfiles = profiles.length > 0;
	return (
		<label className="grid gap-1 text-xs text-muted-foreground">
			<span>制作模式</span>
			<select
				aria-label="制作模式"
				className="h-8 w-full rounded-sm border border-border bg-background px-2 text-xs text-foreground outline-none focus:border-ring"
				disabled={disabled || !hasProfiles}
				value={hasProfiles ? (value ?? "") : ""}
				onChange={(event) => onChange(event.target.value)}
			>
				<option value="">{hasProfiles ? "未选择" : "未配置制作模式"}</option>
				{profiles.map((profile) => (
					<option key={profile.id} value={profile.id}>
						{profile.label}
					</option>
				))}
			</select>
			{!hasProfiles ? (
				<span className="leading-4">六模式定义尚未恢复；当前不会用占位名称替代。</span>
			) : null}
		</label>
	);
};
