package organizationtenancy_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "success", want: "none"},
		{name: "not found", err: organizationtenancy.ErrOrganizationNotFound, want: "not_found"},
		{name: "invalid", err: &organizationtenancy.InvalidOrganizationError{Reason: "bad input"}, want: "invalid"},
		{name: "slug conflict", err: organizationtenancy.ErrOrganizationSlugConflict, want: "slug_conflict"},
		{name: "already deleted", err: organizationtenancy.ErrOrganizationAlreadyDeleted, want: "already_deleted"},
		{name: "not deleted", err: organizationtenancy.ErrOrganizationNotDeleted, want: "not_deleted"},
		{name: "has schools", err: &organizationtenancy.OrganizationHasSchoolsError{SchoolCount: 2}, want: "has_schools"},
		{name: "school not found", err: organizationtenancy.ErrSchoolNotFound, want: "school_not_found"},
		{name: "invalid school", err: &organizationtenancy.InvalidSchoolError{Reason: "bad input"}, want: "invalid_school"},
		{name: "school slug conflict", err: organizationtenancy.ErrSchoolSlugConflict, want: "school_slug_conflict"},
		{name: "school subdomain conflict", err: organizationtenancy.ErrSchoolDomainConflict, want: "school_subdomain_conflict"},
		{name: "school already deleted", err: organizationtenancy.ErrSchoolAlreadyDeleted, want: "school_already_deleted"},
		{name: "school not deleted", err: organizationtenancy.ErrSchoolNotDeleted, want: "school_not_deleted"},
		{name: "school organization deleted", err: organizationtenancy.ErrOrganizationDeleted, want: "school_organization_deleted"},
		{name: "unexpected", err: errors.New("database unavailable"), want: "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := organizationtenancy.ErrorCode(tt.err); got != tt.want {
				t.Fatalf("ErrorCode(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
