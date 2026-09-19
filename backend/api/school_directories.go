package api

import (
	"context"
	"errors"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	tagScanOperatorAPI "github.com/moto-nrw/project-phoenix/modules/devicefleet/inbound/operator"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// Organisation & Tenancy owns platform.schools (#3253). The HTTP resources
// and the worker read schools through their own ports; the adapters below
// bind those ports to the public capability.

// schedulerTenantDirectory lists the tenant IDs the worker iterates.
type schedulerTenantDirectory struct {
	schools organizationModule.Query
}

func (d schedulerTenantDirectory) ListActiveTenantIDs(ctx context.Context) ([]int64, error) {
	schools, err := d.schools.ListActiveSchools(ctx)
	return schoolIDs(schools), err
}

func (d schedulerTenantDirectory) ListNonDeletedTenantIDs(ctx context.Context) ([]int64, error) {
	schools, err := d.schools.ListNonDeletedSchools(ctx)
	return schoolIDs(schools), err
}

func schoolIDs(schools []organizationModule.School) []int64 {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		ids = append(ids, school.ID)
	}
	return ids
}

// deviceSchoolDirectory answers the device school guard.
type deviceSchoolDirectory struct {
	schools organizationModule.Query
}

func (d deviceSchoolDirectory) FindSchool(ctx context.Context, id int64) (*deviceauth.School, error) {
	school, err := findSchool(ctx, d.schools.FindSchool, id)
	if err != nil || school == nil {
		return nil, err
	}
	return &deviceauth.School{Deleted: school.IsDeleted()}, nil
}

// schoolName reads the display name of one school.
func schoolName(schools organizationModule.Query) func(context.Context, int64) (string, error) {
	return func(ctx context.Context, id int64) (string, error) {
		school, err := schools.FindSchool(ctx, id)
		if err != nil {
			return "", err
		}
		return school.Name, nil
	}
}

// accountSchoolMemberships lists the schools an account holds an active
// membership in.
type accountSchoolMemberships func(ctx context.Context, accountID int64) ([]int64, error)

// authSchoolDirectory answers the auth routes.
type authSchoolDirectory struct {
	schools     organizationModule.Query
	memberships accountSchoolMemberships
}

func (d authSchoolDirectory) GetSchoolByID(ctx context.Context, id int64) (*authAPI.TenantSchool, error) {
	school, err := findSchool(ctx, d.schools.FindSchool, id)
	if err != nil || school == nil {
		return nil, err
	}
	view := tenantSchool(*school, "")
	return &view, nil
}

func (d authSchoolDirectory) GetSchoolBySubdomain(ctx context.Context, subdomain string) (*authAPI.TenantSchool, error) {
	school, err := findSchool(ctx, d.schools.FindSchoolBySubdomain, subdomain)
	if err != nil || school == nil {
		return nil, err
	}
	views, err := d.withOrganizations(ctx, []organizationModule.School{*school})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (d authSchoolDirectory) ListPublicSchools(ctx context.Context) ([]authAPI.TenantSchool, error) {
	schools, err := d.schools.ListPublicSchools(ctx)
	if err != nil {
		return nil, err
	}
	return d.withOrganizations(ctx, schools)
}

func (d authSchoolDirectory) ListActiveSchoolsByAccountID(ctx context.Context, accountID int64) ([]authAPI.TenantSchool, error) {
	if d.memberships == nil {
		return nil, errors.New("auth school directory: membership query is required")
	}
	ids, err := d.memberships(ctx, accountID)
	if err != nil || len(ids) == 0 {
		return []authAPI.TenantSchool{}, err
	}
	schools, err := d.schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	active := make([]organizationModule.School, 0, len(schools))
	for _, school := range schools {
		if school.Active && !school.IsDeleted() {
			active = append(active, school)
		}
	}
	return d.withOrganizations(ctx, active)
}

func (d authSchoolDirectory) withOrganizations(ctx context.Context, schools []organizationModule.School) ([]authAPI.TenantSchool, error) {
	ids := make([]int64, 0, len(schools))
	seen := make(map[int64]bool, len(schools))
	for _, school := range schools {
		if !seen[school.OrganizationID] {
			seen[school.OrganizationID] = true
			ids = append(ids, school.OrganizationID)
		}
	}
	names := make(map[int64]string, len(ids))
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
		views = append(views, tenantSchool(school, names[school.OrganizationID]))
	}
	return views, nil
}

