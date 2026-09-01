package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const schoolTableAlias = `platform.schools AS "school"`

// SchoolRepository provides read access to school (tenant) records.
type SchoolRepository struct {
	db *bun.DB
}

// NewSchoolRepository creates a new school repository.
func NewSchoolRepository(db *bun.DB) platform.SchoolRepository {
	return &SchoolRepository{db: db}
}

// Create inserts a new school record.
func (r *SchoolRepository) Create(ctx context.Context, school *platform.School) error {
	if school == nil {
		return fmt.Errorf("school cannot be nil")
	}
	if err := school.Validate(); err != nil {
		return err
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(school).
		ModelTableExpr("platform.schools").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "create school", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// Update updates an existing school record.
func (r *SchoolRepository) Update(ctx context.Context, school *platform.School) error {
	if school == nil {
		return fmt.Errorf("school cannot be nil")
	}
	if err := school.Validate(); err != nil {
		return err
	}
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Column("organization_id", "name", "slug", "subdomain", "address", "city", "zip", "phone", "email", "active", "hidden", "settings").
		Where(`"school".id = ?`, school.ID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update school", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(result, 1, "update school")
}

// FindByID returns a school by its ID.
func (r *SchoolRepository) FindByID(ctx context.Context, id int64) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &modelBase.DatabaseError{Op: "find school by id", Err: base.TranslateNotFound(err)}
		}
		return nil, err
	}
	return school, nil
}

// findByIDWithLock returns a school by ID while acquiring the given row-level
// lock. `lockClause` is passed verbatim to bun's `For(...)` builder (e.g.
// "SHARE", "UPDATE"). `op` labels the operation in DatabaseError on wrapping
// failure. Must be called within a transaction.
func (r *SchoolRepository) findByIDWithLock(ctx context.Context, id int64, lockClause, op string) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".id = ?`, id).
		For(lockClause).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &modelBase.DatabaseError{Op: op, Err: base.TranslateNotFound(err)}
		}
		return nil, err
	}
	return school, nil
}

// FindByIDForShare returns a school by ID while acquiring a FOR SHARE lock on
// the row. Prevents concurrent UPDATE (e.g., SoftDelete) from committing until
// the calling transaction completes. Must be called within a transaction.
func (r *SchoolRepository) FindByIDForShare(ctx context.Context, id int64) (*platform.School, error) {
	return r.findByIDWithLock(ctx, id, "SHARE", "find school by id for share")
}

// FindByIDForUpdate returns a school by ID while acquiring a FOR UPDATE lock
// on the row. Serializes concurrent read-modify-write cycles (e.g., JSONB
// settings updates) by blocking other transactions from reading or writing
// the same row until the calling transaction completes. Must be called within
// a transaction.
func (r *SchoolRepository) FindByIDForUpdate(ctx context.Context, id int64) (*platform.School, error) {
	return r.findByIDWithLock(ctx, id, "UPDATE", "find school by id for update")
}

// FindBySlug returns a non-deleted school by its slug.
// Soft-deleted schools are filtered out (returns nil, nil). Callers that need to
// distinguish "doesn't exist" from "exists but deleted" should use FindBySubdomain,
// which intentionally includes soft-deleted schools.
func (r *SchoolRepository) FindBySlug(ctx context.Context, slug string) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".slug = ?`, slug).
		Where(`"school".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return school, nil
}

// FindByOrganizationAndSlug returns a school by its organization-scoped slug.
func (r *SchoolRepository) FindByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".organization_id = ?`, organizationID).
		Where(`"school".slug = ?`, slug).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find school by organization and slug", Err: base.TranslateNotFound(err)}
	}
	return school, nil
}

// FindBySubdomain returns a school by its subdomain, preloading the Organization relation.
// Intentionally includes soft-deleted schools so callers (login, tenant resolution) can
// distinguish "deleted" from "not found" and return appropriate error messages.
func (r *SchoolRepository) FindBySubdomain(ctx context.Context, subdomain string) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Relation("Organization").
		Where(`"school".subdomain = ?`, subdomain).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return school, nil
}

