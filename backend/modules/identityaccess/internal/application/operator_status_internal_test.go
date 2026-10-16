package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeactivateOperatorLeavesTheTimestampToTheDatabase(t *testing.T) {
	t.Parallel()
	fixture := newOperatorProvisioningFixture(t)
	actor := fixture.seedInviter(1)
	target := fixture.seedOperatorWithEmail(2, "leaving@example.com")
	fixture.provisioning.now = func() time.Time {
		t.Fatal("the database trigger must set the deactivation timestamp")
		return time.Time{}
	}

	deactivated, err := fixture.provisioning.ChangeOperatorStatus(context.Background(), domain.OperatorStatusChange{
		OperatorID: target.ID,
		ActorID:    actor.ID,
	})

	require.NoError(t, err)
	assert.False(t, deactivated.Active)
	assert.Nil(t, deactivated.DeactivatedAt)
	assert.Nil(t, fixture.store.operators[target.ID].DeactivatedAt)
}

// Account retention delegates its calendar cutoff to the PostgreSQL store:
// that keeps its database-generated deactivation times and the cutoff on the
// same clock. The application still opens the administrative transaction.
func TestDeleteDeactivatedOperatorsUsesAnAdministrativeTransaction(t *testing.T) {
	t.Parallel()
	fixture := newOperatorProvisioningFixture(t)

	deleted, err := fixture.provisioning.DeleteDeactivatedOperators(context.Background())

	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Equal(t, 1, fixture.runtime.adminTxCount)
	assert.Equal(t, []string{"DeleteDeactivatedOperators"}, fixture.store.recorded())
}
