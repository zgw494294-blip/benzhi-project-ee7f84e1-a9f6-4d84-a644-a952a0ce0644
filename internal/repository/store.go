package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ready(ctx context.Context) error {
	var one int
	if err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return err
	}
	return nil
}

// cancelledCause returns the request-cancellation cause (ctx.Err()) when the
// context has been cancelled or expired. This lets callers surface the
// original cancellation instead of the database/sql "transaction already
// committed or rolled back" error produced by the automatic rollback that Go
// performs when a transaction's context is cancelled mid-flight.
func cancelledCause(ctx context.Context) error {
	return ctx.Err()
}

func (s *Store) Transaction(ctx context.Context, fn func(*Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		if cause := cancelledCause(ctx); cause != nil {
			return cause
		}
		return err
	}
	w := &Tx{tx: tx}
	if err := fn(w); err != nil {
		_ = tx.Rollback()
		if cause := cancelledCause(ctx); cause != nil {
			return cause
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		if cause := cancelledCause(ctx); cause != nil {
			return cause
		}
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *Store) View(ctx context.Context, fn func(*Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		if cause := cancelledCause(ctx); cause != nil {
			return cause
		}
		return err
	}
	w := &Tx{tx: tx}
	if err := fn(w); err != nil {
		_ = tx.Rollback()
		if cause := cancelledCause(ctx); cause != nil {
			return cause
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		if cause := cancelledCause(ctx); cause != nil {
			return cause
		}
		return err
	}
	return nil
}
