package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/presenceprojection"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// PresenceReads serves the retained legacy reads that join the Timetable plan
// with the execution and attendance Student Presence owns since #2762. Each
// read goes through the tenant-safe timetable projection inside the caller's
// tenant transaction.
type PresenceReads struct{ database postgres.Database }

func NewPresenceReads(db *bun.DB) (PresenceReads, error) {
	if db == nil {
		return PresenceReads{}, errors.New("timetable presence reads: database is required")
	}
	return PresenceReads{database: databaseRuntime(db)}, nil
}

// OpenParticipant is a participant whose block presence is still open.
type OpenParticipant struct {
	ParticipantID int64
	StudentID     int64
}

// CourseInstanceRow counts the occurrences of one course in a period.
type CourseInstanceRow struct {
	CourseID           int64
	Name               string
	CategoryName       string
	MaxParticipants    int
	HeldInstances      int
	CancelledInstances int
}

// CourseParticipationRow aggregates one child's attendance in one course.
type CourseParticipationRow struct {
	CourseID    int64
	StudentID   int64
	PresentDays int
	AbsentDays  int
	OpenDays    int
}

func (r PresenceReads) CountNonAbsentParticipants(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return presenceprojection.CountNonAbsentParticipants(ctx, db, tenantID, instanceIDs)
}

func (r PresenceReads) ListParallelPresence(ctx context.Context, excludedInstanceID int64, date string, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return presenceprojection.ListParallelPresence(ctx, db, tenantID, excludedInstanceID, timezone.Date(date), studentIDs)
}

func (r PresenceReads) ListPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time, enrolled bool, autoExceptionIDs []int64) ([]scheduleModels.PartialAbsenceBlock, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return presenceprojection.ListPartialAbsenceBlocks(ctx, db, tenantID, studentID, timezone.Date(date), from, enrolled, autoExceptionIDs)
}

func (r PresenceReads) ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) ([]*scheduleModels.ScheduledInstanceRow, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return presenceprojection.ListScheduledInstancesForStudent(ctx, db, tenantID, studentID, timezone.Date(from), timezone.Date(to))
}

func (r PresenceReads) HasPlannedSlotsInRange(ctx context.Context, from, to string) (bool, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return false, err
	}
	return presenceprojection.HasPlannedSlotsInRange(ctx, db, tenantID, timezone.Date(from), timezone.Date(to))
}

func (r PresenceReads) ListOpenParticipants(ctx context.Context, studentIDs []int64) ([]OpenParticipant, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.ListOpenParticipants(ctx, db, tenantID, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]OpenParticipant, 0, len(rows))
	for _, row := range rows {
		result = append(result, OpenParticipant{ParticipantID: row.ParticipantID, StudentID: row.StudentID})
	}
	return result, nil
}

// LatestAttendedBlockDate returns the ISO day of the last block the child was
// checked in to, or nil.
func (r PresenceReads) LatestAttendedBlockDate(ctx context.Context, studentID int64) (*string, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	day, err := presenceprojection.LatestAttendedBlockDate(ctx, db, tenantID, studentID)
	if err != nil || day == nil {
		return nil, err
	}
	text := day.String()
	return &text, nil
}

func (r PresenceReads) CourseInstances(ctx context.Context, from, to, today string) ([]CourseInstanceRow, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.CourseInstances(ctx, db, tenantID, timezone.Date(from), timezone.Date(to), timezone.Date(today))
	if err != nil {
		return nil, err
	}
	result := make([]CourseInstanceRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, CourseInstanceRow(row))
	}
	return result, nil
}

func (r PresenceReads) CourseParticipation(ctx context.Context, from, to, today string) ([]CourseParticipationRow, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.CourseParticipation(ctx, db, tenantID, timezone.Date(from), timezone.Date(to), timezone.Date(today))
	if err != nil {
		return nil, err
	}
	result := make([]CourseParticipationRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, CourseParticipationRow(row))
	}
	return result, nil
}

// LegacyInstanceFilter selects activity instances the way the retained
// repositories ask for them; see presenceprojection.LegacyInstanceFilter.
type LegacyInstanceFilter presenceprojection.LegacyInstanceFilter

// LegacyParticipantFilter selects participants the way the retained
// repositories ask for them; see presenceprojection.LegacyParticipantFilter.
type LegacyParticipantFilter presenceprojection.LegacyParticipantFilter

// ListLegacyInstances lists activity instances joined with their execution
// in one statement.
func (r PresenceReads) ListLegacyInstances(ctx context.Context, filter LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return presenceprojection.ListLegacyInstances(ctx, db, tenantID, presenceprojection.LegacyInstanceFilter(filter))
}

// ListLegacyParticipants lists participants joined with their attendance in
// one statement.
func (r PresenceReads) ListLegacyParticipants(ctx context.Context, filter LegacyParticipantFilter) ([]*scheduleModels.InstanceStudent, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return presenceprojection.ListLegacyParticipants(ctx, db, tenantID, presenceprojection.LegacyParticipantFilter(filter))
}
