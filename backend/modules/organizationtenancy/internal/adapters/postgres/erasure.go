package postgres

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/uptrace/bun"
)

type ErasureStore struct{ db *bun.DB }
type erasureConnectionKey struct{}

func NewErasureStore(db *bun.DB) (*ErasureStore, error) {
	if db == nil {
		return nil, errors.New("erasure: database required")
	}
	return &ErasureStore{db: db}, nil
}

// Each SQL function call commits independently using PostgreSQL autocommit.
// The session is reserved for the workflow, then physically discarded: neither
// SET ROLE, session timeouts nor the coordination lock can leak into the pool.
func (s *ErasureStore) withConnection(ctx context.Context, fn func(context.Context, bun.Conn) error) error {
	if conn, ok := ctx.Value(erasureConnectionKey{}).(bun.Conn); ok {
		return fn(ctx, conn)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return errors.New("erasure: database session unavailable")
	}
	defer func() {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	}()
	if _, err = conn.ExecContext(ctx, `SET ROLE phoenix_school_erasure`); err != nil {
		return errors.New("erasure: dedicated role unavailable")
	}
	return fn(context.WithValue(ctx, erasureConnectionKey{}, conn), conn)
}

func (s *ErasureStore) WithSchoolLock(ctx context.Context, id int64, fn func(context.Context) error) error {
	return s.withConnection(ctx, func(ctx context.Context, conn bun.Conn) (runErr error) {
		var locked bool
		err := conn.NewRaw(`SELECT pg_try_advisory_lock(hashtextextended('school-erasure-cli:' || ?::text,0))`, id).Scan(ctx, &locked)
		if err != nil {
			return errors.New("erasure: coordination unavailable")
		}
		if !locked {
			return errors.New("erasure: school already running")
		}
		// Closing a socket does not wait for PostgreSQL to release session locks.
		// Acknowledge the unlock before a connected caller can start the next run,
		// even if this run failed or its caller cancelled. Still discard the session.
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			var unlocked bool
			err := conn.NewRaw(`SELECT pg_advisory_unlock(hashtextextended('school-erasure-cli:' || ?::text,0))`, id).Scan(cleanupCtx, &unlocked)
			if err != nil || !unlocked {
				runErr = errors.Join(runErr, errors.New("erasure: coordination release failed"))
			}
		}()
		// DB-function transactions are deliberately not nested in the lock.
		return fn(ctx)
	})
}

func (s *ErasureStore) Call(ctx context.Context, lockTimeout, statementTimeout time.Duration, operation string, args ...any) (json.RawMessage, error) {
	var result string
	err := s.withConnection(ctx, func(ctx context.Context, conn bun.Conn) error {
		if _, err := conn.ExecContext(ctx, `SELECT set_config('lock_timeout',?,false),set_config('statement_timeout',?,false)`, strconv.FormatInt(lockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(statementTimeout.Milliseconds(), 10)+"ms"); err != nil {
			return err
		}
		switch operation {
		case "preview":
			return conn.NewRaw(`SELECT platform.school_erasure_preview(?,?)::text`, args...).Scan(ctx, &result)
		case "schools":
			return conn.NewRaw(`SELECT platform.school_erasure_schools(?,?,?)::text`, args...).Scan(ctx, &result)
		case "begin":
			return conn.NewRaw(`SELECT platform.begin_school_erasure(?,?,?)::text`, args...).Scan(ctx, &result)
		case "read":
			return conn.NewRaw(`SELECT platform.read_school_erasure(?)::text`, args...).Scan(ctx, &result)
		case "erase":
			return conn.NewRaw(`SELECT platform.erase_school(?,?)::text`, args...).Scan(ctx, &result)
		case "phase":
			return conn.NewRaw(`SELECT platform.school_erasure_file_phase(?,?,?::jsonb)::text`, args...).Scan(ctx, &result)
		case "organization_check":
			return conn.NewRaw(`SELECT platform.organization_erasure_check(?,?,?)::text`, args...).Scan(ctx, &result)
		case "organization_begin":
			return conn.NewRaw(`SELECT platform.begin_organization_erasure(?,?,?)::text`, args...).Scan(ctx, &result)
		case "organization_erase":
			return conn.NewRaw(`SELECT platform.erase_organization(?,?,?)::text`, args...).Scan(ctx, &result)
		default:
			return errors.New("unknown erasure operation")
		}
	})
	// SQL errors can contain row values. Never return them to the CLI.
	if err != nil {
		return nil, errors.New("erasure: database step failed; inspect receipt and retry within maintenance window")
	}
	if result == "" || result == "null" {
		return nil, errors.New("erasure: receipt not found")
	}
	return json.RawMessage(result), nil
}
