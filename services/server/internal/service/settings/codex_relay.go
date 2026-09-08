package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mediago-dev/mediago-drama/services/server/internal/service/shared"
)

const (
	codexRelaySettingsKey       = "codex.relay.settings.v1"
	codexRelayAPIKeyPrefix      = "codex-relay:"
	codexRelayAPIKeySuffix      = ":api-key"
	codexRelayProviderID        = "mediago-codex-relay"
	codexRelayLocalBearerToken  = "mediago-codex-relay"
	codexRelayLocalTokenEnv     = "MEDIAGO_CODEX_RELAY_TOKEN"
	unifiedCodexRelayProfileID  = "unified-openai-compatible"
	unifiedCodexRelayModel      = "gpt-5"
	// Agent turns with high reasoning can legitimately need more than a minute
	// before the upstream sends its first response. A 60-second deadline turns
	// that normal wait into a local 502 and makes the ACP client reconnect in a
	// loop. The request context still allows cancellation when the user stops
	// the run; this limit only prevents an unattended request from waiting
	// forever.
	codexRelayDefaultHTTPClient = 5 * time.Minute
	codexRelayCheckHTTPClient   = 10 * time.Second
	codexRelayCheckBodyLimit    = 1024 * 1024
)

// CodexRelayProtocol identifies the upstream protocol used by one relay profile.
type CodexRelayProtocol string

const (
	// CodexRelayProtocolAuto uses the last detected compatible protocol, falling back to Chat Completions before detection.
	CodexRelayProtocolAuto CodexRelayProtocol = "auto"
	// CodexRelayProtocolResponses sends Codex Responses API payloads to a compatible upstream.
	CodexRelayProtocolResponses CodexRelayProtocol = "responses"
	// CodexRelayProtocolChatCompletions converts Codex Responses requests to Chat Completions.
	CodexRelayProtocolChatCompletions CodexRelayProtocol = "chatCompletions"
)

// CodexRelayAPIKeyStatus describes the redacted credential state for one relay profile.
type CodexRelayAPIKeyStatus struct {
	Configured bool   `json:"configured"`
	Source     string `json:"source"`
	Masked     string `json:"masked,omitempty"`
}

// CodexRelayProfile describes one Codex relay target.
type CodexRelayProfile struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	BaseURL          string                 `json:"baseURL"`
	Model            string                 `json:"model"`
	Protocol         CodexRelayProtocol     `json:"protocol"`
	DetectedProtocol CodexRelayProtocol     `json:"detectedProtocol,omitempty"`
	Enabled          bool                   `json:"enabled"`
	APIKey           CodexRelayAPIKeyStatus `json:"apiKey"`
}

// CodexRelaySettingsResponse is returned by the Codex relay settings API.
type CodexRelaySettingsResponse struct {
	Enabled         bool                `json:"enabled"`
	ActiveProfileID string              `json:"activeProfileId,omitempty"`
	Profiles        []CodexRelayProfile `json:"profiles"`
}

// CodexRelayCheckRequest chooses which relay profile to probe.
type CodexRelayCheckRequest struct {
	ProfileID string `json:"profileId"`
}

// CodexRelayCheckResponse describes an upstream reachability check for a relay profile.
type CodexRelayCheckResponse struct {
	OK                       bool               `json:"ok"`
	ProfileID                string             `json:"profileId"`
	BaseURL                  string             `json:"baseURL"`
	StatusCode               int                `json:"statusCode"`
	Models                   []string           `json:"models"`
	ResponsesSupported       bool               `json:"responsesSupported"`
	ChatCompletionsSupported bool               `json:"chatCompletionsSupported"`
	RecommendedProtocol      CodexRelayProtocol `json:"recommendedProtocol,omitempty"`
}

type codexRelayModelsPayload struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// CodexRelayProfileMutation stores non-secret profile fields.
type CodexRelayProfileMutation struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	BaseURL          string             `json:"baseURL"`
	Model            string             `json:"model"`
	Protocol         CodexRelayProtocol `json:"protocol"`
	DetectedProtocol CodexRelayProtocol `json:"detectedProtocol,omitempty"`
	Enabled          bool               `json:"enabled"`
}

// CodexRelaySettingsMutation replaces the non-secret relay settings.
type CodexRelaySettingsMutation struct {
	Enabled         bool                        `json:"enabled"`
	ActiveProfileID string                      `json:"activeProfileId"`
	Profiles        []CodexRelayProfileMutation `json:"profiles"`
}

// CodexRelayRuntimeConfig describes the process config generated for Codex ACP.
type CodexRelayRuntimeConfig struct {
	ConfigDir  string
	CodexHome  string
	Env        map[string]string
	Configured bool
}

// CodexRuntimeHomeDescriptor describes an optional isolated Codex home without preparing it.
type CodexRuntimeHomeDescriptor struct {
	CodexHome string
	Isolated  bool
}

type codexRelayStoredSettings struct {
	Enabled         bool                        `json:"enabled"`
	ActiveProfileID string                      `json:"activeProfileId,omitempty"`
	Profiles        []CodexRelayProfileMutation `json:"profiles,omitempty"`
}

