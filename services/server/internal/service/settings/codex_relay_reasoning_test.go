package settings

import (
	"encoding/json"
	"testing"
)

func TestResponsesReasoningEffortReachesChatCompletions(t *testing.T) {
	for _, effort := range []string{"", "none", "minimal", "low", "medium", "high", "xhigh"} {
		t.Run("effort-"+effort, func(t *testing.T) {
			request := map[string]any{"model": "vendor/model:variant", "input": "fixture", "stream": true}
			if effort != "" {
				request["reasoning"] = map[string]string{"effort": effort, "summary": "auto"}
			}
			body, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			raw, stream, err := codexRelayResponsesToChatRequest(body, "")
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			want := `"` + effort + `"`
			if effort == "" {
				want = ""
			}
			if !stream || string(result["reasoning_effort"]) != want || string(result["model"]) != `"vendor/model:variant"` || result["reasoning"] != nil {
				t.Fatalf("unexpected chat request %s", raw)
			}
		})
	}
}
