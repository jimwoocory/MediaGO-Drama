package settings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	core "github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
)

// UnifiedModel describes media output and the selected protocol, not vision input.
type UnifiedModel struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Protocol string `json:"protocol"`
	Source   string `json:"source"`
	Enabled  bool   `json:"enabled"`
	Reason   string `json:"reason,omitempty"`
}

// UnifiedModelsResponse keeps discovery failures visible without discarding saved models.
type UnifiedModelsResponse struct {
	Models  []UnifiedModel `json:"models"`
	Warning string         `json:"warning,omitempty"`
}

type unifiedSnapshot struct {
	Models      []UnifiedModel `json:"models"`
	Overrides   []UnifiedModel `json:"overrides"`
	LastAttempt time.Time      `json:"lastAttempt"`
	Warning     string         `json:"warning,omitempty"`
}

func (service *Settings) unifiedScope(ctx context.Context) (string, string, string, error) {
	if service == nil || service.apiKeys == nil {
		return "", "", "", nil
	}
	key, _, err := service.GetAPIKey(ctx, core.ProviderUnified)
	if err != nil {
		return "", "", "", err
	}
	base := service.AIHubMixBaseURL()
	hash := sha256.Sum256([]byte(base + "\n" + key))
	return "generation.unified.models." + hex.EncodeToString(hash[:]), base, key, nil
}

func (service *Settings) readUnifiedSnapshot(key string) (unifiedSnapshot, bool, error) {
	var snapshot unifiedSnapshot
	if service.appSettings == nil {
		return snapshot, false, nil
	}
	raw, found, err := service.appSettings.GetAppSetting(key)
	if err != nil || !found {
		return snapshot, found, err
	}
	err = json.Unmarshal([]byte(raw), &snapshot)
	return snapshot, found, err
}