// GetCodexRelaySettings returns the saved Codex relay settings with redacted key status.
func (service *Settings) GetCodexRelaySettings(ctx context.Context) (CodexRelaySettingsResponse, error) {
	_ = ctx
	stored, err := service.loadCodexRelayStoredSettings()
	if err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	return service.codexRelaySettingsResponse(stored)
}

// SaveCodexRelaySettings stores non-secret Codex relay settings.
func (service *Settings) SaveCodexRelaySettings(ctx context.Context, input CodexRelaySettingsMutation) (CodexRelaySettingsResponse, error) {
	_ = ctx
	if service.appSettings == nil {
		return CodexRelaySettingsResponse{}, ErrAppSettingStoreMissing
	}
	stored, err := normalizeCodexRelaySettings(input)
	if err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	previous, err := service.loadCodexRelayStoredSettings()
	if err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	if len(stored.Profiles) == 0 {
		if err := service.clearRemovedCodexRelayAPIKeys(previous, stored); err != nil {
			return CodexRelaySettingsResponse{}, err
		}
		if err := service.appSettings.ClearAppSetting(codexRelaySettingsKey); err != nil {
			return CodexRelaySettingsResponse{}, err
		}
		return service.codexRelaySettingsResponse(codexRelayStoredSettings{})
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return CodexRelaySettingsResponse{}, fmt.Errorf("encoding codex relay settings: %w", err)
	}
	if err := service.appSettings.SetAppSetting(codexRelaySettingsKey, string(raw)); err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	if err := service.clearRemovedCodexRelayAPIKeys(previous, stored); err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	return service.codexRelaySettingsResponse(stored)
}

// SetCodexRelayProfileAPIKey stores a Codex relay profile API key.
func (service *Settings) SetCodexRelayProfileAPIKey(ctx context.Context, profileID string, apiKey string) (CodexRelaySettingsResponse, error) {
	if service.apiKeys == nil {
		return CodexRelaySettingsResponse{}, ErrAPIKeyProviderNotFound
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return CodexRelaySettingsResponse{}, fmt.Errorf("%w: profile id is required", ErrCodexRelayInvalid)
	}
	if strings.TrimSpace(apiKey) == "" {
		return CodexRelaySettingsResponse{}, ErrAPIKeyRequired
	}
	stored, err := service.loadCodexRelayStoredSettings()
	if err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	if !codexRelayStoredProfileExists(stored, profileID) {
		return CodexRelaySettingsResponse{}, ErrCodexRelayNotConfigured
	}
	if err := service.apiKeys.Set(CodexRelayAPIKeyName(profileID), apiKey); err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	return service.GetCodexRelaySettings(ctx)
}

// ClearCodexRelayProfileAPIKey removes a Codex relay profile API key.
func (service *Settings) ClearCodexRelayProfileAPIKey(ctx context.Context, profileID string) (CodexRelaySettingsResponse, error) {
	if service.apiKeys == nil {
		return CodexRelaySettingsResponse{}, ErrAPIKeyProviderNotFound
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return CodexRelaySettingsResponse{}, fmt.Errorf("%w: profile id is required", ErrCodexRelayInvalid)
	}
	if err := service.apiKeys.Clear(CodexRelayAPIKeyName(profileID)); err != nil {
		return CodexRelaySettingsResponse{}, err
	}
	return service.GetCodexRelaySettings(ctx)
}

