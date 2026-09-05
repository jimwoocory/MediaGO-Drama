package generation

// ProviderCodexImage is the native ChatGPT-subscription image tool, not an API model.
const ProviderCodexImage = "codex-image"

// CodexImageRoute is advertised only after the server confirms a managed login
// and imageGeneration capability. No API key is required by this route.
func CodexImageRoute() ModelRoute {
	return ModelRoute{ID: "codex.image-generation", LegacyModelID: "codex.image-generation",
		FamilyID: "codex-image", VersionID: "codex.image-generation", Label: "Codex 生图（ChatGPT 订阅）",
		Kind: KindImage, Provider: ProviderCodexImage, Model: "image_generation", Adapter: "codex.image-generation",
		Status: RouteStatusAvailable, Params: []ParamSpec{}}
}
