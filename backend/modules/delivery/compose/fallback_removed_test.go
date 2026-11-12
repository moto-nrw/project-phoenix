package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// fallbackRemovedProvider answers as the mailer of a deployment whose fallback was
// taken out of the configuration: a mail that asks for it ends, every other leaves.
type fallbackRemovedProvider struct {
	acceptingProvider
	accepted, refused *int
}

func (p fallbackRemovedProvider) SendEmail(_ context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	if intent.UseFallback {
		*p.refused++
		return deliveryContract.ProviderResult{}, fmt.Errorf("%w: no fallback", deliveryContract.ErrPermanent)
	}
	*p.accepted++
	return deliveryContract.ProviderResult{MessageID: "again@mail.example", ProviderMessageID: "provider-again"}, nil
}

// The request stays open from before the change of configuration. Through the real
// worker and store: the mail ends on its first try and never returns to the queue
// (#5957).
func TestAnOpenFallbackRequestEndsOnceTheFallbackIsGone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	var mail int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox
		(tenant_id, kind, recipient, payload, security, status, next_retry_at, fallback_requested_at)
		VALUES (?, 'welcome', '{"address":"code@example.org"}'::jsonb, '{}'::jsonb, TRUE, 'pending', NOW() - INTERVAL '1 minute', NOW())
		RETURNING id`, testpkg.Tenant(t)).Scan(ctx, &mail))
	runtime, err := NewModule(ModuleDependencies{DB: db, People: noGuardians{}, Observe: func(deliveryContract.Observation) {}})
	require.NoError(t, err)
	accepted, refused := 0, 0
	worker, err := NewWorker(runtime, fallbackRemovedProvider{accepted: &accepted, refused: &refused})
	require.NoError(t, err)

	_, err = worker.RunOnce(ctx, 50, 5)
	require.ErrorContains(t, err, "failed for good", "the run names the mail it gave up on")
	assert.NotContains(t, err.Error(), "code@example.org")
	for range 2 {
		processed, err := worker.RunOnce(ctx, 50, 5)
		require.NoError(t, err)
		assert.Zero(t, processed, "it does not come back")
	}

	assert.Zero(t, accepted, "nothing goes back over the provider")
	assert.Equal(t, 1, refused, "the request ends on its first try")
	var status string
	var requested bool
	require.NoError(t, db.NewRaw(`SELECT status, fallback_requested_at IS NOT NULL FROM platform.email_outbox WHERE id = ?`, mail).Scan(ctx, &status, &requested))
	assert.Equal(t, "dead_letter", status, "the mail shows as given up")
	assert.True(t, requested, "the record of the request stays")
}
