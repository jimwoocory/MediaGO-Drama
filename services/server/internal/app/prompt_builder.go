package app

import (
	"context"
	"log/slog"
	"strings"

	mediamcp "github.com/mediago-dev/mediago-drama/packages/mcp/pkg/mcp"
	serviceproductionprofile "github.com/mediago-dev/mediago-drama/services/server/internal/service/productionprofile"
	serviceprompt "github.com/mediago-dev/mediago-drama/services/server/internal/service/prompt"
	serviceskill "github.com/mediago-dev/mediago-drama/services/server/internal/service/skill"
)

type projectProductionConfigLoader interface {
	LoadProjectConfig(projectID string) (mediamcp.ProjectConfig, error)
}

func buildACPPrompt(request agentRunRequest) string {
	return buildACPPromptWithMaxSectionChars(request, 0)
}

func buildACPPromptWithMaxSectionChars(request agentRunRequest, maxSectionChars int) string {
	return serviceprompt.BuildACPPrompt(request, promptBuildOptionsWithSkillRegistry(request, maxSectionChars, nil))
}

func promptBuildOptionsWithMaxSectionChars(request agentRunRequest, maxSectionChars int) serviceprompt.PromptBuildOptions {
	return promptBuildOptionsWithSkillRegistry(request, maxSectionChars, nil)
}

func promptBuildOptionsWithSkillRegistry(_ agentRunRequest, maxSectionChars int, registry *serviceskill.Registry) serviceprompt.PromptBuildOptions {
	if registry == nil {
		registry = serviceskill.NewRegistry()
	}
	skills, err := registry.List(context.Background())
	if err != nil {
		slog.Warn("agent skill index unavailable", "error", err)
	}
	descriptors := make([]serviceprompt.SkillDescriptor, 0, len(skills))
	for _, skill := range skills {
		descriptors = append(descriptors, serviceprompt.SkillDescriptor{
			Name:        skill.Name,
			Description: skill.Description,
		})
	}
	return serviceprompt.PromptBuildOptions{
		MaxSectionChars: maxSectionChars,
		Skills:          descriptors,
	}
}

func productionPlanningContext(
	loader projectProductionConfigLoader,
	registry *serviceproductionprofile.Registry,
	projectID string,
) string {
	projectID = strings.TrimSpace(projectID)
	if loader == nil || registry == nil || projectID == "" {
		return ""
	}
	config, err := loader.LoadProjectConfig(projectID)
	if err != nil {
		slog.Debug("production profile project config unavailable", "project_id", projectID, "error", err)
		return ""
	}
	profileID := strings.TrimSpace(config.Production.ProfileID)
	if profileID == "" {
		return ""
	}
	directive, ok := registry.Resolve(profileID, config.Production.TargetDurationSeconds)
	if !ok {
		slog.Warn("project production profile is not registered", "project_id", projectID, "profile_id", profileID)
		return ""
	}
	return strings.TrimSpace(directive.Prompt)
}
