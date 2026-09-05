package acp

import (
	"context"
	"encoding/json"
	acp "github.com/coder/acp-go-sdk"
	"testing"
)

func TestProviderReasoningProcessConfig(t *testing.T) {
	for _, effort := range []string{"default", "none", "minimal", "low", "medium", "high", "xhigh"} {
		t.Run(effort, func(t *testing.T) {
			original := ProcessConfig{Env: map[string]string{"CODEX_CONFIG": `{"model":"opaque/model","model_reasoning_effort":"high","skills":{"config":[]}}`}}
			config, err := applyProviderReasoningConfig(original, agentRunRequest{Model: agentACPConfigSelection{Value: "api-fixture:opaque/model"}, Reasoning: agentACPConfigSelection{Source: "providerReasoning", Value: "provider:" + effort}})
			if err != nil {
				t.Fatal(err)
			}
			var values map[string]json.RawMessage
			if err := json.Unmarshal([]byte(config.Env["CODEX_CONFIG"]), &values); err != nil {
				t.Fatal(err)
			}
			want := `"` + effort + `"`
			if effort == "default" {
				want = ""
			}
			if string(values["model_reasoning_effort"]) != want || string(values["model"]) != `"opaque/model"` || string(values["skills"]) != `{"config":[]}` {
				t.Fatalf("unexpected config: %s", config.Env["CODEX_CONFIG"])
			}
			if original.Env["CODEX_CONFIG"] != `{"model":"opaque/model","model_reasoning_effort":"high","skills":{"config":[]}}` {
				t.Fatal("mutated shared process config")
			}
		})
	}
}

func TestProviderReasoningDoesNotBorrowOAuthOrAcceptInvalidEffort(t *testing.T) {
	for _, tc := range []struct {
		model, source, value string
		invalid              bool
	}{
		{"chatgpt:gpt-5.5", "configOption", "high", false},
		{"api-fixture:model", "configOption", "high", false},
		{"api-fixture:model", "providerReasoning", "high", true},
		{"api-fixture:model", "providerReasoning", "provider:invalid", true},
	} {
		config, err := applyProviderReasoningConfig(ProcessConfig{}, agentRunRequest{Model: agentACPConfigSelection{Value: tc.model}, Reasoning: agentACPConfigSelection{Source: tc.source, Value: tc.value}})
		if (err != nil) != tc.invalid {
			t.Fatalf("unexpected error: %v", err)
		}
		if !tc.invalid && config.Env != nil {
			t.Fatal("modified unrelated config")
		}
	}
}

func TestPrepareProcessConfigAppliesProviderReasoning(t *testing.T) {
	runner := &acpAgentRunner{processConfigProvider: ProcessConfigProviderFunc(func(_ context.Context, request ProcessConfigRequest) (ProcessConfig, error) {
		if !request.ProviderReasoningEnabled {
			t.Fatal("reasoning catalogue not enabled for explicit selection")
		}
		return ProcessConfig{Env: map[string]string{"CODEX_CONFIG": `{"model":"fixture"}`}}, nil
	})}
	config, err := runner.prepareProcessConfig(context.Background(), "codex-acp", nil, agentRunRequest{Model: agentACPConfigSelection{Value: "gateway-aihubmix:fixture"}, Reasoning: agentACPConfigSelection{Source: "providerReasoning", Value: "provider:high"}})
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(config.Env["CODEX_CONFIG"]), &values); err != nil {
		t.Fatal(err)
	}
	if values["model_reasoning_effort"] != "high" {
		t.Fatal("run selection never reached process config")
	}
}

func TestExplicitProviderReasoningIsAppliedAndErrorsAreNotHidden(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		conn := &recordingACPSessionConfigurator{}
		if rejected {
			conn.invalidModelValue = "high"
		}
		err := applyACPSessionSelections(context.Background(), conn, acp.SessionId("fixture"), agentRunRequest{
			Model:     agentACPConfigSelection{Value: "api-fixture:opaque/model"},
			Reasoning: agentACPConfigSelection{Source: "providerReasoning", Value: "provider:high"},
		}, nil)
		if (err != nil) != rejected {
			t.Fatalf("rejected=%t error=%v", rejected, err)
		}
		if len(conn.configRequests) != 2 || conn.configRequests[1].ValueId.Value != "high" {
			t.Fatal("explicit API effort not applied")
		}
	}
}
