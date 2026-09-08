package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentModelContextWindow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		models []openAIModelListItem
		model  string
		want   int
	}{
		{"declared", []openAIModelListItem{{ID: "DeepSeek-V3.2", ContextLength: 131072}}, "DeepSeek-V3.2", 131072},
		{"aliases", []openAIModelListItem{{ID: "DeepSeek-V3.2", ContextLength: 131072, ContextWindow: 65536}}, "DeepSeek-V3.2", 65536},
		{"known DeepSeek V4 fallback", nil, "deepseek-v4-pro-0813", 1_000_000},
		{"declared DeepSeek V4 limit wins", []openAIModelListItem{{ID: "deepseek-v4-pro-0813", ContextLength: 131072}}, "deepseek-v4-pro-0813", 131072},
		{"missing", nil, "DeepSeek-V3.2", 32768},
		{"different model", []openAIModelListItem{{ID: "deepseek-v3.2", ContextLength: 131072}}, "DeepSeek-V3.2", 32768},
		{"invalid", []openAIModelListItem{{ID: "DeepSeek-V3.2", ContextLength: -1, ContextWindow: 999999999}}, "DeepSeek-V3.2", 32768},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := agentModelContextWindow(tc.models, tc.model); got != tc.want {
				t.Fatalf("window=%d want=%d", got, tc.want)
			}
		})
	}
}

func TestAgentModelContextMetadataDoesNotHideModelList(t *testing.T) {
	for _, raw := range []string{`null`, `"unknown"`, `{}`, `[]`, `-1`, `1e50`, `131072.5`} {
		var result openAIModelListResponse
		if err := json.Unmarshal([]byte(`{"data":[{"id":"fixture","context_length":`+raw+`}]}`), &result); err != nil || len(result.Data) != 1 || result.Data[0].ID != "fixture" {
			t.Fatalf("metadata %s hid model: %v", raw, err)
		}
		if got := agentModelContextWindow(result.Data, "fixture"); got != 32768 {
			t.Fatalf("invalid metadata %s gave window %d", raw, got)
		}
	}
	var result openAIModelListResponse
	if err := json.Unmarshal([]byte(`{"data":[{"id":"fixture","context_length":"131072"}]}`), &result); err != nil {
		t.Fatal(err)
	}
	if got := agentModelContextWindow(result.Data, "fixture"); got != 131072 {
		t.Fatalf("numeric string: %d", got)
	}
}

func TestAgentModelCompactLimitLeavesOutputRoom(t *testing.T) {
	for _, window := range []int{4096, 8192, 32768, 65536, 131072, 2_000_000} {
		limit := agentModelCompactLimit(window)
		if limit <= 0 || limit > window*80/100 || window*95/100-limit < min(4096, window/4) {
			t.Fatalf("unsafe budget window=%d limit=%d", window, limit)
		}
	}
}

func TestAgentProviderDeclaredContextReachesStartupAndSession(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("wrong metadata request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"DeepSeek-V3.2","context_length":131072}]}`))
	}))
	defer upstream.Close()
	service := NewSettingsWithStores(&memoryAPIKeyStore{values: map[string]string{agentModelProviderAIHubMix: "fixture"}}, nil, &memoryAppSettingStore{values: map[string]string{aihubmixBaseURLSettingKey: upstream.URL + "/v1"}})
	config, err := service.PrepareAgentProviderRuntimeConfig(context.Background(), t.TempDir(), "http://127.0.0.1:8080/relay", "gateway-aihubmix:DeepSeek-V3.2")
	if err != nil {
		t.Fatal(err)
	}
	var session struct {
		Window  int `json:"model_context_window"`
		Compact int `json:"model_auto_compact_token_limit"`
	}
	if err := json.Unmarshal([]byte(config.Env["CODEX_CONFIG"]), &session); err != nil {
		t.Fatal(err)
	}
	if session.Window != 131072 || session.Compact != 104857 {
		t.Fatalf("unexpected session budget: %+v", session)
	}
	startup, err := os.ReadFile(filepath.Join(config.CodexHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(startup), "model_context_window = 131072") || !strings.Contains(string(startup), "request_max_retries = 2") {
		t.Fatal("startup budget or retry limit missing")
	}
	catalog, err := os.ReadFile(filepath.Join(config.CodexHome, "jw-model-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(catalog), `"context_window":131072`) {
		t.Fatal("catalog context does not match session")
	}
}
