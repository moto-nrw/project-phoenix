package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
		return &modelBase.DatabaseError{Op: "create school", Err: err}
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
		return &modelBase.DatabaseError{Op: "update school", Err: err}
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
			return nil, &modelBase.DatabaseError{Op: "find school by id", Err: err}
		}
		return nil, err
	}
	return school, nil
}

// FindBySlug returns a school by its slug. Excludes soft-deleted schools.
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
// Intentionally includes soft-deleted schools to prevent slug reuse while a school is in the trash.
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
		return nil, &modelBase.DatabaseError{Op: "find school by organization and slug", Err: err}
	}
	return school, nil
}

// FindBySubdomain returns a school by its subdomain, preloading the Organization relation.
// Excludes soft-deleted schools.
func (r *SchoolRepository) FindBySubdomain(ctx context.Context, subdomain string) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Relation("Organization").
		Where(`"school".subdomain = ?`, subdomain).
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

// List returns all schools including soft-deleted ones.
// The operator dashboard needs both active and deleted schools (for the trash view).
// Use ListActive for the public-facing tenant selector.
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

// ListActive returns all active schools.
func (r *SchoolRepository) ListActive(ctx context.Context) ([]platform.School, error) {
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

// FindActiveByAccountID returns all active schools the given account has access to.
// Hidden schools are intentionally included — hidden only affects the public landing page.
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

// SoftDelete sets deleted_at on a school. The active field is preserved so that
// Restore can return the school to its pre-deletion state.
func (r *SchoolRepository) SoftDelete(ctx context.Context, id int64) error {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		ModelTableExpr(schoolTableAlias).
		Set(`deleted_at = NOW()`).
		Where(`"school".id = ?`, id).
		Where(`"school".deleted_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "soft delete school", Err: err}
	}
	return base.AssertRowsAffected(result, 1, "soft delete school")
}

// Restore clears deleted_at, returning the school to its pre-deletion state.
// The active field is untouched — it retains whatever value it had before soft-delete.
func (r *SchoolRepository) Restore(ctx context.Context, id int64) error {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		ModelTableExpr(schoolTableAlias).
		Set(`deleted_at = NULL`).
		Where(`"school".id = ?`, id).
		Where(`"school".deleted_at IS NOT NULL`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "restore school", Err: err}
	}
	return base.AssertRowsAffected(result, 1, "restore school")
}

// FindDeletedOlderThan returns soft-deleted schools older than the given duration.
func (r *SchoolRepository) FindDeletedOlderThan(ctx context.Context, olderThan time.Duration) ([]*platform.School, error) {
	cutoff := time.Now().Add(-olderThan)
	var schools []*platform.School
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schools).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".deleted_at IS NOT NULL`).
		Where(`"school".deleted_at < ?`, cutoff).
		OrderExpr(`"school".deleted_at ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}

// PurgeTenant calls the platform.purge_tenant() stored procedure to permanently
// delete all data for a soft-deleted tenant. Returns a JSONB summary.
func (r *SchoolRepository) PurgeTenant(ctx context.Context, tenantID int64) ([]byte, error) {
	var result []byte
	err := base.GetDB(ctx, r.db).NewRaw(
		"SELECT platform.purge_tenant(?)", tenantID,
	).Scan(ctx, &result)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "purge tenant", Err: err}
	}
	return result, nil
}
