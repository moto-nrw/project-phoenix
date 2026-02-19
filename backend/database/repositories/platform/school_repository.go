package platform

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
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

// FindByID returns a school by its ID.
func (r *SchoolRepository) FindByID(ctx context.Context, id int64) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return school, nil
}

// FindBySlug returns a school by its slug.
func (r *SchoolRepository) FindBySlug(ctx context.Context, slug string) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".slug = ?`, slug).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return school, nil
}

// FindBySubdomain returns a school by its subdomain, preloading the Organization relation.
func (r *SchoolRepository) FindBySubdomain(ctx context.Context, subdomain string) (*platform.School, error) {
	school := new(platform.School)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(school).
		ModelTableExpr(schoolTableAlias).
		Relation("Organization").
		Where(`"school".subdomain = ?`, subdomain).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return school, nil
}

// ListActive returns all active schools.
func (r *SchoolRepository) ListActive(ctx context.Context) ([]platform.School, error) {
	var schools []platform.School
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&schools).
		ModelTableExpr(schoolTableAlias).
		Where(`"school".active = true`).
		OrderExpr(`"school".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}

// FindActiveByAccountID returns all active schools the given account has access to.
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
		OrderExpr(`"school".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return schools, nil
}
