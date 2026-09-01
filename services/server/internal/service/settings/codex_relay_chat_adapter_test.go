package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCodexRelayChatCompletionsAdapterConvertsTextAndSSE(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotRequest codexChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		gotAuth = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode chat request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":"chatcmpl_1","created":123,"model":"deepseek-v4-pro","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"E2E_OK"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
	}))
	defer server.Close()

	service := newChatRelayTestSettings(t, server.URL+"/v1")
	body := []byte(`{"model":"deepseek-v4-pro","instructions":"system rule","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],"stream":true}`)
	response, err := service.OpenCodexRelayRequest(context.Background(), http.MethodPost, "/v1/responses", body, localRelayHeaders())
	if err != nil {
		t.Fatalf("OpenCodexRelayRequest returned error: %v", err)
	}
	defer response.Body.Close()
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer sk-chat-upstream" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotRequest.Stream {
		t.Fatal("upstream Chat Completions request must be non-streaming")
	}
	if len(gotRequest.Messages) != 2 || gotRequest.Messages[0].Role != "system" || gotRequest.Messages[1].Role != "user" {
		t.Fatalf("messages = %#v", gotRequest.Messages)
	}
	payload, _ := io.ReadAll(response.Body)
	text := string(payload)
	for _, want := range []string{"response.created", "response.output_text.delta", `"delta":"E2E_OK"`, "response.completed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("SSE missing %q:\n%s", want, text)
		}
	}
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type = %q", response.Header.Get("Content-Type"))
	}
}

func TestCodexRelayChatCompletionsAdapterMapsFunctionCallsBothDirections(t *testing.T) {
	var gotRequest codexChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode chat request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":"chatcmpl_tool","model":"deepseek-v4-pro","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_123","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.txt\"}"}}]}}]}`)
	}))
	defer server.Close()

	service := newChatRelayTestSettings(t, server.URL+"/v1")
	body := []byte(`{
		"model":"deepseek-v4-pro",
		"input":[
			{"type":"function_call","call_id":"call_prev","name":"list_files","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_prev","output":"[a.txt]"}
		],
		"tools":[{"type":"function","name":"read_file","description":"Read a file","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}],
		"stream":true
	}`)
	response, err := service.OpenCodexRelayRequest(context.Background(), http.MethodPost, "/responses", body, localRelayHeaders())
	if err != nil {
		t.Fatalf("OpenCodexRelayRequest returned error: %v", err)
	}
	defer response.Body.Close()
	if len(gotRequest.Messages) != 2 || len(gotRequest.Messages[0].ToolCalls) != 1 || gotRequest.Messages[1].ToolCallID != "call_prev" {
		t.Fatalf("converted messages = %#v", gotRequest.Messages)
	}
	if len(gotRequest.Tools) != 1 || gotRequest.Tools[0].Function.Name != "read_file" {
		t.Fatalf("converted tools = %#v", gotRequest.Tools)
	}
	payload, _ := io.ReadAll(response.Body)
	text := string(payload)
	for _, want := range []string{"response.function_call_arguments.delta", `"call_id":"call_123"`, `"name":"read_file"`, "response.completed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("tool SSE missing %q:\n%s", want, text)
		}
	}
}

func TestCheckCodexRelayAllowsChatCompletionsProfile(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"data":[{"id":"deepseek-v4-pro"}]}`)
	}))
	defer server.Close()

	service := newChatRelayTestSettings(t, server.URL+"/v1")
	result, err := service.CheckCodexRelay(context.Background(), CodexRelayCheckRequest{})
	if err != nil {
		t.Fatalf("CheckCodexRelay returned error: %v", err)
	}
	if !result.OK || gotPath != "/v1/models" || len(result.Models) != 1 || result.Models[0] != "deepseek-v4-pro" {
		t.Fatalf("result=%#v path=%q", result, gotPath)
	}
}

func newChatRelayTestSettings(t *testing.T, baseURL string) *Settings {
	t.Helper()
	service := NewSettingsWithStores(
		&memoryAPIKeyStore{values: map[string]string{}},
		nil,
		&memoryAppSettingStore{values: map[string]string{}},
	)
	ctx := context.Background()
	if _, err := service.SaveCodexRelaySettings(ctx, CodexRelaySettingsMutation{
		Enabled:         true,
		ActiveProfileID: "chat-relay",
		Profiles: []CodexRelayProfileMutation{{
			ID:       "chat-relay",
			Name:     "Chat Relay",
			BaseURL:  baseURL,
			Model:    "deepseek-v4-pro",
			Protocol: CodexRelayProtocolChatCompletions,
			Enabled:  true,
		}},
	}); err != nil {
		t.Fatalf("SaveCodexRelaySettings: %v", err)
	}
	if _, err := service.SetCodexRelayProfileAPIKey(ctx, "chat-relay", "sk-chat-upstream"); err != nil {
		t.Fatalf("SetCodexRelayProfileAPIKey: %v", err)
	}
	return service
}

func localRelayHeaders() http.Header {
	return http.Header{"Authorization": []string{"Bearer " + codexRelayLocalBearerToken}, "Content-Type": []string{"application/json"}}
}
