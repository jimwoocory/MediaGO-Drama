package codexapp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type testWriteCloser struct{ bytes.Buffer }

func (*testWriteCloser) Close() error { return nil }

func TestImageSessionAcceptsLargeResult(t *testing.T) {
	result := strings.Repeat("A", 1024*1024)
	raw, _ := json.Marshal(map[string]any{"method": "item/completed", "params": map[string]string{"result": result}})
	session := &Session{scan: newMessageScanner(bytes.NewReader(append(raw, '\n')))}
	message, err := session.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(message.Params, []byte(result)) {
		t.Fatal("large image result was truncated")
	}
}

func TestImageSessionDoesNotMistakeServerRequestForResponse(t *testing.T) {
	input := `{"id":1,"method":"item/requestApproval","params":{}}` + "\n" + `{"id":1,"result":{"ok":true}}` + "\n"
	session := &Session{scan: newMessageScanner(strings.NewReader(input)), stdin: &testWriteCloser{}}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := session.Call(context.Background(), "test", nil, &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("server request swallowed matching RPC response")
	}
	message, err := session.Next(context.Background())
	if err != nil || message.Method != "item/requestApproval" {
		t.Fatal("approval request not retained for rejection")
	}
}
