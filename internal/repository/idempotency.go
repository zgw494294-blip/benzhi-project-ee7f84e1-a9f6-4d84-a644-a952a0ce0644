package repository

import (
	"database/sql"
	"errors"
	"time"
)

type IdempotencyRecord struct {
	Scope       string
	Key         string
	RequestHash string
	ResourceID  string
	CreatedAt   time.Time
}

func (t *Tx) FindIdempotency(scope, key string) (IdempotencyRecord, bool, error) {
	var r IdempotencyRecord
	var created string
	err := t.tx.QueryRow(`SELECT scope, key, request_hash, resource_id, created_at FROM idempotency_records WHERE scope=? AND key=?`, scope, key).
		Scan(&r.Scope, &r.Key, &r.RequestHash, &r.ResourceID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	r.CreatedAt, err = parseTime(created)
	return r, true, err
}

func (t *Tx) InsertIdempotency(r IdempotencyRecord) error {
	_, err := t.tx.Exec(`INSERT INTO idempotency_records(scope, key, request_hash, resource_id, created_at) VALUES (?, ?, ?, ?, ?)`, r.Scope, r.Key, r.RequestHash, r.ResourceID, formatTime(r.CreatedAt))
	return err
}