// List returns all schools.
func (r *SchoolRepository) List(ctx context.Context) ([]*platform.School, error) {
	var schools []*platform.School
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schools).
		ModelTableExpr(schoolTableAlias).
		Relation("Organization").
		OrderExpr(`"school".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}

// ListNonDeleted returns all tenants that still retain data. Unlike
// ListActive, it includes inactive schools so maintenance workers can clear
// sensitive files after user access has been disabled.
func (r *SchoolRepository) ListNonDeleted(ctx context.Context) ([]platform.School, error) {
	var schools []platform.School
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schools).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".deleted_at IS NULL`).
		OrderExpr(`"school".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}

// ListActive returns all active, non-deleted schools.
// Used by the scheduler to iterate tenants — deleted schools must not receive scheduled jobs.
func (r *SchoolRepository) ListActive(ctx context.Context) ([]platform.School, error) {
	var schools []platform.School
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schools).
		ModelTableExpr(schoolTableAlias).
		Relation("Organization").
		Where(`"school".active = true`).
		Where(`"school".deleted_at IS NULL`).
		OrderExpr(`"school".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}

// ListPublic returns all active, non-hidden schools for the public landing page / tenant selector.
// Hidden schools are excluded here but NOT in ListActive, because the scheduler uses ListActive
// to iterate tenants for cleanup, session-end, and break tasks — hiding a school must not
// stop its scheduled jobs.
func (r *SchoolRepository) ListPublic(ctx context.Context) ([]platform.School, error) {
	var schools []platform.School
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schools).
		ModelTableExpr(schoolTableAlias).
		Relation("Organization").
		Where(`"school".active = true`).
		Where(`"school".hidden = false`).
		Where(`"school".deleted_at IS NULL`).
		OrderExpr(`"school".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}

// FindActiveByAccountID returns all active, non-deleted schools the given account has access to.
func (r *SchoolRepository) FindActiveByAccountID(ctx context.Context, accountID int64) ([]platform.School, error) {
	var schools []platform.School
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schools).
		ModelTableExpr(schoolTableAlias).
		Relation("Organization").
		Join(`INNER JOIN auth.account_tenants AS "at" ON "at".tenant_id = "school".id`).
		Where(`"at".account_id = ?`, accountID).
		Where(`"at".status = ?`, "active").
		Where(`"school".active = true`).
		Where(`"school".deleted_at IS NULL`).
		OrderExpr(`"school".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}

// SoftDelete sets deleted_at on a school. Fails if the school is already deleted.
func (r *SchoolRepository) SoftDelete(ctx context.Context, id int64) error {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		ModelTableExpr(schoolTableAlias).
		Set(`deleted_at = NOW()`).
		Where(`"school".id = ?`, id).
		Where(`"school".deleted_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete school", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(result, 1, "soft delete school")
}

// CountByIDs counts how many of the given IDs exist in the schools table,
// excluding soft-deleted rows. Used to enforce that new announcement targeting
// cannot reference trashed schools. Historical school targets already present
// on an announcement are allowed through by the service layer via a diff
// against the existing record, so re-saving does not reactivate stale targets.
func (r *SchoolRepository) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*platform.School)(nil)).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".id IN (?)`, bun.List(ids)).
		Where(`"school".deleted_at IS NULL`).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count schools by ids", Err: base.TranslateNotFound(err)}
	}
	return count, nil
}

// Restore clears deleted_at on a soft-deleted school. Fails if the school is not deleted.
func (r *SchoolRepository) Restore(ctx context.Context, id int64) error {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		ModelTableExpr(schoolTableAlias).
		Set(`deleted_at = NULL`).
		Where(`"school".id = ?`, id).
		Where(`"school".deleted_at IS NOT NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "restore school", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(result, 1, "restore school")
}
