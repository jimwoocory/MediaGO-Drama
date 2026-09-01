package acp

import (
	"reflect"
	"testing"
)

func TestActiveCommandArgvForModelUsesModelBackendResolver(t *testing.T) {
	runner := NewACPAgentRunnerWithDocumentMCPConfigPathAndArgv(
		"codex-acp",
		t.TempDir(),
		"",
		nil,
		nil,
		func() []string { return []string{"codex-acp"} },
	)
	runner.SetModelBackendArgvResolver(func(model string) []string {
		switch model {
		case "aihubmix/deepseek-v4-pro", "deepseek/deepseek-chat", "mediago/gpt-5.5":
			return []string{"opencode", "acp"}
		default:
			return []string{"codex-acp"}
		}
	})

	for _, test := range []struct {
		name  string
		model string
		want  []string
	}{
		{name: "codex bare model", model: "gpt-5.5", want: []string{"codex-acp"}},
		{name: "empty inspection model", model: "", want: []string{"codex-acp"}},
		{name: "aihubmix", model: "aihubmix/deepseek-v4-pro", want: []string{"opencode", "acp"}},
		{name: "deepseek", model: "deepseek/deepseek-chat", want: []string{"opencode", "acp"}},
		{name: "mediago", model: "mediago/gpt-5.5", want: []string{"opencode", "acp"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			command, args := runner.activeCommandArgvForModel(test.model)
			got := append([]string{command}, args...)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("argv = %#v, want %#v", got, test.want)
			}
		})
	}
}
