package acp

import "strings"

var stableMediaGoToolNames = map[string]struct{}{
	"list_projects":         {},
	"load_skill":            {},
	"get_project_config":    {},
	"update_project_config": {},
	"list_comments":         {},
	"get_comment":           {},
	"mutate_comment":        {},
	"ask_user_selection":    {},
	"ask_user_form":         {},
	"await_user_selection":  {},
	"generate_media":        {},
	"generate_media_batch":  {},
}

// CanonicalACPToolName maps backend-specific ACP tool labels to one stable MediaGo name.
// The original ACP title is preserved separately for display.
func CanonicalACPToolName(explicitKind string, title string) string {
	token := normalizedACPToolToken(title)
	if _, ok := stableMediaGoToolNames[token]; ok {
		return token
	}

	switch token {
	case "list_files", "list_file", "list_directory", "list_directories", "list_dir", "ls", "glob", "find_files":
		return "list_files"
	case "read", "read_file", "read_text_file", "readtextfile", "cat", "view_file", "get_file":
		return "read"
	case "write", "write_file", "write_text_file", "writetextfile", "create_file", "save_file":
		return "write"
	case "edit", "edit_file", "patch", "apply_patch", "applypatch", "replace", "replace_file", "multi_edit", "multiedit":
		return "edit"
	case "grep", "search", "search_files", "find", "ripgrep", "rg":
		return "search"
	case "bash", "shell", "terminal", "exec", "execute", "run", "run_command", "command", "powershell", "pwsh":
		return "execute"
	}

	lowerTitle := strings.ToLower(strings.TrimSpace(title))
	switch {
	case containsAny(lowerTitle, "list files", "list directory", "列出文件", "文件列表", "目录列表"):
		return "list_files"
	case containsAny(lowerTitle, "read file", "读取文件", "读取 ", "查看文件"):
		return "read"
	case containsAny(lowerTitle, "write file", "create file", "写入文件", "创建文件", "保存文件"):
		return "write"
	case containsAny(lowerTitle, "edit file", "apply patch", "编辑文件", "修改文件", "更新文件"):
		return "edit"
	case containsAny(lowerTitle, "search files", "grep", "搜索文件", "查找文件"):
		return "search"
	case containsAny(lowerTitle, "run command", "execute command", "运行命令", "执行命令"):
		return "execute"
	}

	switch strings.ToLower(strings.TrimSpace(explicitKind)) {
	case "read":
		return "read"
	case "edit":
		return "edit"
	case "execute":
		return "execute"
	case "search":
		return "search"
	}
	return token
}

func normalizedACPToolToken(title string) string {
	value := strings.ToLower(strings.TrimSpace(title))
	value = strings.TrimPrefix(value, "tool:")
	value = strings.TrimSpace(value)
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		value = value[slash+1:]
	}
	if namespace := strings.LastIndex(value, "__"); namespace >= 0 {
		value = value[namespace+2:]
	}
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return strings.Trim(value, "_:")
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