// CheckCodexRelay verifies a Codex relay profile can authenticate against its upstream.
func (service *Settings) CheckCodexRelay(ctx context.Context, input CodexRelayCheckRequest) (CodexRelayCheckResponse, error) {
	active, apiKey, err := service.codexRelayProfileWithKey(input.ProfileID, true)
	if errors.Is(err, ErrCodexRelayNotConfigured) && strings.TrimSpace(input.ProfileID) == "" {
		active, apiKey, err = service.activeCodexRelayProfileWithUnifiedFallback(ctx, true)
	}
	if err != nil {
		return CodexRelayCheckResponse{}, err
	}
	result := CodexRelayCheckResponse{
		ProfileID: active.ID,
		BaseURL:   active.BaseURL,
		Models:    []string{},
	}

	modelsURL, err := codexRelayUpstreamURL(active.BaseURL, "/v1/models")
	if err != nil {
		return result, err
	}
	modelsRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return result, fmt.Errorf("creating codex relay check request: %w", err)
	}
	modelsRequest.Header.Set("Accept", "application/json")
	modelsRequest.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: codexRelayCheckHTTPClient}
	modelsResponse, err := client.Do(modelsRequest)
	if err != nil {
		return result, fmt.Errorf("%w：连接上游失败，请检查 Base URL", ErrCodexRelayCheckFailed)
	}
	modelsBody := readLimitedCodexRelayCheckBody(modelsResponse.Body)
	modelsResponse.Body.Close()
	result.StatusCode = modelsResponse.StatusCode
	if codexRelayCheckStatusAuthFailed(modelsResponse.StatusCode) || codexRelayBodyLooksInvalidAPIKey(modelsBody) {
		return result, fmt.Errorf("%w：上游返回 %d，请检查 API Key 和 Base URL", ErrCodexRelayCheckFailed, modelsResponse.StatusCode)
	}
	if modelsResponse.StatusCode >= http.StatusOK && modelsResponse.StatusCode < http.StatusMultipleChoices {
		if !json.Valid([]byte(modelsBody)) {
			return result, fmt.Errorf("%w：模型接口返回的不是 JSON，请检查 Base URL 是否指向 /v1 API，而不是供应商网页", ErrCodexRelayCheckFailed)
		}
		result.Models = codexRelayModelIDs(modelsBody)
	}

	responsesSupported, _, responsesErr := probeCodexRelayProtocol(ctx, client, active, apiKey, CodexRelayProtocolResponses)
	chatSupported, _, chatErr := probeCodexRelayProtocol(ctx, client, active, apiKey, CodexRelayProtocolChatCompletions)
	result.ResponsesSupported = responsesSupported
	result.ChatCompletionsSupported = chatSupported
	switch {
	case responsesSupported:
		result.RecommendedProtocol = CodexRelayProtocolResponses
	case chatSupported:
		result.RecommendedProtocol = CodexRelayProtocolChatCompletions
	}

	selected := effectiveCodexRelayProtocol(active)
	if active.Protocol == CodexRelayProtocolAuto && result.RecommendedProtocol != "" {
		if err := service.persistDetectedCodexRelayProtocol(active.ID, result.RecommendedProtocol); err != nil {
			return result, err
		}
		selected = result.RecommendedProtocol
	}
	if selected == CodexRelayProtocolResponses && !responsesSupported {
		if chatSupported {
			return result, fmt.Errorf("%w：Responses 不可用；检测到 Chat Completions 可用，建议切换为 Auto 或 Chat Completions", ErrCodexRelayCheckFailed)
		}
		return result, fmt.Errorf("%w：Responses 不可用（%v）", ErrCodexRelayCheckFailed, responsesErr)
	}
	if selected == CodexRelayProtocolChatCompletions && !chatSupported {
		if responsesSupported {
			return result, fmt.Errorf("%w：Chat Completions 不可用；检测到 Responses 可用，建议切换为 Auto 或 Responses", ErrCodexRelayCheckFailed)
		}
		return result, fmt.Errorf("%w：Chat Completions 不可用（%v）", ErrCodexRelayCheckFailed, chatErr)
	}
	if !responsesSupported && !chatSupported {
		return result, fmt.Errorf("%w：未检测到兼容的 Responses 或 Chat Completions 端点", ErrCodexRelayCheckFailed)
	}
	result.OK = true
	return result, nil
}

func probeCodexRelayProtocol(ctx context.Context, client *http.Client, profile CodexRelayProfileMutation, apiKey string, protocol CodexRelayProtocol) (bool, int, error) {
	var (
		endpoint string
		payload  []byte
		err      error
	)
	switch protocol {
	case CodexRelayProtocolResponses:
		endpoint, err = codexRelayUpstreamURL(profile.BaseURL, "/v1/responses")
		payload, _ = json.Marshal(map[string]any{
			"model":             profile.Model,
			"input":             "ping",
			"max_output_tokens": 1,
			"stream":            false,
		})
	case CodexRelayProtocolChatCompletions:
		endpoint, err = codexRelayChatCompletionsUpstreamURL(profile.BaseURL)
		payload, _ = json.Marshal(map[string]any{
			"model":      profile.Model,
			"messages":   []map[string]string{{"role": "user", "content": "ping"}},
			"max_tokens": 1,
			"stream":     false,
		})
	default:
		return false, 0, fmt.Errorf("unsupported probe protocol %q", protocol)
	}
	if err != nil {
		return false, 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return false, 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)
	response, err := client.Do(request)
	if err != nil {
		return false, 0, err
	}
	defer response.Body.Close()
	body := readLimitedCodexRelayCheckBody(response.Body)
	if codexRelayCheckStatusAuthFailed(response.StatusCode) || codexRelayBodyLooksInvalidAPIKey(body) {
		return false, response.StatusCode, fmt.Errorf("authentication failed")
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		if !json.Valid([]byte(body)) {
			return false, response.StatusCode, fmt.Errorf("upstream returned non-JSON data; check that Base URL points to the API endpoint")
		}
		return true, response.StatusCode, nil
	}
	return false, response.StatusCode, fmt.Errorf("upstream returned %d", response.StatusCode)
}

func (service *Settings) persistDetectedCodexRelayProtocol(profileID string, protocol CodexRelayProtocol) error {
	if protocol != CodexRelayProtocolResponses && protocol != CodexRelayProtocolChatCompletions {
		return nil
	}
	stored, err := service.loadCodexRelayStoredSettings()
	if err != nil {
		return err
	}
	changed := false
	for index := range stored.Profiles {
		if stored.Profiles[index].ID != profileID {
			continue
		}
		if stored.Profiles[index].DetectedProtocol != protocol {
			stored.Profiles[index].DetectedProtocol = protocol
			changed = true
		}
		break
	}
	if !changed {
		return nil
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encoding codex relay settings: %w", err)
	}
	return service.appSettings.SetAppSetting(codexRelaySettingsKey, string(raw))
}

