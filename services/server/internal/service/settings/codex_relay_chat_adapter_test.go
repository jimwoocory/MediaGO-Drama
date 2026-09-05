package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestCodexResponsesInputDowngradesDeveloperRoleForCompatibleGateways(t *testing.T) {
	messages, err := codexResponsesInputToChatMessages("", json.RawMessage(`[
		{"type":"message","role":"developer","content":[{"type":"input_text","text":"use tools when needed"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}
	]`))
	if err != nil {
		t.Fatalf("codexResponsesInputToChatMessages returned error: %v", err)
	}
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" {
		t.Fatalf("messages = %#v, want developer downgraded to system", messages)
	}
}

func TestCodexResponsesToolsToChatNormalizesEmptyMCPToolSchema(t *testing.T) {
	tools, err := codexResponsesToolsToChat([]json.RawMessage{json.RawMessage(`{
		"type":"function",
		"name":"mcp_mediago_drama_get_project_config",
		"description":"read project config",
		"strict":true,
		"parameters":{"$schema":"https://json-schema.org/draft/2020-12/schema","additionalProperties":false}
	}`)})
	if err != nil {
		t.Fatalf("codexResponsesToolsToChat returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, want one tool", tools)
	}
	parameters, ok := tools[0].Function.Parameters.(map[string]any)
	if !ok {
		t.Fatalf("parameters = %#v, want object schema", tools[0].Function.Parameters)
	}
	if parameters["type"] != "object" {
		t.Fatalf("type = %#v, want object", parameters["type"])
	}
	properties, ok := parameters["properties"].(map[string]any)
	if !ok || len(properties) != 0 {
		t.Fatalf("properties = %#v, want empty object", parameters["properties"])
	}
	if _, ok := parameters["$schema"]; ok {
		t.Fatalf("parameters retained $schema: %#v", parameters)
	}
	encoded, err := json.Marshal(tools[0])
	if err != nil {
		t.Fatalf("marshal tool: %v", err)
	}
	if strings.Contains(string(encoded), `"strict"`) {
		t.Fatalf("chat-compatible tool retained strict: %s", encoded)
	}
}
func TestCheckCodexRelayAllowsChatCompletionsProfile(t *testing.T) {
	var gotPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPaths = append(gotPaths, request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/models":
			fmt.Fprint(writer, `{"data":[{"id":"deepseek-v4-pro"}]}`)
		case "/v1/responses":
			writer.WriteHeader(http.StatusNotFound)
			fmt.Fprint(writer, `{"error":{"message":"not found"}}`)
		case "/v1/chat/completions":
			fmt.Fprint(writer, `{"id":"chatcmpl_probe","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := newChatRelayTestSettings(t, server.URL+"/v1")
	result, err := service.CheckCodexRelay(context.Background(), CodexRelayCheckRequest{})
	if err != nil {
		t.Fatalf("CheckCodexRelay returned error: %v", err)
	}
	wantPaths := []string{"/v1/models", "/v1/responses", "/v1/chat/completions"}
	if !result.OK || result.ResponsesSupported || !result.ChatCompletionsSupported || result.RecommendedProtocol != CodexRelayProtocolChatCompletions || len(result.Models) != 1 || result.Models[0] != "deepseek-v4-pro" || !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("result=%#v paths=%#v", result, gotPaths)
	}
}

func TestAutoProtocolPersistsChatDetectionAndUsesChatAdapter(t *testing.T) {
	var gotPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPaths = append(gotPaths, request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/models":
			fmt.Fprint(writer, `{"data":[{"id":"deepseek-v4-flash"}]}`)
		case "/v1/responses":
			writer.WriteHeader(http.StatusNotFound)
			fmt.Fprint(writer, `{"error":{"message":"not found"}}`)
		case "/v1/chat/completions":
			fmt.Fprint(writer, `{"id":"chatcmpl_1","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := NewSettingsWithStores(
		&memoryAPIKeyStore{values: map[string]string{}},
		nil,
		&memoryAppSettingStore{values: map[string]string{}},
	)
	ctx := context.Background()
	if _, err := service.SaveCodexRelaySettings(ctx, CodexRelaySettingsMutation{
		Enabled:         true,
		ActiveProfileID: "auto-relay",
		Profiles: []CodexRelayProfileMutation{{
			ID: "auto-relay", Name: "Auto Relay", BaseURL: server.URL + "/v1", Model: "deepseek-v4-flash", Protocol: CodexRelayProtocolAuto, Enabled: true,
		}},
	}); err != nil {
		t.Fatalf("SaveCodexRelaySettings returned error: %v", err)
	}
	if _, err := service.SetCodexRelayProfileAPIKey(ctx, "auto-relay", "sk-auto"); err != nil {
		t.Fatalf("SetCodexRelayProfileAPIKey returned error: %v", err)
	}

	check, err := service.CheckCodexRelay(ctx, CodexRelayCheckRequest{})
	if err != nil {
		t.Fatalf("CheckCodexRelay returned error: %v", err)
	}
	if check.RecommendedProtocol != CodexRelayProtocolChatCompletions {
		t.Fatalf("recommended protocol = %q, want chat completions", check.RecommendedProtocol)
	}
	settings, err := service.GetCodexRelaySettings(ctx)
	if err != nil {
		t.Fatalf("GetCodexRelaySettings returned error: %v", err)
	}
	if len(settings.Profiles) != 1 || settings.Profiles[0].Protocol != CodexRelayProtocolAuto || settings.Profiles[0].DetectedProtocol != CodexRelayProtocolChatCompletions {
		t.Fatalf("settings = %#v, want auto profile with detected chat completions", settings)
	}

	response, err := service.OpenCodexRelayRequest(
		ctx,
		http.MethodPost,
		"/v1/responses",
		[]byte(`{"model":"deepseek-v4-flash","input":"hello","stream":false}`),
		http.Header{"Authorization": []string{"Bearer " + codexRelayLocalBearerToken}},
	)
	if err != nil {
		t.Fatalf("OpenCodexRelayRequest returned error: %v", err)
	}
	defer response.Body.Close()
	if got := gotPaths[len(gotPaths)-1]; got != "/v1/chat/completions" {
		t.Fatalf("runtime upstream path = %q, want /v1/chat/completions", got)
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
