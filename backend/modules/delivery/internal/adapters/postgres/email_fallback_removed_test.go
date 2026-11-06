package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fallbackRemovedProvider answers as the mailer of a deployment whose fallback was
// taken out of the configuration: a mail that asks for it ends, every other leaves.
type fallbackRemovedProvider struct{ sends, refused int }

func (p *fallbackRemovedProvider) Send(_ context.Context, intent domain.Intent) (deliveryContract.ProviderResult, error) {
	if intent.UseFallback {
		p.refused++
		return deliveryContract.ProviderResult{}, fmt.Errorf("%w: %s", domain.ErrPermanent, "no fallback")
	}
	p.sends++
	return deliveryContract.ProviderResult{MessageID: "again@mail.example", ProviderMessageID: "provider-again"}, nil
}

// The request stays open from before the change of configuration. Through the real
// worker and store: the mail ends on its first try and never returns to the queue.
func TestAnOpenFallbackRequestEndsOnceTheFallbackIsGone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "fallback-removed", true)
	sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
	event := eventAt(eventMailToken(t, db, mail.ID), "suppressed", domain.EmailDeliverySuppressed, time.Now())
	event.Reroute, event.KeepsAddress = true, true
	result, err := store.RecordEmailDeliveryEvent(ctx, event)
	require.NoError(t, err)
	require.True(t, result.Rerouted)

	provider := &fallbackRemovedProvider{}
	commands := application.NewWorkerCommands(store.Store, store.withinAdmin, store.withinTenant)
	worker := application.NewWorker(commands, provider, func(domain.Observation) {})
	for range 3 {
		_, err := worker.RunOnce(ctx, 50, 5)
		require.NoError(t, err)
	}

	assert.Zero(t, provider.sends, "nothing goes back over the provider")
	assert.Equal(t, 1, provider.refused, "the request ends on its first try")
	status, _, requested := mailStatus(t, db, mail.ID)
	assert.Equal(t, "dead_letter", status, "the mail shows as given up")
	assert.True(t, requested, "the record of the request stays")
}
