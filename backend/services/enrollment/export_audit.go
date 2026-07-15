package enrollment

import (
	"context"
	"fmt"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// exportAuditEntry validates the acting account, defaults the role (the
// actor_role column is NOT NULL and never carries an empty string), and
// builds the base DataAccessLog row shared by all enrollment export-audit
// writers. Metadata stays with each writer - it is the observable audit
// content and differs per export.
func exportAuditEntry(errPrefix string, actorAccountID int64, actorRole, resourceType string, rangeStart, rangeEnd, accessedAt time.Time) (*auditModels.DataAccessLog, error) {
	if actorAccountID <= 0 {
		return nil, fmt.Errorf("%s: actor account id required", errPrefix)
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}
	return &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		ResourceType:   resourceType,
		RangeStart:     rangeStart,
		RangeEnd:       rangeEnd,
		AccessedAt:     accessedAt,
	}, nil
}

// writeExportAudit persists the audit row, wrapping failures with the
// writer's historical error prefix.
func writeExportAudit(ctx context.Context, repo auditModels.DataAccessLogRepository, entry *auditModels.DataAccessLog, errPrefix string) error {
	if err := repo.Create(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}
