package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

type recordingEngine struct {
	ports.Store
	created organizationtenancy.CreateOrganization
}

func (e *recordingEngine) Create(_ context.Context, input organizationtenancy.CreateOrganization) (organizationtenancy.Organization, domain.OperationStats, error) {
	e.created = input
	return organizationtenancy.Organization{Name: input.Name, Slug: input.Slug, Active: input.Active}, domain.OperationStats{}, nil
}
func (*recordingEngine) FindBySlug(context.Context, string) (organizationtenancy.Organization, bool, domain.OperationStats, error) {
	return organizationtenancy.Organization{}, false, domain.OperationStats{}, nil
}
func TestCommandNormalizesOrganizationAtPublicSeam(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := New(engine, ownerTestTransaction{}, func(ports.Observation) {})
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

	module := New(&recordingEngine{}, ownerTestTransaction{}, func(ports.Observation) {})
	_, err := module.CreateOrganization(context.Background(), organizationtenancy.CreateOrganization{
		Name: "Platform", Slug: "operator", Active: true,
	})
	if !errors.Is(err, organizationtenancy.ErrInvalidOrganization) {
		t.Fatalf("CreateOrganization() error = %v, want ErrInvalidOrganization", err)
	}
}
