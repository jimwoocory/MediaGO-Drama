package acp

import (
	"strings"
	"testing"
)

func TestPlanProgressContractUsesBothInstructionDeliveryPaths(t *testing.T) {
	runner := &acpAgentRunner{buildPrompt: func(AgentRunRequest) string { return "写作约束" }}
	request := agentRunRequest{Prompt: "继续剧本"}
	fixed := runner.fixedInstructions(request)
	if !strings.Contains(fixed, acpPlanProgressInstructions) || !strings.HasPrefix(fixed, "写作约束") {
		t.Fatal("native instructions must include both writing and plan reporting contracts")
	}
	if prompt := runner.buildPromptForRequest(request, fixed, false); !strings.Contains(prompt, acpPlanProgressInstructions) {
		t.Fatal("inline delivery lost plan reporting contract")
	}
	if prompt := runner.buildPromptForRequest(request, fixed, true); prompt != request.Prompt {
		t.Fatal("native delivery must not duplicate instructions into user input")
	}
	if instructionFingerprint("codex", "native", fixed) == instructionFingerprint("codex", "native", "写作约束") {
		t.Fatal("old resident sessions must not retain instructions without plan reporting")
	}
}
