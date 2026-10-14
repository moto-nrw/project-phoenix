package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// LogRetention records in audit.data_deletions that a nightly retention step
// deleted deleted rows of one data kind for the school of ctx (#3950). The
// entry names no person and holds no deleted content; kind is one of the
// data-kind deletion types of models/audit.
func (c *Command) LogRetention(ctx context.Context, kind string, deleted, retentionDays int) error {
	if !auditModels.IsDataKindRetention(kind) {
		return fmt.Errorf("audit retention: unknown data kind %q", kind)
	}
	entry := auditModels.NewRetentionDataDeletion(kind, deleted)
	entry.SetMetadata("retention_days", retentionDays)
	return c.Append(ctx, entry)
}
