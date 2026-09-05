package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentProvidersKeepOAuthAndGatewaysIndependent(t *testing.T) {
	ctx := context.Background()
	host := t.TempDir()
	t.Setenv("CODEX_HOME", host)
	keyStore := &memoryAPIKeyStore{values: map[string]string{}}
	service := NewSettingsWithStores(keyStore, nil, &memoryAppSettingStore{values: map[string]string{}})
	model := "deepseek/DeepSeek-V3.2:fast"
	newUpstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer fixture-"+name {
				t.Errorf("request used the wrong provider credential")
			}
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/v1/models" {
				fmt.Fprintf(w, `{"data":[{"id":%q},{"id":"gpt-image-1"},{"id":"tts-1"}]}`, model)
				return
			}
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Model != model {
				t.Errorf("upstream model = %q", body.Model)
			}
			fmt.Fprintf(w, `{"provider":%q}`, name)
		}))
	}
	a, b := newUpstream("a"), newUpstream("b")
	defer a.Close()
	defer b.Close()
	mutation := CodexRelaySettingsMutation{Enabled: true, ActiveProfileID: "a", Profiles: []CodexRelayProfileMutation{
		{ID: "a", Name: "Tokease", BaseURL: a.URL + "/v1", Model: model, Protocol: CodexRelayProtocolResponses, Enabled: true},
		{ID: "b", Name: "OpenRouter", BaseURL: b.URL + "/v1", Model: model, Protocol: CodexRelayProtocolResponses, Enabled: true},
	}}
	if _, err := service.SaveCodexRelaySettings(ctx, mutation); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if err := keyStore.Set(CodexRelayAPIKeyName(id), "fixture-"+id); err != nil {
			t.Fatal(err)
		}
	}
	models, err := service.ListAgentProviderRuntimeModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].ProviderID != "api-a" || models[1].ProviderID != "api-b" || models[0].ModelID != model {
		t.Fatalf("provider models = %#v", models)
	}
	workspace := t.TempDir()
	apiA, err := service.PrepareAgentProviderRuntimeConfig(ctx, workspace, "http://127.0.0.1:8080/api/v1/codex-relay", "api-a:"+model)
	if err != nil {
		t.Fatal(err)
	}
	apiB, err := service.PrepareAgentProviderRuntimeConfig(ctx, workspace, "http://127.0.0.1:8080/api/v1/codex-relay", "api-b:"+model)
	if err != nil {
		t.Fatal(err)
	}
	apiAReasoning, err := service.PrepareAgentProviderRuntimeConfig(ctx, workspace, "http://127.0.0.1:8080/api/v1/codex-relay", "api-a:"+model, true)
	if err != nil {
		t.Fatal(err)
	}
	if apiAReasoning.CodexHome == apiA.CodexHome {
		t.Fatal("default and explicit effort overwrite a live catalogue")
	}
	for _, runtime := range []CodexRelayRuntimeConfig{apiA, apiAReasoning} {
		raw, err := os.ReadFile(filepath.Join(runtime.CodexHome, "jw-model-catalog.json"))
		if err != nil {
			t.Fatal(err)
		}
		var catalog struct {
			Models []struct {
				Levels  []struct{ Effort string } `json:"supported_reasoning_levels"`
				Enabled bool                      `json:"supports_reasoning_summaries"`
				Default *string                   `json:"default_reasoning_level"`
			}
		}
		if err := json.Unmarshal(raw, &catalog); err != nil {
			t.Fatal(err)
		}
		want := runtime.CodexHome == apiAReasoning.CodexHome
		if len(catalog.Models) != 1 || catalog.Models[0].Enabled != want || catalog.Models[0].Default != nil {
			t.Fatal("invalid reasoning catalogue")
		}
		if want && (len(catalog.Models[0].Levels) != 6 || catalog.Models[0].Levels[4].Effort != "high") {
			t.Fatal("explicit effort choices missing")
		}
		if !want && len(catalog.Models[0].Levels) != 0 {
			t.Fatal("default acquired implicit effort")
		}
	}
	if apiA.CodexHome == apiB.CodexHome || apiA.CodexHome == host {
		t.Fatal("providers share an authentication home")
	}
	raw, err := os.ReadFile(filepath.Join(apiA.CodexHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "/providers/api-a/v1") || strings.Contains(string(raw), "fixture-") {
		t.Fatal("provider config is not bound safely")
	}
	oauth, err := service.PrepareAgentProviderRuntimeConfig(ctx, workspace, "unused", "chatgpt:gpt-5.5")
	if err != nil {
		t.Fatal(err)
	}
	if oauth.Configured || oauth.CodexHome != host || oauth.Env["OPENAI_API_KEY"] != "" || !strings.Contains(oauth.Env["CODEX_CONFIG"], `"model_provider":"openai"`) {
		t.Fatalf("OAuth environment diverged: %#v", oauth)
	}
	mutation.ActiveProfileID = "b"
	mutation.Enabled = false
	if _, err := service.SaveCodexRelaySettings(ctx, mutation); err != nil {
		t.Fatal(err)
	}
	// Switching the default to OAuth must not reroute either live API provider.
	for _, id := range []string{"a", "b"} {
		body, _ := json.Marshal(map[string]string{"model": model})
		response, err := service.OpenCodexRelayRequest(ctx, http.MethodPost, "/providers/api-"+id+"/v1/responses", body, http.Header{"Authorization": []string{"Bearer " + codexRelayLocalBearerToken}})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(raw) != fmt.Sprintf(`{"provider":%q}`, id) {
			t.Fatalf("provider %s was rerouted: %s (%v)", id, raw, err)
		}
	}
	if _, err := service.PrepareAgentProviderRuntimeConfig(ctx, workspace, "unused", "api-missing:"+model); err == nil {
		t.Fatal("unknown provider silently fell back")
	}
}

