package videoapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
)

func TestProviderCreatesAndPollsVideo(t *testing.T) {
	var createModel string
	var createPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer sk-video" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1/videos":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm() error = %v", err)
			}
			createModel = request.FormValue("model")
			createPrompt = request.FormValue("prompt")
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": "video-task-1", "status": "queued", "model": createModel,
			})
		case request.Method == http.MethodGet && request.URL.Path == "/v1/videos/video-task-1":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": "video-task-1", "status": "succeeded", "model": "seedance-custom", "video_url": "https://example.test/result.mp4",
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	provider, err := NewProvider(Config{BaseURL: server.URL + "/v1", APIKey: "sk-video", Model: "seedance-custom"})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	created, err := provider.Generate(context.Background(), generation.Request{Kind: generation.KindVideo, Prompt: "camera pushes in"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if createModel != "seedance-custom" || createPrompt != "camera pushes in" {
		t.Fatalf("create form model/prompt = %q/%q", createModel, createPrompt)
	}
	if created.ID != "video-task-1" || created.Status != "submitted" {
		t.Fatalf("created = %#v", created)
	}

	completed, err := provider.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if completed.Status != "completed" || len(completed.Assets) != 1 || completed.Assets[0].URL != "https://example.test/result.mp4" {
		t.Fatalf("completed = %#v", completed)
	}
}

func TestProviderRejectsReferences(t *testing.T) {
	provider, err := NewProvider(Config{BaseURL: "https://example.test/v1", APIKey: "sk-video", Model: "video-model"})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	_, err = provider.Generate(context.Background(), generation.Request{
		Kind: generation.KindVideo, Prompt: "animate", ReferenceURLs: []string{"https://example.test/ref.png"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not support reference URLs") {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestProviderAcceptsFullVideosEndpoint(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		if request.Method == http.MethodPost {
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": "video-task-2", "status": "queued",
			})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"id": "video-task-2", "status": "succeeded", "video_url": "https://example.test/result.mp4",
		})
	}))
	defer server.Close()

	provider, err := NewProvider(Config{
		BaseURL: server.URL + "/v1/videos/",
		APIKey:  "sk-video",
		Model:   "seedance-custom",
	})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	created, err := provider.Generate(context.Background(), generation.Request{
		Kind: generation.KindVideo, Prompt: "camera pushes in",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if _, err := provider.Get(context.Background(), created.ID); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := strings.Join(paths, ","), "/v1/videos,/v1/videos/video-task-2"; got != want {
		t.Fatalf("request paths = %q, want %q", got, want)
	}
}
