package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/uptrace/bun"
)

type ErasureStore struct{ db *bun.DB }

func NewErasureStore(db *bun.DB) (*ErasureStore, error) {
	if db == nil {
		return nil, errors.New("erasure: database required")
	}
	if db.DB.Stats().MaxOpenConnections == 1 {
		return nil, errors.New("erasure: at least two database pool connections required")
	}
	return &ErasureStore{db: db}, nil
}

// The lock transaction spans the workflow; phase transactions still commit
// independently. A lost CLI connection releases the coordination lock.
func (s *ErasureStore) WithSchoolLock(ctx context.Context, id int64, fn func() error) error {
	var callbackErr error
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var locked bool
		if err := tx.NewRaw(`SELECT pg_try_advisory_xact_lock(hashtextextended('school-erasure-cli:' || ?::text,0))`, id).Scan(ctx, &locked); err != nil || !locked {
			return errors.New("erasure: school already running or coordination unavailable")
		}
		callbackErr = fn()
		return callbackErr
	})
	if callbackErr != nil {
		return callbackErr
	}
	if err != nil {
		return errors.New("erasure: coordination transaction failed; inspect receipt before retry")
	}
	return nil
}

func (s *ErasureStore) Call(ctx context.Context, lockTimeout, statementTimeout time.Duration, operation string, args ...any) (json.RawMessage, error) {
	var result string
	err := s.db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_school_erasure`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('lock_timeout',?,true),set_config('statement_timeout',?,true)`, strconv.FormatInt(lockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(statementTimeout.Milliseconds(), 10)+"ms"); err != nil {
			return err
		}
		switch operation {
		case "preview":
			return tx.NewRaw(`SELECT platform.school_erasure_preview(?,?)::text`, args...).Scan(ctx, &result)
		case "schools":
			return tx.NewRaw(`SELECT platform.school_erasure_schools(?,?,?)::text`, args...).Scan(ctx, &result)
		case "begin":
			return tx.NewRaw(`SELECT platform.begin_school_erasure(?,?,?)::text`, args...).Scan(ctx, &result)
		case "read":
			return tx.NewRaw(`SELECT platform.read_school_erasure(?)::text`, args...).Scan(ctx, &result)
		case "erase":
			return tx.NewRaw(`SELECT platform.erase_school(?,?)::text`, args...).Scan(ctx, &result)
		case "phase":
			return tx.NewRaw(`SELECT platform.school_erasure_file_phase(?,?,?::jsonb)::text`, args...).Scan(ctx, &result)
		default:
			return errors.New("unknown erasure operation")
		}
	})
	// SQL errors may contain row values. Never return them to the CLI.
	if err != nil {
		return nil, errors.New("erasure: database step failed; inspect receipt and retry within maintenance window")
	}
	if result == "" || result == "null" {
		return nil, errors.New("erasure: receipt not found")
	}
	return json.RawMessage(result), nil
}
