package repositories

import (
	"context"
	"errors"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type SchoolMembershipQuery interface {
	FindActiveByAccountID(context.Context, int64) ([]authModels.AccountTenant, error)
}

type schoolCapabilityAdapter struct {
	schools     organizationtenancy.Capability
	memberships SchoolMembershipQuery
}

func NewSchoolCapabilityAdapter(schools organizationtenancy.Capability, memberships SchoolMembershipQuery) platformModels.SchoolRepository {
	if schools == nil {
		panic("school capability adapter: schools are required")
	}
	return &schoolCapabilityAdapter{schools: schools, memberships: memberships}
}

func (a *schoolCapabilityAdapter) Create(ctx context.Context, school *platformModels.School) error {
	if school == nil {
		return errors.New("school cannot be nil")
	}
	created, err := a.schools.CreateSchool(ctx, createSchoolInput(school))
	if err != nil {
		return err
	}
	*school = *toLegacySchool(created)
	return nil
}

func (a *schoolCapabilityAdapter) Update(ctx context.Context, school *platformModels.School) error {
	if school == nil {
		return errors.New("school cannot be nil")
	}
	updated, err := a.schools.UpdateSchool(ctx, updateSchoolInput(school))
	if err != nil {
		return err
	}
	*school = *toLegacySchool(updated)
	return nil
}

func (a *schoolCapabilityAdapter) FindByID(ctx context.Context, id int64) (*platformModels.School, error) {
	value, err := a.schools.FindSchool(ctx, id)
	return legacySchoolResult(value, err, "find school by id", false)
}

func (a *schoolCapabilityAdapter) FindByIDForShare(ctx context.Context, id int64) (*platformModels.School, error) {
	value, err := a.schools.FindSchoolForShare(ctx, id)
	return legacySchoolResult(value, err, "find school by id for share", false)
}

func (a *schoolCapabilityAdapter) FindByIDForUpdate(ctx context.Context, id int64) (*platformModels.School, error) {
	value, err := a.schools.FindSchoolForMutation(ctx, id)
	return legacySchoolResult(value, err, "find school by id for update", false)
}

func (a *schoolCapabilityAdapter) FindBySlug(ctx context.Context, slug string) (*platformModels.School, error) {
	value, err := a.schools.FindSchoolBySlug(ctx, slug)
	return legacySchoolResult(value, err, "find school by slug", true)
}

func (a *schoolCapabilityAdapter) FindByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (*platformModels.School, error) {
	value, err := a.schools.FindSchoolByOrganizationAndSlug(ctx, organizationID, slug)
	return legacySchoolResult(value, err, "find school by organization and slug", true)
}

func (a *schoolCapabilityAdapter) FindBySubdomain(ctx context.Context, subdomain string) (*platformModels.School, error) {
	value, err := a.schools.FindSchoolBySubdomain(ctx, subdomain)
	result, err := legacySchoolResult(value, err, "find school by subdomain", true)
	if err != nil || result == nil {
		return result, err
	}
	if err := a.enrichOrganizations(ctx, []*platformModels.School{result}); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *schoolCapabilityAdapter) List(ctx context.Context) ([]*platformModels.School, error) {
	values, err := a.schools.ListSchools(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*platformModels.School, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacySchool(value))
	}
	if err := a.enrichOrganizations(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *schoolCapabilityAdapter) ListNonDeleted(ctx context.Context) ([]platformModels.School, error) {
	values, err := a.schools.ListNonDeletedSchools(ctx)
	return legacySchoolList(values, err)
}

func (a *schoolCapabilityAdapter) ListActive(ctx context.Context) ([]platformModels.School, error) {
	values, err := a.schools.ListActiveSchools(ctx)
	return a.legacySchoolListWithOrganizations(ctx, values, err)
}

func (a *schoolCapabilityAdapter) ListPublic(ctx context.Context) ([]platformModels.School, error) {
	values, err := a.schools.ListPublicSchools(ctx)
	return a.legacySchoolListWithOrganizations(ctx, values, err)
}

func (a *schoolCapabilityAdapter) FindActiveByAccountID(ctx context.Context, accountID int64) ([]platformModels.School, error) {
	if a.memberships == nil {
		return nil, errors.New("school capability adapter: membership query is required")
	}
	memberships, err := a.memberships.FindActiveByAccountID(ctx, accountID)
	if err != nil || len(memberships) == 0 {
		return []platformModels.School{}, err
	}
	ids := make([]int64, 0, len(memberships))
	for _, membership := range memberships {
		ids = append(ids, membership.TenantID)
	}
	values, err := a.schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	active := make([]organizationtenancy.School, 0, len(values))
	for _, value := range values {
		if value.Active && !value.IsDeleted() {
			active = append(active, value)
		}
	}
	return a.legacySchoolListWithOrganizations(ctx, active, nil)
}

func (a *schoolCapabilityAdapter) legacySchoolListWithOrganizations(ctx context.Context, values []organizationtenancy.School, err error) ([]platformModels.School, error) {
	result, err := legacySchoolList(values, err)
	if err != nil {
		return nil, err
	}
	pointers := make([]*platformModels.School, len(result))
	for index := range result {
		pointers[index] = &result[index]
	}
	if err := a.enrichOrganizations(ctx, pointers); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *schoolCapabilityAdapter) enrichOrganizations(ctx context.Context, schools []*platformModels.School) error {
	ids := make([]int64, 0, len(schools))
	seen := make(map[int64]struct{}, len(schools))
	for _, school := range schools {
		if school == nil {
			continue
		}
		if _, ok := seen[school.OrganizationID]; ok {
			continue
		}
		seen[school.OrganizationID] = struct{}{}
		ids = append(ids, school.OrganizationID)
	}
	if len(ids) == 0 {
		return nil
	}
	organizations, err := a.schools.ListOrganizationsByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]*platformModels.Organization, len(organizations))
	for _, organization := range organizations {
		legacy := &platformModels.Organization{
			Name: organization.Name, Slug: organization.Slug, Active: organization.Active,
			DeletedAt: organization.DeletedAt, Settings: organization.Settings,
		}
		legacy.ID = organization.ID
		legacy.CreatedAt = organization.CreatedAt
		legacy.UpdatedAt = organization.UpdatedAt
		byID[organization.ID] = legacy
	}
	for _, school := range schools {
		if school != nil {
			school.Organization = byID[school.OrganizationID]
		}
	}
	return nil
}

