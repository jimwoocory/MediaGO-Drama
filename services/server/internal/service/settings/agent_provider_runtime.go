package settings

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mediago-dev/mediago-drama/services/server/internal/service/shared"
	"github.com/pelletier/go-toml/v2"
)

// AgentProviderRuntimeModel keeps the gateway identity separate from the upstream model ID.
type AgentProviderRuntimeModel struct {
	Value         string
	Name          string
	ProviderID    string
	ProviderLabel string
	ModelID       string
	Description   string
}

// ListAgentProviderRuntimeModels exposes every configured third-party provider independently.
func (service *Settings) ListAgentProviderRuntimeModels(ctx context.Context) ([]AgentProviderRuntimeModel, error) {
	stored, err := service.loadCodexRelayStoredSettings()
	if err != nil {
		return nil, err
	}
	result := []AgentProviderRuntimeModel{}
	appendModels := func(provider string, profile CodexRelayProfileMutation, key string) {
		models := []string{profile.Model}
		description := ""
		if discovered, discoverErr := fetchOpenAICompatibleModels(ctx, profile.BaseURL, key); discoverErr == nil {
			for _, model := range discovered {
				if mediagoGatewayModelLooksTaskOnly(mediagoGatewayModel{ID: model.ID}) {
					continue
				}
				models = append(models, model.ID)
			}
		} else {
			description = "模型列表暂不可用；使用已配置的 Model ID。"
		}
		seen := map[string]bool{}
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" || seen[model] {
				continue
			}
			seen[model] = true
			result = append(result, AgentProviderRuntimeModel{Value: provider + ":" + model, Name: model, ProviderID: provider, ProviderLabel: profile.Name, ModelID: model, Description: description})
		}
	}
	// Enabled selects the default route for old sessions, not provider availability.
	for _, profile := range stored.Profiles {
		if !profile.Enabled {
			continue
		}
		_, key, keyErr := service.codexRelayProfileWithKey(profile.ID, true)
		if errors.Is(keyErr, ErrCodexRelayNotConfigured) {
			continue
		}
		if keyErr != nil {
			return nil, keyErr
		}
		appendModels("api-"+profile.ID, profile, key)
	}
	if profile, key, profileErr := service.unifiedCodexRelayProfile(ctx, false); profileErr == nil {
		// The unified entry has no manually selected model. Never invent one when discovery fails.
		profile.Model = ""
		appendModels("gateway-"+agentModelProviderAIHubMix, profile, key)
	} else if !errors.Is(profileErr, ErrCodexRelayNotConfigured) {
		return nil, profileErr
	}
	profiles, _, err := service.officialAgentRuntimeProfilesExcept(ctx, agentModelProviderCompatible)
	if err != nil {
		if len(result) > 0 {
			return result, nil
		}
		return nil, err
	}
	for _, profile := range profiles {
		// The migrated unified gateway is already represented above.
		if profile.ProviderID == agentModelProviderAIHubMix {
			continue
		}
		provider := "gateway-" + profile.ProviderID
		result = append(result, AgentProviderRuntimeModel{Value: provider + ":" + profile.Model, Name: profile.ModelDisplayName, ProviderID: provider, ProviderLabel: profile.ProviderLabel, ModelID: profile.Model})
	}
	return result, nil
}

func (service *Settings) agentProviderProfile(ctx context.Context, provider string) (CodexRelayProfileMutation, string, error) {
	if provider == "gateway-"+agentModelProviderAIHubMix || provider == "gateway-"+agentModelProviderCompatible {
		return service.unifiedCodexRelayProfile(ctx, false)
	}
	if id, ok := strings.CutPrefix(provider, "api-"); ok {
		profile, key, err := service.codexRelayProfileWithKey(id, true)
		if err == nil && !profile.Enabled {
			err = ErrCodexRelayNotConfigured
		}
		return profile, key, err
	}
	if id, ok := strings.CutPrefix(provider, "gateway-"); ok {
		for _, spec := range service.officialAgentRuntimeProfileSpecs() {
			if spec.ProviderID == id && (spec.PlatformID == "" || service.modelPlatformEnabled(spec.PlatformID)) {
				key, err := service.agentRuntimeAPIKey(spec.CredentialKeyName, spec.LegacyProfileID, profileIDFromProviderID(id))
				if err != nil {
					return CodexRelayProfileMutation{}, "", err
				}
				if key == "" {
					return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
				}
				return CodexRelayProfileMutation{ID: provider, Name: spec.ProviderLabel, BaseURL: spec.BaseURL, Protocol: CodexRelayProtocolChatCompletions, Enabled: true}, key, nil
			}
		}
	}
	return CodexRelayProfileMutation{}, "", ErrCodexRelayNotConfigured
}

