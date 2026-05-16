package active

import (
	"context"
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
	entry.Date = timezone.DateOfUTC(entry.Date)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(entry).
		ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO UPDATE").
		Set("reported_at = EXCLUDED.reported_at").
		Set("cleared_at = NULL").
		Set("source = EXCLUDED.source").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert student status day", Err: err}
	}
	return nil
}

func (r *StudentStatusDayRepository) MarkCleared(ctx context.Context, studentID int64, status string, date time.Time, clearedAt time.Time, source string) error {
	dateOnly := timezone.DateOfUTC(date)
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.StudentStatusDay)(nil)).
		ModelTableExpr("active.student_status_days").
		Set("cleared_at = ?", clearedAt).
		Set("source = ?", source).
		Where("student_id = ?", studentID).
		Where("date = ?", dateOnly).
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

func (r *StudentStatusDayRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*active.StudentStatusDay, error) {
	var entries []*active.StudentStatusDay
	startOnly := timezone.DateOfUTC(startDate)
	endOnly := timezone.DateOfUTC(endDate)

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		Where(`"student_status_day".student_id = ?`, studentID).
		Where(`"student_status_day".date >= ?`, startOnly).
		Where(`"student_status_day".date <= ?`, endOnly).
		OrderExpr(`"student_status_day".date DESC`).
		OrderExpr(`"student_status_day".reported_at DESC`)

	if where, val, ok := base.TenantWhere(ctx, "student_status_day"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find student status days by date range", Err: err}
	}
	return entries, nil
}
