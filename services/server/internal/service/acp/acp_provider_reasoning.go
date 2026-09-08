package acp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mediago-dev/mediago-drama/services/server/internal/service/shared"
)

// applyProviderReasoningConfig forwards an explicit API effort through the isolated
// process config. It does not borrow the account channel's ACP option catalogue.
func applyProviderReasoningConfig(config ProcessConfig, request agentRunRequest) (ProcessConfig, error) {
	provider, _, explicit := shared.SplitAgentModelRef(request.Model.Value)
	if !explicit || provider == "chatgpt" || request.Reasoning.Source != "providerReasoning" {
		return config, nil
	}
	effort, ok := strings.CutPrefix(request.Reasoning.Value, "provider:")
	if !ok {
		return ProcessConfig{}, fmt.Errorf("invalid provider reasoning selection")
	}
	switch effort {
	case "default", "none", "minimal", "low", "medium", "high", "xhigh":
	default:
		return ProcessConfig{}, fmt.Errorf("invalid provider reasoning effort %q", effort)
	}
	acpLog().Info("provider reasoning configured", "provider", provider, "reasoning_effort", effort)
	values := map[string]json.RawMessage{}
	if raw := strings.TrimSpace(config.Env["CODEX_CONFIG"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return ProcessConfig{}, fmt.Errorf("reading provider process config: %w", err)
		}
	}
	if values == nil {
		values = map[string]json.RawMessage{}
	}
	delete(values, "model_reasoning_effort")
	if effort != "default" {
		encoded, err := json.Marshal(effort)
		if err != nil {
			return ProcessConfig{}, err
		}
		values["model_reasoning_effort"] = encoded
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ProcessConfig{}, fmt.Errorf("encoding provider process config: %w", err)
	}
	config.Env = cloneProcessConfigEnv(config.Env)
	config.Env["CODEX_CONFIG"] = string(raw)
	return config, nil
}
