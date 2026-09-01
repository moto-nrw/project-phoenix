package enrollment_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	orm "github.com/uptrace/bun"
)

type testDeliveryProvider struct{}

func (testDeliveryProvider) SendEmail(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	return delivery.ProviderResult{}, nil
}

func (testDeliveryProvider) SendPush(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	return delivery.ProviderResult{}, nil
}

type testGuardianDirectory struct{}

func (testGuardianDirectory) ResolveGuardianDisplays(context.Context, []int64) ([]delivery.GuardianDisplay, error) {
	return nil, nil
}

type testEnrollmentDelivery struct{ module *delivery.Module }

func newTestEnrollmentDelivery(t *testing.T, db *orm.DB) testEnrollmentDelivery {
	t.Helper()
	runtime, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB: db, Provider: testDeliveryProvider{}, People: testGuardianDirectory{}, Observe: func(delivery.Observation) {},
	})
	require.NoError(t, err)
	return testEnrollmentDelivery{module: runtime.Module}
}

func (a testEnrollmentDelivery) CountRelatedEmails(ctx context.Context, relatedType string, relatedID int64) (int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := a.module.Statuses(ctx, tenantID.Int64(), delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID})
	return len(rows), err
}

func (a testEnrollmentDelivery) CancelRelatedEmails(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	return a.module.Cancel(ctx, tenantID.Int64(), delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

func enqueueTestEnrollmentEmail(t *testing.T, db *orm.DB, tenantID, requestID int64, key string, payload map[string]any) delivery.Enqueued {
	t.Helper()
	capability := newTestEnrollmentDelivery(t, db)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	var result delivery.Enqueued
	testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ orm.Tx) error {
		var enqueueErr error
		result, enqueueErr = enqueueTestEnrollmentEmailInContext(ctx, capability, tenantID, requestID, key, encoded)
		return enqueueErr
	})
	return result
}

func enqueueTestEnrollmentEmailInContext(ctx context.Context, capability testEnrollmentDelivery, tenantID, requestID int64, key string, payload json.RawMessage) (delivery.Enqueued, error) {
	return capability.module.EnqueueEmail(ctx, delivery.EmailIntent{
		TenantID: tenantID, Template: "enrollment_test", Recipient: delivery.EmailRecipient{Address: "guardian@example.invalid"},
		Payload: payload, IdempotencyKey: key, Related: delivery.RelatedEntity{Type: "enrollment_request", ID: requestID},
	})
}
