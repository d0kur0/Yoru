package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestProtocolRecoversAfterInvalidRequestAndStopsAtEOF(t *testing.T) {
	var output bytes.Buffer
	input := "invalid\n{\"id\":2,\"method\":\"unknown\"}\n{\"id\":3,\"method\":\"status\"}\n"
	if err := serve(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("responses: %s", output.String())
	}
	var invalid, unknown response
	json.Unmarshal([]byte(lines[0]), &invalid)
	json.Unmarshal([]byte(lines[1]), &unknown)
	if invalid.Error != "invalid_request" || unknown.ID != 2 || unknown.Error != "method_not_found" {
		t.Fatal(output.String())
	}
	var last struct {
		ID     int    `json:"id"`
		Result status `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &last); err != nil {
		t.Fatal(err)
	}
	if last.ID != 3 || !last.Result.BackendReady || last.Result.CoreReady || last.Result.Connected {
		t.Fatalf("untruthful status: %+v", last)
	}
}

func TestOversizedMessageIsRejected(t *testing.T) {
	if err := serve(strings.NewReader(strings.Repeat("x", 65537)), &bytes.Buffer{}); err == nil {
		t.Fatal("expected bounded input")
	}
}