func (service *Settings) writeUnifiedSnapshot(key string, snapshot unifiedSnapshot) error {
	if service.appSettings == nil {
		return ErrAppSettingStoreMissing
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return service.appSettings.SetAppSetting(key, string(raw))
}

// ListUnifiedModels discovers once per endpoint/credential and supports explicit refresh.
func (service *Settings) ListUnifiedModels(ctx context.Context, refresh bool) (UnifiedModelsResponse, error) {
	if service == nil {
		return UnifiedModelsResponse{Models: []UnifiedModel{}}, nil
	}
	service.unifiedModelsMu.Lock()
	defer service.unifiedModelsMu.Unlock()
	return service.listUnifiedModels(ctx, refresh)
}

func (service *Settings) listUnifiedModels(ctx context.Context, refresh bool) (UnifiedModelsResponse, error) {
	result := UnifiedModelsResponse{Models: []UnifiedModel{}}
	scope, base, key, err := service.unifiedScope(ctx)
	if err != nil {
		return result, err
	}
	if base == "" || strings.TrimSpace(key) == "" {
		return result, nil
	}
	snapshot, found, err := service.readUnifiedSnapshot(scope)
	if err != nil {
		return result, err
	}
	if refresh || !found || snapshot.LastAttempt.IsZero() || (snapshot.Warning != "" && time.Since(snapshot.LastAttempt) > time.Minute) {
		models, fetchErr := fetchOpenAICompatibleModels(ctx, base, key)
		snapshot.LastAttempt = time.Now()
		if fetchErr != nil {
			snapshot.Warning = "读取统一接口模型失败，保留已保存的映射；可刷新重试或手动添加模型。"
		} else {
			snapshot.Warning = ""
			snapshot.Models = []UnifiedModel{}
			for _, model := range models {
				snapshot.Models = append(snapshot.Models, classifyUnifiedModelAtEndpoint(model, base)...)
			}
		}
		if err := service.writeUnifiedSnapshot(scope, snapshot); err != nil {
			return result, err
		}
	}
	result.Warning = snapshot.Warning
	byKey := map[string]UnifiedModel{}
	for _, model := range snapshot.Models {
		byKey[model.ID+"\n"+model.Kind] = model
	}
	for _, model := range snapshot.Overrides {
		delete(byKey, model.ID+"\nunknown")
		byKey[model.ID+"\n"+model.Kind] = model
	}
	for _, model := range byKey {
		result.Models = append(result.Models, model)
	}
	sort.Slice(result.Models, func(i, j int) bool {
		return result.Models[i].ID+result.Models[i].Kind < result.Models[j].ID+result.Models[j].Kind
	})
	return result, nil
}

// SetUnifiedModel saves a protocol override using the already configured credential.
func (service *Settings) SetUnifiedModel(ctx context.Context, model UnifiedModel) (UnifiedModelsResponse, error) {
	service.unifiedModelsMu.Lock()
	defer service.unifiedModelsMu.Unlock()
	route, valid := core.UnifiedRoute(model.ID, model.Protocol)
	if !valid {
		return UnifiedModelsResponse{}, fmt.Errorf("%w: 模型 ID 或生成协议无效", ErrAgentModelInvalid)
	}
	scope, base, key, err := service.unifiedScope(ctx)
	if err != nil {
		return UnifiedModelsResponse{}, err
	}
	if base == "" || strings.TrimSpace(key) == "" {
		return UnifiedModelsResponse{}, fmt.Errorf("%w: 请先配置统一接口", ErrAgentModelInvalid)
	}
	snapshot, _, err := service.readUnifiedSnapshot(scope)
	if err != nil {
		return UnifiedModelsResponse{}, err
	}
	model.ID, model.Kind, model.Source, model.Reason = route.Model, string(route.Kind), "manual", ""
	next := []UnifiedModel{}
	for _, item := range snapshot.Overrides {
		if item.ID != model.ID || item.Kind != model.Kind {
			next = append(next, item)
		}
	}
	snapshot.Overrides = append(next, model)
	if err := service.writeUnifiedSnapshot(scope, snapshot); err != nil {
		return UnifiedModelsResponse{}, err
	}
	return service.listUnifiedModels(ctx, false)
}

func classifyUnifiedModel(item openAIModelListItem) []UnifiedModel {
	id := strings.TrimSpace(item.ID)
	if id == "" {
		return nil
	}
	var meta struct {
		Output       []string `json:"output_modalities"`
		Endpoints    []string `json:"supported_endpoint_types"`
		Architecture struct {
			Output []string `json:"output_modalities"`
		} `json:"architecture"`
	}
	_ = json.Unmarshal(item.Metadata, &meta)
	outputs := append(meta.Output, meta.Architecture.Output...)
	name := strings.ToLower(id)
	protocols := map[string]string{}
	for _, endpoint := range meta.Endpoints {
		switch strings.TrimPrefix(strings.TrimLeft(strings.ToLower(endpoint), "/"), "v1/") {
		case "images/generations", "image-generation":
			protocols["images"] = "metadata"
		case "audio/speech", "text-to-speech":
			protocols["speech"] = "metadata"
		case "videos", "video-generation":
			protocols["videos"] = "metadata"
		case "chat/completions":
			if stringSliceContainsFold(outputs, "image") {
				protocols["chat-image"] = "metadata"
			}
		}
	}
	if len(protocols) == 0 {
		switch {
		case strings.Contains(name, "gemini") && strings.Contains(name, "image"), strings.Contains(name, "nano-banana"):
			protocols["chat-image"] = "inferred"
		case strings.Contains(name, "gpt-image"), strings.Contains(name, "dall-e"), strings.Contains(name, "seedream"), stringSliceContainsFold(outputs, "image"):
			protocols["images"] = "inferred"
		}
		if strings.HasPrefix(name, "tts-") || strings.Contains(name, "-tts") || strings.HasPrefix(name, "speech-") || stringSliceContainsFold(outputs, "audio") {
			if !strings.Contains(name, "realtime") && !strings.Contains(name, "transcri") {
				protocols["speech"] = "inferred"
			}
		}
		if strings.Contains(name, "sora") || strings.Contains(name, "veo") || strings.Contains(name, "seedance") || stringSliceContainsFold(outputs, "video") {
			protocols["videos"] = "inferred"
		}
	}
	// One deterministic wire protocol per output kind; manual overrides can change it.
	if _, exists := protocols["images"]; exists {
		delete(protocols, "chat-image")
	}
	models := []UnifiedModel{}
	for protocol, source := range protocols {
		route, ok := core.UnifiedRoute(id, protocol)
		if !ok {
			continue
		}
		reason := ""
		if source == "inferred" {
			reason = "按模型名称或输出类型匹配；若供应商协议不同，请修改协议。"
		}
		models = append(models, UnifiedModel{ID: id, Kind: string(route.Kind), Protocol: protocol, Source: source, Enabled: true, Reason: reason})
	}
	if len(models) == 0 {
		models = append(models, UnifiedModel{ID: id, Kind: "unknown", Source: "unknown", Reason: "未识别到生成能力；文本模型仍由 Agent 管理，生图等模型可手动指定协议。"})
	}
	return models
}

func classifyUnifiedModelAtEndpoint(item openAIModelListItem, base string) []UnifiedModel {
	models := classifyUnifiedModel(item)
	endpoint, err := url.Parse(base)
	if err == nil && strings.EqualFold(endpoint.Hostname(), "openrouter.ai") {
		// Like the existing OpenRouter adapter, use Chat Completions for image output.
		for i := range models {
			if models[i].Kind == "image" && models[i].Source == "inferred" {
				models[i].Protocol = "chat-image"
			}
		}
	}
	return models
}

func openAIModelHasOnlyMediaOutput(item openAIModelListItem) bool {
	var meta struct {
		Output       []string `json:"output_modalities"`
		Architecture struct {
			Output []string `json:"output_modalities"`
		} `json:"architecture"`
	}
	if json.Unmarshal(item.Metadata, &meta) != nil {
		return false
	}
	outputs := append(meta.Output, meta.Architecture.Output...)
	return len(outputs) > 0 && !stringSliceContainsFold(outputs, "text") && (stringSliceContainsFold(outputs, "image") || stringSliceContainsFold(outputs, "audio") || stringSliceContainsFold(outputs, "video"))
}
