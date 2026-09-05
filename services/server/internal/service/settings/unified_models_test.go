package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestUnifiedClassificationUsesOutputNotVisionInput(t *testing.T) {
	for _, tc := range []struct{ raw, kind, protocol string }{
		{`{"id":"vision-only","input_modalities":["text","image"],"output_modalities":["text"]}`, "unknown", ""},
		{`{"id":"custom-picture","output_modalities":["image"],"supported_endpoint_types":["/v1/chat/completions"]}`, "image", "chat-image"},
		{`{"id":"custom-voice","supported_endpoint_types":["/audio/speech"]}`, "audio", "speech"},
		{`{"id":"custom-video","supported_endpoint_types":["videos"]}`, "video", "videos"},
		{`{"id":"GPT-Image-2"}`, "image", "images"},
		{`{"id":"gemini-image-preview"}`, "image", "chat-image"},
	} {
		var item openAIModelListItem
		if err := json.Unmarshal([]byte(tc.raw), &item); err != nil {
			t.Fatal(err)
		}
		got := classifyUnifiedModel(item)
		if len(got) != 1 || got[0].Kind != tc.kind || got[0].Protocol != tc.protocol {
			t.Errorf("%s => %#v", tc.raw, got)
		}
	}
}

func TestUnifiedOpenRouterAndMultipleOutputs(t *testing.T) {
	var item openAIModelListItem
	if err := json.Unmarshal([]byte(`{"id":"Custom/Multi","architecture":{"output_modalities":["image","audio","video"]}}`), &item); err != nil {
		t.Fatal(err)
	}
	models := classifyUnifiedModelAtEndpoint(item, "https://openrouter.ai/api/v1")
	if len(models) != 3 {
		t.Fatalf("lost output capabilities: %#v", models)
	}
	for _, model := range models {
		if model.Kind == "image" && model.Protocol != "chat-image" {
			t.Fatal("wrong OpenRouter image wire protocol")
		}
	}
	if !openAIModelHasOnlyMediaOutput(item) {
		t.Fatal("media-only model would leak into Agent list")
	}
}

func TestUnifiedDiscoveryOverridesRefreshAndCredentialIsolation(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if fail.Load() {
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"gpt-image-2"},{"id":"tts-1"},{"id":"sora-2"},{"id":"text-only"}]}`)
	}))
	defer server.Close()
	keys := &memoryAPIKeyStore{values: map[string]string{"aihubmix": "test-key-one"}}
	store := &memoryAppSettingStore{values: map[string]string{}}
	service := NewSettingsWithStores(keys, nil, store)
	ctx := context.Background()
	if _, err := service.SetAIHubMixSettings(ctx, AIHubMixSettings{BaseURL: server.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	models, err := service.ListUnifiedModels(ctx, false)
	if err != nil || len(models.Models) != 4 {
		t.Fatalf("discovery: %#v %v", models, err)
	}
	for i := 0; i < 5; i++ {
		if _, err := service.ListUnifiedModels(ctx, false); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("discovery repeated %d times", calls.Load())
	}
	if _, err := service.SetUnifiedModel(ctx, UnifiedModel{ID: "gpt-image-2", Protocol: "chat-image", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	models, err = service.ListUnifiedModels(ctx, true)
	if err != nil || models.Warning == "" {
		t.Fatal("refresh failure was hidden")
	}
	for _, m := range models.Models {
		if m.ID == "gpt-image-2" && (m.Protocol != "chat-image" || m.Source != "manual") {
			t.Errorf("override lost: %#v", m)
		}
	}
	if _, err := service.ListUnifiedModels(ctx, false); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("failed discovery hammered upstream")
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := service.SetUnifiedModel(ctx, UnifiedModel{ID: fmt.Sprintf("custom-%d", i), Protocol: "images", Enabled: true}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	models, _ = service.ListUnifiedModels(ctx, false)
	if len(models.Models) != 14 {
		t.Errorf("concurrent overrides lost: %d", len(models.Models))
	}
	for key, value := range store.values {
		if strings.Contains(key+value, "test-key-one") {
			t.Fatal("secret leaked in model cache")
		}
	}
	if err := keys.Set("aihubmix", "test-key-two"); err != nil {
		t.Fatal(err)
	}
	models, err = service.ListUnifiedModels(ctx, false)
	if err != nil || len(models.Models) != 0 {
		t.Fatalf("old account models leaked: %#v %v", models, err)
	}
	if err := keys.Clear("aihubmix"); err != nil {
		t.Fatal(err)
	}
	models, _ = service.ListUnifiedModels(ctx, false)
	if len(models.Models) != 0 {
		t.Fatal("models survived credential clear")
	}
}
