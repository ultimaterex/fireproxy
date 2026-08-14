package agentws

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	raw, err := json.Marshal(Envelope{Type: TypeLogsBatch, LogsBatch: &LogsBatch{
		ID:    "b1",
		Lines: []LogLine{{TS: 1, Source: "unbound", Severity: "err", Line: "boom"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.Type != TypeLogsBatch || env.LogsBatch == nil || len(env.LogsBatch.Lines) != 1 {
		t.Fatalf("%+v", env)
	}
}
