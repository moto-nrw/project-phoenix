package organizationtenancy

import (
	"context"
	"errors"
	"strings"
	"time"
)

type InvalidSchoolError struct{ Reason string }

func (e *InvalidSchoolError) Error() string { return e.Reason }
func (e *InvalidSchoolError) Unwrap() error { return ErrInvalidSchool }

var (
	ErrSchoolNotFound       = errors.New("school not found")
	ErrInvalidSchool        = errors.New("invalid school")
	ErrSchoolSlugConflict   = errors.New("school slug already exists in organization")
	ErrSchoolDomainConflict = errors.New("school subdomain already exists")
	ErrSchoolAlreadyDeleted = errors.New("school is already deleted")
	ErrSchoolNotDeleted     = errors.New("school is not deleted")
	ErrOrganizationDeleted  = errors.New("school organization is deleted")
)

type School struct {
	ID             int64         `json:"id"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	OrganizationID int64         `json:"organization_id"`
	Name           string        `json:"name"`
	Slug           string        `json:"slug"`
	Subdomain      string        `json:"subdomain"`
	Active         bool          `json:"active"`
	Hidden         bool          `json:"hidden"`
	DeletedAt      *time.Time    `json:"deleted_at,omitempty"`
	Settings       string        `json:"settings,omitempty"`
	Address        string        `json:"address,omitempty"`
	City           string        `json:"city,omitempty"`
	Zip            string        `json:"zip,omitempty"`
	Phone          string        `json:"phone,omitempty"`
	Email          string        `json:"email,omitempty"`
	DevicePinHash  string        `json:"-"`
	Organization   *Organization `json:"organization,omitempty"`
}

func (s School) IsDeleted() bool { return s.DeletedAt != nil }

type CreateSchool struct {
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Active         bool
	Hidden         bool
	Settings       string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
	DevicePinHash  string
}

type UpdateSchool struct {
	ID             int64
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Active         bool
	Hidden         bool
	Settings       string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
	DevicePinHash  string
}

func (m *Module) CreateSchool(ctx context.Context, input CreateSchool) (School, error) {
	normalizeSchool(&input.Name, &input.Slug, &input.Subdomain)
	if err := validateSchool(input.OrganizationID, input.Name, input.Slug, input.Subdomain, input.Email); err != nil {
		return School{}, err
	}
	return m.engine.CreateSchool(ctx, input)
}

func (m *Module) UpdateSchool(ctx context.Context, input UpdateSchool) (School, error) {
	normalizeSchool(&input.Name, &input.Slug, &input.Subdomain)
	if input.ID <= 0 {
		return School{}, invalidSchool("school ID is required")
	}
	if err := validateSchool(input.OrganizationID, input.Name, input.Slug, input.Subdomain, input.Email); err != nil {
		return School{}, err
	}
	return m.engine.UpdateSchool(ctx, input)
}

func (m *Module) SoftDeleteSchool(ctx context.Context, id int64) (School, error) {
	if id <= 0 {
		return School{}, invalidSchool("school ID is required")
	}
	return m.engine.SoftDeleteSchool(ctx, id)
}

func (m *Module) RestoreSchool(ctx context.Context, id int64) (School, error) {
	if id <= 0 {
		return School{}, invalidSchool("school ID is required")
	}
	return m.engine.RestoreSchool(ctx, id)
}

func (m *Module) FindSchool(ctx context.Context, id int64) (School, error) {
	return m.findSchool(ctx, id, "")
}

func (m *Module) FindSchoolForShare(ctx context.Context, id int64) (School, error) {
	return m.findSchool(ctx, id, "SHARE")
}

func (m *Module) FindSchoolForMutation(ctx context.Context, id int64) (School, error) {
	return m.findSchool(ctx, id, "UPDATE")
}

func (m *Module) findSchool(ctx context.Context, id int64, lock string) (School, error) {
	if id <= 0 {
		return School{}, invalidSchool("school ID is required")
	}
	return m.engine.FindSchoolByID(ctx, id, lock)
}

func (m *Module) FindSchoolBySlug(ctx context.Context, slug string) (School, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if err := ValidateSlug(slug); err != nil {
		return School{}, invalidSchool(err.Error())
	}
	return m.engine.FindSchoolBySlug(ctx, slug)
}

func (m *Module) FindSchoolByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (School, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if organizationID <= 0 {
		return School{}, invalidSchool("organization ID is required")
	}
	if err := ValidateSlug(slug); err != nil {
		return School{}, invalidSchool(err.Error())
	}
	return m.engine.FindSchoolByOrganizationAndSlug(ctx, organizationID, slug)
}

func (m *Module) FindSchoolBySubdomain(ctx context.Context, subdomain string) (School, error) {
	subdomain = strings.TrimSpace(strings.ToLower(subdomain))
	if err := validateSubdomain(subdomain); err != nil {
		return School{}, err
	}
	return m.engine.FindSchoolBySubdomain(ctx, subdomain)
}

func (m *Module) ListSchools(ctx context.Context) ([]School, error) {
	return m.engine.ListSchools(ctx)
}

func (m *Module) ListSchoolsByID(ctx context.Context, ids []int64) ([]School, error) {
	if len(ids) == 0 {
		return []School{}, nil
	}
	if err := validateSchoolIDs(ids); err != nil {
		return nil, err
	}
	return m.engine.ListSchoolsByID(ctx, ids)
}

func (m *Module) ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]School, error) {
	if organizationID <= 0 {
		return nil, invalidSchool("organization ID is required")
	}
	return m.engine.ListSchoolsByOrganization(ctx, organizationID)
}

func (m *Module) ListNonDeletedSchools(ctx context.Context) ([]School, error) {
	return m.engine.ListNonDeletedSchools(ctx)
}

func (m *Module) ListActiveSchools(ctx context.Context) ([]School, error) {
	return m.engine.ListActiveSchools(ctx)
}

func (m *Module) ListPublicSchools(ctx context.Context) ([]School, error) {
	return m.engine.ListPublicSchools(ctx)
}

func (m *Module) CountSchoolsByID(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if err := validateSchoolIDs(ids); err != nil {
		return 0, err
	}
	return m.engine.CountSchoolsByID(ctx, ids)
}

func validateSchool(organizationID int64, name, slug, subdomain, email string) error {
	if organizationID <= 0 {
		return invalidSchool("organization ID is required")
	}
	if name == "" {
		return invalidSchool("name is required")
	}
	if len(name) > MaxNameLength {
		return invalidSchool("name must not exceed 200 characters")
	}
	if err := ValidateSlug(slug); err != nil {
		return invalidSchool(err.Error())
	}
	if err := validateSubdomain(subdomain); err != nil {
		return err
	}
	if len(email) > MaxEmailLength {
		return invalidSchool("email must not exceed 255 characters")
	}
	return nil
}

func validateSubdomain(subdomain string) error {
	if subdomain == "" {
		return invalidSchool("subdomain is required")
	}
	if len(subdomain) > MaxDNSLabelLength {
		return invalidSchool("subdomain must not exceed 63 characters (DNS label limit)")
	}
	if !slugPattern.MatchString(subdomain) {
		return invalidSchool("subdomain must be a valid DNS label: lowercase letters, numbers, and hyphens (no leading/trailing hyphens)")
	}
	if reservedSlugs[subdomain] {
		return invalidSchool("subdomain is reserved for infrastructure use")
	}
	return nil
}

func validateSchoolIDs(ids []int64) error {
	for _, id := range ids {
		if id <= 0 {
			return invalidSchool("school IDs must be positive")
		}
	}
	return nil
}

func normalizeSchool(name, slug, subdomain *string) {
	*name = strings.TrimSpace(*name)
	*slug = strings.TrimSpace(strings.ToLower(*slug))
	*subdomain = strings.TrimSpace(strings.ToLower(*subdomain))
}

func invalidSchool(reason string) error { return &InvalidSchoolError{Reason: reason} }
