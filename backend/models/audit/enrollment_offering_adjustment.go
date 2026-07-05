package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// EnrollmentOfferingAdjustment records an admin correction to one approved
// child's care-offering selection. Rows are append-only.
type EnrollmentOfferingAdjustment struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	RequestID          int64           `bun:"request_id,notnull" json:"request_id"`
	RequestChildID     int64           `bun:"request_child_id,notnull" json:"request_child_id"`
	StudentID          int64           `bun:"student_id,notnull" json:"student_id"`
	ActorAccountID     int64           `bun:"actor_account_id,notnull" json:"actor_account_id"`
	ActorRole          string          `bun:"actor_role,notnull" json:"actor_role"`
	ActorNameSnapshot  *string         `bun:"actor_name_snapshot" json:"actor_name_snapshot,omitempty"`
	ActorEmailSnapshot *string         `bun:"actor_email_snapshot" json:"actor_email_snapshot,omitempty"`
	Reason             string          `bun:"reason,notnull" json:"reason"`
	Before             json.RawMessage `bun:"before_json,type:jsonb,notnull" json:"before"`
	After              json.RawMessage `bun:"after_json,type:jsonb,notnull" json:"after"`
	ChangedAt          time.Time       `bun:"changed_at,notnull,default:now()" json:"changed_at"`
}

func (e *EnrollmentOfferingAdjustment) GetID() interface{} {
	return e.ID
}

func (e *EnrollmentOfferingAdjustment) GetCreatedAt() time.Time {
	return e.ChangedAt
}

func (e *EnrollmentOfferingAdjustment) GetUpdatedAt() time.Time {
	return e.ChangedAt
}

type EnrollmentOfferingAdjustmentRepository interface {
	Create(ctx context.Context, entry *EnrollmentOfferingAdjustment) error
	ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*EnrollmentOfferingAdjustment, error)
}