func (a *schoolCapabilityAdapter) SoftDelete(ctx context.Context, id int64) error {
	_, err := a.schools.SoftDeleteSchool(ctx, id)
	return err
}

func (a *schoolCapabilityAdapter) Restore(ctx context.Context, id int64) error {
	_, err := a.schools.RestoreSchool(ctx, id)
	return err
}

func (a *schoolCapabilityAdapter) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	return a.schools.CountSchoolsByID(ctx, ids)
}

func legacySchoolResult(value organizationtenancy.School, err error, operation string, nilNotFound bool) (*platformModels.School, error) {
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		if nilNotFound {
			return nil, nil
		}
		return nil, platformRepo.NewSchoolNotFoundError(operation)
	}
	if err != nil {
		return nil, err
	}
	return toLegacySchool(value), nil
}

func legacySchoolList(values []organizationtenancy.School, err error) ([]platformModels.School, error) {
	if err != nil {
		return nil, err
	}
	result := make([]platformModels.School, 0, len(values))
	for _, value := range values {
		result = append(result, *toLegacySchool(value))
	}
	return result, nil
}

func toLegacySchool(value organizationtenancy.School) *platformModels.School {
	var organization *platformModels.Organization
	if value.Organization != nil {
		organization = &platformModels.Organization{
			Name: value.Organization.Name, Slug: value.Organization.Slug, Active: value.Organization.Active,
			DeletedAt: value.Organization.DeletedAt, Settings: value.Organization.Settings,
		}
		organization.ID = value.Organization.ID
		organization.CreatedAt = value.Organization.CreatedAt
		organization.UpdatedAt = value.Organization.UpdatedAt
	}
	result := &platformModels.School{
		OrganizationID: value.OrganizationID, Name: value.Name, Slug: value.Slug, Subdomain: value.Subdomain,
		Active: value.Active, Hidden: value.Hidden, DeletedAt: value.DeletedAt, Settings: value.Settings,
		Address: value.Address, City: value.City, Zip: value.Zip, Phone: value.Phone, Email: value.Email,
		DevicePinHash: value.DevicePinHash, Organization: organization,
	}
	result.ID = value.ID
	result.CreatedAt = value.CreatedAt
	result.UpdatedAt = value.UpdatedAt
	return result
}

func createSchoolInput(value *platformModels.School) organizationtenancy.CreateSchool {
	return organizationtenancy.CreateSchool{
		OrganizationID: value.OrganizationID, Name: value.Name, Slug: value.Slug, Subdomain: value.Subdomain,
		Active: value.Active, Hidden: value.Hidden, Settings: value.Settings, Address: value.Address,
		City: value.City, Zip: value.Zip, Phone: value.Phone, Email: value.Email, DevicePinHash: value.DevicePinHash,
	}
}

func updateSchoolInput(value *platformModels.School) organizationtenancy.UpdateSchool {
	input := createSchoolInput(value)
	return organizationtenancy.UpdateSchool{
		ID: value.ID, OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug,
		Subdomain: input.Subdomain, Active: input.Active, Hidden: input.Hidden, Settings: input.Settings,
		Address: input.Address, City: input.City, Zip: input.Zip, Phone: input.Phone, Email: input.Email,
		DevicePinHash: input.DevicePinHash,
	}
}
