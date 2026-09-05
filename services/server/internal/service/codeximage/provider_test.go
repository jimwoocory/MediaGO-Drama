package codeximage

import (
	"context"
	"encoding/json"
	core "github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
	"github.com/mediago-dev/mediago-drama/services/server/internal/platform/codexapp"
	"io"
	"testing"
)

const pngResult = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j/p8AAAAASUVORK5CYII="

type fakeSession struct {
	t            *testing.T
	messages     []codexapp.Message
	closed       bool
	accountType  string
	imageEnabled bool
	started      bool
}

func (f *fakeSession) Call(_ context.Context, method string, input any, output any) error {
	raw := "{}"
	switch method {
	case "account/read":
		raw = `{"account":{"type":"` + f.accountType + `"}}`
	case "modelProvider/capabilities/read":
		if f.imageEnabled {
			raw = `{"imageGeneration":true}`
		}
	case "config/read":
		raw = `{"config":{"mcp_servers":{"unsafe_external":{"command":"unused"}}}}`
	case "model/list":
		raw = `{"data":[{"model":"official-test-model","isDefault":true}]}`
	case "thread/start":
		f.started = true
		params := input.(map[string]any)
		if params["modelProvider"] != "openai" || params["model"] != "official-test-model" || params["sandbox"] != "read-only" || params["approvalPolicy"] != "never" || params["ephemeral"] != true {
			f.t.Errorf("unsafe image session: %#v", params)
		}
		config := params["config"].(map[string]any)
		if config["mcp_servers.unsafe_external.enabled"] != false || config["features.shell_tool"] != false || config["features.apps"] != false || config["features.plugins"] != false {
			f.t.Error("unrelated tools not disabled")
		}
		raw = `{"thread":{"id":"thread-image"}}`
	case "turn/start":
		raw = `{"turn":{"id":"turn-image"}}`
	default:
		f.t.Errorf("unexpected method %s", method)
	}
	return json.Unmarshal([]byte(raw), output)
}
func (f *fakeSession) Next(context.Context) (codexapp.Message, error) {
	if len(f.messages) == 0 {
		return codexapp.Message{}, io.EOF
	}
	m := f.messages[0]
	f.messages = f.messages[1:]
	return m, nil
}
func (f *fakeSession) Close() { f.closed = true }
func notify(method string, value any) codexapp.Message {
	raw, _ := json.Marshal(value)
	return codexapp.Message{Method: method, Params: raw}
}

func TestCodexImageResultIsRealDeduplicatedAndIsolated(t *testing.T) {
	item := imageItem{ID: "image-1", Type: "imageGeneration", Status: "completed", Result: pngResult}
	fake := &fakeSession{t: t, accountType: "chatgpt", imageEnabled: true, messages: []codexapp.Message{
		notify("item/completed", map[string]any{"threadId": "unrelated", "turnId": "turn-image", "item": item}),
		notify("item/completed", map[string]any{"threadId": "thread-image", "turnId": "turn-image", "item": item}),
		notify("turn/completed", map[string]any{"threadId": "thread-image", "turn": map[string]any{"id": "turn-image", "status": "completed", "items": []imageItem{item}}}),
	}}
	provider := &Provider{StartSession: func(context.Context) (codexapp.Client, error) { return fake, nil }}
	result, err := provider.Generate(context.Background(), core.Request{Kind: core.KindImage, Prompt: "one test image"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 1 || result.Assets[0].Base64 != pngResult || result.Assets[0].MIMEType != "image/png" || result.Status != "completed" {
		t.Fatalf("invalid result %#v", result)
	}
	if !fake.closed {
		t.Fatal("session not closed")
	}
}

func TestCodexImageRejectsNoLoginCapabilityTextAndExtraPermissions(t *testing.T) {
	for _, tc := range []struct {
		name, account string
		enabled       bool
		messages      []codexapp.Message
	}{
		{"api-key-not-subscription", "apiKey", true, nil},
		{"no-image-capability", "chatgpt", false, nil},
		{"text-is-not-an-image", "chatgpt", true, []codexapp.Message{notify("turn/completed", map[string]any{"threadId": "thread-image", "turn": map[string]any{"id": "turn-image", "status": "completed", "items": []imageItem{}}})}},
		{"extra-permission", "chatgpt", true, []codexapp.Message{{Method: "item/commandExecution/requestApproval", ID: json.RawMessage("42")}}},
		{"bad-image-data", "chatgpt", true, []codexapp.Message{notify("item/completed", map[string]any{"threadId": "thread-image", "turnId": "turn-image", "item": imageItem{ID: "bad", Type: "imageGeneration", Status: "completed", Result: "not-an-image"}})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeSession{t: t, accountType: tc.account, imageEnabled: tc.enabled, messages: tc.messages}
			provider := &Provider{StartSession: func(context.Context) (codexapp.Client, error) { return fake, nil }}
			if _, err := provider.Generate(context.Background(), core.Request{Kind: core.KindImage, Prompt: "test"}); err == nil {
				t.Fatal("invalid success")
			}
			if !fake.closed {
				t.Fatal("session leaked")
			}
			if (tc.account != "chatgpt" || !tc.enabled) && fake.started {
				t.Fatal("started generation without eligibility")
			}
		})
	}
}
