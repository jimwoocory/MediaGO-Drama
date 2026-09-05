package generation

import (
	"context"
	"fmt"
	core "github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
	"github.com/mediago-dev/mediago-drama/services/server/internal/service/settings"
	"net/http"
	"net/http/httptest"
	"testing"
)

type unifiedTestStore map[string]string

func (s unifiedTestStore) Get(k string) (string, string, error) { return s[k], "settings", nil }
func (s unifiedTestStore) Set(k, v string) error                { s[k] = v; return nil }
func (s unifiedTestStore) Clear(k string) error                 { delete(s, k); return nil }
func (s unifiedTestStore) GetAppSetting(k string) (string, bool, error) {
	v, ok := s[k]
	return v, ok, nil
}
func (s unifiedTestStore) SetAppSetting(k, v string) error { s[k] = v; return nil }
func (s unifiedTestStore) ClearAppSetting(k string) error  { delete(s, k); return nil }

func TestUnifiedWorkbenchCatalogAndDisabledRouteGuard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"gpt-image-2"},{"id":"tts-1"},{"id":"sora-2"}]}`)
	}))
	defer server.Close()
	store := unifiedTestStore{"aihubmix": "fake-key"}
	service := settings.NewSettingsWithStores(store, nil, store)
	if _, err := service.SetAIHubMixSettings(context.Background(), settings.AIHubMixSettings{BaseURL: server.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	workflow := NewGenerationService(service, nil, nil)
	catalog := workflow.ListGenerationModels()
	kinds := map[core.Kind]bool{}
	for _, route := range catalog.Routes {
		if route.Provider != core.ProviderUnified {
			continue
		}
		if !route.Configured {
			t.Errorf("workbench model disabled: %s", route.ID)
		}
		kinds[route.Kind] = true
		found := false
		for _, version := range catalog.Versions {
			if version.ID == route.VersionID {
				found = true
			}
		}
		if !found {
			t.Fatal("workbench version missing")
		}
		if _, err := workflow.newGenerationProvider(route); err != nil {
			t.Fatalf("route not executable: %v", err)
		}
	}
	if !kinds[core.KindImage] || !kinds[core.KindAudio] || !kinds[core.KindVideo] {
		t.Fatalf("missing workbench kinds: %v", kinds)
	}
	imageRoute, _ := core.UnifiedRoute("gpt-image-2", "images")
	if _, err := service.SetUnifiedModel(context.Background(), settings.UnifiedModel{ID: "gpt-image-2", Protocol: "images", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if workflow.RouteConfigured(imageRoute.ID) {
		t.Fatal("disabled model remained executable")
	}
	for _, route := range workflow.ListGenerationModels().Routes {
		if route.ID == imageRoute.ID {
			t.Fatal("disabled route remained in selector catalog")
		}
	}
	forged, _ := core.UnifiedRoute("not-discovered", "images")
	if _, err := workflow.newGenerationProvider(forged); err == nil {
		t.Fatal("forged route bypassed catalog")
	}
	if _, err := RouteForGenerationTaskID(imageRoute.ID + ":upstream-id"); err != nil {
		t.Fatal("persisted task no longer resolvable")
	}
}
