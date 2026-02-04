package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableActiveWorkSessions = "active.work_sessions"

// WorkSessionStatus constants
const (
	WorkSessionStatusPresent    = "present"
	WorkSessionStatusHomeOffice = "home_office"
)

type WorkSession struct {
	base.Model     `bun:"schema:active,table:work_sessions"`
	StaffID        int64      `bun:"staff_id,notnull" json:"staff_id"`
	Date           time.Time  `bun:"date,notnull,type:date" json:"date"`
	Status         string     `bun:"status,notnull,default:'present'" json:"status"`
	CheckInTime    time.Time  `bun:"check_in_time,notnull" json:"check_in_time"`
	CheckOutTime   *time.Time `bun:"check_out_time" json:"check_out_time,omitempty"`
	BreakMinutes   int        `bun:"break_minutes,notnull,default:0" json:"break_minutes"`
	Notes          string     `bun:"notes" json:"notes,omitempty"`
	AutoCheckedOut bool       `bun:"auto_checked_out,notnull,default:false" json:"auto_checked_out"`
	CreatedBy      int64      `bun:"created_by,notnull" json:"created_by"`
	UpdatedBy      *int64     `bun:"updated_by" json:"updated_by,omitempty"`

	Staff *users.Staff `bun:"rel:belongs-to,join:staff_id=id" json:"staff,omitempty"`
}

// BeforeAppendModel implements the model hook for schema-qualified queries
// Must handle ALL query types: SelectQuery, UpdateQuery, DeleteQuery, InsertQuery
func (ws *WorkSession) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessions)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessions)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessions)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActiveWorkSessions)
	}
	return nil
}

func (ws *WorkSession) GetID() interface{}      { return ws.ID }
func (ws *WorkSession) GetCreatedAt() time.Time { return ws.CreatedAt }
func (ws *WorkSession) GetUpdatedAt() time.Time { return ws.UpdatedAt }
func (ws *WorkSession) TableName() string       { return tableActiveWorkSessions }

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

func (ws *WorkSession) CheckOut() {
	now := time.Now()
	ws.CheckOutTime = &now
}

// NetMinutes calculates net work time in minutes (gross minus breaks)
func (ws *WorkSession) NetMinutes() int {
	var end time.Time
	if ws.CheckOutTime != nil {
		end = *ws.CheckOutTime
	} else {
		end = time.Now()
	}
	gross := int(end.Sub(ws.CheckInTime).Minutes())
	net := gross - ws.BreakMinutes
	if net < 0 {
		return 0
	}
	return net
}

// IsOvertime returns true if net work time exceeds 10 hours (600 minutes)
func (ws *WorkSession) IsOvertime() bool {
	return ws.NetMinutes() > 600
}

// IsBreakCompliant checks if breaks comply with German labor law (§4 ArbZG)
func (ws *WorkSession) IsBreakCompliant() bool {
	net := ws.NetMinutes()
	if net <= 360 { // <= 6h: no break required
		return true
	}
	if net <= 540 { // <= 9h: 30 min break required
		return ws.BreakMinutes >= 30
	}
	// > 9h: 45 min break required
	return ws.BreakMinutes >= 45
}
