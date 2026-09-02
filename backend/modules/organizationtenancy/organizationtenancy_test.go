package organizationtenancy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type recordingEngine struct {
	created organizationtenancy.CreateOrganization
}

func (e *recordingEngine) Create(_ context.Context, input organizationtenancy.CreateOrganization) (organizationtenancy.Organization, error) {
	e.created = input
	return organizationtenancy.Organization{Name: input.Name, Slug: input.Slug, Active: input.Active}, nil
}

func (e *recordingEngine) Update(context.Context, organizationtenancy.UpdateOrganization) (organizationtenancy.Organization, error) {
	return organizationtenancy.Organization{}, nil
}

func (e *recordingEngine) SoftDelete(context.Context, int64) (organizationtenancy.Organization, error) {
	return organizationtenancy.Organization{}, nil
}
func (e *recordingEngine) Restore(context.Context, int64) (organizationtenancy.Organization, error) {
	return organizationtenancy.Organization{}, nil
}

func (e *recordingEngine) FindByID(context.Context, int64) (organizationtenancy.Organization, error) {
	return organizationtenancy.Organization{}, nil
}

func (e *recordingEngine) FindBySlug(context.Context, string) (organizationtenancy.Organization, error) {
	return organizationtenancy.Organization{}, nil
}

func (e *recordingEngine) List(context.Context) ([]organizationtenancy.Organization, error) {
	return nil, nil
}

func (e *recordingEngine) ListByIDs(context.Context, []int64) ([]organizationtenancy.Organization, error) {
	return nil, nil
}

func (e *recordingEngine) CountByIDs(context.Context, []int64) (int, error) { return 0, nil }

func (e *recordingEngine) FindForMutation(context.Context, int64) (organizationtenancy.Organization, error) {
	return organizationtenancy.Organization{}, nil
}

func (e *recordingEngine) FindForSchoolMutation(context.Context, int64) (organizationtenancy.Organization, error) {
	return organizationtenancy.Organization{}, nil
}

func (e *recordingEngine) CreateSchool(context.Context, organizationtenancy.CreateSchool) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) UpdateSchool(context.Context, organizationtenancy.UpdateSchool) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) SoftDeleteSchool(context.Context, int64) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) RestoreSchool(context.Context, int64) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) FindSchoolByID(context.Context, int64, string) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) FindSchoolBySlug(context.Context, string) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) FindSchoolByOrganizationAndSlug(context.Context, int64, string) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) FindSchoolBySubdomain(context.Context, string) (organizationtenancy.School, error) {
	return organizationtenancy.School{}, nil
}
func (e *recordingEngine) ListSchools(context.Context) ([]organizationtenancy.School, error) {
	return nil, nil
}
func (e *recordingEngine) ListSchoolsByID(context.Context, []int64) ([]organizationtenancy.School, error) {
	return nil, nil
}
func (e *recordingEngine) ListSchoolsByOrganization(context.Context, int64) ([]organizationtenancy.School, error) {
	return nil, nil
}
func (e *recordingEngine) ListNonDeletedSchools(context.Context) ([]organizationtenancy.School, error) {
	return nil, nil
}
func (e *recordingEngine) ListActiveSchools(context.Context) ([]organizationtenancy.School, error) {
	return nil, nil
}
func (e *recordingEngine) ListPublicSchools(context.Context) ([]organizationtenancy.School, error) {
	return nil, nil
}
func (e *recordingEngine) CountSchoolsByID(context.Context, []int64) (int, error) {
	return 0, nil
}

func TestCommandNormalizesOrganizationAtPublicSeam(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := organizationtenancy.NewModule(engine)
	created, err := module.CreateOrganization(context.Background(), organizationtenancy.CreateOrganization{
		Name:   "  Stadt Köln  ",
		Slug:   "  STADT-KOELN  ",
		Active: true,
	})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if engine.created.Name != "Stadt Köln" || engine.created.Slug != "stadt-koeln" {
		t.Fatalf("normalized input = %#v", engine.created)
	}
	if created.Name != "Stadt Köln" || created.Slug != "stadt-koeln" {
		t.Fatalf("created organization = %#v", created)
	}
}

func TestCommandRejectsReservedSlugAtPublicSeam(t *testing.T) {
	t.Parallel()

	module := organizationtenancy.NewModule(&recordingEngine{})
	_, err := module.CreateOrganization(context.Background(), organizationtenancy.CreateOrganization{
		Name: "Platform", Slug: "operator", Active: true,
	})
	if !errors.Is(err, organizationtenancy.ErrInvalidOrganization) {
		t.Fatalf("CreateOrganization() error = %v, want ErrInvalidOrganization", err)
	}
}

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
