// Package organizationtenancy is the public Organization & Tenancy capability.
package organizationtenancy

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
	slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	// reservedSlugs are infrastructure subdomains that never name a tenant.
	// Frontend equivalent: frontend/src/lib/reserved-slugs.ts; both lists
	// must stay in sync.
	reservedSlugs = map[string]bool{
		"www": true, "api": true, "operator": true, "parents": true,
		"eltern": true, "schule": true, "school": true, "traeger": true, "grafana": true,
		"pyreportal": true, "help": true, "admin": true, "app": true,
		"dashboard": true, "analytics": true, "status": true, "mail": true,
		"staging": true, "demo": true, "ingest": true, "caterer": true,
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
	OrganizationQuery
	SchoolQuery
	SchoolListQuery
	TraegerMembershipQuery
	TraegerErasureOrderQuery
}

// OrganizationQuery reads organizations and locks them before mutation.
type OrganizationQuery interface {
	FindOrganization(context.Context, int64) (Organization, error)
	FindOrganizationBySlug(context.Context, string) (Organization, error)
	ListOrganizations(context.Context) ([]Organization, error)
	ListOrganizationsByID(context.Context, []int64) ([]Organization, error)
	CountOrganizationsByID(context.Context, []int64) (int, error)
	FindOrganizationForMutation(context.Context, int64) (Organization, error)
	FindOrganizationForSchoolMutation(context.Context, int64) (Organization, error)
	// FindVisibleTraegerName returns the name of the Träger (organization)
	// a school's own portal shows. Only a Träger with more than one
	// operational school has a visible level; for a Träger with a single
	// OGS, and for a deleted organization, the name is empty (#4656).
	FindVisibleTraegerName(context.Context, int64) (string, error)
}

// SchoolQuery resolves individual schools and their mutation locks.
type SchoolQuery interface {
	FindSchool(context.Context, int64) (School, error)
	FindSchoolForShare(context.Context, int64) (School, error)
	FindSchoolForMutation(context.Context, int64) (School, error)
	FindSchoolBySlug(context.Context, string) (School, error)
	FindSchoolByOrganizationAndSlug(context.Context, int64, string) (School, error)
	FindSchoolBySubdomain(context.Context, string) (School, error)
}

// SchoolListQuery lists and counts schools for directory consumers.
type SchoolListQuery interface {
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
	SetSchoolChildQuota(context.Context, int64, *ChildQuota) (School, error)
	UpdateSchoolSettings(ctx context.Context, id int64, update func(string) (string, error)) error
	SoftDeleteSchool(context.Context, int64) (School, error)
	RestoreSchool(context.Context, int64) (School, error)
}

type Capability interface {
	Query
	Command
	TraegerMembershipCommand
	TraegerErasureOrderCommand
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
	case errors.Is(err, ErrOrganizationInactive):
		return "organization_inactive"
	default:
		return "internal_error"
	}
}

func invalid(reason string) error { return &InvalidOrganizationError{Reason: reason} }
