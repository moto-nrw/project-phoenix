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
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tableExprActiveStudentStatusDaysAsStatusDay = `active.student_status_days AS "student_status_day"`

type StudentStatusDayRepository struct {
	*base.Repository[*active.StudentStatusDay]
	db       *bun.DB
	slotRepo scheduleModels.InstanceStudentRepository
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

// ListOverviewWithOptions returns the matching rows in a deterministic
// date/student order. The overview's name order and its pagination are
// applied by the service once it has resolved the persons, so callers that
// need name order pass options without pagination and page the sorted
// result themselves.
func (r *StudentStatusDayRepository) ListOverviewWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StudentStatusDay, error) {
	var rows []*active.StudentStatusDay
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		ColumnExpr(`"student_status_day".*`).
		OrderExpr(`"student_status_day".date ASC`).
		OrderExpr(`"student_status_day".student_id ASC`).
		OrderExpr(`"student_status_day".reported_at DESC`).
		OrderExpr(`"student_status_day".id ASC`)
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
// Custom raw-SQL method (backend-conventions Rule 2/11 exception): the
// INSERT...SELECT upsert spans users.students → active.student_status_days
// and is not expressible through the generic shape. Column names are
// interpolated but MUST be trusted constants from the scheduler's fixed
// flag set ("sick"/"sick_since", "excused"/"excused_since") — never user
// input. Tenant scoping comes from the caller's per-tenant RLS transaction.
func (r *StudentStatusDayRepository) ArchiveAndClearStatusFlag(
	ctx context.Context,
	flagColumn, sinceColumn, status string,
	date timezone.Date,
	reportedFallback time.Time,
	source string,
) (int64, error) {
	db := base.GetDB(ctx, r.db)

	upsertQuery := fmt.Sprintf(`
		INSERT INTO active.student_status_days
			(tenant_id, student_id, date, status, reported_at, cleared_at, source)
		SELECT tenant_id, id, ?, ?, COALESCE(%s, ?), ?, ?
		FROM users.students
		WHERE %s = TRUE
		ON CONFLICT (tenant_id, student_id, date, status) DO UPDATE
		SET reported_at = EXCLUDED.reported_at,
		    cleared_at = EXCLUDED.cleared_at,
		    source = EXCLUDED.source;
	`, sinceColumn, flagColumn)
	if _, err := db.NewRaw(upsertQuery, date, status, reportedFallback, reportedFallback, source).Exec(ctx); err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "archive status flag",
			Err: base.TranslateNotFound(err),
		}
	}

	clearQuery := fmt.Sprintf(
		`UPDATE users.students SET %s = FALSE, %s = NULL WHERE %s = TRUE`,
		flagColumn, sinceColumn, flagColumn,
	)
	res, err := db.NewRaw(clearQuery).Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "clear status flag",
			Err: base.TranslateNotFound(err),
		}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "clear status flag rows affected",
			Err: base.TranslateNotFound(err),
		}
	}
	return affected, nil
}

// CountEffectiveDashboardAbsences counts the dashboard's effective absence
// buckets for one date. It bridges the legacy live flags on users.students and
// the newer date-scoped rows in active.student_status_days without double
// counting overlaps. Sick wins over excused/class_trip, matching
// api/students/status_days_response.go.
func (r *StudentStatusDayRepository) CountEffectiveDashboardAbsences(ctx context.Context, date timezone.Date) (*active.StudentStatusCounts, error) {
	counts := &active.StudentStatusCounts{}
	db := base.GetDB(ctx, r.db)

	perStudent := db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id`).
		ColumnExpr(`COALESCE("student".sick, FALSE) AS flag_sick`).
		ColumnExpr(`COALESCE("student".excused, FALSE) AS flag_excused`).
		ColumnExpr(`COALESCE(BOOL_OR("student_status_day".status = ?), FALSE) AS day_sick`, active.StudentStatusDaySick).
		ColumnExpr(`COALESCE(BOOL_OR("student_status_day".status = ?), FALSE) AS day_excused`, active.StudentStatusDayExcused).
		ColumnExpr(`COALESCE(BOOL_OR("student_status_day".status = ?), FALSE) AS day_class_trip`, active.StudentStatusDayClassTrip).
		Join(`
			LEFT JOIN active.student_status_days AS "student_status_day"
				ON "student_status_day".tenant_id = "student".tenant_id
				AND "student_status_day".student_id = "student".id
				AND "student_status_day".date = ?
				AND "student_status_day".cleared_at IS NULL
		`, date).
		Where(`"student".status = ?`, string(usersModels.StudentStatusActive)).
		GroupExpr(`"student".id, "student".sick, "student".excused`)
	perStudent = base.WithTenantFilter(ctx, perStudent, "student")

	query := db.NewSelect().
		TableExpr(`(?) AS "effective_student_status"`, perStudent).
		ColumnExpr(`
			COUNT(*) FILTER (
				WHERE "effective_student_status".flag_sick
					OR "effective_student_status".day_sick
			) AS sick_count
		`).
		ColumnExpr(`
			COUNT(*) FILTER (
				WHERE NOT ("effective_student_status".flag_sick OR "effective_student_status".day_sick)
					AND (
						"effective_student_status".flag_excused
						OR "effective_student_status".day_excused
						OR "effective_student_status".day_class_trip
					)
			) AS excused_count
		`).
		ColumnExpr(`COUNT(*) AS total_count`)
	if err := query.Scan(ctx, counts); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count effective dashboard absences",
			Err: base.TranslateNotFound(err),
		}
	}
	return counts, nil
}
