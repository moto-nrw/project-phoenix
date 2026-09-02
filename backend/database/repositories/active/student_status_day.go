package active

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const tableExprActiveStudentStatusDaysAsStatusDay = `active.student_status_days AS "student_status_day"`

type StudentStatusDayRepository struct {
	*base.Repository[*active.StudentStatusDay]
	db       *bun.DB
	slotRepo scheduleModels.InstanceStudentRepository
	students StudentDirectory
}

// BindStudentDirectory installs the People Directory the dashboard absence
// counts read the active students and their live flags through (#2662).
func (r *StudentStatusDayRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
}

func NewStudentStatusDayRepository(db *bun.DB) active.StudentStatusDayOverviewRepository {
	repo := base.NewRepository[*active.StudentStatusDay](db, "active.student_status_days", "StudentStatusDay")
	repo.TenantScoped = true
	return &StudentStatusDayRepository{
		Repository: repo,
		db:         db,
		slotRepo:   scheduleRepo.NewInstanceStudentRepository(db),
	}
}

// ListOverviewWithOptions returns the matching rows in the supplied person
// order. The service owns person lookups, while PostgreSQL applies that order
// before pagination.
func (r *StudentStatusDayRepository) ListOverviewWithOptions(ctx context.Context, options *modelBase.QueryOptions, orderedStudentIDs []int64) ([]*active.StudentStatusDay, error) {
	var rows []*active.StudentStatusDay
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		ColumnExpr(`"student_status_day".*`)
	if len(orderedStudentIDs) > 0 {
		query = query.OrderExpr(`"student_status_day".date ASC`).
			OrderExpr(`array_position(?::bigint[], "student_status_day".student_id) ASC`, pgdialect.Array(orderedStudentIDs)).
			OrderExpr(`"student_status_day".reported_at DESC`).
			OrderExpr(`"student_status_day".id ASC`)
	} else {
		query = query.OrderExpr(`"student_status_day".date ASC`).
			OrderExpr(`"student_status_day".student_id ASC`).
			OrderExpr(`"student_status_day".reported_at DESC`).
			OrderExpr(`"student_status_day".id ASC`)
	}
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"student_status_day".tenant_id = ?`, tenantID)
	}
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("student_status_day")
			query = base.ApplyFilter(query, options.Filter)
		}
		if options.Pagination != nil {
			query = base.ApplyPagination(query, *options.Pagination)
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list status day overview", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

func (r *StudentStatusDayRepository) UpsertReported(ctx context.Context, entry *active.StudentStatusDay) error {
	if entry == nil {
		return fmt.Errorf("student status day cannot be nil")
	}
	if entry.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			entry.SetTenantID(tid)
		}
	}
	if entry.Date.IsZero() {
		return fmt.Errorf("student status day date is required")
	}

	err := base.GetDB(ctx, r.db).NewInsert().
		Model(entry).
		ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO UPDATE").
		Set("reported_at = EXCLUDED.reported_at").
		Set("cleared_at = NULL").
		Set("source = EXCLUDED.source").
		// Note handling depends on whether the existing row is still active.
		// On an active re-report (cleared_at IS NULL — e.g. editing other
		// fields while sick) preserve the prior reason when none is supplied.
		// On a reactivation of a previously-cleared row, take the incoming
		// note verbatim so a stale reason from the old, superseded report
		// can't resurface. The references read the OLD row, since Postgres
		// evaluates all SET expressions against the pre-update tuple.
		Set(`note = CASE
			WHEN student_status_days.cleared_at IS NULL
			THEN COALESCE(EXCLUDED.note, student_status_days.note)
			ELSE EXCLUDED.note
		END`).
		Set(`guardian_account_id = CASE
			WHEN student_status_days.cleared_at IS NULL AND EXCLUDED.note IS NULL
			THEN student_status_days.guardian_account_id
			ELSE EXCLUDED.guardian_account_id
		END`).
		Returning("id").
		Scan(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert student status day", Err: base.TranslateNotFound(err)}
	}
	_, err = r.slotRepo.ApplyStatusDay(ctx, entry.StudentID, entry.Date, entry.ID, attendanceSubstatus(entry.Status))
	return err
}

func attendanceSubstatus(status string) string {
	switch status {
	case active.StudentStatusDaySick:
		return scheduleModels.AttendanceSubstatusSick
	case active.StudentStatusDayExcused:
		return scheduleModels.AttendanceSubstatusExcused
	case active.StudentStatusDayClassTrip:
		return scheduleModels.AttendanceSubstatusFieldTrip
	default:
		return scheduleModels.AttendanceSubstatusOther
	}
}

func (r *StudentStatusDayRepository) MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error {
	ids, err := r.findActiveIDs(ctx, studentID, status, []timezone.Date{date})
	if err != nil {
		return err
	}
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.StudentStatusDay)(nil)).
		ModelTableExpr("active.student_status_days").
		Set("cleared_at = ?", clearedAt).
		Set("source = ?", source).
		Where("student_id = ?", studentID).
		Where("date = ?", date).
		Where("status = ?", status).
		Where("cleared_at IS NULL")

	if tid := tenant.FromContext(ctx); tid != 0 {
		query = query.Where("tenant_id = ?", tid)
	}

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark student status day cleared", Err: base.TranslateNotFound(err)}
	}
	return r.releaseStatusDays(ctx, ids)
}

func (r *StudentStatusDayRepository) MarkClearedByID(ctx context.Context, id int64, clearedAt time.Time, source string) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.StudentStatusDay)(nil)).
		ModelTableExpr("active.student_status_days").
		Set("cleared_at = ?", clearedAt).
		Set("source = ?", source).
		Where("id = ?", id).
		Where("cleared_at IS NULL")

	if tid := tenant.FromContext(ctx); tid != 0 {
		query = query.Where("tenant_id = ?", tid)
	}

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark student status day cleared by id", Err: base.TranslateNotFound(err)}
	}
	_, err := r.slotRepo.ReleaseStatusDay(ctx, id)
	return err
}

func (r *StudentStatusDayRepository) MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, clearedAt time.Time, source string) error {
	if len(dates) == 0 {
		return nil
	}

	dateOnly := make([]timezone.Date, 0, len(dates))
	seen := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		if _, ok := seen[date]; ok {
			continue
		}
		seen[date] = struct{}{}
		dateOnly = append(dateOnly, date)
	}
	ids, err := r.findActiveIDs(ctx, studentID, status, dateOnly)
	if err != nil {
		return err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.StudentStatusDay)(nil)).
		ModelTableExpr("active.student_status_days").
		Set("cleared_at = ?", clearedAt).
		Set("source = ?", source).
		Where("student_id = ?", studentID).
		Where("status = ?", status).
		Where("date IN (?)", bun.List(dateOnly)).
		Where("cleared_at IS NULL")

	if tid := tenant.FromContext(ctx); tid != 0 {
		query = query.Where("tenant_id = ?", tid)
	}

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark student status days cleared for dates", Err: base.TranslateNotFound(err)}
	}
	return r.releaseStatusDays(ctx, ids)
}

func (r *StudentStatusDayRepository) findActiveIDs(
	ctx context.Context, studentID int64, status string, dates []timezone.Date,
) ([]int64, error) {
	if len(dates) == 0 {
		return []int64{}, nil
	}
	var ids []int64
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		ColumnExpr(`"student_status_day".id`).
		Where(`"student_status_day".student_id = ?`, studentID).
		Where(`"student_status_day".status = ?`, status).
		Where(`"student_status_day".date IN (?)`, bun.List(dates)).
		Where(`"student_status_day".cleared_at IS NULL`)
	query = base.WithTenantFilter(ctx, query, "student_status_day")
	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active student status day ids", Err: base.TranslateNotFound(err)}
	}
	return ids, nil
}

func (r *StudentStatusDayRepository) releaseStatusDays(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		if _, err := r.slotRepo.ReleaseStatusDay(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *StudentStatusDayRepository) FindActiveByID(ctx context.Context, id int64) (*active.StudentStatusDay, error) {
	entry := new(active.StudentStatusDay)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(entry).
		ModelTableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		Where(`"student_status_day".id = ?`, id).
		Where(`"student_status_day".cleared_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "student_status_day")

	if err := query.Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, &modelBase.DatabaseError{Op: "find active student status day by id", Err: base.TranslateNotFound(err)}
	}
	return entry, nil
}

