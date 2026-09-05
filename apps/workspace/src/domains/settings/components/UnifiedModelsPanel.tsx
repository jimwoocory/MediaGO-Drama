import { useState } from "react";
import useSWR, { useSWRConfig } from "swr";
import httpClient from "@/shared/lib/http";
import { generationModelsKey } from "@/domains/generation/api/generation";
import { Button } from "@/shared/components/ui/button";
import { Input } from "@/shared/components/ui/input";
import { useToast } from "@/hooks/useToast";
import { settingsErrorMessage } from "@/domains/settings/lib/settings-error";

export const unifiedModelsKey = "/settings/unified-models";
type UnifiedModel = {
	id: string;
	kind: string;
	protocol: string;
	source: string;
	enabled: boolean;
	reason?: string;
};
type ModelList = { models: UnifiedModel[]; warning?: string };
const labels: Record<string, string> = {
	image: "图片工作台",
	audio: "音频工作台",
	video: "视频工作台",
	unknown: "待确认／文本",
};
const protocols = [
	["images", "图片 · Images API"],
	["chat-image", "图片 · Chat Completions"],
	["speech", "音频 · Speech API"],
	["videos", "视频 · Videos API"],
];
const getModels = async () =>
	(await httpClient.get<ModelList>(unifiedModelsKey, { timeout: 30000 })).data;

// UnifiedModelsPanel edits capability mappings without duplicating credentials.
export const UnifiedModelsPanel = ({ configured }: { configured: boolean }) => {
	const { data, mutate, isLoading, error } = useSWR(
		configured ? unifiedModelsKey : null,
		getModels,
	);
	const { mutate: mutateGlobal } = useSWRConfig();
	const toast = useToast();
	const [modelID, setModelID] = useState("");
	const [protocol, setProtocol] = useState("images");
	const [busy, setBusy] = useState(false);
	const [filter, setFilter] = useState("");
	const invalidateWorkbench = () =>
		mutateGlobal(generationModelsKey, undefined, { revalidate: true });
	const refresh = async () => {
		setBusy(true);
		try {
			const result = (
				await httpClient.get<ModelList>(unifiedModelsKey, {
					params: { refresh: true },
					timeout: 30000,
				})
			).data;
			await mutate(result, false);
			await invalidateWorkbench();
		} catch (err) {
			toast.error("刷新模型失败", { description: settingsErrorMessage(err, "请检查统一接口配置") });
		} finally {
			setBusy(false);
		}
	};
	const save = async (model: Pick<UnifiedModel, "id" | "protocol" | "enabled">) => {
		setBusy(true);
		try {
			const result = (await httpClient.put<ModelList>(unifiedModelsKey, model, { timeout: 30000 }))
				.data;
			await mutate(result, false);
			await invalidateWorkbench();
			toast.success(model.enabled ? "模型已加入对应工作台" : "模型已停用");
		} catch (err) {
			toast.error("保存模型失败", { description: settingsErrorMessage(err, "保存模型协议失败") });
		} finally {
			setBusy(false);
		}
	};
	if (!configured)
		return (
			<p className="mt-3 text-xs text-muted-foreground">
				配置一次统一接口，即可共享凭据使用文本、图片、音频和视频模型。
			</p>
		);
	const models = data?.models ?? [];
	const visible = models.filter((model) => model.id.toLowerCase().includes(filter.toLowerCase()));
	return (
		<div className="mt-4 space-y-3 rounded-md border border-border p-4">
			<div className="flex items-center justify-between gap-3">
				<h4 className="text-sm font-medium">工作台模型分配</h4>
				<Button size="sm" variant="outline" disabled={busy} onClick={() => void refresh()}>
					刷新模型
				</Button>
			</div>
			<p className="text-xs text-muted-foreground">
				同一份地址和密钥，无需重复填写。自动匹配不代表上游调用已验证；若协议不同，请点选模型后修改。
			</p>
			<p className="text-xs">
				{["image", "audio", "video"]
					.map(
						(kind) =>
							`${labels[kind]} ${models.filter((m) => m.kind === kind && m.enabled).length}`,
					)
					.join(" · ")}
			</p>
			{isLoading ? <p role="status">正在读取模型…</p> : null}
			{error || data?.warning ? (
				<p role="alert" className="text-sm text-destructive">
					{data?.warning || "读取模型失败，请刷新重试或手动添加。"}
				</p>
			) : null}
			<Input
				aria-label="筛选统一接口模型"
				placeholder="搜索模型 ID"
				value={filter}
				onChange={(event) => setFilter(event.target.value)}
			/>
			<div className="max-h-64 overflow-auto">
				{visible.map((model) => (
					<div
						key={`${model.id}:${model.kind}`}
						className="flex items-center justify-between gap-3 border-b border-border py-2 text-xs"
					>
						<button
							type="button"
							className="min-w-0 text-left"
							onClick={() => {
								setModelID(model.id);
								setProtocol(model.protocol || "images");
							}}
						>
							<span className="block break-all font-mono">{model.id}</span>
							<span className="text-muted-foreground">
								{labels[model.kind] || model.kind} · {model.enabled ? "已分配" : "未启用"} ·{" "}
								{model.source === "manual"
									? "手动指定"
									: model.source === "metadata"
										? "接口声明"
										: model.source === "inferred"
											? "自动匹配"
											: "待确认"}
							</span>
						</button>
						{model.protocol ? (
							<Button
								size="sm"
								variant="ghost"
								disabled={busy}
								onClick={() => void save({ ...model, enabled: !model.enabled })}
							>
								{model.enabled ? "停用" : "启用"}
							</Button>
						) : null}
					</div>
				))}
			</div>
			<form
				className="flex flex-wrap gap-2"
				onSubmit={(event) => {
					event.preventDefault();
					void save({ id: modelID.trim(), protocol, enabled: true });
				}}
			>
				<Input
					className="min-w-48 flex-1"
					aria-label="生成模型 ID"
					placeholder="未识别的模型可填写准确 ID"
					value={modelID}
					onChange={(event) => setModelID(event.target.value)}
				/>
				<select
					aria-label="生成模型协议"
					className="rounded-md border border-border bg-background px-2 text-sm"
					value={protocol}
					onChange={(event) => setProtocol(event.target.value)}
				>
					{protocols.map(([value, label]) => (
						<option key={value} value={value}>
							{label}
						</option>
					))}
				</select>
				<Button type="submit" size="sm" disabled={busy || !modelID.trim()}>
					加入工作台
				</Button>
			</form>
		</div>
	);
};
