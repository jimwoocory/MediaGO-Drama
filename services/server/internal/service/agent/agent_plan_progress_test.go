package agent

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPlanCheckpointSurvivesLaterExecutionAndReload(t *testing.T) {
	conversation := AgentConversationRecord{RunID: "run", Status: "running"}
	tool := func(id, status string) {
		conversation = upsertProjectedACPToolCall(conversation, AgentEvent{RunID: "run", TurnID: "run"}, AgentACPEvent{ToolCallID: id, Status: status})
	}
	tool("read", "completed")
	tool("write", "in_progress")
	entries := []AgentACPPlanEntry{{Content: "写剧本", Status: "in_progress"}, {Content: "校验", Status: "pending"}}
	event := AgentEvent{RunID: "run", TurnID: "run", ItemID: "plan"}
	conversation = upsertProjectedACPPlan(conversation, event, entries)
	tool("write", "completed")
	tool("verify", "completed")
	var restored AgentConversationRecord
	data, err := json.Marshal(conversation)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	index := findProjectedCurrentTurnPlanIndex(restored.Messages, event)
	if index < 0 {
		t.Fatal("plan missing after reload")
	}
	checkpoint := restored.Messages[index].Metadata["planToolStates"].(map[string]any)
	if checkpoint["write"] != "in_progress" || checkpoint["verify"] != nil {
		t.Fatalf("later tools changed the plan checkpoint: %#v", checkpoint)
	}
	if got := projectedPlanToolStates(restored.Messages, "run"); !reflect.DeepEqual(got, map[string]string{"read": "completed", "write": "completed", "verify": "completed"}) {
		t.Fatalf("latest execution = %#v", got)
	}
	conversation = upsertProjectedACPPlan(conversation, event, []AgentACPPlanEntry{{Content: "完成", Status: "completed"}})
	if len(conversation.Messages) != 4 {
		t.Fatalf("plan update duplicated an item: %d", len(conversation.Messages))
	}
	if got := conversation.Messages[index].Metadata["planToolStates"].(map[string]string)["verify"]; got != "completed" {
		t.Fatal("new plan did not advance checkpoint")
	}
}

func TestLatePlanPreservesTerminalStatusAndTurnOwnership(t *testing.T) {
	for _, status := range []string{"completed", "failed", "cancelled", "interrupted", "paused"} {
		t.Run(status, func(t *testing.T) {
			conversation := AgentConversationRecord{RunID: "run", Status: status, Messages: []AgentChatMessageRecord{
				{ID: "old", Kind: "plan", TurnID: "old"},
				{ID: "user", Role: "user", TurnID: "run"},
				{ID: "current", Kind: "plan", TurnID: "run"},
			}}
			conversation = upsertProjectedACPPlan(conversation, AgentEvent{RunID: "run", TurnID: "old"}, []AgentACPPlanEntry{{Content: "旧轮结束", Status: "completed"}})
			if conversation.Status != status {
				t.Fatalf("late plan revived %s", status)
			}
			if conversation.Messages[0].Content != "旧轮结束" || conversation.Messages[2].Content != "" {
				t.Fatal("late plan overwrote current run")
			}
		})
	}
}

func TestPlanToolCheckpointExcludesOtherRuns(t *testing.T) {
	messages := []AgentChatMessageRecord{
		{ID: "old-user", Role: "user", TurnID: "old"},
		{ID: "legacy-old", Kind: "tool"},
		{ID: "user", Role: "user", TurnID: "run"},
		{ID: "legacy-current", Kind: "tool"},
		{ID: "old-tool", Kind: "tool", TurnID: "old"},
	}
	if got := projectedPlanToolStates(messages, "run"); !reflect.DeepEqual(got, map[string]string{"legacy-current": "pending"}) {
		t.Fatalf("checkpoint = %#v", got)
	}
	if got := projectedPlanToolStates(messages, "old"); !reflect.DeepEqual(got, map[string]string{"old-tool": "pending"}) {
		t.Fatalf("old checkpoint = %#v", got)
	}
}
