package app

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	instructionpack "github.com/mediago-dev/mediago-drama/packages/instructions/pkg/pack"
	mediamcp "github.com/mediago-dev/mediago-drama/packages/mcp/pkg/mcp"
	serviceproductionprofile "github.com/mediago-dev/mediago-drama/services/server/internal/service/productionprofile"
	serviceprompt "github.com/mediago-dev/mediago-drama/services/server/internal/service/prompt"
	servicepromptpack "github.com/mediago-dev/mediago-drama/services/server/internal/service/promptpack"
	serviceskill "github.com/mediago-dev/mediago-drama/services/server/internal/service/skill"
)

type productionConfigLoaderStub struct {
	config mediamcp.ProjectConfig
	err    error
}

func (stub productionConfigLoaderStub) LoadProjectConfig(string) (mediamcp.ProjectConfig, error) {
	return stub.config, stub.err
}

func TestProductionPlanningContextSkipsMissingAndUnknownProfiles(t *testing.T) {
	registry := serviceproductionprofile.MustBuiltinRegistry()
	if got := productionPlanningContext(productionConfigLoaderStub{}, registry, "project-a"); got != "" {
		t.Fatalf("empty profile context = %q, want empty", got)
	}
	loader := productionConfigLoaderStub{config: mediamcp.ProjectConfig{
		Production: mediamcp.ProjectProductionConfig{ProfileID: "unknown-mode", TargetDurationSeconds: 900},
	}}
	if got := productionPlanningContext(loader, registry, "project-a"); got != "" {
		t.Fatalf("unknown profile context = %q, want empty", got)
	}
	if got := productionPlanningContext(productionConfigLoaderStub{err: fmt.Errorf("missing")}, registry, "project-a"); got != "" {
		t.Fatalf("load error context = %q, want empty", got)
	}
}

func TestProductionPlanningContextInjectsRegisteredProfileIntoFixedPrompt(t *testing.T) {
	registry, err := serviceproductionprofile.NewRegistryFromJSON([]byte(`{
		"schemaVersion":1,
		"profiles":[{
			"id":"mode-a","label":"测试模式","description":"fixture","version":1,
			"preferredShotDurationSeconds":{"min":4,"target":6,"max":10},
			"planning":{"targetShotsPerMinute":12,"continuityStrength":0.9}
		}]
	}`))
	if err != nil {
		t.Fatalf("NewRegistryFromJSON() error = %v", err)
	}
	loader := productionConfigLoaderStub{config: mediamcp.ProjectConfig{
		Production: mediamcp.ProjectProductionConfig{ProfileID: "mode-a", TargetDurationSeconds: 900},
	}}
	context := productionPlanningContext(loader, registry, "project-a")
	for _, want := range []string{"# Production Planning Context", "测试模式 (mode-a)", "项目目标时长：900 秒", "连续性强度：0.9"} {
		if !strings.Contains(context, want) {
			t.Fatalf("production context missing %q:\n%s", want, context)
		}
	}
	prompt := serviceprompt.BuildACPPrompt(agentRunRequest{ProjectID: "project-a"}, serviceprompt.PromptBuildOptions{
		MaxSectionChars:   12000,
		ProductionContext: context,
	})
	if !strings.Contains(prompt, context) {
		t.Fatalf("fixed prompt does not include production context:\n%s", prompt)
	}
}

func TestPromptSkillIndexUsesSharedWorkspaceRegistry(t *testing.T) {
	registry := serviceskill.NewRegistryWithStore(sharedSkillIndexStore{
		entries: []servicepromptpack.Entry{{
			Kind:        instructionpack.KindSkill,
			Slug:        "shot-schema",
			Name:        "shot-schema",
			Description: "拆分镜头并写入 Shot Schema",
		}},
	})
	codex := promptBuildOptionsWithSkillRegistry(agentRunRequest{
		ProjectID: "project-a",
		Model:     agentACPConfigSelection{Value: "gpt-5.6"},
	}, 12000, registry)
	deepseek := promptBuildOptionsWithSkillRegistry(agentRunRequest{
		ProjectID: "project-a",
		Model:     agentACPConfigSelection{Value: "deepseek/deepseek-chat"},
	}, 12000, registry)
	if len(codex.Skills) != 1 || codex.Skills[0].Name != "shot-schema" {
		t.Fatalf("codex skills = %#v, want shared workspace skill index", codex.Skills)
	}
	if !reflect.DeepEqual(codex.Skills, deepseek.Skills) {
		t.Fatalf("Codex and DeepSeek skill indexes diverged: %#v vs %#v", codex.Skills, deepseek.Skills)
	}
	prompt := serviceprompt.BuildACPPrompt(agentRunRequest{ProjectID: "project-a"}, codex)
	if !strings.Contains(prompt, "# 可用 Skills") || !strings.Contains(prompt, "`shot-schema`：拆分镜头并写入 Shot Schema") {
		t.Fatalf("fixed prompt missing shared skill index:\n%s", prompt)
	}
}

type sharedSkillIndexStore struct {
	entries []servicepromptpack.Entry
}

func (store sharedSkillIndexStore) ListEntries(_ context.Context, kind instructionpack.Kind) ([]servicepromptpack.Entry, error) {
	if kind != instructionpack.KindSkill {
		return nil, nil
	}
	return append([]servicepromptpack.Entry(nil), store.entries...), nil
}

func (store sharedSkillIndexStore) GetEntry(context.Context, instructionpack.Kind, string) (servicepromptpack.Entry, error) {
	return servicepromptpack.Entry{}, servicepromptpack.ErrEntryNotFound
}

func (store sharedSkillIndexStore) SaveEntry(context.Context, instructionpack.Kind, string, servicepromptpack.Entry) (servicepromptpack.Entry, error) {
	return servicepromptpack.Entry{}, servicepromptpack.ErrEntryNotFound
}

func (store sharedSkillIndexStore) CreateEntry(context.Context, instructionpack.Kind, servicepromptpack.Entry) (servicepromptpack.Entry, error) {
	return servicepromptpack.Entry{}, servicepromptpack.ErrEntryExists
}

func (store sharedSkillIndexStore) ResetEntry(context.Context, instructionpack.Kind, string) (servicepromptpack.Entry, error) {
	return servicepromptpack.Entry{}, servicepromptpack.ErrEntryNotFound
}

func (store sharedSkillIndexStore) DeleteEntry(context.Context, instructionpack.Kind, string) error {
	return servicepromptpack.ErrEntryNotFound
}

func (store sharedSkillIndexStore) HideEntry(context.Context, instructionpack.Kind, string) error {
	return servicepromptpack.ErrEntryNotFound
}