func (r *StudentStatusDayRepository) FindActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*active.StudentStatusDay, error) {
	entries, err := r.findByStudentAndDateRange(ctx, studentID, startDate, endDate, true)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *StudentStatusDayRepository) FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*active.StudentStatusDay, error) {
	if len(studentIDs) == 0 {
		return []*active.StudentStatusDay{}, nil
	}

	var entries []*active.StudentStatusDay
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		Where(`"student_status_day".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"student_status_day".date = ?`, date).
		Where(`"student_status_day".cleared_at IS NULL`).
		OrderExpr(`"student_status_day".student_id ASC`).
		OrderExpr(`"student_status_day".reported_at DESC`)

	query = base.WithTenantFilter(ctx, query, "student_status_day")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active student status days by student ids and date", Err: base.TranslateNotFound(err)}
	}
	return entries, nil
}

// FindSignedOffByStudentIDsAndDate loads the day statuses that count as a valid
// registered sign-off for a date: rows still active (cleared_at IS NULL) plus
// rows archived by the end-of-day scheduler (source = "end_of_day"). The
// scheduler archives the legacy sick/excused flags after the configured clear
// time by stamping cleared_at and source = end_of_day, which the active-only
// query would drop — but a child who was sick/excused all day is still a
// registered absence, not an unexplained "Fehlt" (#1565 review pass 1). A row
// cleared by any other source (next_checkin, manual, parent, planned) is a
// genuine revocation and stays excluded.
func (r *StudentStatusDayRepository) FindSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*active.StudentStatusDay, error) {
	if len(studentIDs) == 0 {
		return []*active.StudentStatusDay{}, nil
	}

	var entries []*active.StudentStatusDay
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		Where(`"student_status_day".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"student_status_day".date = ?`, date).
		Where(`("student_status_day".cleared_at IS NULL OR "student_status_day".source = ?)`, active.StudentStatusSourceEndOfDay).
		OrderExpr(`"student_status_day".student_id ASC`).
		OrderExpr(`"student_status_day".reported_at DESC`)

	query = base.WithTenantFilter(ctx, query, "student_status_day")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find signed-off student status days by student ids and date", Err: base.TranslateNotFound(err)}
	}
	return entries, nil
}

