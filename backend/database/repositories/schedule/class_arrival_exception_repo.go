// backend/database/repositories/schedule/class_arrival_exception_repo.go
package schedule

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const classArrivalExceptionTableExpr = `education.class_arrival_exceptions AS "class_arrival_exception"`

// ClassArrivalExceptionRepository implements schedule.ClassArrivalExceptionRepository.
type ClassArrivalExceptionRepository struct {
	*base.Repository[*schedule.ClassArrivalException]
	db *bun.DB
}

// NewClassArrivalExceptionRepository creates a new ClassArrivalExceptionRepository.
func NewClassArrivalExceptionRepository(db *bun.DB) schedule.ClassArrivalExceptionRepository {
	repo := base.NewRepository[*schedule.ClassArrivalException](db, "education.class_arrival_exceptions", "ClassArrivalException")
	repo.TenantScoped = true
	return &ClassArrivalExceptionRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByClassesAndDateRange loads the exceptions of the given classes inside
// [from, to] in one query, matched on the normalized class like every other
// school_class join in the codebase.
func (r *ClassArrivalExceptionRepository) FindByClassesAndDateRange(
	ctx context.Context,
	classes []string,
	from, to timezone.Date,
) ([]*schedule.ClassArrivalException, error) {
	rows := make([]*schedule.ClassArrivalException, 0)
	normalized := normalizedExceptionClassKeys(classes)
	if len(normalized) == 0 || to.Before(from) {
		return rows, nil
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(classArrivalExceptionTableExpr).
		Where(`LOWER(BTRIM("class_arrival_exception".school_class)) IN (?)`, bun.List(normalized)).
		Where(`"class_arrival_exception".date >= ?`, from).
		Where(`"class_arrival_exception".date <= ?`, to).
		OrderExpr(`"class_arrival_exception".date ASC, LOWER(BTRIM("class_arrival_exception".school_class)) ASC`)
	query = base.WithTenantFilter(ctx, query, "class_arrival_exception")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find class arrival exceptions by classes and date range", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// Upsert replaces the exception of one class and date. The unique index on
// the normalized class plus date is the race-safe backstop.
func (r *ClassArrivalExceptionRepository) Upsert(ctx context.Context, row *schedule.ClassArrivalException) error {
	base.EnsureTenantID(ctx, row)
	err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr(`education.class_arrival_exceptions`).
		On("CONFLICT (tenant_id, (LOWER(BTRIM(school_class))), date) DO UPDATE").
		Set("arrival_time = EXCLUDED.arrival_time").
		Set("reason = EXCLUDED.reason").
		Set("school_class = EXCLUDED.school_class").
		Set("created_by = EXCLUDED.created_by").
		Set("origin = EXCLUDED.origin").
		Set("updated_at = NOW()").
		Returning("*").
		Scan(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert class arrival exception", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// DeleteByClassAndDate removes the exception of one class and date.
func (r *ClassArrivalExceptionRepository) DeleteByClassAndDate(
	ctx context.Context,
	schoolClass string,
	date timezone.Date,
) (bool, error) {
	key := normalizeExceptionClass(schoolClass)
	if key == "" {
		return false, nil
	}
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.ClassArrivalException)(nil)).
		ModelTableExpr(classArrivalExceptionTableExpr).
		Where(`LOWER(BTRIM("class_arrival_exception".school_class)) = ?`, key).
		Where(`"class_arrival_exception".date = ?`, date)
	query = base.WithTenantFilter(ctx, query, "class_arrival_exception")

	result, err := query.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "delete class arrival exception", Err: base.TranslateNotFound(err)}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "delete class arrival exception", Err: err}
	}
	return affected > 0, nil
}

// normalizeExceptionClass mirrors the LOWER(BTRIM(school_class)) identity the
// unique index uses. It repeats internal/schoolclass.Normalize on purpose:
// the timetable repositories may not import the school-structure domain.
func normalizeExceptionClass(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}

// normalizedExceptionClassKeys deduplicates the normalized form of the given
// classes.
func normalizedExceptionClassKeys(classes []string) []string {
	seen := make(map[string]bool, len(classes))
	out := make([]string, 0, len(classes))
	for _, class := range classes {
		key := normalizeExceptionClass(class)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}
