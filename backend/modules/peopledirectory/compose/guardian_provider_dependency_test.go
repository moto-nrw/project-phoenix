package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type primaryPhoneGuardians struct {
	peoplePorts.GuardianService
	primary *[]int64
}

func (s primaryPhoneGuardians) SetPrimaryPhone(_ context.Context, phoneID int64) error {
	*s.primary = append(*s.primary, phoneID)
	return nil
}

type directTenantTx struct{ runs *int }

func (r directTenantTx) InTenantTx(ctx context.Context, fn func(context.Context) error) error {
	*r.runs++
	return fn(ctx)
}

// The guardian provider passed in Dependencies serves the guardian calls from
// construction on, in its tenant transaction; an incomplete provider is a
// construction error.
func TestNew_TakesTheGuardianProvider(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	deps := Dependencies{StudentDirectory: testpkg.StudentDirectory{}, DB: db, Observe: func(Observation) {}}

	var primary []int64
	runs := 0
	deps.GuardianProvider = &GuardianDirectoryProvider{
		Guardians: primaryPhoneGuardians{primary: &primary}, Runtime: directTenantTx{runs: &runs},
		PermissionGranted: func(any) bool { return true },
	}
	module, err := New(deps, nil)
	require.NoError(t, err)
	phoneID := int64(len(t.Name()))
	require.NoError(t, module.SetPrimaryGuardianPhone(context.Background(), phoneID))
	require.Equal(t, []int64{phoneID}, primary)
	require.Equal(t, 1, runs)

	deps.GuardianProvider = &GuardianDirectoryProvider{Guardians: primaryPhoneGuardians{primary: &primary}, Runtime: directTenantTx{runs: &runs}}
	_, err = New(deps, nil)
	require.ErrorContains(t, err, "guardian provider needs")
}
