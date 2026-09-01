package acp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	maxAgentToolRounds      = 12
	maxAgentToolCalls       = 10
	maxAgentFileMutations   = 4
	maxIdenticalToolRepeats = 1
)

type toolLoopGuardDecision struct {
	ForceFinalize bool
	Reason        string
}

type toolLoopGuard struct {
	rounds              int
	totalCalls          int
	seenCallIDs         map[string]bool
	fileMutations       map[string]int
	lastSignature       string
	identicalRepeats    int
	forceFinalizeReason string
}

func (guard *toolLoopGuard) reset() {
	*guard = toolLoopGuard{}
}

func (guard *toolLoopGuard) observe(toolCallID string, toolKind string, title string, rawInput []byte) toolLoopGuardDecision {
	if guard.forceFinalizeReason != "" {
		return toolLoopGuardDecision{ForceFinalize: true, Reason: guard.forceFinalizeReason}
	}
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID != "" {
		if guard.seenCallIDs == nil {
			guard.seenCallIDs = map[string]bool{}
		}
		if guard.seenCallIDs[toolCallID] {
			return toolLoopGuardDecision{}
		}
		guard.seenCallIDs[toolCallID] = true
	}

	if isInteractiveWaitTool(title, rawInput) {
		return toolLoopGuardDecision{}
	}

	guard.rounds++
	guard.totalCalls++
	if guard.rounds > maxAgentToolRounds {
		return guard.forceFinalize(fmt.Sprintf("工具循环已达到 %d 轮安全上限", maxAgentToolRounds))
	}
	if guard.totalCalls > maxAgentToolCalls {
		return guard.forceFinalize(fmt.Sprintf("工具调用已达到 %d 次安全上限", maxAgentToolCalls))
	}

	signature := normalizedToolSignature(title, rawInput)
	if signature != "" && signature == guard.lastSignature {
		guard.identicalRepeats++
		if guard.identicalRepeats > maxIdenticalToolRepeats {
			return guard.forceFinalize("检测到重复的相同工具调用，已停止继续循环")
		}
	} else {
		guard.lastSignature = signature
		guard.identicalRepeats = 0
	}

	if isMutationTool(toolKind, title) {
		if path := toolInputPath(rawInput); path != "" {
			if guard.fileMutations == nil {
				guard.fileMutations = map[string]int{}
			}
			guard.fileMutations[path]++
			if guard.fileMutations[path] > maxAgentFileMutations {
				return guard.forceFinalize(fmt.Sprintf("单文件 %s 已达到 %d 次修改安全上限", path, maxAgentFileMutations))
			}
		}
	}

	return toolLoopGuardDecision{}
}

func (guard *toolLoopGuard) forceFinalize(reason string) toolLoopGuardDecision {
	guard.forceFinalizeReason = strings.TrimSpace(reason)
	return toolLoopGuardDecision{ForceFinalize: true, Reason: guard.forceFinalizeReason}
}

func normalizedToolSignature(title string, rawInput []byte) string {
	return strings.ToLower(strings.TrimSpace(title)) + "\n" + strings.TrimSpace(string(rawInput))
}

func toolInputPath(rawInput []byte) string {
	var input map[string]any
	if len(rawInput) == 0 || json.Unmarshal(rawInput, &input) != nil {
		return ""
	}
	for _, key := range []string{"path", "file_path", "filePath", "filename", "file"} {
		if value, ok := input[key].(string); ok && strings.TrimSpace(value) != "" {
			return filepath.Clean(strings.TrimSpace(value))
		}
	}
	return ""
}

func isMutationTool(toolKind string, title string) bool {
	value := strings.ToLower(strings.TrimSpace(toolKind + " " + title))
	for _, marker := range []string{"edit", "write", "delete", "move", "rename", "patch", "修改", "写入", "删除", "移动", "重命名"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func isInteractiveWaitTool(title string, rawInput []byte) bool {
	value := strings.ToLower(strings.TrimSpace(title + " " + string(rawInput)))
	return strings.Contains(value, "await_user_selection")
}
