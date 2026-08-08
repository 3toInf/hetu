package api

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	req := Request{Op: "list_sessions", Body: json.RawMessage(`{"project_id":0}`)}
	if err := Encode(&buf, req); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Op != "list_sessions" {
		t.Fatalf("op=%q", got.Op)
	}
}
