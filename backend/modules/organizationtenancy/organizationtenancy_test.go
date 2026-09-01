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
