package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/require"
)

func TestErasureFileCatalogRequiresReviewedDeletionPath(t *testing.T) {
	t.Parallel()
	raw, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	require.NoError(t, json.Unmarshal(raw, &catalog))
	require.NoError(t, validateErasureFiles(catalog.Files))
	catalog.Files = append(catalog.Files, organizationtenancy.LifecycleFileKind{Kind: "unreviewed", Layout: "tenantDir"})
	require.Error(t, validateErasureFiles(catalog.Files))
	catalog.Files = catalog.Files[:len(catalog.Files)-1]
	catalog.Files[0].Layout = "account"
	require.Error(t, validateErasureFiles(catalog.Files))
}

// raceStore answers like the database when a Träger package is released
// between the package check and the begin: no package at first, then the
// refused begin, then one package.
type raceStore struct {
	begun    bool
	beginErr error
	packages string
}

func (s *raceStore) CheckSchoolLock(context.Context) error { return nil }
func (s *raceStore) WithSchoolLock(ctx context.Context, _ int64, run func(context.Context) error) error {
	return run(ctx)
}
func (s *raceStore) Call(_ context.Context, _, _ time.Duration, operation string, _ ...any) (json.RawMessage, error) {
	switch {
	case operation == "organization_begin":
		s.begun = true
		return nil, s.beginErr
	case s.begun:
		return json.RawMessage(s.packages), nil
	default:
		return json.RawMessage("0"), nil
	}
}

type acceptingLedger struct{}

func (acceptingLedger) Check([]int64, string) error  { return nil }
func (acceptingLedger) Record(json.RawMessage) error { return nil }

// The store hides SQL errors. A begin refused because of a package released
// since the check names the package and what to do (#6665); any other
// refusal keeps the store's error.
func TestBeginOrganizationNamesAPackageReleasedSinceTheCheck(t *testing.T) {
	t.Parallel()
	refused := errors.New("erasure: database step failed")
	for packages, want := range map[string]string{"1": "released return package that is not purged", "0": refused.Error()} {
		service := &Erasure{store: &raceStore{beginErr: refused, packages: packages}, ledger: acceptingLedger{}}
		err := service.beginOrganization(t.Context(), organizationtenancy.ErasureRequest{}, nil)
		require.ErrorContains(t, err, want)
	}
}
