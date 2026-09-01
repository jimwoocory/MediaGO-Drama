package acp

import (
	"context"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
)

func TestSessionUpdateSuppressesSplitDSMLToolCalls(t *testing.T) {
	events := []agentEvent{}
	client := &acpClient{
		runID:        "run-dsml",
		acceptUpdate: true,
		publish: func(event agentEvent) {
			events = append(events, event)
		},
	}

	chunks := []string{
		"先检查结果。<｜DS",
		"ML｜tool_calls><｜DSML｜invoke name=\"write\"><｜DSML｜parameter name=\"path\">a.md</｜DSML｜parameter>",
		"</｜DSML｜invoke></｜DSML｜tool_calls>已经完成。",
	}
	for _, text := range chunks {
		update := acpsdk.SessionUpdate{AgentMessageChunk: &acpsdk.SessionUpdateAgentMessageChunk{
			Content: acpsdk.TextBlock(text),
		}}
		if err := client.SessionUpdate(context.Background(), acpsdk.SessionNotification{Update: update}); err != nil {
			t.Fatalf("SessionUpdate returned error: %v", err)
		}
	}

	transcript := client.messageText()
	if transcript != "先检查结果。已经完成。" {
		t.Fatalf("transcript = %q", transcript)
	}
	for _, event := range events {
		if strings.Contains(event.Delta, "DSML") || strings.Contains(event.Message, "DSML") {
			t.Fatalf("DSML leaked in event: %#v", event)
		}
	}
}

func TestFilterDSMLChunkSupportsASCIIProviderVariant(t *testing.T) {
	client := &acpClient{}
	visible := client.filterDSMLChunk("before<||DSML||tool_calls>hidden</||DSML||tool_calls>after")
	if visible != "beforeafter" {
		t.Fatalf("visible = %q, want beforeafter", visible)
	}
}

func TestSessionUpdateSuppressesDSMLInThoughtChunks(t *testing.T) {
	events := []agentEvent{}
	client := &acpClient{
		runID:        "run-dsml-thought",
		acceptUpdate: true,
		publish: func(event agentEvent) {
			events = append(events, event)
		},
	}

	update := acpsdk.SessionUpdate{AgentThoughtChunk: &acpsdk.SessionUpdateAgentThoughtChunk{
		Content: acpsdk.TextBlock("可见思考。<||DSML||tool_calls>hidden-tool</||DSML||tool_calls>继续思考。"),
	}}
	if err := client.SessionUpdate(context.Background(), acpsdk.SessionNotification{Update: update}); err != nil {
		t.Fatalf("SessionUpdate returned error: %v", err)
	}
	client.finishThoughts()

	if len(events) != 1 || events[0].ACP == nil || events[0].ACP.Kind != "thought" {
		t.Fatalf("events = %#v, want one thought event", events)
	}
	if events[0].ACP.Thought != "可见思考。继续思考。" {
		t.Fatalf("thought = %q", events[0].ACP.Thought)
	}
	for _, event := range events {
		if strings.Contains(event.Delta, "DSML") || strings.Contains(event.Message, "DSML") || (event.ACP != nil && strings.Contains(event.ACP.Thought, "DSML")) {
			t.Fatalf("DSML leaked in thought event: %#v", event)
		}
	}
}
