package repositories

import (
	"context"
	"fmt"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// NewTestAuditStore composes the Audit append store over the test database
// for narrow test compositions.
func NewTestAuditStore(db *bun.DB) auditModels.AppendStore {
	return auditRepo.NewAppender(newTestAuditRuntime(db))
}

func newTestAuditRuntime(db *bun.DB) auditRepo.Runtime {
	return func(ctx context.Context) (bun.IDB, int64) {
		tenantID := auditModels.TenantIDFromContext(ctx)
		raw, ok := auditModels.TransactionFromContext(ctx)
		if !ok {
			return db, tenantID
		}
		switch tx := raw.(type) {
		case bun.Tx:
			return tx, tenantID
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID
			}
		}
		panic(fmt.Sprintf("test audit: unsupported transaction %T", raw))
	}
}

// NewFileEventTestRepository retains the Audit command write boundary while
// composing only the file-event query contract.
func NewFileEventTestRepository(db *bun.DB, command auditModels.Command) auditModels.FileEventRepository {
	return fileEventCommand{
		auditRepo.NewFileEventRepository(newTestAuditRuntime(db)),
		command,
	}
}
