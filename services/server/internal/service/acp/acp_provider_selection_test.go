package acp

import (
	"context"
	acp "github.com/coder/acp-go-sdk"
	"testing"
)

func TestProviderSelectionPassesOpaqueModelAndPreservesPermission(t *testing.T) {
	for _, provider := range []string{"api-tokease", "api-openrouter", "gateway-deepseek", "chatgpt"} {
		t.Run(provider, func(t *testing.T) {
			configurator := &recordingACPSessionConfigurator{}
			model := "vendor/model:variant"
			err := applyACPSessionSelections(context.Background(), configurator, acp.SessionId("session"), agentRunRequest{
				Model:      agentACPConfigSelection{Value: provider + ":" + model},
				Permission: agentACPConfigSelection{Source: AgentRuntimeConfigSourceMode, Value: "ask"},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(configurator.configRequests) != 1 || string(configurator.configRequests[0].ValueId.Value) != model {
				t.Fatalf("model selection = %#v", configurator.configRequests)
			}
			if len(configurator.modeRequests) != 1 || configurator.modeRequests[0].ModeId != "ask" {
				t.Fatal("permission selection lost")
			}
		})
	}
}

func TestProviderSelectionDoesNotApplyStaleOAuthReasoning(t *testing.T) {
	for _, provider := range []string{"api-tokease", "gateway-aihubmix", "gateway-openai-compatible", "chatgpt"} {
		t.Run(provider, func(t *testing.T) {
			conn := &recordingACPSessionConfigurator{}
			err := applyACPSessionSelections(context.Background(), conn, acp.SessionId("session"), agentRunRequest{
				Model:     agentACPConfigSelection{Value: provider + ":DeepSeek-V3.2"},
				Reasoning: agentACPConfigSelection{Source: AgentRuntimeConfigSourceOption, ConfigID: "reasoning_effort", Value: "high"},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if provider == "chatgpt" {
				want = 2
			}
			if len(conn.configRequests) != want {
				t.Fatalf("got %d selections, want %d", len(conn.configRequests), want)
			}
		})
	}
}
