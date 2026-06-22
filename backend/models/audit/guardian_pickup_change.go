package audit

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Audited field names for a guardian pickup/relationship change. Only the
// safety-critical authority flags are audited; notes/priority are annotations.
const (
	GuardianPickupFieldCanPickup        = "can_pickup"
	GuardianPickupFieldEmergencyContact = "is_emergency_contact"
)

// GuardianPickupChange records a single parent-portal change to a guardian's
// per-child can_pickup / is_emergency_contact flag. Rows are append-only. The
// actor name/email are snapshotted so the trail survives account deletion.
type GuardianPickupChange struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	StudentID         int64 `bun:"student_id,notnull" json:"student_id"`
	GuardianProfileID int64 `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	// ActorAccountID is nulled if the acting account is later deleted; the
	// name/email snapshot preserves who made the change.
	ActorAccountID     *int64    `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	ActorNameSnapshot  *string   `bun:"actor_name_snapshot" json:"actor_name_snapshot,omitempty"`
	ActorEmailSnapshot *string   `bun:"actor_email_snapshot" json:"actor_email_snapshot,omitempty"`
	FieldName          string    `bun:"field_name,notnull" json:"field_name"`
	OldValue           bool      `bun:"old_value,notnull" json:"old_value"`
	NewValue           bool      `bun:"new_value,notnull" json:"new_value"`
	ChangedAt          time.Time `bun:"changed_at,notnull,default:now()" json:"changed_at"`
}

func (e *GuardianPickupChange) TableName() string {
	return "audit.guardian_pickup_changes"
}

func (e *GuardianPickupChange) GetID() interface{} {
	return e.ID
}

func (e *GuardianPickupChange) GetCreatedAt() time.Time {
	return e.ChangedAt
}

func (e *GuardianPickupChange) GetUpdatedAt() time.Time {
	return e.ChangedAt
}

type GuardianPickupChangeRepository interface {
	Create(ctx context.Context, entry *GuardianPickupChange) error
	ListByStudentID(ctx context.Context, studentID int64) ([]*GuardianPickupChange, error)
}
