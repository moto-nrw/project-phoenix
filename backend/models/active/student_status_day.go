package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	StudentStatusDaySick      = "sick"
	StudentStatusDayExcused   = "excused"
	StudentStatusDayClassTrip = "class_trip"
	// StudentStatusDayPresent is not a stored status: it is what a day with no
	// active status row means. Named here so every reader that has to answer
	// "what does this day look like now" says the same word.
	StudentStatusDayPresent = "present"
)

func StudentStatusDayStatuses() []string {
	return []string{
		StudentStatusDaySick,
		StudentStatusDayExcused,
		StudentStatusDayClassTrip,
	}
}

func StudentStatusDayStatusesExcept(status string) []string {
	statuses := StudentStatusDayStatuses()
	result := make([]string, 0, len(statuses)-1)
	for _, candidate := range statuses {
		if candidate != status {
			result = append(result, candidate)
		}
	}
	return result
}

const (
	StudentStatusSourceManual      = "manual"
	StudentStatusSourcePlanned     = "planned"
	StudentStatusSourceNextCheckin = "next_checkin"
	StudentStatusSourceEndOfDay    = "end_of_day"
	// StudentStatusSourceParent marks a status day reported by a
	// guardian through the parents portal (vs. entered by staff).
	StudentStatusSourceParent = "parent"
)

type StudentStatusDay struct {
	base.Model `bun:"schema:active,table:student_status_days"`
	base.TenantModel
	StudentID  int64         `bun:"student_id,notnull" json:"student_id"`
	Date       timezone.Date `bun:"date,notnull,type:date" json:"date"`
	Status     string        `bun:"status,notnull" json:"status"`
	ReportedAt time.Time     `bun:"reported_at,notnull" json:"reported_at"`
	ClearedAt  *time.Time    `bun:"cleared_at" json:"cleared_at,omitempty"`
	Source     string        `bun:"source,notnull" json:"source"`
	// GuardianAccountID identifies who supplied a parent-authored note. It is
	// never serialized: other guardians may see the effective absence, but not
	// the author or free-text reason.
	GuardianAccountID *int64 `bun:"guardian_account_id" json:"-"`
	// Note carries an optional free-text reason supplied alongside the
	// status (currently only parent sick notes set it). Nullable.
	Note *string `bun:"note" json:"note,omitempty"`
}

type StudentStatusCounts struct {
	Sick    int `bun:"sick_count"`
	Excused int `bun:"excused_count"`
	// Total is every active student of the tenant, absent or not. The
	// dashboard needs it to derive "at home" as the remainder after presence
	// and the absence buckets, which is why it comes from the same query —
	// counting it separately would let the two drift apart under a concurrent
	// write.
	Total int `bun:"total_count"`
}

// StudentStatusDayRepository persists broad day statuses (sick / excused /
// class trip) and CASCADES them into per-slot attendance: UpsertReported and
// the MarkCleared* methods also apply/release the status on matching
// schedule.instance_students rows (see #1913). The cascade lives here — not in
// a service — because absence, parent, and active services call this
// repository directly; repo placement guarantees no write path skips it.
type StudentStatusDayRepository interface {
	// UpsertReported inserts or refreshes a reported status day AND marks the
	// student's still-expected slots on that date absent with status-day
	// provenance (schedule.instance_students.student_status_day_id).
	UpsertReported(ctx context.Context, entry *StudentStatusDay) error
	// ArchiveAndClearStatusFlag archives a legacy boolean student flag into
	// student_status_days for the date and clears the flag on
	// users.students. Returns the number of students cleared. Column names
	// must be trusted constants, never user input.
	ArchiveAndClearStatusFlag(ctx context.Context, flagColumn, sinceColumn, status string, date timezone.Date, reportedFallback time.Time, source string) (int64, error)
	// CountEffectiveDashboardAbsences counts today's effective dashboard
	// absence buckets from live flags and status-day rows, applying the same
	// precedence as student responses: sick wins, class trip counts as excused.
	CountEffectiveDashboardAbsences(ctx context.Context, date timezone.Date) (*StudentStatusCounts, error)
	// MarkCleared / MarkClearedByID / MarkClearedForDates clear status days
	// AND release the cascade: slot absences owned by the cleared status day
	// revert to the latest remaining active status for that date, or back to
	// expected (absent when the instance already completed).
	MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error
	MarkClearedByID(ctx context.Context, id int64, clearedAt time.Time, source string) error
	MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, clearedAt time.Time, source string) error
	FindActiveByID(ctx context.Context, id int64) (*StudentStatusDay, error)
	FindActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*StudentStatusDay, error)
	FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*StudentStatusDay, error)
	// FindSignedOffByStudentIDsAndDate returns active rows plus end-of-day
	// archived rows (source = "end_of_day") for the date — the full set of
	// valid registered sign-offs for that day.
	FindSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*StudentStatusDay, error)
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*StudentStatusDay, error)
}

// StudentStatusDayOverviewRepository adds the ordered read required by the
// paginated absence overview.
type StudentStatusDayOverviewRepository interface {
	StudentStatusDayRepository
	ListOverviewWithOptions(ctx context.Context, options *base.QueryOptions, orderedStudentIDs []int64) ([]*StudentStatusDay, error)
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
}
