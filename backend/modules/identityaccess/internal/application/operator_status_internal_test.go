package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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

	deactivated, err := fixture.provisioning.ChangeOperatorStatus(context.Background(), identityaccess.OperatorStatusChange{
		OperatorID: target.ID,
		ActorID:    actor.ID,
	})

	require.NoError(t, err)
	assert.False(t, deactivated.Active)
	assert.Nil(t, deactivated.DeactivatedAt)
	assert.Nil(t, fixture.store.operators[target.ID].DeactivatedAt)
}

func TestOperatorStatusChangeRollsBackWhenTheLedgerWriteFails(t *testing.T) {
	t.Parallel()
	for name, active := range map[string]bool{"deactivation": false, "reactivation": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := newOperatorProvisioningFixture(t)
			actor := fixture.seedInviter(1)
			target := fixture.seedOperatorWithEmail(2, "leaving@example.com")
			target.Active = !active
			fixture.store.operators[target.ID] = target
			fixture.store.operatorTokens[1] = domain.OperatorSession{ID: 1, OperatorID: target.ID}
			fixture.audit.err = errFakeStore
			fixture.provisioning.runtime = &operatorStatusRollbackRuntime{fakeRuntime: fixture.runtime, store: fixture.store}

			_, err := fixture.provisioning.ChangeOperatorStatus(context.Background(), identityaccess.OperatorStatusChange{
				OperatorID: target.ID,
				ActorID:    actor.ID,
				Active:     active,
			})

			require.ErrorIs(t, err, errFakeStore)
			assert.Equal(t, !active, fixture.store.operators[target.ID].Active)
			assert.Len(t, fixture.store.operatorSessionsOf(target.ID), 1)
		})
	}
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

// operatorStatusRollbackRuntime models the database transaction that keeps
// an operator's status, sessions and ledger entry together.
type operatorStatusRollbackRuntime struct {
	*fakeRuntime
	store *fakeStore
}

func (r *operatorStatusRollbackRuntime) WithAdminTx(ctx context.Context, fn func(context.Context) error) error {
	r.store.mu.Lock()
	operators := make(map[int64]domain.Operator, len(r.store.operators))
	for id, operator := range r.store.operators {
		operators[id] = operator
	}
	sessions := make(map[int64]domain.OperatorSession, len(r.store.operatorTokens))
	for id, session := range r.store.operatorTokens {
		sessions[id] = session
	}
	r.store.mu.Unlock()

	err := r.fakeRuntime.WithAdminTx(ctx, fn)
	if err == nil {
		return nil
	}
	r.store.mu.Lock()
	r.store.operators = operators
	r.store.operatorTokens = sessions
	r.store.mu.Unlock()
	return err
}
