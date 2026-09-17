package auth_test

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// testSchoolDirectory binds the auth routes to the Organisation & Tenancy
// capability the way the serving root does; the account memberships come
// straight from the seeded auth.account_tenants rows.
// Callers pass the pool from testpkg.SetupTestDB.
type testSchoolDirectory struct {
	schools organizationtenancy.Query
	db      *bun.DB
	runtime tenant.UnitOfWork
}

func (d testSchoolDirectory) GetSchoolByID(ctx context.Context, id int64) (*authAPI.TenantSchool, error) {
	ctx = tenant.WithUnitOfWork(ctx, d.runtime)
	return d.find(ctx, func() (organizationtenancy.School, error) { return d.schools.FindSchool(ctx, id) })
}

func (d testSchoolDirectory) GetSchoolBySubdomain(ctx context.Context, subdomain string) (*authAPI.TenantSchool, error) {
	ctx = tenant.WithUnitOfWork(ctx, d.runtime)
	return d.find(ctx, func() (organizationtenancy.School, error) { return d.schools.FindSchoolBySubdomain(ctx, subdomain) })
}

func (d testSchoolDirectory) ListPublicSchools(ctx context.Context) ([]authAPI.TenantSchool, error) {
	ctx = tenant.WithUnitOfWork(ctx, d.runtime)
	schools, err := d.schools.ListPublicSchools(ctx)
	if err != nil {
		return nil, err
	}
	return d.views(ctx, schools)
}

func (d testSchoolDirectory) ListActiveSchoolsByAccountID(ctx context.Context, accountID int64) ([]authAPI.TenantSchool, error) {
	ctx = tenant.WithUnitOfWork(ctx, d.runtime)
	var ids []int64
	err := d.db.NewSelect().
		TableExpr(`auth.account_tenants AS "at"`).
		Column("at.tenant_id").
		Where(`"at".account_id = ?`, accountID).
		Where(`"at".status = 'active'`).
		Scan(ctx, &ids)
	if err != nil || len(ids) == 0 {
		return []authAPI.TenantSchool{}, err
	}
	schools, err := d.schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	active := make([]organizationtenancy.School, 0, len(schools))
	for _, school := range schools {
		if school.Active && !school.IsDeleted() {
			active = append(active, school)
		}
	}
	return d.views(ctx, active)
}

func (d testSchoolDirectory) find(ctx context.Context, lookup func() (organizationtenancy.School, error)) (*authAPI.TenantSchool, error) {
	school, err := lookup()
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	views, err := d.views(ctx, []organizationtenancy.School{school})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (d testSchoolDirectory) views(ctx context.Context, schools []organizationtenancy.School) ([]authAPI.TenantSchool, error) {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		ids = append(ids, school.OrganizationID)
	}
	names := map[int64]string{}
	if len(ids) > 0 {
		organizations, err := d.schools.ListOrganizationsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, organization := range organizations {
			names[organization.ID] = organization.Name
		}
	}
	views := make([]authAPI.TenantSchool, 0, len(schools))
	for _, school := range schools {
		views = append(views, authAPI.TenantSchool{
			ID: school.ID, OrganizationID: school.OrganizationID, OrganizationName: names[school.OrganizationID],
			Name: school.Name, Slug: school.Slug, Subdomain: school.Subdomain, Settings: school.Settings,
			Active: school.Active, Hidden: school.Hidden, Deleted: school.IsDeleted(),
		})
	}
	return views, nil
}
