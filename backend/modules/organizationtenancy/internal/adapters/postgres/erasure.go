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

func NewErasureStore(db *bun.DB) (*ErasureStore, error) {
	if db == nil {
		return nil, errors.New("erasure: database required")
	}
	if db.Stats().MaxOpenConnections == 1 {
		return nil, errors.New("erasure: pool requires at least two connections for coordination and work")
	}
	return &ErasureStore{db: db}, nil
}

// Each SQL function call commits independently using PostgreSQL autocommit.
// Each step uses its own session, separate from the workflow coordination lock.
// Physically discard sessions so SET ROLE and timeouts cannot leak into the pool.
func (s *ErasureStore) withConnection(ctx context.Context, fn func(context.Context, bun.Conn) error) error {
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
	return fn(ctx, conn)
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
		lock := &erasureLock{conn: conn, lastConfirmed: time.Now()}
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if releaseErr := lock.release(cleanupCtx, id); releaseErr != nil && !errors.Is(runErr, releaseErr) {
				runErr = errors.Join(runErr, releaseErr)
			}
		}()
		// Never lend the coordination session to a work step: pgdriver closes
		// its socket on 57014 in ExecContext. Keep the lock through the entire
		// callback, including file and ledger work, and acknowledge its release.
		// DB-function transactions remain independent of the lock session.
		return fn(context.WithValue(ctx, erasureLockKey{}, lock))
	})
}

func (s *ErasureStore) Call(ctx context.Context, lockTimeout, statementTimeout time.Duration, operation string, args ...any) (json.RawMessage, error) {
	if err := s.CheckSchoolLock(ctx); err != nil {
		return nil, err
	}
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
