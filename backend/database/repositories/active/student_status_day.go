package active

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tableExprActiveStudentStatusDaysAsStatusDay = `active.student_status_days AS "student_status_day"`

type StudentStatusDayRepository struct {
	*base.Repository[*active.StudentStatusDay]
	db *bun.DB
}

func NewStudentStatusDayRepository(db *bun.DB) active.StudentStatusDayRepository {
	repo := base.NewRepository[*active.StudentStatusDay](db, "active.student_status_days", "StudentStatusDay")
	repo.TenantScoped = true
	return &StudentStatusDayRepository{Repository: repo, db: db}
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

	_, err := base.GetDB(ctx, r.db).NewInsert().
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
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert student status day", Err: err}
	}
	return nil
}

func (r *StudentStatusDayRepository) MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error {
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
		return &modelBase.DatabaseError{Op: "mark student status day cleared", Err: err}
	}
	return nil
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
		return &modelBase.DatabaseError{Op: "mark student status day cleared by id", Err: err}
	}
	return nil
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
		return &modelBase.DatabaseError{Op: "mark student status days cleared for dates", Err: err}
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

	if where, val, ok := base.TenantWhere(ctx, "student_status_day"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, &modelBase.DatabaseError{Op: "find active student status day by id", Err: err}
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

	if where, val, ok := base.TenantWhere(ctx, "student_status_day"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active student status days by student ids and date", Err: err}
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

	if where, val, ok := base.TenantWhere(ctx, "student_status_day"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find student status days by date range", Err: err}
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
			Err: err,
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
			Err: err,
		}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "clear status flag rows affected",
			Err: err,
		}
	}
	return affected, nil
}

// CountActiveClassTripStudents counts distinct students with an uncleared
// class-trip status entry for the date, excluding students currently flagged
// sick or excused. Custom method (backend-conventions Rule 2): join on
// users.students with COALESCE flag predicates for the dashboard analytics.
func (r *StudentStatusDayRepository) CountActiveClassTripStudents(ctx context.Context, date timezone.Date) (int, error) {
	var count int
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.student_status_days AS "student_status_day"`).
		Join(`JOIN users.students AS "student" ON "student".id = "student_status_day".student_id AND "student".tenant_id = "student_status_day".tenant_id`).
		Where(`"student_status_day".date = ?`, date).
		Where(`"student_status_day".status = ?`, active.StudentStatusDayClassTrip).
		Where(`"student_status_day".cleared_at IS NULL`).
		Where(`COALESCE("student".sick, FALSE) = FALSE`).
		Where(`COALESCE("student".excused, FALSE) = FALSE`).
		ColumnExpr(`COUNT(DISTINCT "student_status_day".student_id)`)
	if where, val, ok := base.TenantWhere(ctx, "student_status_day"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx, &count); err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count active class trip students",
			Err: err,
		}
	}
	return count, nil
}
