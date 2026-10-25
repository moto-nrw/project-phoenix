package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/stretchr/testify/require"
)

type invitationCatererStore struct {
	ports.CatererStore
}

func (s *invitationCatererStore) FindSchoolCaterer(context.Context) (domain.CatererSupply, bool, domain.OperationStats, error) {
	return domain.CatererSupply{}, true, domain.OperationStats{}, nil
}

// The caterer portal workflow owns the supply lock (ADR 0067 addendum,
// condition 3): identity refuses the invitation while a caterer supplies the
// school and takes no lock of its own.
func TestCatererInvitationRefusedWhileTheSchoolIsSupplied(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	caterers := &CatererAccess{store: &invitationCatererStore{}, runtime: f.runtime}
	err := f.runtime.WithTenantTx(context.Background(), 10, func(ctx context.Context) error {
		_, _, err := caterers.createInvitation(ctx, domain.CatererInvitationRequest{
			Email: "kitchen@example.com", CatererName: "Kitchen", CreatedBy: 1,
		})
		return err
	})
	require.ErrorIs(t, err, domain.ErrCatererAlreadySupplies)
	require.Empty(t, f.runtime.locks, "the workflow holds the supply lock, not identity")
}