func tenantSchool(school organizationModule.School, organizationName string) authAPI.TenantSchool {
	return authAPI.TenantSchool{
		ID: school.ID, OrganizationID: school.OrganizationID, OrganizationName: organizationName,
		Name: school.Name, Slug: school.Slug, Subdomain: school.Subdomain, Settings: school.Settings,
		Active: school.Active, Hidden: school.Hidden, Deleted: school.IsDeleted(),
	}
}

// enrollmentSchoolDirectory answers the public enrollment routes.
type enrollmentSchoolDirectory struct {
	schools organizationModule.Query
}

func (d enrollmentSchoolDirectory) GetSchoolBySlug(ctx context.Context, slug string) (*enrollmentAPI.PublicSchool, error) {
	school, err := findSchool(ctx, d.schools.FindSchoolBySlug, slug)
	if err != nil || school == nil {
		return nil, err
	}
	return &enrollmentAPI.PublicSchool{ID: school.ID, Deleted: school.IsDeleted()}, nil
}

// parentSchoolDirectory answers the parent enrollment routes.
type parentSchoolDirectory struct {
	schools organizationModule.Query
}

func (d parentSchoolDirectory) GetSchoolBySubdomain(ctx context.Context, subdomain string) (*parentAPI.EnrollmentSchool, error) {
	school, err := findSchool(ctx, d.schools.FindSchoolBySubdomain, subdomain)
	if err != nil || school == nil {
		return nil, err
	}
	return &parentAPI.EnrollmentSchool{ID: school.ID, Active: school.Active, Hidden: school.Hidden, Deleted: school.IsDeleted()}, nil
}

// studentSchoolDirectory answers the student export routes.
type studentSchoolDirectory struct {
	schools organizationModule.Query
}

func (d studentSchoolDirectory) GetSchoolByID(ctx context.Context, id int64) (*studentsAPI.ExportSchool, error) {
	school, err := findSchool(ctx, d.schools.FindSchool, id)
	if err != nil || school == nil {
		return nil, err
	}
	return &studentsAPI.ExportSchool{Name: school.Name}, nil
}

// findSchool runs one school lookup and reports a missing school as nil.
func findSchool[K any](ctx context.Context, find func(context.Context, K) (organizationModule.School, error), key K) (*organizationModule.School, error) {
	school, err := find(ctx, key)
	if errors.Is(err, organizationModule.ErrSchoolNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &school, nil
}

// tagScanSchoolDirectory labels and narrows the operator review of
// unregistered RFID scans (#3232).
type tagScanSchoolDirectory struct {
	schools organizationModule.Query
}

func (d tagScanSchoolDirectory) ListSchoolsByID(ctx context.Context, ids []int64) ([]tagScanOperatorAPI.School, error) {
	schools, err := d.schools.ListSchoolsByID(ctx, ids)
	return tagScanSchools(schools), err
}

func (d tagScanSchoolDirectory) ListSchoolsByOrganization(ctx context.Context, id int64) ([]tagScanOperatorAPI.School, error) {
	schools, err := d.schools.ListSchoolsByOrganization(ctx, id)
	return tagScanSchools(schools), err
}

func (d tagScanSchoolDirectory) ListOrganizationsByID(ctx context.Context, ids []int64) ([]tagScanOperatorAPI.Organization, error) {
	organizations, err := d.schools.ListOrganizationsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]tagScanOperatorAPI.Organization, 0, len(organizations))
	for _, organization := range organizations {
		result = append(result, tagScanOperatorAPI.Organization{ID: organization.ID, Name: organization.Name})
	}
	return result, nil
}

func tagScanSchools(schools []organizationModule.School) []tagScanOperatorAPI.School {
	if schools == nil {
		return nil
	}
	result := make([]tagScanOperatorAPI.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, tagScanOperatorAPI.School{ID: school.ID, OrganizationID: school.OrganizationID, Name: school.Name})
	}
	return result
}
