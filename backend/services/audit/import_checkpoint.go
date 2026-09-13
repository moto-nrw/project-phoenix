package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// ListImportCheckpoints is Audit's tenant-scoped resume query. The workflow
// appends new receipts through Append in the same UnitOfWork as owner writes.
func (c *Command) ListImportCheckpoints(ctx context.Context, key string) ([]auditModels.ImportCheckpoint, error) {
	reader, ok := c.store.(auditModels.ImportCheckpointReader)
	if !ok {
		return nil, fmt.Errorf("audit command: import checkpoint reader is required")
	}
	return reader.ListImportCheckpoints(ctx, key)
}
