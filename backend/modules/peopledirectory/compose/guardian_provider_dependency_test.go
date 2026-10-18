package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/users"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type namedGuardianService struct {
	peoplePorts.GuardianService
	firstName string
}

func (s namedGuardianService) GetGuardianByID(_ context.Context, id int64) (*users.GuardianProfile, error) {
	profile := &users.GuardianProfile{FirstName: s.firstName}
	profile.ID = id
	return profile, nil
}

type directTenantTx struct{}

func (directTenantTx) InTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// The guardian provider passed in Dependencies serves the guardian calls from
// construction on; an incomplete provider is a construction error.
func TestNew_TakesTheGuardianProvider(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	deps := Dependencies{StudentDirectory: testpkg.StudentDirectory{}, DB: db, Observe: func(Observation) {}}

	deps.GuardianProvider = &GuardianDirectoryProvider{
		Guardians: namedGuardianService{firstName: t.Name()}, Runtime: directTenantTx{},
		PermissionGranted: func(any) bool { return true },
	}
	module, err := New(deps)
	require.NoError(t, err)
	guardianID := int64(len(t.Name()))
	guardian, err := module.FindGuardian(context.Background(), guardianID)
	require.NoError(t, err)
	require.Equal(t, t.Name(), guardian.FirstName)
	require.Equal(t, guardianID, guardian.ID)

	deps.GuardianProvider = &GuardianDirectoryProvider{Guardians: namedGuardianService{}, Runtime: directTenantTx{}}
	_, err = New(deps)
	require.ErrorContains(t, err, "guardian provider needs")
}