// PrepareCodexRelayRuntimeConfig writes a Codex home configured for the active relay profile.
func (service *Settings) PrepareCodexRelayRuntimeConfig(ctx context.Context, workspaceDir string, relayBaseURL string) (CodexRelayRuntimeConfig, error) {
	_ = ctx
	active, _, err := service.activeCodexRelayProfileWithUnifiedFallback(ctx, true)
	if err != nil {
		if err == ErrCodexRelayNotConfigured {
			return CodexRelayRuntimeConfig{}, nil
		}
		return CodexRelayRuntimeConfig{}, err
	}
	relayBaseURL = strings.TrimRight(strings.TrimSpace(relayBaseURL), "/")
	if relayBaseURL == "" {
		return CodexRelayRuntimeConfig{}, fmt.Errorf("%w: local relay url is empty", ErrCodexRelayInvalid)
	}

	codexHome := filepath.Join(shared.WorkspacePathsFor(workspaceDir).GlobalMetadataDir(), "runtime", "agents", "codex", "home")
	return prepareCodexProfileRuntime(active, codexHome, relayBaseURL)
}

var codexRuntimeFilesMu sync.Mutex

func prepareCodexProfileRuntime(active CodexRelayProfileMutation, codexHome, relayBaseURL string, contextWindow ...int) (CodexRelayRuntimeConfig, error) {
	options := codexProfileRuntimeOptions{}
	if len(contextWindow) > 0 {
		options.ContextWindow = contextWindow[0]
	}
	return prepareCodexProfileRuntimeWithOptions(active, codexHome, relayBaseURL, options)
}

type codexProfileRuntimeOptions struct {
	ContextWindow int
	Reasoning     bool
}

func prepareCodexProfileRuntimeWithOptions(active CodexRelayProfileMutation, codexHome, relayBaseURL string, options codexProfileRuntimeOptions) (CodexRelayRuntimeConfig, error) {
	codexRuntimeFilesMu.Lock()
	defer codexRuntimeFilesMu.Unlock()
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return CodexRelayRuntimeConfig{}, fmt.Errorf("creating codex relay home: %w", err)
	}
	configText := renderCodexRelayConfig(active, relayBaseURL)
	if options.ContextWindow > 0 {
		window := options.ContextWindow
		catalogPath, err := writeAgentProviderModelCatalogWithReasoning(codexHome, active.Model, window, options.Reasoning)
		if err != nil {
			return CodexRelayRuntimeConfig{}, err
		}
		configText = fmt.Sprintf("model_catalog_json = %q\nmodel_context_window = %d\nmodel_auto_compact_token_limit = %d\n", catalogPath, window, agentModelCompactLimit(window)) + configText
		// These are provider-table keys: bound native retry loops for gateways.
		configText += "request_max_retries = 2\nstream_max_retries = 2\n"
	}
	if err := writeCodexRuntimeFile(filepath.Join(codexHome, "config.toml"), []byte(configText)); err != nil {
		return CodexRelayRuntimeConfig{}, fmt.Errorf("writing codex relay config: %w", err)
	}
	// Keep auth.json for older bundled Codex builds while the custom provider
	// uses env_key. The token only authenticates the loopback bridge; the real
	// upstream key stays in the settings store and is never written here.
	authText := `{"OPENAI_API_KEY":"` + codexRelayLocalBearerToken + `"}` + "\n"
	if err := writeCodexRuntimeFile(filepath.Join(codexHome, "auth.json"), []byte(authText)); err != nil {
		return CodexRelayRuntimeConfig{}, fmt.Errorf("writing codex relay auth: %w", err)
	}
	return CodexRelayRuntimeConfig{
		ConfigDir:  codexHome,
		CodexHome:  codexHome,
		Configured: true,
		Env: map[string]string{
			"CODEX_HOME":            codexHome,
			"OPENAI_API_KEY":        codexRelayLocalBearerToken,
			codexRelayLocalTokenEnv: codexRelayLocalBearerToken,
		},
	}, nil
}

func writeCodexRuntimeFile(path string, content []byte) error {
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, content) {
		return nil
	}
	return os.WriteFile(path, content, 0o600)
}

// DescribeCodexRuntimeHome returns the isolated Codex home used by an active relay without writing files.
func (service *Settings) DescribeCodexRuntimeHome(ctx context.Context, workspaceDir string) (CodexRuntimeHomeDescriptor, error) {
	_ = ctx
	if _, _, err := service.activeCodexRelayProfileWithUnifiedFallback(ctx, false); err != nil {
		if errors.Is(err, ErrCodexRelayNotConfigured) {
			return CodexRuntimeHomeDescriptor{}, nil
		}
		return CodexRuntimeHomeDescriptor{}, err
	}
	return CodexRuntimeHomeDescriptor{
		CodexHome: filepath.Join(shared.WorkspacePathsFor(workspaceDir).GlobalMetadataDir(), "runtime", "agents", "codex", "home"),
		Isolated:  true,
	}, nil
}

