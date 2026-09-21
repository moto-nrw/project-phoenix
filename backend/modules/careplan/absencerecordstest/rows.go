// Package absencerecordstest maps the two Care Plan absence tables,
// active.student_status_days and active.excused_absence_requests, for test
// fixtures and for tests that arrange or assert stored rows directly. Care
// Plan's internal Postgres adapter owns the runtime mapping; production code
// exchanges the plain values of modules/careplan/absencerecords (#3422).
package absencerecordstest

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// StudentStatusDayRow is one stored row of active.student_status_days.
type StudentStatusDayRow struct {
	bun.BaseModel     `bun:"table:active.student_status_days,alias:student_status_day"`
	ID                int64         `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt         time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt         time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID          int64         `bun:"tenant_id,notnull" json:"tenant_id"`
	StudentID         int64         `bun:"student_id,notnull" json:"student_id"`
	Date              timezone.Date `bun:"date,notnull,type:date" json:"date"`
	Status            string        `bun:"status,notnull" json:"status"`
	ReportedAt        time.Time     `bun:"reported_at,notnull" json:"reported_at"`
	ClearedAt         *time.Time    `bun:"cleared_at" json:"cleared_at,omitempty"`
	Source            string        `bun:"source,notnull" json:"source"`
	GuardianAccountID *int64        `bun:"guardian_account_id" json:"-"`
	Note              *string       `bun:"note" json:"note,omitempty"`
}

// ExcusedAbsenceRequestRow is one stored row of active.excused_absence_requests.
type ExcusedAbsenceRequestRow struct {
	bun.BaseModel  `bun:"table:active.excused_absence_requests,alias:excused_absence_request"`
	ID             int64           `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt      time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt      time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID       int64           `bun:"tenant_id,notnull" json:"tenant_id"`
	StudentID      int64           `bun:"student_id,notnull" json:"student_id"`
	SubmittedBy    int64           `bun:"submitted_by,notnull" json:"submitted_by"`
	Dates          []timezone.Date `bun:"dates,type:jsonb,notnull" json:"dates"`
	Note           string          `bun:"note,notnull" json:"note"`
	AbsenceStatus  string          `bun:"absence_status,notnull,default:'excused'" json:"absence_status"`
	Status         string          `bun:"status,notnull,default:'pending'" json:"status"`
	DecisionReason *string         `bun:"decision_reason" json:"decision_reason,omitempty"`
	ReviewedBy     *int64          `bun:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time      `bun:"reviewed_at" json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time      `bun:"applied_at" json:"applied_at,omitempty"`
}
