import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	beginProviderLogin,
	clearAPIKey,
	completeProviderLogin,
	getAIHubMixSettings,
	getAPIKeys,
	getModelPlatforms,
	getSpeechAPISettings,
	getVideoAPISettings,
	saveAIHubMixSettings,
	saveAPIKey,
	saveVideoAPISettings,
	type APIKeyLoginChallenge,
	type APIKeyListResponse,
	type ModelPlatformsResponse,
} from "@/domains/settings/api/settings";
import { agentBackendsKey, isAgentRuntimeConfigKey } from "@/domains/agent/api/agent";
import { generationModelsKey } from "@/domains/generation/api/generation";
import { openExternalUrl } from "@/shared/desktop/actions";
import { useSettingsNavigationStore } from "@/lib/stores/settings";
import { ConfirmDialog } from "@/shared/components/callable/ConfirmDialog";
import { Settings } from "./Settings";

const swrMocks = vi.hoisted(() => ({
	mutate: vi.fn(),
}));
const toastMocks = vi.hoisted(() => ({ error: vi.fn() }));

vi.mock("swr", async (importOriginal) => {
	const actual = await importOriginal<typeof import("swr")>();
	return {
		...actual,
		useSWRConfig: () => ({ mutate: swrMocks.mutate }),
	};
});

vi.mock("@/domains/settings/api/settings", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/domains/settings/api/settings")>();
	return {
		...actual,
		beginProviderLogin: vi.fn(),
		clearAPIKey: vi.fn(),
		completeProviderLogin: vi.fn(),
		getAIHubMixSettings: vi.fn(),
		getAPIKeys: vi.fn(),
		getJianyingDraftSettings: vi.fn(),
		getModelPlatforms: vi.fn(),
		getSpeechAPISettings: vi.fn(),
		getVideoAPISettings: vi.fn(),
		saveAIHubMixSettings: vi.fn(),
		saveAPIKey: vi.fn(),
		saveVideoAPISettings: vi.fn(),
		saveJianyingDraftSettings: vi.fn(),
	};
});

vi.mock("@/hooks/useToast", () => ({
	useToast: () => ({
		error: toastMocks.error,
		info: vi.fn(),
		success: vi.fn(),
		warning: vi.fn(),
	}),
}));

vi.mock("@/domains/settings/components/CodexSkillsPanel", () => ({
	CodexSkillsPanel: () => <h2>Codex 全局技能</h2>,
}));

vi.mock("@/shared/desktop/actions", () => ({
	openExternalUrl: vi.fn(),
	pickDesktopDirectory: vi.fn(),
}));

