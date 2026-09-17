package compose

import (
	"context"
	"strings"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// classArrivalExceptionRepository serves the retained class arrival exception
// contract from the Timetable persistence adapter (#3220).
type classArrivalExceptionRepository struct {
	store *postgres.Store
}

// NewClassArrivalExceptionRepository binds the retained class arrival
// exception contract. Timetable owns education.class_arrival_exceptions.
func NewClassArrivalExceptionRepository(db *bun.DB) schedule.ClassArrivalExceptionRepository {
	return classArrivalExceptionRepository{store: postgres.New(databaseRuntime(db))}
}

// FindByClassesAndDateRange loads the exceptions of the given classes inside
// [from, to] in one query, matched on the normalized class like every other
// school_class join in the codebase.
func (r classArrivalExceptionRepository) FindByClassesAndDateRange(
	ctx context.Context,
	classes []string,
	from, to schedule.Date,
) ([]*schedule.ClassArrivalException, error) {
	normalized := normalizedExceptionClassKeys(classes)
	if len(normalized) == 0 || to.Before(from) {
		return make([]*schedule.ClassArrivalException, 0), nil
	}
	rows, err := r.store.FindClassArrivalExceptions(ctx, normalized, from, to)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find class arrival exceptions by classes and date range", Err: translateNotFound(err)}
	}
	return rows, nil
}

// Upsert replaces the exception of one class and date.
func (r classArrivalExceptionRepository) Upsert(ctx context.Context, row *schedule.ClassArrivalException) error {
	if err := r.store.UpsertClassArrivalException(ctx, row); err != nil {
		return &modelBase.DatabaseError{Op: "upsert class arrival exception", Err: translateNotFound(err)}
	}
	return nil
}

// DeleteByClassAndDate removes the exception of one class and date.
func (r classArrivalExceptionRepository) DeleteByClassAndDate(
	ctx context.Context,
	schoolClass string,
	date schedule.Date,
) (bool, error) {
	key := normalizeExceptionClass(schoolClass)
	if key == "" {
		return false, nil
	}
	affected, err := r.store.DeleteClassArrivalException(ctx, key, date)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "delete class arrival exception", Err: translateNotFound(err)}
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
