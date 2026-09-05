package acp

import (
	"context"
	"fmt"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// Counts and identical inputs do not prove lack of progress: reads can see
// new file contents, shell commands can append, and Skills can be reloaded.
func TestPromptAllowsLongToolSequences(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		kind        acp.ToolKind
		input       func(int) map[string]any
		compact     bool
	}{
		{"distinct calls", "read", acp.ToolKindRead, func(i int) map[string]any { return map[string]any{"path": fmt.Sprintf("chapter-%d.md", i)} }, false},
		{"repeated reads", "read", acp.ToolKindRead, func(int) map[string]any { return map[string]any{"path": "script.md"} }, false},
		{"same file batches", "edit", acp.ToolKindEdit, func(i int) map[string]any {
			return map[string]any{"path": "script.md", "content": fmt.Sprintf("chapter-%d", i)}
		}, false},
		{"repeated shell append", "shell_command", acp.ToolKindExecute, func(int) map[string]any {
			return map[string]any{"command": "Add-Content -LiteralPath script.md -Value next"}
		}, false},
		{"skill reload", "mcp.mediago_drama.load_skill", acp.ToolKindExecute, func(int) map[string]any {
			return map[string]any{"server": "mediago_drama", "tool": "load_skill", "arguments": map[string]string{"name": "scene-writer"}}
		}, false},
		{"interactive wait", "await_user_selection", acp.ToolKindOther, func(int) map[string]any { return map[string]any{"selectionId": "same"} }, false},
		{"batches across compactions", "edit", acp.ToolKindEdit, func(i int) map[string]any {
			return map[string]any{"path": "script.md", "content": fmt.Sprintf("chapter-%d", i)}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const batches = 60
			client := &acpClient{}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			completed, published := 0, 0
			client.publish = func(event agentEvent) {
				if event.ACP != nil && event.ACP.Status == "completed" {
					published++
				}
			}
			conn := safetyPromptFunc(func(ctx context.Context, _ acp.PromptRequest) (acp.PromptResponse, error) {
				for i := 0; i < batches; i++ {
					id := acp.ToolCallId(fmt.Sprintf("batch-%d", i))
					// The real ACP sender does not inspect a notification return value.
					_ = client.SessionUpdate(ctx, acp.SessionNotification{Update: acp.StartToolCall(id, tc.title, acp.WithStartKind(tc.kind), acp.WithStartRawInput(tc.input(i)))})
					if err := ctx.Err(); err != nil {
						return acp.PromptResponse{}, err
					}
					_ = client.SessionUpdate(ctx, acp.SessionNotification{Update: acp.UpdateToolCall(id, acp.WithUpdateStatus(acp.ToolCallStatusCompleted), acp.WithUpdateRawOutput(map[string]any{"batch": i, "exit_code": 0}))})
					completed++
					if tc.compact && i%12 == 11 {
						for _, chunk := range []string{"*Context compacted to fit ", "the model's context window.*\n\n"} {
							_ = client.SessionUpdate(ctx, acp.SessionNotification{Update: acp.UpdateAgentMessageText(chunk)})
						}
						if err := ctx.Err(); err != nil {
							return acp.PromptResponse{}, err
						}
					}
				}
				_ = client.SessionUpdate(ctx, acp.SessionNotification{Update: acp.UpdateAgentMessageText("All batches finished.")})
				return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
			})
			response, err := promptACPSession(ctx, conn, client, acp.PromptRequest{}, nil)
			if err != nil || response.StopReason != acp.StopReasonEndTurn || completed != batches || published != batches {
				t.Fatalf("long task interrupted: completed=%d published=%d stop=%s err=%v", completed, published, response.StopReason, err)
			}
		})
	}
}
