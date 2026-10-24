package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// traegerStore answers the two reads FindVisibleTraegerName makes and
// records the school filter it asked for.
type traegerStore struct {
	ports.Store
	organization  organizationtenancy.Organization
	found         bool
	schools       int
	listedState   string
	listedOrgID   int64
	organizationN int
}

func (s *traegerStore) ListSchools(_ context.Context, _ []int64, organizationID *int64, state string) ([]organizationtenancy.School, domain.OperationStats, error) {
	s.listedState, s.listedOrgID = state, *organizationID
	return make([]organizationtenancy.School, s.schools), domain.OperationStats{}, nil
}

func (s *traegerStore) FindByID(context.Context, int64, string) (organizationtenancy.Organization, bool, domain.OperationStats, error) {
	s.organizationN++
	return s.organization, s.found, domain.OperationStats{}, nil
}

func TestFindVisibleTraegerName(t *testing.T) {
	t.Parallel()

	deletedAt := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		schools      int
		organization organizationtenancy.Organization
		found        bool
		want         string
	}{
		{name: "several operational schools", schools: 2, organization: organizationtenancy.Organization{ID: 7, Name: "Träger Nord"}, found: true, want: "Träger Nord"},
		{name: "single OGS stays unchanged", schools: 1, organization: organizationtenancy.Organization{ID: 7, Name: "Träger Nord"}, found: true},
		{name: "deleted organization", schools: 3, organization: organizationtenancy.Organization{ID: 7, Name: "Träger Nord", DeletedAt: &deletedAt}, found: true},
		{name: "missing organization", schools: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := &traegerStore{organization: tt.organization, found: tt.found, schools: tt.schools}
			module := New(store, ownerTestTransaction{}, func(ports.Observation) {})

			got, err := module.FindVisibleTraegerName(context.Background(), 7)
			if err != nil {
				t.Fatalf("FindVisibleTraegerName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("FindVisibleTraegerName() = %q, want %q", got, tt.want)
			}
			if store.listedState != "active" || store.listedOrgID != 7 {
				t.Fatalf("school filter = (%d, %q), want only the operational schools of organization 7", store.listedOrgID, store.listedState)
			}
			if tt.schools < minVisibleTraegerSchools && store.organizationN != 0 {
				t.Fatalf("a single-OGS Träger must not read the organization")
			}
		})
	}
}

func TestFindVisibleTraegerNameRejectsInvalidOrganization(t *testing.T) {
	t.Parallel()

	module := New(&traegerStore{}, ownerTestTransaction{}, func(ports.Observation) {})
	if _, err := module.FindVisibleTraegerName(context.Background(), 0); !errors.Is(err, organizationtenancy.ErrInvalidOrganization) {
		t.Fatalf("FindVisibleTraegerName(0) error = %v, want ErrInvalidOrganization", err)
	}
}