describe("Settings API key page", () => {
	it("shows the server validation reason and keeps the configuration dialog open", async () => {
		vi.mocked(saveAIHubMixSettings).mockRejectedValueOnce({
			code: 400,
			message: "Base URL path ends with /vl; use /v1 (number 1)",
		});
		renderSettings();
		fireEvent.click(await screen.findByRole("button", { name: "编辑 第三方" }));
		const dialog = await screen.findByRole("dialog", { name: "配置 第三方" });
		fireEvent.change(within(dialog).getByLabelText("第三方 Base URL"), {
			target: { value: "https://tokease.cn/vl" },
		});
		fireEvent.click(within(dialog).getByRole("button", { name: "保存" }));
		await waitFor(() =>
			expect(toastMocks.error).toHaveBeenCalledWith("保存失败", {
				description: expect.stringContaining("number 1"),
			}),
		);
		expect(dialog).toBeInTheDocument();
		expect(saveAPIKey).not.toHaveBeenCalled();
	});
	beforeEach(() => {
		vi.clearAllMocks();
		useSettingsNavigationStore.setState({ activeTab: "api-keys" });
		vi.mocked(getAIHubMixSettings).mockResolvedValue({ baseURL: "https://aihubmix.com/v1" });
		vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({}));
		vi.mocked(getModelPlatforms).mockResolvedValue(modelPlatformsResponse());
		vi.mocked(getSpeechAPISettings).mockResolvedValue({
			baseURL: "",
			model: "gpt-4o-mini-tts",
			voice: "alloy",
		});
		vi.mocked(getVideoAPISettings).mockResolvedValue({ baseURL: "", model: "" });
		vi.mocked(saveVideoAPISettings).mockResolvedValue({
			baseURL: "https://video.example.test/v1",
			model: "seedance-custom",
		});
		vi.mocked(saveAIHubMixSettings).mockResolvedValue({ baseURL: "https://aihubmix.com/v1" });
		vi.mocked(saveAPIKey).mockResolvedValue(apiKeysResponse({ mediagoConfigured: true }));
	});

	afterEach(() => {
		cleanup();
	});

	it("shows purpose-specific interfaces without a duplicate capability matrix", async () => {
		renderSettings();

		const unifiedHeading = await screen.findByRole("heading", { name: "统一接口（第三方）" });
		const speechHeading = screen.getByRole("heading", {
			name: "音频接口（第三方 Speech API）",
		});
		const videoHeading = screen.getByRole("heading", {
			name: "视频接口（第三方 Video API）",
		});
		expect(screen.queryByRole("heading", { name: "模型提供方与能力" })).not.toBeInTheDocument();
		expect(unifiedHeading).toBeInTheDocument();
		expect(
			speechHeading.compareDocumentPosition(videoHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
		).toBeTruthy();
		expect(screen.getByText(/自动分配到 Agent、图片、音频和视频工作台/)).toBeInTheDocument();
		expect(screen.getByText(/无需即梦授权/)).toBeInTheDocument();
	});
	it("shows a configured credential only on its actual settings row", async () => {
		vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({ aihubmixConfigured: true }));

		renderSettings();

		expect(await screen.findByText("sk••••••456")).toBeInTheDocument();
		expect(screen.queryByText(/已真实接入的能力/)).not.toBeInTheDocument();
	});
	it("loads 第三方 Base URL in its independent configuration dialog", async () => {
		vi.mocked(getAIHubMixSettings).mockResolvedValue({
			baseURL: "https://gateway.example.test/v1",
		});

		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "编辑 第三方" }));
		const dialog = await screen.findByRole("dialog", { name: "配置 第三方" });
		expect(within(dialog).getByLabelText("第三方 Base URL")).toHaveValue(
			"https://gateway.example.test/v1",
		);
		expect(within(dialog).getByLabelText("第三方 API Key")).toBeInTheDocument();
	});
	it("does not mark unconfigured third-party providers as configured", async () => {
		renderSettings();

		expect(await screen.findByRole("heading", { name: "统一接口（第三方）" })).toBeInTheDocument();
		expect(screen.queryByText("凭据已配置")).not.toBeInTheDocument();
		expect(screen.getAllByText("第三方").length).toBeGreaterThan(0);
	});
	it("keeps the actual interfaces visible when the model-platform allowlist is empty", async () => {
		vi.mocked(getModelPlatforms).mockResolvedValue({ platforms: [] });

		renderSettings();

		expect(await screen.findByRole("heading", { name: "统一接口（第三方）" })).toBeInTheDocument();
		expect(
			screen.getByRole("heading", { name: "音频接口（第三方 Speech API）" }),
		).toBeInTheDocument();
		expect(
			screen.getByRole("heading", { name: "视频接口（第三方 Video API）" }),
		).toBeInTheDocument();
	});
	it("keeps other providers collapsed by default even when one is configured", async () => {
		vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({ openrouterConfigured: true }));
		renderSettings();

		expect(await screen.findByRole("button", { name: /其他接入方式/ })).toBeInTheDocument();
		expect(screen.queryByRole("heading", { name: "自定义接口" })).not.toBeInTheDocument();
		expect(screen.queryByRole("heading", { name: "官方供应商" })).not.toBeInTheDocument();
	});

	it("saves 第三方 Base URL and API key together", async () => {
		vi.mocked(saveAIHubMixSettings).mockResolvedValue({
			baseURL: "https://gateway.example.test/v1",
		});
		vi.mocked(saveAPIKey).mockResolvedValue(apiKeysResponse({ aihubmixConfigured: true }));

		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "编辑 第三方" }));
		const dialog = await screen.findByRole("dialog", { name: "配置 第三方" });
		fireEvent.change(within(dialog).getByLabelText("第三方 Base URL"), {
			target: { value: "https://gateway.example.test/v1" },
		});
		fireEvent.change(within(dialog).getByLabelText("第三方 API Key"), {
			target: { value: "sk-aihubmix-123456" },
		});
		fireEvent.click(within(dialog).getByRole("button", { name: "保存" }));

		await waitFor(() =>
			expect(saveAIHubMixSettings).toHaveBeenCalledWith("https://gateway.example.test/v1"),
		);
		expect(saveAPIKey).toHaveBeenCalledWith("aihubmix", "sk-aihubmix-123456");
		expectModelDependentCachesRevalidated();
	});
	it("saves a Video API that feeds the video generation workbench", async () => {
		vi.mocked(saveAPIKey).mockResolvedValue(apiKeysResponse({ videoapiConfigured: true }));
		renderSettings();

		expect(
			await screen.findByRole("heading", { name: "视频接口（第三方 Video API）" }),
		).toBeInTheDocument();
		fireEvent.click(screen.getByRole("button", { name: "编辑 第三方 Video API" }));
		const dialog = await screen.findByRole("dialog", { name: "配置 第三方 Video API" });
		fireEvent.change(within(dialog).getByLabelText("第三方 Video API Base URL"), {
			target: { value: "https://video.example.test/v1/videos" },
		});
		fireEvent.change(within(dialog).getByLabelText("第三方 Video API 模型 ID"), {
			target: { value: "seedance-custom" },
		});
		fireEvent.change(within(dialog).getByLabelText("第三方 Video API API Key"), {
			target: { value: "sk-video-123456" },
		});
		fireEvent.click(within(dialog).getByRole("button", { name: "保存" }));

		await waitFor(() =>
			expect(saveVideoAPISettings).toHaveBeenCalledWith({
				baseURL: "https://video.example.test/v1/videos",
				model: "seedance-custom",
			}),
		);
		expect(saveAPIKey).toHaveBeenCalledWith("videoapi", "sk-video-123456");
		expectModelDependentCachesRevalidated();
	});

	it("keeps AIHubMix unconfigured when only its endpoint is saved", async () => {
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "编辑 第三方" }));
		const dialog = await screen.findByRole("dialog", { name: "配置 第三方" });
		fireEvent.click(within(dialog).getByRole("button", { name: "保存" }));

		await waitFor(() =>
			expect(saveAIHubMixSettings).toHaveBeenCalledWith("https://aihubmix.com/v1"),
		);
		expect(saveAPIKey).not.toHaveBeenCalled();
		expect(screen.queryByText("sk••••••456")).not.toBeInTheDocument();
	});
	it("clears an AIHubMix credential only after confirmation", async () => {
		vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({ aihubmixConfigured: true }));
		vi.mocked(clearAPIKey).mockResolvedValue(apiKeysResponse({}));
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "第三方 更多操作" }));
		fireEvent.click(await screen.findByRole("menuitem", { name: "清除 API Key" }));
		expect(clearAPIKey).not.toHaveBeenCalled();
		fireEvent.click(await screen.findByRole("button", { name: "清除 API Key" }));

		await waitFor(() => expect(clearAPIKey).toHaveBeenCalledWith("aihubmix"));
		expectModelDependentCachesRevalidated();
	});
	it("clears a configured API key from the row menu only after confirmation", async () => {
		vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({ openrouterConfigured: true }));
		vi.mocked(clearAPIKey).mockResolvedValue(apiKeysResponse({}));
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: /其他接入方式/ }));
		fireEvent.click(screen.getByRole("button", { name: "OpenRouter 更多操作" }));
		fireEvent.click(await screen.findByRole("menuitem", { name: "清除 API Key" }));

		expect(clearAPIKey).not.toHaveBeenCalled();
		expect(
			await screen.findByRole("alertdialog", { name: "清除 OpenRouter API Key？" }),
		).toBeVisible();
		fireEvent.click(screen.getByRole("button", { name: "取消" }));
		await waitFor(() =>
			expect(
				screen.queryByRole("alertdialog", { name: "清除 OpenRouter API Key？" }),
			).not.toBeInTheDocument(),
		);
		expect(clearAPIKey).not.toHaveBeenCalled();

		fireEvent.click(screen.getByRole("button", { name: "OpenRouter 更多操作" }));
		fireEvent.click(await screen.findByRole("menuitem", { name: "清除 API Key" }));
		fireEvent.click(screen.getByRole("button", { name: "清除 API Key" }));

		await waitFor(() => expect(clearAPIKey).toHaveBeenCalledWith("openrouter"));
	});

	it("does not render non-editable routing metadata as form inputs", async () => {
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: /其他接入方式/ }));
		const customSection = screen.getByRole("heading", { name: "自定义接口" }).closest("section");
		expect(customSection).toBeTruthy();
		fireEvent.click(within(customSection as HTMLElement).getByRole("button", { name: /编辑/ }));

		const dialog = await screen.findByRole("dialog", { name: "配置 OpenRouter" });
		expect(dialog).toBeInTheDocument();
		expect(screen.getByLabelText("OpenRouter API Key")).toBeInTheDocument();
		expect(screen.queryByLabelText("供应商 ID")).not.toBeInTheDocument();
		expect(screen.queryByLabelText("端点策略")).not.toBeInTheDocument();
		expect(screen.queryByLabelText("模型路由")).not.toBeInTheDocument();
		expect(screen.queryByLabelText("能力范围")).not.toBeInTheDocument();
	});

	it("shows the jimeng CLI login flow instead of an API key input", async () => {
		renderSettings();

		const cliSection = (await screen.findByRole("heading", { name: "会员 CLI 接入" })).closest(
			"section",
		);
		expect(cliSection).toBeTruthy();
		expect(within(cliSection as HTMLElement).getByText("未登录")).toBeInTheDocument();
		expect(
			within(cliSection as HTMLElement).getByRole("button", { name: "登录" }),
		).toBeInTheDocument();
		expect(screen.queryByText("jimeng")).not.toBeInTheDocument();
	});

	it("clears a pending jimeng login so it can be retried", async () => {
		vi.mocked(beginProviderLogin).mockResolvedValue({
			...apiKeysResponse({}),
			login: {
				status: "pending",
				verificationUri: "https://jimeng.example.test/device",
				userCode: "JIM-ENG",
				deviceCode: "jimeng-device-code",
			},
		});
		vi.mocked(clearAPIKey).mockResolvedValue(apiKeysResponse({}));
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "登录" }));

		expect(await screen.findByRole("button", { name: "确认" })).toBeEnabled();
		fireEvent.click(screen.getByRole("button", { name: "即梦 更多操作" }));
		fireEvent.click(await screen.findByRole("menuitem", { name: "取消登录" }));
		expect(clearAPIKey).not.toHaveBeenCalled();
		fireEvent.click(await screen.findByRole("button", { name: "取消登录" }));

		await waitFor(() => expect(clearAPIKey).toHaveBeenCalledWith("jimeng"));
		await waitFor(() => expect(screen.queryByText("等待浏览器授权")).not.toBeInTheDocument());
		expect(await screen.findByRole("button", { name: "登录" })).toBeEnabled();
	});

	it("renders LibTV and Xiaoyunque CLI providers from platform data", async () => {
		vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({ includeExtraCLI: true }));
		vi.mocked(getModelPlatforms).mockResolvedValue(
			modelPlatformsResponse({ cliProviderIDs: ["libtv", "xiaoyunque"] }),
		);

		renderSettings();

		const cliSection = (await screen.findByRole("heading", { name: "会员 CLI 接入" })).closest(
			"section",
		);
		expect(cliSection).toBeTruthy();
		expect(within(cliSection as HTMLElement).getByText("LibTV")).toBeInTheDocument();
		expect(within(cliSection as HTMLElement).getByText("小云雀")).toBeInTheDocument();
		expect(
			within(cliSection as HTMLElement).getByRole("button", { name: "登录" }),
		).toBeInTheDocument();
		expect(
			screen.queryByText("已有小云雀 Access Key？可通过本地 Pippit CLI 接入。"),
		).not.toBeInTheDocument();
		expect(
			within(cliSection as HTMLElement).queryByLabelText("小云雀 API Key"),
		).not.toBeInTheDocument();
		fireEvent.click(within(cliSection as HTMLElement).getByRole("button", { name: "编辑 小云雀" }));
		const dialog = await screen.findByRole("dialog", { name: "配置 小云雀" });
		expect(within(dialog).getByLabelText("小云雀 API Key")).toBeInTheDocument();
		expect(within(cliSection as HTMLElement).queryByText("即梦")).not.toBeInTheDocument();
	});

	it("saves the Xiaoyunque key from a CLI config dialog", async () => {
		vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({ includeExtraCLI: true }));
		vi.mocked(getModelPlatforms).mockResolvedValue(
			modelPlatformsResponse({ cliProviderIDs: ["xiaoyunque"] }),
		);

		renderSettings();

		const cliSection = (await screen.findByRole("heading", { name: "会员 CLI 接入" })).closest(
			"section",
		);
		expect(cliSection).toBeTruthy();
		fireEvent.click(within(cliSection as HTMLElement).getByRole("button", { name: "编辑 小云雀" }));
		const dialog = await screen.findByRole("dialog", { name: "配置 小云雀" });
		fireEvent.change(within(dialog).getByLabelText("小云雀 API Key"), {
			target: { value: "xyq-access-key-123456" },
		});
		fireEvent.click(within(dialog).getByRole("button", { name: "保存" }));

		await waitFor(() =>
			expect(saveAPIKey).toHaveBeenCalledWith("xiaoyunque", "xyq-access-key-123456"),
		);
	});

	it("refreshes model-dependent caches after an immediate LibTV login", async () => {
		mockLibTVSettings();
		vi.mocked(beginProviderLogin).mockResolvedValue(
			libTVLoginResponse({ status: "completed" }, true),
		);
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "登录" }));

		await waitFor(() => expect(beginProviderLogin).toHaveBeenCalledWith("libtv", false));
		expectModelDependentCachesRevalidated();
	});

	it("refreshes model-dependent caches only after a pending LibTV login is confirmed", async () => {
		mockLibTVSettings();
		vi.mocked(beginProviderLogin).mockResolvedValue(
			libTVLoginResponse({
				status: "pending",
				verificationUri: "https://lib.tv/device",
				deviceCode: "libtv-device-code",
				userCode: "LIB-TV",
			}),
		);
		vi.mocked(completeProviderLogin).mockResolvedValue(
			libTVLoginResponse({ status: "completed" }, true),
		);
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "登录" }));

		expect(await screen.findByText("等待浏览器授权")).toBeInTheDocument();
		expect(swrMocks.mutate).not.toHaveBeenCalled();

		fireEvent.click(screen.getByRole("button", { name: "确认" }));

		await waitFor(() =>
			expect(completeProviderLogin).toHaveBeenCalledWith("libtv", "libtv-device-code"),
		);
		expectModelDependentCachesRevalidated();
	});

	it("refreshes model-dependent caches when polling observes LibTV become configured", async () => {
		let pollLogin: TimerHandler | undefined;
		let finishOpeningLoginPage: (() => void) | undefined;
		const setIntervalSpy = vi
			.spyOn(window, "setInterval")
			.mockImplementation((handler, timeout) => {
				if (timeout === 3000) pollLogin = handler;
				return 1 as unknown as ReturnType<typeof window.setInterval>;
			});
		vi.mocked(openExternalUrl).mockImplementation(
			() =>
				new Promise<void>((resolve) => {
					finishOpeningLoginPage = resolve;
				}),
		);
		mockLibTVSettings();
		vi.mocked(beginProviderLogin).mockResolvedValue(
			libTVLoginResponse({
				status: "pending",
				verificationUri: "https://lib.tv/device",
				userCode: "LIB-TV",
			}),
		);
		renderSettings();

		fireEvent.click(await screen.findByRole("button", { name: "登录" }));

		expect(await screen.findByText("等待浏览器授权")).toBeInTheDocument();
		expect(swrMocks.mutate).not.toHaveBeenCalled();
		await waitFor(() => expect(pollLogin).toBeTypeOf("function"));

		vi.mocked(getAPIKeys).mockResolvedValue(
			apiKeysResponse({ includeExtraCLI: true, libtvConfigured: true }),
		);
		await act(async () => {
			if (typeof pollLogin === "function") pollLogin();
		});

		await waitFor(() => expect(getAPIKeys).toHaveBeenCalledTimes(2));
		expect(screen.getByText("等待浏览器授权")).toBeInTheDocument();
		expect(swrMocks.mutate).not.toHaveBeenCalled();
		await act(async () => {
			finishOpeningLoginPage?.();
		});
		await waitFor(() => expect(screen.queryByText("等待浏览器授权")).not.toBeInTheDocument());
		expectModelDependentCachesRevalidated();
		setIntervalSpy.mockRestore();
	});
});

