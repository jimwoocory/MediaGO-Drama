package document

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mediamcp "github.com/mediago-dev/mediago-drama/packages/mcp/pkg/mcp"
)

func TestWorkspaceStateServiceProjectConfig(t *testing.T) {
	store := newWorkspaceStateService(t, t.TempDir())
	if store.initErr != nil {
		t.Fatalf("initializing workspace store: %v", store.initErr)
	}
	projectID := "config-project"
	projectDir := requireTestProject(t, store, projectID)

	config, err := store.LoadProjectConfig(projectID)
	if err != nil {
		t.Fatalf("LoadProjectConfig returned error: %v", err)
	}
	if config.SchemaVersion != 1 ||
		config.ProjectID != projectID ||
		config.Name != projectID ||
		config.Description != "" ||
		len(config.Overview.CategoryDefaults) != 0 ||
		config.Production.SchemaVersion != 1 ||
		config.Production.ProfileID != "" ||
		config.Production.TargetDurationSeconds != 0 ||
		config.CreatedAt == "" {
		t.Fatalf("config = %#v, want minimal project config", config)
	}

	categoryDefaults := map[string]string{"extra": "video-cinematic-shot", "style": "realistic"}
	result, err := store.SaveProjectConfigPatchInput(projectID, mediamcp.ProjectConfigPatchInput{
		Overview: &mediamcp.ProjectOverviewConfigPatch{CategoryDefaults: categoryDefaults},
	})
	if err != nil {
		t.Fatalf("SaveProjectConfigPatchInput returned error: %v", err)
	}
	if !result.Changed ||
		result.Config.Overview.CategoryDefaults["extra"] != "video-cinematic-shot" ||
		result.Config.Overview.CategoryDefaults["style"] != "" {
		t.Fatalf("result = %#v, want changed category defaults", result)
	}

	profileID := "mode-a"
	targetDuration := 900.0
	productionResult, err := store.SaveProjectConfigPatchInput(projectID, mediamcp.ProjectConfigPatchInput{
		Production: &mediamcp.ProjectProductionConfigPatch{
			ProfileID:             &profileID,
			TargetDurationSeconds: &targetDuration,
		},
	})
	if err != nil {
		t.Fatalf("saving production config returned error: %v", err)
	}
	if !productionResult.Changed ||
		productionResult.Config.Production.SchemaVersion != 1 ||
		productionResult.Config.Production.ProfileID != "mode-a" ||
		productionResult.Config.Production.TargetDurationSeconds != 900 {
		t.Fatalf("production result = %#v, want persisted production config", productionResult)
	}

	data, err := os.ReadFile(filepath.Join(projectDir, "project.media.json"))
	if err != nil {
		t.Fatalf("reading project manifest: %v", err)
	}
	rawManifest := map[string]any{}
	if err := json.Unmarshal(data, &rawManifest); err != nil {
		t.Fatalf("decoding raw manifest: %v", err)
	}
	if _, ok := rawManifest["directories"]; ok {
		t.Fatalf("manifest should not include directories: %s", string(data))
	}
	if _, ok := rawManifest["updatedAt"]; ok {
		t.Fatalf("manifest should not include updatedAt: %s", string(data))
	}
	overview, ok := rawManifest["overview"].(map[string]any)
	defaults, ok := overview["categoryDefaults"].(map[string]any)
	if !ok || defaults["extra"] != "video-cinematic-shot" || defaults["style"] != nil {
		t.Fatalf("overview = %#v, want category defaults persisted", rawManifest["overview"])
	}
	production, ok := rawManifest["production"].(map[string]any)
	if !ok || production["schemaVersion"] != float64(1) || production["profileId"] != "mode-a" || production["targetDurationSeconds"] != float64(900) {
		t.Fatalf("production = %#v, want production config persisted", rawManifest["production"])
	}

	reloaded, err := store.LoadProjectConfig(projectID)
	if err != nil {
		t.Fatalf("reloading project config returned error: %v", err)
	}
	if reloaded.Production.ProfileID != "mode-a" || reloaded.Production.TargetDurationSeconds != 900 {
		t.Fatalf("reloaded production = %#v, want saved values", reloaded.Production)
	}

	negativeDuration := -1.0
	if _, err := store.SaveProjectConfigPatchInput(projectID, mediamcp.ProjectConfigPatchInput{
		Production: &mediamcp.ProjectProductionConfigPatch{TargetDurationSeconds: &negativeDuration},
	}); err == nil {
		t.Fatal("negative targetDurationSeconds should be rejected")
	}

	empty, err := store.SaveProjectConfigPatchInput(projectID, mediamcp.ProjectConfigPatchInput{})
	if err != nil {
		t.Fatalf("empty SaveProjectConfigPatchInput returned error: %v", err)
	}
	if empty.Changed ||
		empty.Config.Overview.CategoryDefaults["extra"] != "video-cinematic-shot" ||
		empty.Config.Production.ProfileID != "mode-a" ||
		empty.Config.Production.TargetDurationSeconds != 900 {
		t.Fatalf("empty result = %#v, want unchanged category defaults", empty)
	}
}