// OpenCodexRelayRequest opens an upstream request for the active Codex relay profile.
func (service *Settings) OpenCodexRelayRequest(ctx context.Context, method string, relayPath string, body []byte, headers http.Header) (*http.Response, error) {
	if !validCodexRelayLocalAuthorization(headers) {
		return nil, ErrCodexRelayUnauthorized
	}
	var active CodexRelayProfileMutation
	var apiKey string
	var err error
	if strings.HasPrefix(relayPath, "/providers/") {
		provider, path, found := strings.Cut(strings.TrimPrefix(relayPath, "/providers/"), "/")
		if !found || provider == "" {
			return nil, ErrCodexRelayInvalid
		}
		relayPath = "/" + path
		active, apiKey, err = service.agentProviderProfile(ctx, provider)
	} else {
		active, apiKey, err = service.activeCodexRelayProfileWithUnifiedFallback(ctx, false)
	}
	if err != nil {
		return nil, err
	}
	upstreamPath := relayPath
	upstreamBody := body
	responseStream := false
	var responseTools []json.RawMessage
	convertChatResponse := false
	if effectiveCodexRelayProtocol(active) == CodexRelayProtocolChatCompletions {
		cleanPath := strings.SplitN(relayPath, "?", 2)[0]
		switch cleanPath {
		case "/v1/models", "/models":
			// Model discovery is protocol-independent.
		case "/v1/responses", "/responses":
			if method != http.MethodPost {
				return nil, fmt.Errorf("%w: chat completions adapter only supports POST /responses", ErrCodexRelayInvalid)
			}
			converted, stream, convertErr := codexRelayResponsesToChatRequest(body, active.Model)
			if convertErr != nil {
				return nil, convertErr
			}
			upstreamBody = converted
			var original codexResponsesRequest
			if err := json.Unmarshal(body, &original); err != nil {
				return nil, err
			}
			responseTools = original.Tools
			responseStream = stream
			upstreamPath = "/v1/chat/completions"
			convertChatResponse = true
		default:
			return nil, fmt.Errorf("%w: chat completions adapter does not support relay path %q", ErrCodexRelayInvalid, cleanPath)
		}
	}
	var upstreamURL string
	if convertChatResponse {
		upstreamURL, err = codexRelayChatCompletionsUpstreamURL(active.BaseURL)
	} else {
		upstreamURL, err = codexRelayUpstreamURL(active.BaseURL, upstreamPath)
	}
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, upstreamURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, fmt.Errorf("creating codex relay request: %w", err)
	}
	copyCodexRelayRequestHeaders(request.Header, headers)
	request.Header.Set("Authorization", "Bearer "+apiKey)
	if request.Header.Get("Content-Type") == "" && len(upstreamBody) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: codexRelayDefaultHTTPClient}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("requesting codex relay upstream: %w", err)
	}
	if convertChatResponse {
		return codexRelayChatResponseToResponses(response, responseStream, responseTools)
	}
	return response, nil
}

// CodexRelayAPIKeyName returns the settings key used for one relay profile secret.
func CodexRelayAPIKeyName(profileID string) string {
	return codexRelayAPIKeyPrefix + strings.TrimSpace(profileID) + codexRelayAPIKeySuffix
}

func (service *Settings) activeCodexRelayProfile() (CodexRelayProfileMutation, string, error) {
	return service.codexRelayProfileWithKey("", false)
}

// activeCodexRelayProfileWithUnifiedFallback keeps the dedicated Codex relay
// authoritative, but lets the API Keys page's OpenAI-compatible gateway power
// Codex when no separate relay profile exists. This makes the single unified
// credential usable by the Codex Harness without copying the upstream secret
// into CODEX_HOME.
func (service *Settings) activeCodexRelayProfileWithUnifiedFallback(
	ctx context.Context,
	discoverModel bool,
) (CodexRelayProfileMutation, string, error) {
	profile, apiKey, err := service.activeCodexRelayProfile()
	if err == nil || !errors.Is(err, ErrCodexRelayNotConfigured) {
		return profile, apiKey, err
	}
	return service.unifiedCodexRelayProfile(ctx, discoverModel)
}

func (service *Settings) unifiedCodexRelayProfile(
	ctx context.Context,
	discoverModel bool,
) (CodexRelayProfileMutation, string, error) {
	if service == nil || service.apiKeys == nil {
		return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
	}
	apiKey, _, err := service.apiKeys.Get(agentModelProviderAIHubMix)
	if err != nil {
		return CodexRelayProfileMutation{}, "", err
	}
	apiKey = strings.TrimSpace(apiKey)
	baseURL := strings.TrimRight(strings.TrimSpace(service.AIHubMixBaseURL()), "/")
	if apiKey == "" || baseURL == "" || !validHTTPURL(baseURL) {
		return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
	}

	model := unifiedCodexRelayModel
	if discoverModel {
		models, modelErr := fetchOpenAICompatibleModels(ctx, baseURL, apiKey)
		if modelErr != nil {
			return CodexRelayProfileMutation{}, "", fmt.Errorf("%w：无法读取第三方模型列表：%v", ErrCodexRelayCheckFailed, modelErr)
		}
		discovered := preferredUnifiedCodexModel(models)
		if discovered == "" {
			return CodexRelayProfileMutation{}, "", fmt.Errorf("%w：第三方模型列表中没有可用于 Agent 的文本模型", ErrCodexRelayCheckFailed)
		}
		model = discovered
	}
	return CodexRelayProfileMutation{
		ID:       unifiedCodexRelayProfileID,
		Name:     "统一接口（第三方）",
		BaseURL:  baseURL,
		Model:    model,
		Protocol: CodexRelayProtocolAuto,
		Enabled:  true,
	}, apiKey, nil
}

