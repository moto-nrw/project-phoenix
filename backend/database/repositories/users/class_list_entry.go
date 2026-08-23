package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// ClassListEntryRepository implements users.ClassListEntryRepository (#2382).
type ClassListEntryRepository struct {
	*base.Repository[*users.ClassListEntry]
	db *bun.DB
}

// NewClassListEntryRepository creates a new ClassListEntryRepository.
func NewClassListEntryRepository(db *bun.DB) users.ClassListEntryRepository {
	repo := base.NewRepository[*users.ClassListEntry](db, "users.class_list_entries", "ClassListEntry")
	repo.TenantScoped = true
	return &ClassListEntryRepository{
		Repository: repo,
		db:         db,
	}
}

// FindBySchoolClass returns the entries of one class, matched like every
// other class comparison via LOWER(BTRIM(...)), name-sorted.
func (r *ClassListEntryRepository) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*users.ClassListEntry, error) {
	var entries []*users.ClassListEntry
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(`users.class_list_entries AS "class_list_entry"`).
		Where(`LOWER(BTRIM("class_list_entry".school_class)) = LOWER(BTRIM(?))`, schoolClass).
		OrderExpr(`LOWER("class_list_entry".last_name) ASC, LOWER("class_list_entry".first_name) ASC, "class_list_entry".id ASC`)

	query = base.WithTenantFilter(ctx, query, "class_list_entry")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by school class",
			Err: err,
		}
	}

	return entries, nil
}

// FindByNameAndClass returns entries matching first name, last name and class
// case-insensitively — the duplicate guard for create and import (mirror of
// StudentRepository.FindByNameAndClass).
func (r *ClassListEntryRepository) FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*users.ClassListEntry, error) {
	var entries []*users.ClassListEntry
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(`users.class_list_entries AS "class_list_entry"`).
		Where(`LOWER(BTRIM("class_list_entry".first_name)) = LOWER(BTRIM(?))`, firstName).
		Where(`LOWER(BTRIM("class_list_entry".last_name)) = LOWER(BTRIM(?))`, lastName).
		Where(`LOWER(BTRIM("class_list_entry".school_class)) = LOWER(BTRIM(?))`, schoolClass)

	query = base.WithTenantFilter(ctx, query, "class_list_entry")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name and class",
			Err: err,
		}
	}

	return entries, nil
}