func TestAgentProviderSkillsKeepDisabledRules(t *testing.T) {
	host := t.TempDir()
	t.Setenv("CODEX_HOME", host)
	skillDir := filepath.Join(host, "skills", "disabled")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(host, "config.toml"), []byte(fmt.Sprintf("[[skills.config]]\npath = %q\nenabled = false\n", skillPath)), 0o600); err != nil {
		t.Fatal(err)
	}
	skills, err := codexProviderSkills(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(skills.Config) != 1 || skills.Config[0].Enabled == nil || *skills.Config[0].Enabled || skills.Config[0].Path != skillPath {
		t.Fatalf("skill rules = %#v", skills)
	}
}

func TestUnifiedProviderListedOnceAndLegacyAliasStillWorks(t *testing.T) {
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"DeepSeek-V3.2"}]}`)
	}))
	defer upstream.Close()
	service := NewSettingsWithStores(&memoryAPIKeyStore{values: map[string]string{agentModelProviderAIHubMix: "fixture"}}, nil,
		&memoryAppSettingStore{values: map[string]string{aihubmixBaseURLSettingKey: upstream.URL + "/v1"}})
	models, err := service.ListAgentProviderRuntimeModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Value != "gateway-aihubmix:DeepSeek-V3.2" {
		t.Fatalf("duplicate/phantom models: %#v", models)
	}
	for _, id := range []string{"gateway-aihubmix", "gateway-openai-compatible"} {
		profile, key, err := service.agentProviderProfile(ctx, id)
		if err != nil || key != "fixture" || profile.BaseURL != upstream.URL+"/v1" {
			t.Fatalf("legacy alias failed: %s %v", id, err)
		}
	}
}

func TestProviderModelCatalogDefinesBridgeContract(t *testing.T) {
	path, err := writeAgentProviderModelCatalog(t.TempDir(), "DeepSeek-V3.2")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Models []struct {
			Slug          string
			ContextWindow int                       `json:"context_window"`
			Levels        []struct{ Effort string } `json:"supported_reasoning_levels"`
		}
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Models) != 1 || catalog.Models[0].Slug != "DeepSeek-V3.2" || catalog.Models[0].ContextWindow != 32768 || len(catalog.Models[0].Levels) != 0 {
		t.Fatalf("invalid bridge contract: %s", raw)
	}
}
