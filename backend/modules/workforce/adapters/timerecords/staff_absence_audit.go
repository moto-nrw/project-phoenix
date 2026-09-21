package timerecords

import (
	"errors"
	"time"
)

// StaffAbsenceAudit is the legacy projection of a Workforce audit record.
type StaffAbsenceAudit struct {
	ID         int64     `json:"id"`
	TenantID   int64     `json:"tenant_id"`
	AbsenceID  int64     `json:"absence_id"`
	FromStatus *string   `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status"`
	ActorID    int64     `json:"actor_id"`
	Note       string    `json:"note,omitempty"`
	ChangedAt  time.Time `json:"changed_at"`
	// TypeChange is set when the Leitung rebooked the absence (#3258).
	TypeChange *StaffAbsenceTypeChange `json:"type_change,omitempty"`
}

// StaffAbsenceTypeChange is the old and new type of a rebooked absence.
type StaffAbsenceTypeChange struct {
	FromType   string `json:"from_type"`
	FromTypeID *int64 `json:"from_type_id,omitempty"`
	ToType     string `json:"to_type"`
	ToTypeID   *int64 `json:"to_type_id,omitempty"`
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