// PrepareAgentProviderRuntimeConfig selects OAuth or a named API without mutating the global active provider.
func (service *Settings) PrepareAgentProviderRuntimeConfig(ctx context.Context, workspaceDir, relayBaseURL, modelValue string, reasoningEnabled ...bool) (CodexRelayRuntimeConfig, error) {
	provider, model, explicit := shared.SplitAgentModelRef(modelValue)
	if !explicit || provider == "chatgpt" {
		// Unqualified historical Codex model IDs now unambiguously use the account channel.
		config := map[string]string{"model_provider": "openai", "forced_login_method": "chatgpt"}
		if model != "" {
			config["model"] = model
		}
		raw, err := json.Marshal(config)
		if err != nil {
			return CodexRelayRuntimeConfig{}, err
		}
		return CodexRelayRuntimeConfig{CodexHome: effectiveCodexHome(), Env: map[string]string{
			"CODEX_HOME": effectiveCodexHome(), "CODEX_CONFIG": string(raw), "OPENAI_API_KEY": "", "CODEX_API_KEY": "", "OPENAI_BASE_URL": "", "DEFAULT_AUTH_REQUEST": "", codexRelayLocalTokenEnv: "",
		}}, nil
	}
	profile, key, err := service.agentProviderProfile(ctx, provider)
	if err != nil {
		return CodexRelayRuntimeConfig{}, err
	}
	profile.Model = model
	window := defaultAgentContextWindow
	if models, discoverErr := fetchOpenAICompatibleModels(ctx, profile.BaseURL, key); discoverErr == nil {
		window = agentModelContextWindow(models, model)
	}
	if err := ctx.Err(); err != nil {
		return CodexRelayRuntimeConfig{}, err
	}
	baseURL := strings.TrimRight(relayBaseURL, "/") + "/providers/" + provider
	// Different models/configurations must not overwrite files used by live processes.
	enableReasoning := len(reasoningEnabled) > 0 && reasoningEnabled[0]
	fingerprint := sha256.Sum256([]byte(fmt.Sprintf("jw-catalog-v3-%d-%t\n", window, enableReasoning) + renderCodexRelayConfig(profile, baseURL)))
	codexHome := filepath.Join(shared.WorkspacePathsFor(workspaceDir).GlobalMetadataDir(), "runtime", "agents", "codex", "providers", fmt.Sprintf("%x", fingerprint[:16]), "home")
	runtimeConfig, err := prepareCodexProfileRuntimeWithOptions(profile, codexHome, baseURL, codexProfileRuntimeOptions{ContextWindow: window, Reasoning: enableReasoning})
	if err != nil {
		return CodexRelayRuntimeConfig{}, err
	}
	skills, err := codexProviderSkills(workspaceDir)
	if err != nil {
		return CodexRelayRuntimeConfig{}, err
	}
	// ACP creates its model manager at process startup, before session-level
	// CODEX_CONFIG overrides. The catalog must also exist in startup config.toml.
	catalogPath := filepath.Join(codexHome, "jw-model-catalog.json")
	config := map[string]interface{}{"model_provider": codexRelayProviderID, "model": model, "forced_login_method": "api", "skills": skills,
		"model_catalog_json": catalogPath, "model_context_window": window, "model_auto_compact_token_limit": agentModelCompactLimit(window),
		"model_reasoning_summary": "none",
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return CodexRelayRuntimeConfig{}, err
	}
	runtimeConfig.Env["CODEX_CONFIG"] = string(raw)
	runtimeConfig.Env["CODEX_API_KEY"] = ""
	runtimeConfig.Env["OPENAI_BASE_URL"] = ""
	runtimeConfig.Env["JW_AGENT_PROVIDER"] = provider
	return runtimeConfig, nil
}

type providerSkillConfig struct {
	Path    string `json:"path,omitempty" toml:"path"`
	Name    string `json:"name,omitempty" toml:"name"`
	Enabled *bool  `json:"enabled,omitempty" toml:"enabled"`
}

type providerSkillsConfig struct {
	Config  []providerSkillConfig `json:"config" toml:"config"`
	Bundled *bool                 `json:"bundled,omitempty" toml:"bundled"`
}

func codexProviderSkills(workspaceDir string) (providerSkillsConfig, error) {
	skills := []providerSkillConfig{}
	seen := map[string]bool{}
	var hostConfig struct {
		Skills providerSkillsConfig `toml:"skills"`
	}
	if raw, err := os.ReadFile(filepath.Join(effectiveCodexHome(), "config.toml")); err == nil {
		if err := toml.Unmarshal(raw, &hostConfig); err != nil {
			return providerSkillsConfig{}, fmt.Errorf("reading host skill configuration: %w", err)
		}
		for _, skill := range hostConfig.Skills.Config {
			if skill.Path == "" {
				continue
			}
			seen[filepath.Clean(skill.Path)] = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return providerSkillsConfig{}, err
	}
	// Keep both installed host skills and the previous JW runtime's skills available.
	legacySkills := filepath.Join(shared.WorkspacePathsFor(workspaceDir).GlobalMetadataDir(), "runtime", "agents", "codex", "home", "skills")
	for _, root := range []string{filepath.Join(effectiveCodexHome(), "skills"), legacySkills} {
		entries, readErr := os.ReadDir(root)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return providerSkillsConfig{}, fmt.Errorf("reading shared skills: %w", readErr)
		}
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name(), "SKILL.md")
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
				if !seen[filepath.Clean(path)] && !seen[filepath.Clean(filepath.Dir(path))] {
					enabled := true
					skills = append(skills, providerSkillConfig{Path: path, Enabled: &enabled})
					seen[filepath.Clean(path)] = true
				}
			}
		}
	}
	// Explicit host rules come last so name-based and last-match disable rules survive.
	return providerSkillsConfig{Config: append(skills, hostConfig.Skills.Config...), Bundled: hostConfig.Skills.Bundled}, nil
}

// DefaultAgentProviderModel returns the selected default API reference, or empty for OAuth.
func (service *Settings) DefaultAgentProviderModel() string {
	stored, err := service.loadCodexRelayStoredSettings()
	if err != nil || !stored.Enabled {
		return ""
	}
	for _, profile := range stored.Profiles {
		if profile.ID == stored.ActiveProfileID && profile.Enabled {
			return "api-" + profile.ID + ":" + profile.Model
		}
	}
	return ""
}
