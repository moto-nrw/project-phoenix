// Package postgres persists the Export Transfer journal in
// audit.export_transfers.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/uptrace/bun"
)

// Database yields the command's scoped transaction. Begin persists intent and
// audit evidence together; claim and outcome settlement use fresh transactions.
type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store { return &Store{database: database} }

// row mirrors audit.export_transfers. There is deliberately no column for the
// username, the password or the host key: the trail records where a file went,
// not how to send another one.
type row struct {
	bun.BaseModel   `bun:"table:export_transfers,alias:export_transfer"`
	TenantID        int64  `bun:"tenant_id"`
	ActorAccountID  *int64 `bun:"actor_account_id"`
	ActorName       string `bun:"actor_name"`
	ExportKind      string `bun:"export_kind"`
	ExportFormat    string `bun:"export_format"`
	Filename        string `bun:"filename"`
	ByteSize        int64  `bun:"byte_size"`
	TargetHost      string `bun:"target_host"`
	TargetPort      int    `bun:"target_port"`
	TargetDirectory string `bun:"target_directory"`
	Status          string `bun:"status"`
	FailureReason   string `bun:"failure_reason"`
}

// Status values, mirrored by the CHECK constraint on the table.
const (
	statusSuccess = "success"
	statusFailed  = "failed"
)

func (s *Store) Begin(ctx context.Context, entry domain.JournalEntry) (int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}

	status := statusFailed
	reason := entry.Reason
	if entry.Success {
		status = statusSuccess
		// A success carries no reason. Storing one anyway would let a failed
		// attempt masquerade as a successful one with an explanation.
		reason = ""
	}
	if status == statusFailed && reason == "" {
		return 0, fmt.Errorf("export transfer postgres: a failed attempt must name a reason")
	}

	record := row{
		TenantID:        tenantID,
		ActorName:       entry.ActorName,
		ExportKind:      entry.Kind,
		ExportFormat:    entry.Format,
		Filename:        entry.Filename,
		ByteSize:        entry.ByteSize,
		TargetHost:      entry.TargetHost,
		TargetPort:      entry.TargetPort,
		TargetDirectory: entry.TargetDirectory,
		Status:          status,
		FailureReason:   reason,
	}
	if entry.ActorAccountID > 0 {
		accountID := entry.ActorAccountID
		record.ActorAccountID = &accountID
	}

	var id int64
	if err := db.NewRaw(`INSERT INTO audit.export_transfers
 (tenant_id,actor_account_id,actor_name,export_kind,export_format,filename,byte_size,target_host,target_port,target_directory,status,failure_reason,dispatch_state,dispatch_expires_at)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,'pending',clock_timestamp()+INTERVAL '2 minutes') RETURNING id`,
		record.TenantID, record.ActorAccountID, record.ActorName, record.ExportKind, record.ExportFormat,
		record.Filename, record.ByteSize, record.TargetHost, record.TargetPort, record.TargetDirectory, record.Status, record.FailureReason).Scan(ctx, &id); err != nil {
		return 0, fmt.Errorf("export transfer postgres: record intent: %w", err)
	}
	return id, nil
}

// Claim installs the dispatch fence for exactly this committed intent.
func (s *Store) Claim(ctx context.Context, id int64) (string, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return "", err
	}
	var token string
	if err := db.NewRaw(`SELECT audit.claim_export_transfer(?,?)`, tenantID, id).Scan(ctx, &token); err != nil {
		return "", err
	}
	if token == "" {
		return "", errors.New("export transfer intent cannot be dispatched")
	}
	return token, nil
}

// Complete settles exactly one live fence. Evidence fields never change.
func (s *Store) Complete(ctx context.Context, id int64, token string, entry domain.JournalEntry) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	var done bool
	if err := db.NewRaw(`SELECT audit.complete_export_transfer(?,?,?,?,?)`, tenantID, id, token, entry.Success, entry.Reason).Scan(ctx, &done); err != nil {
		return err
	}
	if !done {
		return errors.New("export transfer dispatch fence expired")
	}
	return nil
}

// SettleExpired is administrative recovery only. It never has file bytes,
// credentials or a transport, and cannot retry an ambiguous remote operation.
func (s *Store) SettleExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 1000 {
		return 0, errors.New("export transfer recovery batch must be between 1 and 1000")
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	result, err := db.NewRaw(`WITH expired AS (
 SELECT id FROM audit.export_transfers WHERE dispatch_state IN ('pending','dispatching')
 AND dispatch_expires_at <= clock_timestamp() ORDER BY dispatch_expires_at,id LIMIT ? FOR UPDATE SKIP LOCKED)
 UPDATE audit.export_transfers AS attempt SET dispatch_state='unknown', dispatch_token=NULL,
 status='failed',failure_reason='internal_error',updated_at=clock_timestamp()
 FROM expired WHERE attempt.id=expired.id`, limit).Exec(ctx)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	return int(count), err
}

// DeleteExpired removes the school's journal entries before the instant
// through the database function of 1.15.454: the direct DELETE on
// audit.export_transfers stays revoked from the tenant role, and the
// function refuses a cutoff inside the fixed window.
func (s *Store) DeleteExpired(ctx context.Context, before time.Time) (int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(`SELECT audit.delete_expired_export_transfers(?, ?)`, tenantID, before).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("export transfer postgres: delete expired attempts: %w", err)
	}
	return int(deleted), nil
}
