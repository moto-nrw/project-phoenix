package active

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// WorkSessionStatus constants
const (
	WorkSessionStatusPresent    = "present"
	WorkSessionStatusHomeOffice = "home_office"
)

// WorkSessionSource records which channel produced the row (Issue #1368).
// New writes are restricted to App/NFC by WorkSessionService.CheckIn;
// 'unknown' exists only for rows that pre-date migration 1.15.54 and is
// rejected as a write value but accepted as a read value so legacy rows
// survive partial-update flows (break edits, notes patches). The DB CHECK
// constraint chk_work_sessions_source enforces the same set on disk.
const (
	WorkSessionSourceApp     = "app"     // POST /api/time-tracking/check-in (App / Web)
	WorkSessionSourceNFC     = "nfc"     // Auto-stamp from a kiosk-driven scan
	WorkSessionSourceUnknown = "unknown" // Pre-migration legacy rows; never written by new code
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

// WorkSessionWire serializes a session for endpoints that return the raw
// model. JavaScript rounds int64 values above Number.MAX_SAFE_INTEGER while
// parsing numeric JSON, so converting IDs in the frontend would be too late.
type WorkSessionWire struct {
	*WorkSession
}

func (ws WorkSessionWire) MarshalJSON() ([]byte, error) {
	if ws.WorkSession == nil {
		return []byte("null"), nil
	}
	type alias WorkSession
	return json.Marshal(struct {
		*alias
		ID        string  `json:"id"`
		TenantID  string  `json:"tenant_id"`
		StaffID   string  `json:"staff_id"`
		CreatedBy string  `json:"created_by"`
		UpdatedBy *string `json:"updated_by,omitempty"`
	}{
		alias:     (*alias)(ws.WorkSession),
		ID:        strconv.FormatInt(ws.WorkSession.ID, 10),
		TenantID:  strconv.FormatInt(ws.WorkSession.TenantID, 10),
		StaffID:   strconv.FormatInt(ws.WorkSession.StaffID, 10),
		CreatedBy: strconv.FormatInt(ws.WorkSession.CreatedBy, 10),
		UpdatedBy: formatOptionalID(ws.WorkSession.UpdatedBy),
	})
}

func formatOptionalID(id *int64) *string {
	if id == nil {
		return nil
	}
	value := strconv.FormatInt(*id, 10)
	return &value
}

func (ws *WorkSession) Validate() error {
	if ws.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if ws.CheckInTime.IsZero() {
		return errors.New("check-in time is required")
	}
	if ws.Status != WorkSessionStatusPresent && ws.Status != WorkSessionStatusHomeOffice {
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
