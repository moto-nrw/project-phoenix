// Package absencerecords is the retained record shape of the two Care Plan
// absence tables, active.student_status_days and active.excused_absence_requests,
// for the consumers that still exchange whole rows with the Care Plan
// repository adapters in database/repositories instead of the Care Plan
// facade (#3422). Care Plan's internal Postgres adapter owns the persistence;
// these are plain values without ORM mapping, and the repository contracts
// over them are owned by their consumers. The package goes with the last
// consumer that has moved to the facade.
package absencerecords

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
)

// Broad day statuses. The values are the persisted column values.
const (
	StudentStatusDaySick      = excusedrequests.StudentStatusDaySick
	StudentStatusDayExcused   = excusedrequests.StudentStatusDayExcused
	StudentStatusDayClassTrip = excusedrequests.StudentStatusDayClassTrip
	// StudentStatusDayPresent is not a stored status: it is what a day with no
	// active status row means. Named here so every reader that has to answer
	// "what does this day look like now" says the same word.
	StudentStatusDayPresent = excusedrequests.StudentStatusDayPresent
)

// StudentStatusDayStatuses lists the persisted broad day statuses.
func StudentStatusDayStatuses() []string {
	return excusedrequests.StudentStatusDayStatuses()
}

// StudentStatusDayStatusesExcept lists the persisted statuses other than the
// given one.
func StudentStatusDayStatusesExcept(status string) []string {
	return excusedrequests.StudentStatusDayStatusesExcept(status)
}

// Status-day sources. The values are the persisted column values.
const (
	StudentStatusSourceManual      = excusedrequests.StudentStatusSourceManual
	StudentStatusSourcePlanned     = "planned"
	StudentStatusSourceNextCheckin = "next_checkin"
	StudentStatusSourceEndOfDay    = "end_of_day"
	// StudentStatusSourceParent marks a status day reported by a
	// guardian through the parents portal (vs. entered by staff).
	StudentStatusSourceParent = excusedrequests.StudentStatusSourceParent
)

// StudentStatusDay is one row of active.student_status_days.
type StudentStatusDay struct {
	ID         int64         `json:"id"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
	TenantID   int64         `json:"tenant_id"`
	StudentID  int64         `json:"student_id"`
	Date       timezone.Date `json:"date"`
	Status     string        `json:"status"`
	ReportedAt time.Time     `json:"reported_at"`
	ClearedAt  *time.Time    `json:"cleared_at,omitempty"`
	Source     string        `json:"source"`
	// GuardianAccountID identifies who supplied a parent-authored note. It is
	// never serialized: other guardians may see the effective absence, but not
	// the author or free-text reason.
	GuardianAccountID *int64 `json:"-"`
	// Note carries an optional free-text reason supplied alongside the
	// status (currently only parent sick notes set it). Nullable.
	Note *string `json:"note,omitempty"`
}

// StudentStatusCounts is the effective dashboard absence count of one day.
type StudentStatusCounts struct {
	Sick    int
	Excused int
	// Total is every active student of the tenant, absent or not. The
	// dashboard needs it to derive "at home" as the remainder after presence
	// and the absence buckets, which is why it comes from the same query —
	// counting it separately would let the two drift apart under a concurrent
	// write.
	Total int
	// UnaccountedIDs are the active students counted in Total but in neither
	// absence bucket.
	UnaccountedIDs []int64
}
