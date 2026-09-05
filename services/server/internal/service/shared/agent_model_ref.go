package shared

import "strings"

// SplitAgentModelRef separates JW Drama's provider identity from an opaque model ID.
// Only JW-owned namespaces are recognized; slashes and colons in upstream IDs survive.
func SplitAgentModelRef(value string) (provider, model string, ok bool) {
	provider, model, ok = strings.Cut(strings.TrimSpace(value), ":")
	if !ok || model == "" || !(provider == "chatgpt" || strings.HasPrefix(provider, "api-") || strings.HasPrefix(provider, "gateway-")) {
		return "", strings.TrimSpace(value), false
	}
	return provider, model, true
}