func preferredUnifiedCodexModel(models []openAIModelListItem) string {
	bestModel := ""
	bestScore := -1
	for _, item := range models {
		model := strings.TrimSpace(item.ID)
		if model == "" || mediagoGatewayModelLooksTaskOnly(mediagoGatewayModel{ID: model}) {
			continue
		}
		score := unifiedCodexModelScore(model)
		if score > bestScore {
			bestModel = model
			bestScore = score
		}
	}
	return bestModel
}

func unifiedCodexModelScore(model string) int {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(normalized, "codex"):
		return 600
	case strings.Contains(normalized, "gpt-5"):
		return 550
	case normalized == "deepseek-v3.2":
		// Widely supported OpenAI-compatible tool-calling model. Prefer the
		// stable exact route over catalog-only preview/future aliases.
		return 540
	case strings.Contains(normalized, "claude"):
		return 500
	case strings.Contains(normalized, "deepseek"), strings.Contains(normalized, "qwen"):
		return 450
	case strings.Contains(normalized, "glm"), strings.Contains(normalized, "kimi"):
		return 400
	case strings.Contains(normalized, "gpt-4"):
		return 350
	default:
		return 100
	}
}

func (service *Settings) codexRelayProfileWithKey(profileID string, allowGlobalDisabled bool) (CodexRelayProfileMutation, string, error) {
	stored, err := service.loadCodexRelayStoredSettings()
	if err != nil {
		return CodexRelayProfileMutation{}, "", err
	}
	targetProfileID := strings.TrimSpace(profileID)
	checkingActiveProfile := targetProfileID == ""
	if checkingActiveProfile {
		if (!allowGlobalDisabled && !stored.Enabled) || stored.ActiveProfileID == "" {
			return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
		}
		targetProfileID = stored.ActiveProfileID
	}
	for _, profile := range stored.Profiles {
		if profile.ID != targetProfileID {
			continue
		}
		if checkingActiveProfile && !profile.Enabled {
			return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
		}
		if service.apiKeys == nil {
			return CodexRelayProfileMutation{}, "", ErrAPIKeyProviderNotFound
		}
		apiKey, _, err := service.apiKeys.Get(CodexRelayAPIKeyName(profile.ID))
		if err != nil {
			return CodexRelayProfileMutation{}, "", err
		}
		if strings.TrimSpace(apiKey) == "" {
			return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
		}
		return profile, strings.TrimSpace(apiKey), nil
	}
	return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
}

func (service *Settings) loadCodexRelayStoredSettings() (codexRelayStoredSettings, error) {
	if service.appSettings == nil {
		return codexRelayStoredSettings{}, ErrAppSettingStoreMissing
	}
	value, ok, err := service.appSettings.GetAppSetting(codexRelaySettingsKey)
	if err != nil {
		return codexRelayStoredSettings{}, err
	}
	if !ok || strings.TrimSpace(value) == "" {
		return codexRelayStoredSettings{}, nil
	}
	var stored codexRelayStoredSettings
	if err := json.Unmarshal([]byte(value), &stored); err != nil {
		return codexRelayStoredSettings{}, fmt.Errorf("%w: parsing saved settings: %v", ErrCodexRelayInvalid, err)
	}
	return normalizeCodexRelaySettings(CodexRelaySettingsMutation(stored))
}

func (service *Settings) codexRelaySettingsResponse(stored codexRelayStoredSettings) (CodexRelaySettingsResponse, error) {
	profiles := make([]CodexRelayProfile, 0, len(stored.Profiles))
	for _, profile := range stored.Profiles {
		apiKey, source, err := service.apiKeys.Get(CodexRelayAPIKeyName(profile.ID))
		if err != nil {
			return CodexRelaySettingsResponse{}, err
		}
		profiles = append(profiles, CodexRelayProfile{
			ID:               profile.ID,
			Name:             profile.Name,
			BaseURL:          profile.BaseURL,
			Model:            profile.Model,
			Protocol:         profile.Protocol,
			DetectedProtocol: profile.DetectedProtocol,
			Enabled:          profile.Enabled,
			APIKey: CodexRelayAPIKeyStatus{
				Configured: strings.TrimSpace(apiKey) != "",
				Source:     source,
				Masked:     maskAPIKey(apiKey),
			},
		})
	}
	return CodexRelaySettingsResponse{
		Enabled:         stored.Enabled,
		ActiveProfileID: stored.ActiveProfileID,
		Profiles:        profiles,
	}, nil
}

func (service *Settings) clearRemovedCodexRelayAPIKeys(previous codexRelayStoredSettings, next codexRelayStoredSettings) error {
	if len(previous.Profiles) == 0 {
		return nil
	}
	if service.apiKeys == nil {
		return ErrAPIKeyProviderNotFound
	}
	activeIDs := make(map[string]bool, len(next.Profiles))
	for _, profile := range next.Profiles {
		activeIDs[profile.ID] = true
	}
	for _, profile := range previous.Profiles {
		if activeIDs[profile.ID] {
			continue
		}
		if err := service.apiKeys.Clear(CodexRelayAPIKeyName(profile.ID)); err != nil {
			return err
		}
	}
	return nil
}