func (r *StudentStatusDayRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*active.StudentStatusDay, error) {
	return r.findByStudentAndDateRange(ctx, studentID, startDate, endDate, false)
}

func (r *StudentStatusDayRepository) findByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date, activeOnly bool) ([]*active.StudentStatusDay, error) {
	var entries []*active.StudentStatusDay

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		Where(`"student_status_day".student_id = ?`, studentID).
		Where(`"student_status_day".date >= ?`, startDate).
		Where(`"student_status_day".date <= ?`, endDate).
		OrderExpr(`"student_status_day".date DESC`).
		OrderExpr(`"student_status_day".reported_at DESC`)

	if activeOnly {
		query = query.Where(`"student_status_day".cleared_at IS NULL`)
	}

	query = base.WithTenantFilter(ctx, query, "student_status_day")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find student status days by date range", Err: base.TranslateNotFound(err)}
	}
	return entries, nil
}

// ArchiveAndClearStatusFlag archives the given legacy student status flag
// into active.student_status_days for the given date (upsert) and then
// clears the flag + its timestamp on users.students for every flagged
// student. Returns the number of students cleared.
//
// The People Directory supplies the flagged students and clears its own
// columns. This repository only owns the archived status-day upsert.
func (r *StudentStatusDayRepository) ArchiveAndClearStatusFlag(
	ctx context.Context,
	flagColumn, sinceColumn, status string,
	date timezone.Date,
	reportedFallback time.Time,
	source string,
) (int64, error) {
	if r.students == nil {
		return 0, errStudentDirectoryRequired
	}
	if (status == active.StudentStatusDaySick && (flagColumn != "sick" || sinceColumn != "sick_since")) ||
		(status == active.StudentStatusDayExcused && (flagColumn != "excused" || sinceColumn != "excused_since")) {
		return 0, fmt.Errorf("status flag columns do not match status %q", status)
	}
	students, err := r.students.ListStudentsWithStatusFlag(ctx, status)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "list students with status flag", Err: base.TranslateNotFound(err)}
	}
	if len(students) == 0 {
		return 0, nil
	}
	entries := make([]*active.StudentStatusDay, 0, len(students))
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		reportedAt := reportedFallback
		if status == active.StudentStatusDaySick && student.SickSince != nil {
			reportedAt = *student.SickSince
		}
		if status == active.StudentStatusDayExcused && student.ExcusedSince != nil {
			reportedAt = *student.ExcusedSince
		}
		entries = append(entries, &active.StudentStatusDay{
			TenantModel: modelBase.TenantModel{TenantID: student.TenantID}, StudentID: student.ID,
			Date: date, Status: status, ReportedAt: reportedAt, ClearedAt: &reportedFallback, Source: source,
		})
		ids = append(ids, student.ID)
	}
	if _, err := base.GetDB(ctx, r.db).NewInsert().Model(&entries).
		ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO UPDATE").
		Set("reported_at = EXCLUDED.reported_at").
		Set("cleared_at = EXCLUDED.cleared_at").
		Set("source = EXCLUDED.source").Exec(ctx); err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "archive status flag",
			Err: base.TranslateNotFound(err),
		}
	}
	affected, err := r.students.ClearStudentStatusFlags(ctx, ids, status)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "clear status flag",
			Err: base.TranslateNotFound(err),
		}
	}
	return affected, nil
}
