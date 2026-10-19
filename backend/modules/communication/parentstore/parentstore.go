package parentstore

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/uptrace/bun"
)

// GuardianLinkRows is the student-guardian relationship row source the
// inbox and audience projections join in their own statements; the roots
// bind the guardian-link projection (ADR 0058).
type GuardianLinkRows interface {
	GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery
}

func postgresDatabase(db *bun.DB) parentpostgres.Database {
	return parentpostgres.NewParentRecordDatabase(db)
}
func ProjectionDatabase(db *bun.DB) func(context.Context) (bun.IDB, int64, error) {
	return parentpostgres.NewProjectionDatabase(db)
}
