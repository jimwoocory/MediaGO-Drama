package generation

import (
	"context"
	"testing"

	coregeneration "github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
	"github.com/mediago-dev/mediago-drama/services/server/internal/service/settings"
)

type videoAPIAppSettingStore struct {
	values map[string]string
}

func (store *videoAPIAppSettingStore) GetAppSetting(key string) (string, bool, error) {
	value, ok := store.values[key]
	return value, ok, nil
}

func (store *videoAPIAppSettingStore) SetAppSetting(key string, value string) error {
	if store.values == nil {
		store.values = map[string]string{}
	}
	store.values[key] = value
	return nil
}

func (store *videoAPIAppSettingStore) ClearAppSetting(key string) error {
	delete(store.values, key)
	return nil
}

func TestListGenerationModelsIncludesConfiguredVideoAPI(t *testing.T) {
	settingsSvc := settings.NewSettingsWithStores(
		&generationTestAPIKeyStore{values: map[string]string{
			coregeneration.ProviderVideoAPI: "sk-video",
		}},
		nil,
		&videoAPIAppSettingStore{values: map[string]string{}},
	)
	if _, err := settingsSvc.SetVideoAPISettings(context.Background(), settings.VideoAPISettings{
		BaseURL: "https://video.example.test/v1",
		Model:   "seedance-custom",
	}); err != nil {
		t.Fatalf("SetVideoAPISettings() error = %v", err)
	}

	catalog := NewGenerationService(settingsSvc, nil, nil).ListGenerationModels()
	route, ok := findVideoAPIRoute(catalog)
	if !ok {
		t.Fatalf("catalog is missing route %q", coregeneration.RouteVideoAPICompatible)
	}
	if !route.Configured {
		t.Fatalf("route %q configured = false, want true", route.ID)
	}
	if route.Model != "seedance-custom" {
		t.Fatalf("route model = %q, want seedance-custom", route.Model)
	}

	foundVersion := false
	for _, version := range catalog.Versions {
		if version.FamilyID != coregeneration.FamilyVideoAPI {
			continue
		}
		foundVersion = true
		if version.Label != "seedance-custom" || version.CanonicalModel != "seedance-custom" {
			t.Fatalf("video API version = %#v", version)
		}
	}
	if !foundVersion {
		t.Fatal("catalog is missing the Video API version")
	}
}

func TestListGenerationModelsKeepsVideoAPIHiddenUntilFullyConfigured(t *testing.T) {
	settingsSvc := settings.NewSettingsWithStores(
		&generationTestAPIKeyStore{values: map[string]string{
			coregeneration.ProviderVideoAPI: "sk-video",
		}},
		nil,
		&videoAPIAppSettingStore{values: map[string]string{}},
	)
	catalog := NewGenerationService(settingsSvc, nil, nil).ListGenerationModels()
	route, ok := findVideoAPIRoute(catalog)
	if !ok {
		t.Fatalf("catalog is missing route %q", coregeneration.RouteVideoAPICompatible)
	}
	if route.Configured {
		t.Fatalf("route %q configured = true without Base URL / Model", route.ID)
	}
}

func findVideoAPIRoute(catalog GenerationModelsResponse) (coregeneration.ModelRoute, bool) {
	for _, route := range catalog.Routes {
		if route.ID == coregeneration.RouteVideoAPICompatible {
			return route, true
		}
	}
	return coregeneration.ModelRoute{}, false
}
