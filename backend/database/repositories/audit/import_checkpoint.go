package audit

import (
	"context"
	"encoding/json"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// ListImportCheckpoints uses both the ambient tenant and RLS. It deliberately
// rejects a tenantless runtime, even when the underlying connection is admin.
func (a *Appender) ListImportCheckpoints(ctx context.Context, key string) ([]auditModels.ImportCheckpoint, error) {
	db, tenantID, err := database(ctx, a.runtime)
	if err != nil {
		return nil, err
	}
	if tenantID <= 0 {
		return nil, fmt.Errorf("import checkpoints require a tenant")
	}
	var rows []struct {
		Receipt json.RawMessage `bun:"receipt"`
	}
	err = db.NewSelect().TableExpr("audit.data_imports").
		ColumnExpr("metadata -> 'import_checkpoint' AS receipt").
		Where("tenant_id = ?", tenantID).
		Where("metadata -> 'import_checkpoint' IS NOT NULL").
		Where("metadata -> 'import_checkpoint' ->> 'import_key' = ?", key).
		OrderExpr("id ASC").Scan(ctx, &rows)
	if err != nil {
		return nil, wrapDatabase("read import checkpoints", err)
	}
	checkpoints := make([]auditModels.ImportCheckpoint, 0, len(rows))
	for _, row := range rows {
		var checkpoint auditModels.ImportCheckpoint
		if err := json.Unmarshal(row.Receipt, &checkpoint); err != nil {
			return nil, fmt.Errorf("decode import checkpoint: %w", err)
		}
		if err := checkpoint.Validate(); err != nil {
			return nil, err
		}
		checkpoints = append(checkpoints, checkpoint)
	}
	return checkpoints, nil
}
