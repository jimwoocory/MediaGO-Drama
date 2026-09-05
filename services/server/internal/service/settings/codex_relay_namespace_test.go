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

func TestCodexRelayNamespaceToolsRoundTrip(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request codexChatRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if len(request.Tools) != 3 || request.Tools[0].Function.Name != "mcp_a__read" || request.Tools[1].Function.Name != "mcp_b__read" || request.Tools[2].Function.Name != "shell_command" {
					t.Errorf("namespace tools missing or collided: %#v", request.Tools)
				}
				if request.Messages[0].ToolCalls[0].Function.Name != "mcp_a__read" {
					t.Errorf("history lost namespace")
				}
				choice, _ := json.Marshal(request.ToolChoice)
				if !strings.Contains(string(choice), "mcp_b__read") {
					t.Errorf("forced tool lost namespace: %s", choice)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"fixture","choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_b","type":"function","function":{"name":"mcp_b__read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
			}))
			defer upstream.Close()
			service := newChatRelayTestSettings(t, upstream.URL+"/v1")
			body := []byte(fmt.Sprintf(`{"model":"model","stream":%t,"input":[{"type":"function_call","namespace":"mcp_a","name":"read","arguments":"{}","call_id":"call_a"},{"type":"function_call_output","call_id":"call_a","output":"fixture"}],"tools":[{"type":"namespace","name":"mcp_a","tools":[{"type":"function","name":"read","parameters":{"type":"object"}}]},{"type":"namespace","name":"mcp_b","tools":[{"type":"function","name":"read","parameters":{"type":"object"}}]},{"type":"function","name":"shell_command","parameters":{"type":"object"}}],"tool_choice":{"type":"function","namespace":"mcp_b","name":"read"}}`, stream))
			response, err := service.OpenCodexRelayRequest(context.Background(), http.MethodPost, "/v1/responses", body, localRelayHeaders())
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`"namespace":"mcp_b"`, `"name":"read"`, `"call_id":"call_b"`} {
				if !strings.Contains(string(data), want) {
					t.Errorf("response missing %s: %s", want, data)
				}
			}
		})
	}
}

func TestCodexRelayNamespaceLongNamesRemainUnique(t *testing.T) {
	namespace := strings.Repeat("provider_", 12)
	a, b := codexChatToolName(namespace, "read"), codexChatToolName(namespace, "write")
	if len(a) > 64 || len(b) > 64 || a == b {
		t.Fatalf("invalid tool aliases: %s %s", a, b)
	}
}