describe("Settings Codex skills page", () => {
	afterEach(() => {
		cleanup();
	});

	it.each(["codex", "opencode"])(
		"keeps the Codex skills tab valid when the active backend is %s",
		(activeBackendID) => {
			useSettingsNavigationStore.setState({ activeTab: "codex-skills" });

			render(
				<MemoryRouter>
					<SWRConfig
						value={{
							fallback: {
								[agentBackendsKey]: {
									activeId: activeBackendID,
									backends: [],
								},
							},
							provider: () => new Map(),
							revalidateOnMount: false,
						}}
					>
						<Settings />
					</SWRConfig>
				</MemoryRouter>,
			);

			expect(screen.getByRole("heading", { name: "Codex 全局技能" })).toBeInTheDocument();
		},
	);
});

const renderSettings = () =>
	render(
		<MemoryRouter>
			<SWRConfig value={{ provider: () => new Map() }}>
				<Settings />
				<ConfirmDialog />
			</SWRConfig>
		</MemoryRouter>,
	);

const apiKeysResponse = ({
	aihubmixConfigured = false,
	includeExtraCLI = false,
	libtvConfigured = false,
	mediagoConfigured = false,
	openrouterConfigured = false,
	videoapiConfigured = false,
}: {
	aihubmixConfigured?: boolean;
	includeExtraCLI?: boolean;
	libtvConfigured?: boolean;
	mediagoConfigured?: boolean;
	openrouterConfigured?: boolean;
	videoapiConfigured?: boolean;
}): APIKeyListResponse => ({
	providers: [
		{
			id: "aihubmix",
			label: "第三方",
			description: "OpenAI-compatible Agent gateway",
			configured: aihubmixConfigured,
			source: aihubmixConfigured ? "settings" : "none",
			masked: aihubmixConfigured ? "sk••••••456" : undefined,
			credentialKind: "apiKey",
			capabilities: ["text"],
		},
		{
			id: "mediago",
			label: "MediaGo聚合平台",
			description: "统一聚合平台",
			configured: mediagoConfigured,
			source: mediagoConfigured ? "settings" : "none",
			masked: mediagoConfigured ? "sk••••3456" : undefined,
			credentialKind: "apiKey",
			capabilities: ["text", "image", "video"],
		},
		{
			id: "openrouter",
			label: "OpenRouter",
			description: "自定义兼容接口",
			configured: openrouterConfigured,
			source: openrouterConfigured ? "settings" : "none",
			masked: openrouterConfigured ? "sk••••7890" : undefined,
			credentialKind: "apiKey",
			capabilities: ["text"],
		},
		{
			id: "jimeng",
			label: "即梦",
			description: "即梦 CLI 接入",
			configured: false,
			source: "none",
			credentialKind: "oauth",
			capabilities: ["image"],
		},
		...(includeExtraCLI
			? [
					{
						id: "libtv",
						label: "LibTV",
						description: "LibTV CLI 接入",
						configured: libtvConfigured,
						source: libtvConfigured ? ("settings" as const) : ("none" as const),
						credentialKind: "oauth",
						capabilities: ["image"],
					},
					{
						id: "xiaoyunque",
						label: "小云雀",
						description: "小云雀 CLI 接入",
						configured: false,
						source: "none" as const,
						credentialKind: "apiKey",
						credentialLabel: "小云雀 Access Key",
						help: "已有小云雀 Access Key？可通过本地 Pippit CLI 接入。",
						placeholder: "输入 XYQ_ACCESS_KEY",
						capabilities: ["image", "video"],
					},
				]
			: []),
		{
			id: "videoapi",
			label: "第三方 Video API",
			description: "第三方视频生成接口",
			configured: videoapiConfigured,
			source: videoapiConfigured ? "settings" : "none",
			masked: videoapiConfigured ? "sk••••video" : undefined,
			credentialKind: "apiKey",
			credentialLabel: "Video API Key",
			placeholder: "输入 Video API Key",
			capabilities: ["video"],
		},
		{
			id: "speechapi",
			label: "第三方 Speech API",
			description: "第三方文本转语音接口",
			configured: false,
			source: "none",
			credentialKind: "apiKey",
			credentialLabel: "Speech API Key",
			placeholder: "输入 Speech API Key",
			capabilities: ["audio"],
		},
		{
			id: "volcengine",
			label: "火山引擎",
			description: "官方供应商",
			configured: false,
			source: "none",
			credentialKind: "apiKey",
			capabilities: ["image"],
		},
	],
});

