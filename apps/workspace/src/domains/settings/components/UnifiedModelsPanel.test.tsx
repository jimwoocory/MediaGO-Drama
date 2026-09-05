import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import useSWR, { SWRConfig } from "swr";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import httpClient from "@/shared/lib/http";
import { generationModelsKey } from "@/domains/generation/api/generation";
import { UnifiedModelsPanel } from "./UnifiedModelsPanel";

vi.mock("@/shared/lib/http", () => ({ default: { get: vi.fn(), put: vi.fn() } }));
vi.mock("@/hooks/useToast", () => ({ useToast: () => ({ success: vi.fn(), error: vi.fn() }) }));
const models = {
	models: [
		{ id: "gpt-image-2", kind: "image", protocol: "images", source: "inferred", enabled: true },
		{ id: "tts-1", kind: "audio", protocol: "speech", source: "metadata", enabled: true },
		{ id: "sora-2", kind: "video", protocol: "videos", source: "inferred", enabled: true },
	],
};
const fetchWorkbench = vi.fn(async () => ({ routes: [] }));
function WorkbenchSubscriber() {
	useSWR(generationModelsKey, fetchWorkbench);
	return null;
}
function mount(configured = true) {
	return render(
		<SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
			<WorkbenchSubscriber />
			<UnifiedModelsPanel configured={configured} />
		</SWRConfig>,
	);
}
afterEach(cleanup);
beforeEach(() => {
	vi.clearAllMocks();
	vi.mocked(httpClient.get).mockResolvedValue({
		data: models,
		code: 0,
		success: true,
		message: "ok",
	});
	vi.mocked(httpClient.put).mockResolvedValue({
		data: models,
		code: 0,
		success: true,
		message: "ok",
	});
});

describe("UnifiedModelsPanel", () => {
	it("shows all media assignments without repeating credential inputs", async () => {
		mount();
		expect(await screen.findByText("gpt-image-2")).toBeTruthy();
		expect(screen.getByText("tts-1")).toBeTruthy();
		expect(screen.getByText("sora-2")).toBeTruthy();
		expect(screen.getByText("图片工作台 1 · 音频工作台 1 · 视频工作台 1")).toBeTruthy();
		expect(screen.queryByLabelText(/API Key/)).toBeNull();
	});
	it("uses the saved credential and refreshes the workbench after manual mapping", async () => {
		mount();
		await screen.findByText("gpt-image-2");
		await waitFor(() => expect(fetchWorkbench).toHaveBeenCalledTimes(1));
		fireEvent.change(screen.getByLabelText("生成模型 ID"), {
			target: { value: "Custom/Exact-ID" },
		});
		fireEvent.change(screen.getByLabelText("生成模型协议"), { target: { value: "chat-image" } });
		fireEvent.click(screen.getByRole("button", { name: "加入工作台" }));
		await waitFor(() =>
			expect(httpClient.put).toHaveBeenCalledWith(
				"/settings/unified-models",
				{
					id: "Custom/Exact-ID",
					protocol: "chat-image",
					enabled: true,
				},
				{ timeout: 30000 },
			),
		);
		await waitFor(() => expect(fetchWorkbench).toHaveBeenCalledTimes(2));
	});
	it("does not request models without a configured key", () => {
		mount(false);
		expect(httpClient.get).not.toHaveBeenCalled();
	});
	it("keeps discovery errors visible", async () => {
		vi.mocked(httpClient.get).mockResolvedValue({
			code: 0,
			success: true,
			message: "ok",
			data: { ...models, warning: "刷新失败，保留模型" },
		});
		mount();
		expect((await screen.findByRole("alert")).textContent).toContain("保留模型");
	});
});
