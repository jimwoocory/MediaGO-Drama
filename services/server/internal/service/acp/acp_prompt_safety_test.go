package acp

import (
	"context"
	"errors"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

type safetyPromptFunc func(context.Context, acp.PromptRequest) (acp.PromptResponse, error)

func (f safetyPromptFunc) Prompt(ctx context.Context, r acp.PromptRequest) (acp.PromptResponse, error) {
	return f(ctx, r)
}

func TestPromptSafetyPreservesUserCancellation(t *testing.T) {
	client := &acpClient{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn := safetyPromptFunc(func(ctx context.Context, r acp.PromptRequest) (acp.PromptResponse, error) {
		cancel()
		<-ctx.Done()
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	})
	_, err := promptACPSession(ctx, conn, client, acp.PromptRequest{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("user cancellation was lost: %v", err)
	}
}

func TestPromptSafetyDistinguishesNativeFailureFromCompaction(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		fail       bool
	}{
		{"rate limit", "exceeded retry limit, last status: 429 Too Many Requests, request id: fixture", true},
		{"split retry failure", "exceeded retry limit, last status: 503 Service Unavailable", true},
		{"quoted error", "请解释这条错误：exceeded retry limit, last status: 429 Too Many Requests", false},
		{"normal rate explanation", "HTTP 429 Too Many Requests 表示上游限流。", false},
		{"repeated compaction", "Context compacted to fit the model's context window.\nContext compacted to fit the model's context window.\n", false},
		{"one compaction", "Context compacted to fit the model's context window.\n已完成工作。", false},
		{"real ACP compaction", "*Context compacted to fit the model's context window.*\n\n*Context compacted to fit the model's context window.*\n\n", false},
		{"failure after compaction", "*Context compacted to fit the model's context window.*\n\nexceeded retry limit, last status: 429 Too Many Requests", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &acpClient{}
			conn := safetyPromptFunc(func(ctx context.Context, r acp.PromptRequest) (acp.PromptResponse, error) {
				for _, part := range []string{tc.text[:len(tc.text)/2], tc.text[len(tc.text)/2:]} {
					_ = client.SessionUpdate(ctx, acp.SessionNotification{Update: acp.UpdateAgentMessageText(part)})
				}
				return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
			})
			_, err := promptACPSession(context.Background(), conn, client, acp.PromptRequest{}, nil)
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v, want failure=%v", err, tc.fail)
			}
		})
	}
}

func TestPromptSafetyClosesUnresponsiveProcess(t *testing.T) {
	client := &acpClient{}
	closed := make(chan struct{})
	conn := safetyPromptFunc(func(ctx context.Context, _ acp.PromptRequest) (acp.PromptResponse, error) {
		client.abortPrompt("fixture safety stop")
		<-closed // Simulate an ACP process that ignores session/cancel.
		return acp.PromptResponse{}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := promptACPSession(ctx, conn, client, acp.PromptRequest{}, func() { close(closed) })
	if ctx.Err() != nil || err == nil || err.Error() != "fixture safety stop" {
		t.Fatalf("safety cancellation not preserved: %v", err)
	}
}
