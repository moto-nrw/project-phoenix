package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/uptrace/bun"
)

// guardianChangeRow omits id and changed_at so the table's BIGSERIAL and
// DEFAULT NOW() fill them, exactly as for the retained repository.
type guardianChangeRow struct {
	bun.BaseModel      `bun:"table:audit.guardian_changes"`
	TenantID           int64   `bun:"tenant_id"`
	StudentID          int64   `bun:"student_id"`
	GuardianProfileID  int64   `bun:"guardian_profile_id"`
	ActorAccountID     *int64  `bun:"actor_account_id"`
	ActorNameSnapshot  *string `bun:"actor_name_snapshot"`
	ActorEmailSnapshot *string `bun:"actor_email_snapshot"`
	ChangeType         string  `bun:"change_type"`
	FieldName          string  `bun:"field_name"`
	OldValue           *string `bun:"old_value"`
	NewValue           *string `bun:"new_value"`
}

type GuardianChanges struct{ database Database }

func NewGuardianChanges(database Database) *GuardianChanges {
	return &GuardianChanges{database: database}
}

func (s *GuardianChanges) RecordGuardianChanges(ctx context.Context, changes []auditlog.GuardianChange) error {
	if len(changes) == 0 {
		return nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	rows := make([]guardianChangeRow, 0, len(changes))
	for _, change := range changes {
		rows = append(rows, guardianChangeRow{
			TenantID: tenantID, StudentID: change.StudentID, GuardianProfileID: change.GuardianProfileID,
			ActorAccountID: change.ActorAccountID, ActorNameSnapshot: change.ActorNameSnapshot, ActorEmailSnapshot: change.ActorEmailSnapshot,
			ChangeType: change.ChangeType, FieldName: change.FieldName, OldValue: change.OldValue, NewValue: change.NewValue,
		})
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("audit database error during append audit.guardian_changes: %w", err)
	}
	return nil
}
