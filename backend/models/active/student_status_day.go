package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableActiveStudentStatusDays = "active.student_status_days"

const (
	StudentStatusDaySick      = "sick"
	StudentStatusDayExcused   = "excused"
	StudentStatusDayClassTrip = "class_trip"
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
	// Note carries an optional free-text reason supplied alongside the
	// status (currently only parent sick notes set it). Nullable.
	Note *string `bun:"note" json:"note,omitempty"`
}

func (s *StudentStatusDay) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActiveStudentStatusDays)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActiveStudentStatusDays)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActiveStudentStatusDays)
	}
	return nil
}

func (s *StudentStatusDay) GetID() any              { return s.ID }
func (s *StudentStatusDay) GetCreatedAt() time.Time { return s.CreatedAt }
func (s *StudentStatusDay) GetUpdatedAt() time.Time { return s.UpdatedAt }
func (s *StudentStatusDay) TableName() string       { return tableActiveStudentStatusDays }

type StudentStatusDayRepository interface {
	UpsertReported(ctx context.Context, entry *StudentStatusDay) error
	// ArchiveAndClearStatusFlag archives a legacy boolean student flag into
	// student_status_days for the date and clears the flag on
	// users.students. Returns the number of students cleared. Column names
	// must be trusted constants, never user input.
	ArchiveAndClearStatusFlag(ctx context.Context, flagColumn, sinceColumn, status string, date timezone.Date, reportedFallback time.Time, source string) (int64, error)
	// CountActiveClassTripStudents counts distinct students with an
	// uncleared class-trip entry for the date, excluding currently
	// sick/excused students. Used by the dashboard analytics.
	CountActiveClassTripStudents(ctx context.Context, date timezone.Date) (int, error)
	MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error
	MarkClearedByID(ctx context.Context, id int64, clearedAt time.Time, source string) error
	MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, clearedAt time.Time, source string) error
	FindActiveByID(ctx context.Context, id int64) (*StudentStatusDay, error)
	FindActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*StudentStatusDay, error)
	FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*StudentStatusDay, error)
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*StudentStatusDay, error)
}
