package unified

import (
	"context"
	"encoding/json"
	"fmt"
	core "github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSharedCredentialMediaProtocols(t *testing.T) {
	for _, tc := range []struct{ protocol, path, response, mime string }{
		{"images", "/v1/images/generations", `{"data":[{"b64_json":"aW1hZ2U="}]}`, "application/json"},
		{"chat-image", "/v1/chat/completions", `{"choices":[{"message":{"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,aW1hZ2U="}}]}}]}`, "application/json"},
		{"speech", "/v1/audio/speech", "audio-bytes", "audio/mpeg"},
		{"videos", "/v1/videos", `{"id":"task-1","status":"queued"}`, "application/json"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			model := "Vendor/Exact-Case:1"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.Method != "POST" {
					t.Errorf("wrong endpoint: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer one-key" {
					t.Error("shared credential not used")
				}
				if tc.protocol == "videos" {
					if err := r.ParseMultipartForm(1024 * 1024); err != nil {
						t.Error(err)
					}
					if r.FormValue("model") != model {
						t.Error("video model changed")
					}
				} else {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["model"] != model {
						t.Errorf("model changed: %v", body["model"])
					}
				}
				w.Header().Set("Content-Type", tc.mime)
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			route, _ := core.UnifiedRoute(model, tc.protocol)
			provider, err := NewProvider(Config{BaseURL: server.URL + "/v1", APIKey: "one-key", Route: route})
			if err != nil {
				t.Fatal(err)
			}
			response, err := provider.Generate(context.Background(), core.Request{Kind: route.Kind, RouteID: route.ID, Prompt: "test prompt"})
			if err != nil {
				t.Fatal(err)
			}
			if response.Model != model {
				t.Errorf("response model = %s", response.Model)
			}
			if tc.protocol == "speech" && response.ID != "" {
				t.Fatal("speech reused route ID as a task ID")
			}
			if tc.protocol == "videos" {
				if response.ID != route.ID+":task-1" {
					t.Fatal("missing video task")
				}
			} else if len(response.Assets) != 1 || response.Assets[0].Kind != route.Kind {
				t.Fatalf("missing media: %#v", response)
			}
		})
	}
}

func TestImageTextResponseIsNotSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"I generated your image"}}]}`)
	}))
	defer server.Close()
	route, _ := core.UnifiedRoute("custom-image", "chat-image")
	provider, _ := NewProvider(Config{BaseURL: server.URL, APIKey: "one-key", Route: route})
	if _, err := provider.Generate(context.Background(), core.Request{Kind: core.KindImage, Prompt: "test"}); err == nil {
		t.Fatal("text accepted as image")
	}
}
