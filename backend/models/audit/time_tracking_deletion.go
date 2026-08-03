package audit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	TimeTrackingDeletionSourceBalanceAdjustment = "balance_adjustment"
	TimeTrackingDeletionSourceAbsence           = "absence"
	TimeTrackingDeletionSourceVacationOpening   = "vacation_opening"
)

// TimeTrackingDeletion is an append-only tombstone for a deleted balance
// adjustment or staff absence (#1417). Payload holds the full deleted row —
// these are working-time records the employer must retain, so unlike the
// data-minimal enrollment deletion audit the values stay.
type TimeTrackingDeletion struct {
	bun.BaseModel `bun:"schema:audit,table:time_tracking_deletions"`
	ID            int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	StaffID    int64           `bun:"staff_id,notnull" json:"staff_id"`
	Source     string          `bun:"source,notnull" json:"source"`
	SourceID   int64           `bun:"source_id,notnull" json:"source_id"`
	DeletedBy  int64           `bun:"deleted_by,notnull" json:"deleted_by"`
	Payload    json.RawMessage `bun:"payload,type:jsonb,notnull" json:"payload"`
	Note       string          `bun:"note,notnull" json:"note"`
	OccurredAt time.Time       `bun:"occurred_at,nullzero,notnull,default:now()" json:"occurred_at"`
}

func (d *TimeTrackingDeletion) Validate() error {
	if d.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if d.Source != TimeTrackingDeletionSourceBalanceAdjustment &&
		d.Source != TimeTrackingDeletionSourceAbsence &&
		d.Source != TimeTrackingDeletionSourceVacationOpening {
		return errors.New("invalid deletion source")
	}
	if d.SourceID <= 0 {
		return errors.New("source_id is required")
	}
	if d.DeletedBy <= 0 {
		return errors.New("deleted_by is required")
	}
	if len(d.Payload) == 0 {
		return errors.New("payload is required")
	}
	return nil
}

// TimeTrackingDeletionRepository is append-only; reads happen through the
// cross-source audit log query.
type TimeTrackingDeletionRepository interface {
	Create(ctx context.Context, event *TimeTrackingDeletion) error
}
