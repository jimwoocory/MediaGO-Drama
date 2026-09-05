package settings

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// writeAgentProviderModelCatalog describes the bridge contract, not unverified
// upstream capabilities. Unknown context sizes use a conservative 32K budget.
// Effort levels describe optional bridge request parameters; the upstream decides
// which levels its model implements. No effort is selected by default.
func writeAgentProviderModelCatalog(home, model string, contextWindows ...int) (string, error) {
	window := defaultAgentContextWindow
	if len(contextWindows) > 0 {
		window = contextWindows[0]
	}
	return writeAgentProviderModelCatalogWithReasoning(home, model, window, false)
}

func writeAgentProviderModelCatalogWithReasoning(home, model string, window int, enabled bool) (string, error) {
	levels := []map[string]string{}
	if enabled {
		for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh"} {
			levels = append(levels, map[string]string{"effort": effort, "description": "Requested effort: " + effort})
		}
	}
	catalog := map[string]any{"models": []map[string]any{{
		"slug": model, "display_name": model, "description": "JW Drama third-party text/tool compatibility profile",
		"default_reasoning_level": nil, "supported_reasoning_levels": levels,
		"shell_type": "shell_command", "visibility": "list", "supported_in_api": true, "priority": 1,
		"base_instructions": "You are the JW Drama project assistant. Follow the workspace instructions. Use the provided tools when needed and respect approval requirements.",
		// The bundled ACP uses this legacy flag to gate effort emission too.
		// Summary generation remains disabled by model_reasoning_summary = none.
		"supports_reasoning_summaries": enabled, "support_verbosity": false,
		"supports_parallel_tool_calls": false, "default_reasoning_summary": "none",
		"apply_patch_tool_type": nil, "web_search_tool_type": "text",
		"truncation_policy": map[string]any{"mode": "tokens", "limit": 6000},
		"context_window":    window, "effective_context_window_percent": 95,
		"input_modalities": []string{"text"}, "experimental_supported_tools": []string{},
		"supports_search_tool": false, "use_responses_lite": false,
	}}}
	raw, err := json.Marshal(catalog)
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, "jw-model-catalog.json")
	if err := writeCodexRuntimeFile(path, raw); err != nil {
		return "", fmt.Errorf("writing provider model catalog: %w", err)
	}
	return path, nil
}

const defaultAgentContextWindow = 32768

func agentModelContextWindow(models []openAIModelListItem, model string) int {
	for _, item := range models {
		if item.ID != model {
			continue
		}
		window := 0
		// When both aliases are present, use the smaller declared limit.
		for _, candidate := range []int{int(item.ContextLength), int(item.ContextWindow)} {
			if candidate >= 4096 && candidate <= 2_000_000 && (window == 0 || candidate < window) {
				window = candidate
			}
		}
		if window != 0 {
			return window
		}
	}
	return defaultAgentContextWindow
}

func agentModelCompactLimit(window int) int {
	// Reserve output room (4K, or one quarter for small models) inside the
	// effective 95% window, independently of the compaction percentage.
	return min(window*80/100, window*95/100-min(4096, window/4))
}
