// Package organizationtenancy is the public Organization & Tenancy capability.
package organizationtenancy

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type InvalidOrganizationError struct{ Reason string }

func (e *InvalidOrganizationError) Error() string { return e.Reason }
func (e *InvalidOrganizationError) Unwrap() error { return ErrInvalidOrganization }

type OrganizationHasSchoolsError struct{ SchoolCount int }

func (e *OrganizationHasSchoolsError) Error() string {
	return fmt.Sprintf("organization has %d existing school(s)", e.SchoolCount)
}
func (e *OrganizationHasSchoolsError) Unwrap() error { return ErrOrganizationHasSchools }

const (
	MaxNameLength     = 200
	MaxSlugLength     = 100
	MaxDNSLabelLength = 63
	MaxEmailLength    = 255
)

var (
	slugPattern   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	reservedSlugs = map[string]bool{
		"www": true, "api": true, "operator": true, "parents": true,
		"eltern": true, "schule": true, "school": true, "grafana": true,
		"pyreportal": true, "help": true, "admin": true, "app": true,
		"dashboard": true, "analytics": true, "status": true, "mail": true,
		"staging": true, "demo": true,
	}

	ErrOrganizationNotFound       = errors.New("organization not found")
	ErrInvalidOrganization        = errors.New("invalid organization")
	ErrOrganizationSlugConflict   = errors.New("organization slug already exists")
	ErrOrganizationAlreadyDeleted = errors.New("organization is already deleted")
	ErrOrganizationNotDeleted     = errors.New("organization is not deleted")
	ErrOrganizationHasSchools     = errors.New("organization has schools")
)

type Organization struct {
	ID        int64      `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	Active    bool       `json:"active"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	Settings  string     `json:"settings,omitempty"`
}

func (o Organization) IsDeleted() bool { return o.DeletedAt != nil }

type CreateOrganization struct {
	Name   string
	Slug   string
	Active bool
}

type UpdateOrganization struct {
	ID     int64
	Name   string
	Slug   string
	Active bool
}

type Query interface {
	FindOrganization(context.Context, int64) (Organization, error)
	FindOrganizationBySlug(context.Context, string) (Organization, error)
	ListOrganizations(context.Context) ([]Organization, error)
	ListOrganizationsByID(context.Context, []int64) ([]Organization, error)
	CountOrganizationsByID(context.Context, []int64) (int, error)
	FindOrganizationForMutation(context.Context, int64) (Organization, error)
	FindOrganizationForSchoolMutation(context.Context, int64) (Organization, error)
	FindSchool(context.Context, int64) (School, error)
	FindSchoolForShare(context.Context, int64) (School, error)
	FindSchoolForMutation(context.Context, int64) (School, error)
	FindSchoolBySlug(context.Context, string) (School, error)
	FindSchoolByOrganizationAndSlug(context.Context, int64, string) (School, error)
	FindSchoolBySubdomain(context.Context, string) (School, error)
	ListSchools(context.Context) ([]School, error)
	ListSchoolsByID(context.Context, []int64) ([]School, error)
	ListSchoolsByOrganization(context.Context, int64) ([]School, error)
	ListNonDeletedSchools(context.Context) ([]School, error)
	ListActiveSchools(context.Context) ([]School, error)
	ListPublicSchools(context.Context) ([]School, error)
	CountSchoolsByID(context.Context, []int64) (int, error)
}

type Command interface {
	CreateOrganization(context.Context, CreateOrganization) (Organization, error)
	UpdateOrganization(context.Context, UpdateOrganization) (Organization, error)
	SoftDeleteOrganization(context.Context, int64) (Organization, error)
	RestoreOrganization(context.Context, int64) (Organization, error)
	CreateSchool(context.Context, CreateSchool) (School, error)
	UpdateSchool(context.Context, UpdateSchool) (School, error)
	SoftDeleteSchool(context.Context, int64) (School, error)
	RestoreSchool(context.Context, int64) (School, error)
}

type Capability interface {
	Query
	Command
}

