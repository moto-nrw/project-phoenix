package audit

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// PersonnelNumberChange is an append-only audit row for a change to a staff
// member's Personalnummer (#1417). The number is what payroll bills against,
// so every change keeps old and new value; note is optional context. Written
// in the same tenant transaction as the update — no change without a trace,
// same contract as audit.time_tracking_deletions.
type PersonnelNumberChange struct {
	bun.BaseModel `bun:"schema:audit,table:personnel_number_changes"`
	ID            int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	StaffID    int64     `bun:"staff_id,notnull" json:"staff_id"`
	ChangedBy  int64     `bun:"changed_by,notnull" json:"changed_by"`
	OldValue   string    `bun:"old_value,notnull" json:"old_value"`
	NewValue   string    `bun:"new_value,notnull" json:"new_value"`
	Note       string    `bun:"note,notnull" json:"note"`
	OccurredAt time.Time `bun:"occurred_at,nullzero,notnull,default:now()" json:"occurred_at"`
}

func (c *PersonnelNumberChange) Validate() error {
	if c.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if c.ChangedBy <= 0 {
		return errors.New("changed_by is required")
	}
	if c.OldValue == c.NewValue {
		return errors.New("old and new value are identical")
	}
	return nil
}

// PersonnelNumberChangeCreator is append-only; reads happen through the
// cross-source audit log query.
type PersonnelNumberChangeCreator interface {
	Create(ctx context.Context, event *PersonnelNumberChange) error
}
