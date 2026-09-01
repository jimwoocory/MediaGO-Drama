package acp

import (
	"context"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
)

func TestCanonicalACPToolNameNormalizesBackendVariants(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		title string
		want  string
	}{
		{name: "codex read", title: "Read", want: "read"},
		{name: "opencode read file", title: "read_file", want: "read"},
		{name: "codex glob", title: "Glob", want: "list_files"},
		{name: "opencode list files", title: "list_files", want: "list_files"},
		{name: "codex write", title: "Write", want: "write"},
		{name: "opencode write file", title: "write_file", want: "write"},
		{name: "codex edit", title: "Edit", want: "edit"},
		{name: "apply patch", title: "apply_patch", want: "edit"},
		{name: "shell", title: "Bash", want: "execute"},
		{name: "search", title: "Grep", want: "search"},
		{name: "mcp namespace", title: "mcp__mediago-drama__generate_media", want: "generate_media"},
		{name: "slash namespace", title: "mediago-drama/load_skill", want: "load_skill"},
		{name: "chinese read", title: "读取文件 README.md", want: "read"},
		{name: "chinese edit", title: "修改文件 第一集.md", want: "edit"},
		{name: "explicit kind fallback", kind: "execute", title: "custom-tool", want: "execute"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanonicalACPToolName(test.kind, test.title); got != test.want {
				t.Fatalf("CanonicalACPToolName(%q, %q) = %q, want %q", test.kind, test.title, got, test.want)
			}
		})
	}
}

func TestInferACPToolKindUsesCanonicalNames(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{title: "Read", want: "read"},
		{title: "read_file", want: "read"},
		{title: "Glob", want: "read"},
		{title: "Write", want: "edit"},
		{title: "edit_file", want: "edit"},
		{title: "Bash", want: "execute"},
		{title: "mcp__mediago-drama__generate_media", want: "execute"},
	}
	for _, test := range tests {
		if got := InferACPToolKind("", test.title); got != test.want {
			t.Fatalf("InferACPToolKind(%q) = %q, want %q", test.title, got, test.want)
		}
	}
}

func TestACPToolEventsPreserveTitleAndExposeCanonicalName(t *testing.T) {
	for _, title := range []string{"Read", "read_file"} {
		t.Run(title, func(t *testing.T) {
			var events []agentEvent
			client := &acpClient{publish: func(event agentEvent) { events = append(events, event) }}
			client.setAcceptingSessionUpdates(true)

			err := client.SessionUpdate(context.Background(), acpsdk.SessionNotification{
				Update: acpsdk.StartToolCall(
					"call-read",
					title,
					acpsdk.WithStartRawInput(map[string]any{"path": "README.md"}),
				),
			})
			if err != nil {
				t.Fatalf("SessionUpdate returned error: %v", err)
			}
			if len(events) != 1 || events[0].ACP == nil {
				t.Fatalf("events = %#v, want one ACP tool event", events)
			}
			if events[0].ACP.ToolName != "read" || events[0].ACP.Title != title || events[0].ACP.ToolKind != "read" {
				t.Fatalf("ACP event = %#v, want canonical read with original title %q", events[0].ACP, title)
			}
		})
	}
}
