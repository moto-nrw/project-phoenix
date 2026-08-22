// backend/database/repositories/education/class_arrival_time.go
package education

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const classArrivalTimeTableExpr = `education.class_arrival_times AS "class_arrival_time"`

// ClassArrivalTimeRepository implements education.ClassArrivalTimeRepository.
type ClassArrivalTimeRepository struct {
	*base.Repository[*education.ClassArrivalTime]
	db *bun.DB
}

// NewClassArrivalTimeRepository creates a new ClassArrivalTimeRepository.
func NewClassArrivalTimeRepository(db *bun.DB) education.ClassArrivalTimeRepository {
	repo := base.NewRepository[*education.ClassArrivalTime](db, "education.class_arrival_times", "ClassArrivalTime")
	repo.TenantScoped = true
	return &ClassArrivalTimeRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByClasses loads the rows for the given classes in one query, matched on
// the normalized class like every other school_class join in the codebase.
func (r *ClassArrivalTimeRepository) FindByClasses(ctx context.Context, classes []string) ([]*education.ClassArrivalTime, error) {
	rows := make([]*education.ClassArrivalTime, 0)
	normalized := normalizedClassKeys(classes)
	if len(normalized) == 0 {
		return rows, nil
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(classArrivalTimeTableExpr).
		Where(`LOWER(BTRIM("class_arrival_time".school_class)) IN (?)`, bun.List(normalized)).
		OrderExpr(`LOWER(BTRIM("class_arrival_time".school_class)) ASC`)
	query = base.WithTenantFilter(ctx, query, "class_arrival_time")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find class arrival times by classes", Err: err}
	}
	return rows, nil
}

// Upsert replaces the weekday map of one class. The unique index on the
// normalized class is the race-safe backstop.
func (r *ClassArrivalTimeRepository) Upsert(ctx context.Context, row *education.ClassArrivalTime) error {
	base.EnsureTenantID(ctx, row)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr(`education.class_arrival_times`).
		On("CONFLICT (tenant_id, LOWER(BTRIM(school_class))) DO UPDATE").
		Set("arrival_times = EXCLUDED.arrival_times").
		Set("school_class = EXCLUDED.school_class").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert class arrival time", Err: err}
	}
	return nil
}

// LockClass serializes a class's read-modify-write updates. Unlike a row lock,
// this also protects concurrent first inserts, when no class row exists yet.
func (r *ClassArrivalTimeRepository) LockClass(ctx context.Context, class string) error {
	key := fmt.Sprintf("class-arrival:%d:%s", tenant.FromContext(ctx), schoolclass.Normalize(class))
	if _, err := base.GetDB(ctx, r.db).NewRaw("SELECT pg_advisory_xact_lock(hashtext(?))", key).Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "lock class arrival times", Err: err}
	}
	return nil
}

// normalizedClassKeys deduplicates the normalized form of the given classes.
func normalizedClassKeys(classes []string) []string {
	seen := make(map[string]bool, len(classes))
	out := make([]string, 0, len(classes))
	for _, class := range classes {
		key := schoolclass.Normalize(class)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}
