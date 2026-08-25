package audit

import (
	"encoding/json"
	"testing"
	"time"

	"buoy-calibration-gate/internal/repository"
)

func TestVerifyEventChainDetectsTampering(t *testing.T) {
	now := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	first := repository.AuditEvent{DossierID: "d-1", Sequence: 1, EventType: "created", Actor: "a", Payload: json.RawMessage(`{"ok":true}`), OccurredAt: now}
	first.EventHash = eventHash(first)
	second := repository.AuditEvent{DossierID: "d-1", Sequence: 2, EventType: "changed", Actor: "b", Payload: json.RawMessage(`{"v":2}`), OccurredAt: now.Add(time.Second), PreviousHash: first.EventHash}
	second.EventHash = eventHash(second)
	if err := VerifyEventChain([]repository.AuditEvent{first, second}); err != nil {
		t.Fatal(err)
	}
	second.Payload = json.RawMessage(`{"v":3}`)
	if err := VerifyEventChain([]repository.AuditEvent{first, second}); err == nil {
		t.Fatal("tampered chain should fail")
	}
}
