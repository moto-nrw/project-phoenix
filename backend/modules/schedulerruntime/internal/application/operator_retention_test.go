package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingOperatorRetention struct{ calls *[]string }

func (r recordingOperatorRetention) DeleteExpiredOperatorAuditLog(context.Context) (int, error) {
	*r.calls = append(*r.calls, "ledger")
	return 2, nil
}

func (r recordingOperatorRetention) DeleteDeactivatedOperators(context.Context) (int, error) {
	*r.calls = append(*r.calls, "accounts")
	return 1, nil
}

// The hourly cleanup deletes the operator ledger before the deactivated
// accounts (#4047): a ledger entry still pointing at an account refuses its
// deletion, so the reverse order would leave every due account in place.
func TestTokenCleanupDeletesTheOperatorLedgerBeforeTheAccounts(t *testing.T) {
	t.Parallel()
	var calls []string
	deps := minimalWorkerDependencies(t)
	recorder := recordingOperatorRetention{calls: &calls}
	deps.OperatorRetention = ports.OperatorRetention{Ledger: recorder, Accounts: recorder}

	scheduler := newScheduler(deps)
	for _, job := range scheduler.cleanupJobs {
		_, err := job.Run(context.Background())
		require.NoError(t, err)
	}

	assert.Equal(t, []string{"ledger", "accounts"}, calls)
}

func TestWorkerRequiresTheOperatorRetention(t *testing.T) {
	t.Parallel()
	deps := minimalWorkerDependencies(t)
	deps.OperatorRetention.Accounts = nil
	require.ErrorContains(t, validateWorkerDependencies(deps), "operator account retention")
	deps = minimalWorkerDependencies(t)
	deps.OperatorRetention.Ledger = nil
	require.ErrorContains(t, validateWorkerDependencies(deps), "operator ledger retention")
}
