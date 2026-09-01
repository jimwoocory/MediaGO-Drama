package acp

import (
	"fmt"
	"strings"
	"testing"
)

func TestToolLoopGuardTotalCallLimitForcesFinalize(t *testing.T) {
	guard := toolLoopGuard{}
	for i := 1; i <= maxAgentToolCalls; i++ {
		decision := guard.observe(fmt.Sprintf("call-%d", i), "read", "read", []byte(fmt.Sprintf(`{"path":"file-%d.md"}`, i)))
		if decision.ForceFinalize {
			t.Fatalf("call %d unexpectedly forced finalize: %s", i, decision.Reason)
		}
	}
	decision := guard.observe("call-over", "read", "read", []byte(`{"path":"overflow.md"}`))
	if !decision.ForceFinalize || !strings.Contains(decision.Reason, "10") {
		t.Fatalf("decision = %#v, want 10-call force finalize", decision)
	}
}

func TestToolLoopGuardPerFileMutationLimit(t *testing.T) {
	guard := toolLoopGuard{}
	for i := 1; i <= maxAgentFileMutations; i++ {
		raw := []byte(fmt.Sprintf(`{"path":"script.md","content":"revision-%d"}`, i))
		if decision := guard.observe(fmt.Sprintf("edit-%d", i), "edit", "edit", raw); decision.ForceFinalize {
			t.Fatalf("mutation %d unexpectedly forced finalize: %s", i, decision.Reason)
		}
	}
	decision := guard.observe("edit-over", "edit", "edit", []byte(`{"path":"script.md","content":"revision-5"}`))
	if !decision.ForceFinalize || !strings.Contains(decision.Reason, "4") {
		t.Fatalf("decision = %#v, want per-file mutation force finalize", decision)
	}
}

func TestToolLoopGuardStopsRepeatedIdenticalCalls(t *testing.T) {
	guard := toolLoopGuard{}
	raw := []byte(`{"path":"same.md"}`)
	if decision := guard.observe("repeat-1", "read", "read", raw); decision.ForceFinalize {
		t.Fatal("first call should be allowed")
	}
	if decision := guard.observe("repeat-2", "read", "read", raw); decision.ForceFinalize {
		t.Fatal("one retry should be allowed")
	}
	if decision := guard.observe("repeat-3", "read", "read", raw); !decision.ForceFinalize {
		t.Fatal("third identical call should force finalize")
	}
}

func TestToolLoopGuardExemptsAwaitUserSelection(t *testing.T) {
	guard := toolLoopGuard{}
	for i := 0; i < 20; i++ {
		decision := guard.observe(fmt.Sprintf("wait-%d", i), "other", "await_user_selection", []byte(`{"selectionId":"same"}`))
		if decision.ForceFinalize {
			t.Fatalf("interactive wait %d unexpectedly forced finalize: %s", i, decision.Reason)
		}
	}
	if guard.totalCalls != 0 || guard.rounds != 0 {
		t.Fatalf("interactive waits consumed budget: calls=%d rounds=%d", guard.totalCalls, guard.rounds)
	}
}
