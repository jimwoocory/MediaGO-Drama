package runtime

import (
	"context"
	"fmt"
	core "github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnifiedVideoPollSurvivesNewRuntime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer shared-key" {
			t.Error("wrong credential")
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/videos":
			fmt.Fprint(w, `{"id":"upstream-1","status":"queued"}`)
		case "GET /v1/videos/upstream-1":
			fmt.Fprint(w, `{"id":"upstream-1","status":"completed","url":"https://fixture.test/video.mp4"}`)
		default:
			t.Errorf("wrong request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	config := Config{UnifiedBaseURL: server.URL + "/v1", Credentials: CredentialResolverFunc(func(_ context.Context, key string) (string, error) {
		if key != core.ProviderUnified {
			t.Errorf("polled wrong provider %q", key)
			return "", fmt.Errorf("wrong provider")
		}
		return "shared-key", nil
	})}
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatal(err)
	}
	route, _ := core.UnifiedRoute("custom/sora", "videos")
	response, err := provider.Generate(context.Background(), core.Request{RouteID: route.ID, Kind: core.KindVideo, Prompt: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != route.ID+":upstream-1" {
		t.Fatalf("task lost route: %s", response.ID)
	}
	restarted, err := NewProvider(config)
	if err != nil {
		t.Fatal(err)
	}
	response, err = restarted.Get(context.Background(), response.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "completed" || len(response.Assets) != 1 {
		t.Fatalf("poll failed: %#v", response)
	}
}
