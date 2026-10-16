package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