type engine interface {
	Create(context.Context, CreateOrganization) (Organization, error)
	Update(context.Context, UpdateOrganization) (Organization, error)
	SoftDelete(context.Context, int64) (Organization, error)
	Restore(context.Context, int64) (Organization, error)
	FindByID(context.Context, int64) (Organization, error)
	FindBySlug(context.Context, string) (Organization, error)
	List(context.Context) ([]Organization, error)
	ListByIDs(context.Context, []int64) ([]Organization, error)
	CountByIDs(context.Context, []int64) (int, error)
	FindForMutation(context.Context, int64) (Organization, error)
	FindForSchoolMutation(context.Context, int64) (Organization, error)
	CreateSchool(context.Context, CreateSchool) (School, error)
	UpdateSchool(context.Context, UpdateSchool) (School, error)
	SoftDeleteSchool(context.Context, int64) (School, error)
	RestoreSchool(context.Context, int64) (School, error)
	FindSchoolByID(context.Context, int64, string) (School, error)
	FindSchoolBySlug(context.Context, string) (School, error)
	FindSchoolByOrganizationAndSlug(context.Context, int64, string) (School, error)
	FindSchoolBySubdomain(context.Context, string) (School, error)
	ListSchools(context.Context) ([]School, error)
	ListSchoolsByID(context.Context, []int64) ([]School, error)
	ListSchoolsByOrganization(context.Context, int64) ([]School, error)
	ListNonDeletedSchools(context.Context) ([]School, error)
	ListActiveSchools(context.Context) ([]School, error)
	ListPublicSchools(context.Context) ([]School, error)
	CountSchoolsByID(context.Context, []int64) (int, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("organization tenancy: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) CreateOrganization(ctx context.Context, input CreateOrganization) (Organization, error) {
	input.Name, input.Slug = normalizeNameAndSlug(input.Name, input.Slug)
	if err := validateNameAndSlug(input.Name, input.Slug); err != nil {
		return Organization{}, err
	}
	return m.engine.Create(ctx, input)
}

func (m *Module) UpdateOrganization(ctx context.Context, input UpdateOrganization) (Organization, error) {
	input.Name, input.Slug = normalizeNameAndSlug(input.Name, input.Slug)
	if input.ID <= 0 {
		return Organization{}, ErrInvalidOrganization
	}
	if err := validateNameAndSlug(input.Name, input.Slug); err != nil {
		return Organization{}, err
	}
	return m.engine.Update(ctx, input)
}

func (m *Module) SoftDeleteOrganization(ctx context.Context, id int64) (Organization, error) {
	if id <= 0 {
		return Organization{}, ErrInvalidOrganization
	}
	return m.engine.SoftDelete(ctx, id)
}

func (m *Module) RestoreOrganization(ctx context.Context, id int64) (Organization, error) {
	if id <= 0 {
		return Organization{}, ErrInvalidOrganization
	}
	return m.engine.Restore(ctx, id)
}

func (m *Module) FindOrganization(ctx context.Context, id int64) (Organization, error) {
	if id <= 0 {
		return Organization{}, ErrInvalidOrganization
	}
	return m.engine.FindByID(ctx, id)
}

func (m *Module) FindOrganizationBySlug(ctx context.Context, slug string) (Organization, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if err := ValidateSlug(slug); err != nil {
		return Organization{}, err
	}
	return m.engine.FindBySlug(ctx, slug)
}

func (m *Module) ListOrganizations(ctx context.Context) ([]Organization, error) {
	return m.engine.List(ctx)
}

func (m *Module) ListOrganizationsByID(ctx context.Context, ids []int64) ([]Organization, error) {
	if len(ids) == 0 {
		return []Organization{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvalidOrganization
		}
	}
	return m.engine.ListByIDs(ctx, ids)
}

func (m *Module) CountOrganizationsByID(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return 0, ErrInvalidOrganization
		}
	}
	return m.engine.CountByIDs(ctx, ids)
}

func (m *Module) FindOrganizationForSchoolMutation(ctx context.Context, id int64) (Organization, error) {
	if id <= 0 {
		return Organization{}, ErrInvalidOrganization
	}
	return m.engine.FindForSchoolMutation(ctx, id)
}

func (m *Module) FindOrganizationForMutation(ctx context.Context, id int64) (Organization, error) {
	if id <= 0 {
		return Organization{}, invalid("organization ID is required")
	}
	return m.engine.FindForMutation(ctx, id)
}

func ValidateSlug(slug string) error {
	if slug == "" {
		return invalid("slug is required")
	}
	if len(slug) > MaxSlugLength {
		return invalid("slug must not exceed 100 characters")
	}
	if !slugPattern.MatchString(slug) {
		return invalid("slug must contain only lowercase letters, numbers, and hyphens (no leading/trailing hyphens)")
	}
	if reservedSlugs[slug] {
		return invalid("slug is reserved for infrastructure use")
	}
	return nil
}

func normalizeNameAndSlug(name, slug string) (string, string) {
	return strings.TrimSpace(name), strings.TrimSpace(strings.ToLower(slug))
}

func validateNameAndSlug(name, slug string) error {
	if name == "" || len(name) > MaxNameLength {
		if name == "" {
			return invalid("name is required")
		}
		return invalid("name must not exceed 200 characters")
	}
	return ValidateSlug(slug)
}

func invalid(reason string) error { return &InvalidOrganizationError{Reason: reason} }

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrOrganizationNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidOrganization):
		return "invalid"
	case errors.Is(err, ErrOrganizationSlugConflict):
		return "slug_conflict"
	case errors.Is(err, ErrOrganizationAlreadyDeleted):
		return "already_deleted"
	case errors.Is(err, ErrOrganizationNotDeleted):
		return "not_deleted"
	case errors.Is(err, ErrOrganizationHasSchools):
		return "has_schools"
	case errors.Is(err, ErrSchoolNotFound):
		return "school_not_found"
	case errors.Is(err, ErrInvalidSchool):
		return "invalid_school"
	case errors.Is(err, ErrSchoolSlugConflict):
		return "school_slug_conflict"
	case errors.Is(err, ErrSchoolDomainConflict):
		return "school_subdomain_conflict"
	case errors.Is(err, ErrSchoolAlreadyDeleted):
		return "school_already_deleted"
	case errors.Is(err, ErrSchoolNotDeleted):
		return "school_not_deleted"
	case errors.Is(err, ErrOrganizationDeleted):
		return "school_organization_deleted"
	default:
		return "internal_error"
	}
}
