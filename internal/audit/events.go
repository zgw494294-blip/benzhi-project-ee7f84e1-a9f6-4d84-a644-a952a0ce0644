package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"buoy-calibration-gate/internal/repository"
)

func Append(tx *repository.Tx, dossierID, eventType, actor string, payload any, now time.Time) (repository.AuditEvent, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return repository.AuditEvent{}, err
	}
	last, found, err := tx.LastAuditEvent(dossierID)
	if err != nil {
		return repository.AuditEvent{}, err
	}
	sequence, previous := int64(1), ""
	if found {
		sequence, previous = last.Sequence+1, last.EventHash
	}
	e := repository.AuditEvent{DossierID: dossierID, Sequence: sequence, EventType: eventType, Actor: actor, Payload: data, OccurredAt: now.UTC(), PreviousHash: previous}
	e.EventHash = eventHash(e)
	if err := tx.InsertAuditEvent(e); err != nil {
		return repository.AuditEvent{}, err
	}
	return e, nil
}

func eventHash(e repository.AuditEvent) string {
	canonical := fmt.Sprintf("%s\n%d\n%s\n%s\n%s\n%s\n%s", e.DossierID, e.Sequence, e.EventType, e.Actor, string(e.Payload), e.OccurredAt.UTC().Format(time.RFC3339Nano), e.PreviousHash)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func VerifyEventChain(events []repository.AuditEvent) error {
	previous := ""
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			return fmt.Errorf("audit sequence gap at %d", i+1)
		}
		if event.PreviousHash != previous {
			return fmt.Errorf("audit previous hash mismatch at %d", i+1)
		}
		if event.EventHash != eventHash(event) {
			return fmt.Errorf("audit event hash mismatch at %d", i+1)
		}
		previous = event.EventHash
	}
	return nil
}
