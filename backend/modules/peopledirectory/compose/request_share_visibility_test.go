package compose

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRequestShareVisibilityRequiresTheLedgerAndTheFamilyProtection(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	shares := NewParentRequestShareEventRepository(db)
	protection := NewFamilyProtectionEventRepository(db)
	const message = "parent: request share visibility needs the sharing ledger and the family protection"
	_, err := NewRequestShareVisibility(nil, protection)
	require.EqualError(t, err, message)
	_, err = NewRequestShareVisibility(shares, nil)
	require.EqualError(t, err, message)
}