func normalizeCodexRelaySettings(input CodexRelaySettingsMutation) (codexRelayStoredSettings, error) {
	stored := codexRelayStoredSettings{
		Enabled:         input.Enabled,
		ActiveProfileID: profileIDFromProviderID(input.ActiveProfileID),
		Profiles:        make([]CodexRelayProfileMutation, 0, len(input.Profiles)),
	}
	seen := map[string]bool{}
	for _, profile := range input.Profiles {
		normalized, err := normalizeCodexRelayProfile(profile)
		if err != nil {
			return codexRelayStoredSettings{}, err
		}
		if seen[normalized.ID] {
			return codexRelayStoredSettings{}, fmt.Errorf("%w: duplicate profile id %q", ErrCodexRelayInvalid, normalized.ID)
		}
		seen[normalized.ID] = true
		stored.Profiles = append(stored.Profiles, normalized)
	}
	if stored.ActiveProfileID == "" && len(stored.Profiles) > 0 {
		stored.ActiveProfileID = stored.Profiles[0].ID
	}
	if stored.ActiveProfileID != "" && !seen[stored.ActiveProfileID] {
		return codexRelayStoredSettings{}, fmt.Errorf("%w: active profile is missing", ErrCodexRelayInvalid)
	}
	return stored, nil
}

func normalizeCodexRelayProfile(profile CodexRelayProfileMutation) (CodexRelayProfileMutation, error) {
	name := strings.TrimSpace(profile.Name)
	id := profileIDFromProviderID(profile.ID)
	if id == "" {
		id = profileIDFromProviderID(name)
	}
	if id == "" {
		return CodexRelayProfileMutation{}, fmt.Errorf("%w: profile id is required", ErrCodexRelayInvalid)
	}
	baseURL, err := normalizeOpenAICompatibleBaseURL(profile.BaseURL)
	if err != nil {
		return CodexRelayProfileMutation{}, fmt.Errorf("%w: %v", ErrCodexRelayInvalid, err)
	}
	model := strings.TrimSpace(profile.Model)
	if model == "" {
		return CodexRelayProfileMutation{}, fmt.Errorf("%w: model is required", ErrCodexRelayInvalid)
	}
	protocol := profile.Protocol
	if protocol == "" {
		protocol = CodexRelayProtocolResponses
	}
	if protocol != CodexRelayProtocolAuto && protocol != CodexRelayProtocolResponses && protocol != CodexRelayProtocolChatCompletions {
		return CodexRelayProfileMutation{}, fmt.Errorf("%w: unsupported protocol", ErrCodexRelayInvalid)
	}
	detectedProtocol := profile.DetectedProtocol
	if protocol != CodexRelayProtocolAuto {
		detectedProtocol = ""
	} else if detectedProtocol != "" && detectedProtocol != CodexRelayProtocolResponses && detectedProtocol != CodexRelayProtocolChatCompletions {
		return CodexRelayProfileMutation{}, fmt.Errorf("%w: unsupported detected protocol", ErrCodexRelayInvalid)
	}
	if name == "" {
		name = id
	}
	return CodexRelayProfileMutation{
		ID:               id,
		Name:             name,
		BaseURL:          baseURL,
		Model:            model,
		Protocol:         protocol,
		DetectedProtocol: detectedProtocol,
		Enabled:          profile.Enabled,
	}, nil
}

func effectiveCodexRelayProtocol(profile CodexRelayProfileMutation) CodexRelayProtocol {
	if profile.Protocol != CodexRelayProtocolAuto {
		return profile.Protocol
	}
	if profile.DetectedProtocol == CodexRelayProtocolResponses || profile.DetectedProtocol == CodexRelayProtocolChatCompletions {
		return profile.DetectedProtocol
	}
	// Compatibility-first before the user runs capability detection.
	return CodexRelayProtocolChatCompletions
}

func codexRelayStoredProfileExists(stored codexRelayStoredSettings, profileID string) bool {
	for _, profile := range stored.Profiles {
		if profile.ID == profileID {
			return true
		}
	}
	return false
}

func renderCodexRelayConfig(profile CodexRelayProfileMutation, relayBaseURL string) string {
	baseURL := strings.TrimRight(relayBaseURL, "/") + "/v1"
	return fmt.Sprintf(
		"model = %q\nmodel_provider = %q\n\n[model_providers.%s]\nname = %q\nwire_api = \"responses\"\nrequires_openai_auth = false\nbase_url = %q\nenv_key = %q\n",
		profile.Model,
		codexRelayProviderID,
		codexRelayProviderID,
		codexRelayProviderID,
		baseURL,
		codexRelayLocalTokenEnv,
	)
}

