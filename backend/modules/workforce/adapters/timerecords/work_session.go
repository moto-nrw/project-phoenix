package timerecords

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

type WorkSession struct {
	base.Model `bun:"schema:active,table:work_sessions"`
	base.TenantModel
	StaffID        int64         `bun:"staff_id,notnull" json:"staff_id"`
	Date           timezone.Date `bun:"date,notnull,type:date" json:"date"`
	Status         string        `bun:"status,notnull,default:'present'" json:"status"`
	Source         string        `bun:"source,notnull,default:'app'" json:"source"`
	CheckInTime    time.Time     `bun:"check_in_time,notnull" json:"check_in_time"`
	CheckOutTime   *time.Time    `bun:"check_out_time" json:"check_out_time,omitempty"`
	ReopenedAt     *time.Time    `bun:"reopened_at" json:"-"`
	BreakMinutes   int           `bun:"break_minutes,notnull,default:0" json:"break_minutes"`
	Notes          string        `bun:"notes" json:"notes,omitempty"`
	AutoCheckedOut bool          `bun:"auto_checked_out,notnull,default:false" json:"auto_checked_out"`
	CreatedBy      int64         `bun:"created_by,notnull" json:"created_by"`
	UpdatedBy      *int64        `bun:"updated_by" json:"updated_by,omitempty"`
}

func (ws *WorkSession) Validate() error {
	if ws.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if ws.CheckInTime.IsZero() {
		return errors.New("check-in time is required")
	}
	if ws.Status != workforce.WorkSessionStatusPresent && ws.Status != workforce.WorkSessionStatusHomeOffice {
		return errors.New("status must be 'present' or 'home_office'")
	}
	// Source is intentionally not re-validated here — write paths gate it at
	// the service boundary, and legacy 'unknown' rows must round-trip cleanly
	// through partial-update flows (break edits, notes patches).
	if ws.CheckOutTime != nil && ws.CheckInTime.After(*ws.CheckOutTime) {
		return errors.New("check-in time must be before check-out time")
	}
	if ws.BreakMinutes < 0 {
		return errors.New("break minutes cannot be negative")
	}
	if ws.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	return nil
}

func (ws *WorkSession) IsActive() bool {
	return ws.CheckOutTime == nil
}
