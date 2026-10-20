package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

func normalizeNameAndSlug(name, slug string) (string, string) {
	return strings.TrimSpace(name), strings.TrimSpace(strings.ToLower(slug))
}

func validateNameAndSlug(name, slug string) error {
	if name == "" || len(name) > organizationtenancy.MaxNameLength {
		if name == "" {
			return invalid("name is required")
		}
		return invalid("name must not exceed 200 characters")
	}
	return organizationtenancy.ValidateSlug(slug)
}

func invalid(reason string) error {
	return &organizationtenancy.InvalidOrganizationError{Reason: reason}
}

func validateSchoolIDs(ids []int64) error {
	for _, id := range ids {
		if id <= 0 {
			return invalidSchool("school IDs must be positive")
		}
	}
	return nil
}

func invalidSchool(reason string) error {
	return &organizationtenancy.InvalidSchoolError{Reason: reason}
}

// UpdateSchoolSettings rewrites the school's settings JSON under a row lock:
// update receives the stored settings and returns the new ones. Every other
// school field keeps its stored value.
func (s *Service) UpdateSchoolSettings(ctx context.Context, id int64, update func(string) (string, error)) error {
	school, err := s.FindSchoolForMutation(ctx, id)
	if err != nil {
		return err
	}
	settings, err := update(school.Settings)
	if err != nil {
		return err
	}
	_, err = s.UpdateSchool(ctx, organizationtenancy.UpdateSchool{
		ID: school.ID, OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug,
		Subdomain: school.Subdomain, Active: school.Active, Hidden: school.Hidden, Settings: settings,
		Address: school.Address, City: school.City, Zip: school.Zip, Phone: school.Phone,
		Email: school.Email,
	})
	return err
}

func (s *Service) FindSchool(ctx context.Context, id int64) (organizationtenancy.School, error) {
	return s.findSchoolByID(ctx, id, "")
}

func (s *Service) FindSchoolForShare(ctx context.Context, id int64) (organizationtenancy.School, error) {
	return s.findSchoolByID(ctx, id, "SHARE")
}

func (s *Service) FindSchoolForMutation(ctx context.Context, id int64) (organizationtenancy.School, error) {
	return s.findSchoolByID(ctx, id, "UPDATE")
}

func (s *Service) ListSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return s.listSchools(ctx, nil, nil, "")
}

func (s *Service) ListSchoolsByID(ctx context.Context, ids []int64) ([]organizationtenancy.School, error) {
	if len(ids) == 0 {
		return []organizationtenancy.School{}, nil
	}
	if err := validateSchoolIDs(ids); err != nil {
		return nil, err
	}
	return s.listSchools(ctx, ids, nil, "")
}

func (s *Service) ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error) {
	if organizationID <= 0 {
		return nil, invalidSchool("organization ID is required")
	}
	return s.listSchools(ctx, nil, &organizationID, "")
}

func (s *Service) ListNonDeletedSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return s.listSchools(ctx, nil, nil, "non_deleted")
}

func (s *Service) ListActiveSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return s.listSchools(ctx, nil, nil, "active")
}

func (s *Service) ListPublicSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return s.listSchools(ctx, nil, nil, "public")
}