func codexRelayUpstreamURL(baseURL string, relayPath string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	relayPath = "/" + strings.TrimLeft(strings.TrimSpace(relayPath), "/")
	relayPathOnly, rawQuery, _ := strings.Cut(relayPath, "?")
	if baseURL == "" || !validHTTPURL(baseURL) {
		return "", fmt.Errorf("%w: baseURL must be an http(s) URL", ErrCodexRelayInvalid)
	}
	if !codexRelayAllowedPath(relayPathOnly) {
		return "", fmt.Errorf("%w: relay path is not allowed", ErrCodexRelayInvalid)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("%w: parsing baseURL: %v", ErrCodexRelayInvalid, err)
	}
	suffix := relayPathOnly
	if strings.HasSuffix(parsed.Path, "/v1") && strings.HasPrefix(suffix, "/v1/") {
		suffix = strings.TrimPrefix(suffix, "/v1")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + suffix
	parsed.RawQuery = rawQuery
	parsed.Fragment = ""
	return parsed.String(), nil
}

func codexRelayAllowedPath(path string) bool {
	path = strings.SplitN(path, "?", 2)[0]
	switch path {
	case "/v1/responses", "/responses", "/v1/models", "/models":
		return true
	default:
		return strings.HasPrefix(path, "/v1/responses/") ||
			strings.HasPrefix(path, "/responses/") ||
			strings.HasPrefix(path, "/v1/models/") ||
			strings.HasPrefix(path, "/models/")
	}
}

func codexRelayChatCompletionsUpstreamURL(baseURL string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || !validHTTPURL(baseURL) {
		return "", fmt.Errorf("%w: baseURL must be an http(s) URL", ErrCodexRelayInvalid)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("%w: parsing baseURL: %v", ErrCodexRelayInvalid, err)
	}
	suffix := "/v1/chat/completions"
	if strings.HasSuffix(parsed.Path, "/v1") {
		suffix = "/chat/completions"
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + suffix
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validCodexRelayLocalAuthorization(headers http.Header) bool {
	auth := strings.TrimSpace(headers.Get("Authorization"))
	return auth == "Bearer "+codexRelayLocalBearerToken
}

func copyCodexRelayRequestHeaders(target http.Header, source http.Header) {
	for _, key := range []string{"Accept", "Content-Type", "User-Agent", "Cache-Control"} {
		value := source.Get(key)
		if strings.TrimSpace(value) != "" {
			target.Set(key, value)
		}
	}
}

func readCodexRelayBody(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading codex relay body: %w", err)
	}
	return body, nil
}

func codexRelayCheckStatusOK(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices ||
		status == http.StatusNotFound ||
		status == http.StatusMethodNotAllowed
}

func codexRelayCheckStatusAuthFailed(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func codexRelayBodyLooksInvalidAPIKey(body string) bool {
	normalized := strings.ToLower(body)
	return strings.Contains(normalized, "invalid_api_key") ||
		strings.Contains(normalized, "invalid api key") ||
		strings.Contains(normalized, "incorrect api key")
}

func readLimitedCodexRelayCheckBody(reader io.Reader) string {
	if reader == nil {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(reader, codexRelayCheckBodyLimit))
	if err != nil {
		return ""
	}
	return string(body)
}

func codexRelayModelIDs(body string) []string {
	payload := codexRelayModelsPayload{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return []string{}
	}
	seen := make(map[string]struct{}, len(payload.Data))
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		model := strings.TrimSpace(item.ID)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	sort.SliceStable(models, func(left int, right int) bool {
		return codexRelayModelLess(models[left], models[right])
	})
	return models
}

type codexRelayModelSortKey struct {
	isGPT       bool
	major       int
	minor       int
	variantRank int
	normalized  string
}

func codexRelayModelLess(left string, right string) bool {
	leftKey := codexRelayModelKey(left)
	rightKey := codexRelayModelKey(right)
	if leftKey.isGPT != rightKey.isGPT {
		return leftKey.isGPT
	}
	if leftKey.isGPT {
		if leftKey.major != rightKey.major {
			return leftKey.major > rightKey.major
		}
		if leftKey.minor != rightKey.minor {
			return leftKey.minor > rightKey.minor
		}
		if leftKey.variantRank != rightKey.variantRank {
			return leftKey.variantRank < rightKey.variantRank
		}
	}
	return leftKey.normalized < rightKey.normalized
}

func codexRelayModelKey(model string) codexRelayModelSortKey {
	normalized := strings.ToLower(strings.TrimSpace(model))
	key := codexRelayModelSortKey{normalized: normalized, variantRank: 100}
	if !strings.HasPrefix(normalized, "gpt-") {
		return key
	}
	versionAndVariant := strings.TrimPrefix(normalized, "gpt-")
	version, variant, _ := strings.Cut(versionAndVariant, "-")
	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		return key
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if majorErr != nil || minorErr != nil {
		return key
	}
	variant, _, _ = strings.Cut(variant, "-")
	key.isGPT = true
	key.major = major
	key.minor = minor
	key.variantRank = codexRelayModelVariantRank(variant)
	return key
}

func codexRelayModelVariantRank(variant string) int {
	switch variant {
	case "":
		return 0
	case "sol":
		return 10
	case "terra":
		return 20
	case "luna":
		return 30
	case "pro":
		return 40
	case "codex":
		return 50
	case "mini":
		return 60
	case "nano":
		return 70
	default:
		return 100
	}
}
