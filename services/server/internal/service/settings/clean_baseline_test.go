package settings

import (
	"context"
	"testing"
)

func TestCleanBaselineHasNoEnabledModelPlatforms(t *testing.T) {
	settings := NewSettingsWithStores(
		&memoryAPIKeyStore{values: map[string]string{}},
		nil,
		&memoryAppSettingStore{values: map[string]string{}},
	)
	if ids := settings.ModelPlatformIDs(); len(ids) != 0 {
		t.Fatalf("ModelPlatformIDs() = %#v, want empty", ids)
	}
	if ids := settings.GenerationCLIProviderIDs(); len(ids) != 0 {
		t.Fatalf("GenerationCLIProviderIDs() = %#v, want empty", ids)
	}
	if platforms := settings.ListModelPlatforms(context.Background()).Platforms; len(platforms) != 0 {
		t.Fatalf("ListModelPlatforms() = %#v, want empty", platforms)
	}
}

func TestCleanBaselineExposesConfigurationEntriesWithoutEnablingProviders(t *testing.T) {
	settings := NewSettingsWithStores(
		&memoryAPIKeyStore{values: map[string]string{}},
		nil,
		&memoryAppSettingStore{values: map[string]string{}},
	)
	list, err := settings.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys() error = %v", err)
	}
	for _, providerID := range []string{
		agentModelProviderAIHubMix,
		"speechapi",
		"videoapi",
		"jimeng",
		"libtv",
		"xiaoyunque",
		"openai",
		"deepseek",
	} {
		provider := providerByID(t, list, providerID)
		if provider.Configured {
			t.Fatalf("provider %q = %#v, want visible but unconfigured", providerID, provider)
		}
	}
	if ids := settings.ModelPlatformIDs(); len(ids) != 0 {
		t.Fatalf("ModelPlatformIDs() = %#v, want empty", ids)
	}
	if ids := settings.GenerationCLIProviderIDs(); len(ids) != 0 {
		t.Fatalf("GenerationCLIProviderIDs() = %#v, want empty", ids)
	}
}
