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

const (
	tablePlatformOrganizations      = "platform.organizations"
	tablePlatformOrganizationsAlias = `platform.organizations AS "organization"`
)

// OrganizationRepository implements platform.OrganizationRepository.
type OrganizationRepository struct {
	*base.Repository[*platform.Organization]
	db *bun.DB
}

// NewOrganizationRepository creates a new OrganizationRepository.
func NewOrganizationRepository(db *bun.DB) platform.OrganizationRepository {
	return &OrganizationRepository{
		Repository: base.NewRepository[*platform.Organization](db, tablePlatformOrganizations, "Organization"),
		db:         db,
	}
}

func (r *OrganizationRepository) Create(ctx context.Context, organization *platform.Organization) error {
	if organization == nil {
		return fmt.Errorf("organization cannot be nil")
	}
	if err := organization.Validate(); err != nil {
		return err
	}
	return r.Repository.Create(ctx, organization)
}

func (r *OrganizationRepository) FindByID(ctx context.Context, id int64) (*platform.Organization, error) {
	organization := new(platform.Organization)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(organization).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Where(`"organization".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find organization by id", Err: err}
	}
	return organization, nil
}

func (r *OrganizationRepository) FindBySlug(ctx context.Context, slug string) (*platform.Organization, error) {
	organization := new(platform.Organization)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(organization).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		Where(`"organization".slug = ?`, slug).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find organization by slug", Err: err}
	}
	return organization, nil
}

func (r *OrganizationRepository) List(ctx context.Context) ([]*platform.Organization, error) {
	var organizations []*platform.Organization
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&organizations).
		ModelTableExpr(tablePlatformOrganizationsAlias).
		OrderExpr(`"organization".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list organizations", Err: err}
	}
	return organizations, nil
}
