package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type StaffAbsenceAudit struct {
	bun.BaseModel `bun:"schema:active,table:staff_absence_audit"`
	ID            int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	AbsenceID  int64     `bun:"absence_id,notnull" json:"absence_id"`
	FromStatus *string   `bun:"from_status" json:"from_status,omitempty"`
	ToStatus   string    `bun:"to_status,notnull" json:"to_status"`
	ActorID    int64     `bun:"actor_id,notnull" json:"actor_id"`
	Note       string    `bun:"note" json:"note,omitempty"`
	ChangedAt  time.Time `bun:"changed_at,nullzero,notnull,default:current_timestamp" json:"changed_at"`
}

func (a *StaffAbsenceAudit) Validate() error {
	if a.AbsenceID <= 0 {
		return errors.New("absence_id is required")
	}
	if a.ToStatus == "" {
		return errors.New("to_status is required")
	}
	if a.ActorID <= 0 {
		return errors.New("actor_id is required")
	}
	return nil
}
