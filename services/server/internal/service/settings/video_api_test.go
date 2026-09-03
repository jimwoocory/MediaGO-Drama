package settings

import (
	"context"
	"testing"
)

func TestVideoAPISettingsStoresBaseURLAndModel(t *testing.T) {
	service := NewSettingsWithStores(
		&memoryAPIKeyStore{values: map[string]string{}},
		nil,
		&memoryAppSettingStore{values: map[string]string{}},
	)

	saved, err := service.SetVideoAPISettings(context.Background(), VideoAPISettings{
		BaseURL: "https://video.example.test/v1/videos",
		Model:   "seedance-custom",
	})
	if err != nil {
		t.Fatalf("SetVideoAPISettings() error = %v", err)
	}
	if saved.BaseURL != "https://video.example.test/v1" || saved.Model != "seedance-custom" {
		t.Fatalf("saved = %#v", saved)
	}

	loaded, err := service.GetVideoAPISettings(context.Background())
	if err != nil {
		t.Fatalf("GetVideoAPISettings() error = %v", err)
	}
	if loaded != saved {
		t.Fatalf("loaded = %#v, want %#v", loaded, saved)
	}
}

func TestVideoAPISettingsRequiresModel(t *testing.T) {
	service := NewSettingsWithStores(
		&memoryAPIKeyStore{values: map[string]string{}},
		nil,
		&memoryAppSettingStore{values: map[string]string{}},
	)
	if _, err := service.SetVideoAPISettings(context.Background(), VideoAPISettings{
		BaseURL: "https://video.example.test/v1",
	}); err == nil {
		t.Fatal("SetVideoAPISettings() accepted an empty model")
	}
}
