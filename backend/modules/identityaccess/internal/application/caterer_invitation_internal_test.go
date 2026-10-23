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
	checkSupply func()
}

func (s *invitationCatererStore) FindSchoolCaterer(context.Context) (domain.Caterer, bool, domain.OperationStats, error) {
	s.checkSupply()
	return domain.Caterer{}, true, domain.OperationStats{}, nil
}

func TestCatererInvitationLocksSchoolBeforeCheckingSupply(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	store := &invitationCatererStore{checkSupply: func() {
		require.Equal(t, []string{"caterer-school:10"}, f.runtime.locks, "the supply check must see acceptance committed under the same lock")
	}}
	caterers := &CatererAccess{store: store, runtime: f.runtime}
	err := f.runtime.WithTenantTx(context.Background(), 10, func(ctx context.Context) error {
		_, _, err := caterers.createInvitation(ctx, domain.CatererInvitationRequest{
			Email: "kitchen@example.com", CatererName: "Kitchen", CreatedBy: 1,
		})
		return err
	})
	require.ErrorIs(t, err, domain.ErrCatererAlreadySupplies)
}
