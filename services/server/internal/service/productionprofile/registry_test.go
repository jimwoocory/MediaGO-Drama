package productionprofile

import (
	"strings"
	"testing"
)

func TestBuiltinProductionProfileManifestIncludesSixModes(t *testing.T) {
	registry, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry() error = %v", err)
	}
	profiles := registry.List()
	if got := len(profiles); got != 6 {
		t.Fatalf("builtin profile count = %d, want 6", got)
	}
	if _, ok := registry.Get("animation"); !ok {
		t.Fatal("missing animation profile")
	}
}

func TestProductionProfileRegistryResolvesPlanningDirective(t *testing.T) {
	registry, err := NewRegistryFromJSON([]byte(`{
		"schemaVersion":1,
		"profiles":[{
			"id":"cinematic-a",
			"label":"测试模式",
			"description":"fixture",
			"version":1,
			"preferredShotDurationSeconds":{"min":4,"target":6,"max":10},
			"planning":{"targetShotsPerMinute":12,"pacingIntensity":0.8,"dialogueWeight":0.7,"voiceoverWeight":0.2,"continuityStrength":0.9,"assetReuseBias":0.75}
		}]
	}`))
	if err != nil {
		t.Fatalf("NewRegistryFromJSON() error = %v", err)
	}
	directive, ok := registry.Resolve(" CINEMATIC-A ", 900)
	if !ok {
		t.Fatal("Resolve() ok = false, want true")
	}
	if directive.TargetDurationSeconds == nil || *directive.TargetDurationSeconds != 900 {
		t.Fatalf("target duration = %#v, want 900", directive.TargetDurationSeconds)
	}
	if directive.ShotCount == nil || directive.ShotCount.Min == nil || *directive.ShotCount.Min != 90 || directive.ShotCount.Target == nil || *directive.ShotCount.Target != 180 || directive.ShotCount.Max == nil || *directive.ShotCount.Max != 225 {
		t.Fatalf("shot count = %#v, want 90/180/225", directive.ShotCount)
	}
	for _, want := range []string{
		"项目目标时长：900 秒；这是编辑目标，不是硬上限。",
		"不得把 Provider 单次生成时长限制写回 ProductionShot",
		"镜头密度：12 镜头/分钟",
		"连续性强度：0.9",
	} {
		if !strings.Contains(directive.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, directive.Prompt)
		}
	}
}

func TestProductionProfileRegistryRejectsInvalidRules(t *testing.T) {
	_, err := NewRegistryFromJSON([]byte(`{
		"schemaVersion":1,
		"profiles":[{"id":"bad","label":"Bad","description":"fixture","version":1,"planning":{"pacingIntensity":1.5}}]
	}`))
	if err == nil || !strings.Contains(err.Error(), "pacingIntensity must be between 0 and 1") {
		t.Fatalf("error = %v, want pacing validation", err)
	}
}
