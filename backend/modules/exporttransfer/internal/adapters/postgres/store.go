// Package postgres persists the Export Transfer journal in
// audit.export_transfers.
package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/uptrace/bun"
)

// Database yields the caller's tenant transaction and its tenant id. The
// journal is written on the SAME transaction as the request, so it cannot end
// up describing a state the database never reached.
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

func (s *Store) Record(ctx context.Context, entry domain.JournalEntry) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
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
		return fmt.Errorf("export transfer postgres: a failed attempt must name a reason")
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

	if _, err := db.NewInsert().Model(&record).ModelTableExpr(`audit.export_transfers`).Exec(ctx); err != nil {
		return fmt.Errorf("export transfer postgres: record attempt: %w", err)
	}
	return nil
}