const modelPlatformsResponse = ({
	cliProviderIDs = ["jimeng"],
	mediagoModels = ["MiniMax M3", "GLM 4.7", "Qwen3.5"],
}: {
	cliProviderIDs?: string[];
	mediagoModels?: string[];
} = {}): ModelPlatformsResponse => ({
	platforms: [
		{
			id: "mediago",
			label: "MediaGo聚合平台",
			kind: "unified",
			description: "统一聚合平台",
			apiKeyProviderId: "mediago",
			modelGroups: [
				{
					label: "文本模型",
					models: mediagoModels,
				},
			],
		},
		{
			id: "openrouter",
			label: "OpenRouter",
			kind: "custom",
			description: "自定义兼容接口",
			apiKeyProviderId: "openrouter",
		},
		...cliProviderIDs.map((providerID) => ({
			id: providerID,
			label: cliPlatformLabel(providerID),
			kind: "cli",
			description: `${cliPlatformLabel(providerID)} CLI 接入`,
			apiKeyProviderId: providerID,
		})),
	],
});

const cliPlatformLabel = (providerID: string) => {
	switch (providerID) {
		case "libtv":
			return "LibTV";
		case "xiaoyunque":
			return "小云雀";
		default:
			return "即梦";
	}
};

const mockLibTVSettings = () => {
	vi.mocked(getAPIKeys).mockResolvedValue(apiKeysResponse({ includeExtraCLI: true }));
	vi.mocked(getModelPlatforms).mockResolvedValue(
		modelPlatformsResponse({ cliProviderIDs: ["libtv"] }),
	);
};

const libTVLoginResponse = (login: APIKeyLoginChallenge, configured = false) => ({
	...apiKeysResponse({ includeExtraCLI: true, libtvConfigured: configured }),
	login,
});

const expectModelDependentCachesRevalidated = () => {
	expect(swrMocks.mutate).toHaveBeenCalledTimes(3);
	expect(swrMocks.mutate).toHaveBeenCalledWith("/settings/unified-models", undefined, {
		revalidate: true,
	});
	expect(swrMocks.mutate).toHaveBeenCalledWith(generationModelsKey, undefined, {
		revalidate: true,
	});
	expect(swrMocks.mutate).toHaveBeenCalledWith(isAgentRuntimeConfigKey, undefined, {
		revalidate: true,
	});
};
