package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type AuditEvent struct {
	DossierID    string          `json:"dossierId"`
	Sequence     int64           `json:"sequence"`
	EventType    string          `json:"eventType"`
	Actor        string          `json:"actor"`
	Payload      json.RawMessage `json:"payload"`
	OccurredAt   time.Time       `json:"occurredAt"`
	PreviousHash string          `json:"previousHash"`
	EventHash    string          `json:"eventHash"`
}

func (t *Tx) LastAuditEvent(dossierID string) (AuditEvent, bool, error) {
	var e AuditEvent
	var occurred string
	err := t.tx.QueryRow(`SELECT dossier_id, sequence, event_type, actor, payload_json, occurred_at, previous_hash, event_hash FROM audit_events WHERE dossier_id=? ORDER BY sequence DESC LIMIT 1`, dossierID).
		Scan(&e.DossierID, &e.Sequence, &e.EventType, &e.Actor, &e.Payload, &occurred, &e.PreviousHash, &e.EventHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return e, false, nil
		}
		return e, false, err
	}
	e.OccurredAt, err = parseTime(occurred)
	return e, true, err
}

func (t *Tx) InsertAuditEvent(e AuditEvent) error {
	_, err := t.tx.Exec(`INSERT INTO audit_events(dossier_id, sequence, event_type, actor, payload_json, occurred_at, previous_hash, event_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, e.DossierID, e.Sequence, e.EventType, e.Actor, []byte(e.Payload), formatTime(e.OccurredAt), e.PreviousHash, e.EventHash)
	return err
}

func (t *Tx) ListAuditEvents(dossierID string) ([]AuditEvent, error) {
	rows, err := t.tx.Query(`SELECT dossier_id, sequence, event_type, actor, payload_json, occurred_at, previous_hash, event_hash FROM audit_events WHERE dossier_id=? ORDER BY sequence`, dossierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var occurred string
		if err := rows.Scan(&e.DossierID, &e.Sequence, &e.EventType, &e.Actor, &e.Payload, &occurred, &e.PreviousHash, &e.EventHash); err != nil {
			return nil, err
		}
		if e.OccurredAt, err = parseTime(occurred); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
