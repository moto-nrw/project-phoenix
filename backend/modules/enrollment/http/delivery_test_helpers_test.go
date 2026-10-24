package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	api "github.com/moto-nrw/project-phoenix/roots/owners"

	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	orm "github.com/uptrace/bun"
)

type testEnrollmentDelivery struct{ module delivery.Capability }

func newTestEnrollmentDelivery(t *testing.T, db *orm.DB) testEnrollmentDelivery {
	t.Helper()
	module, err := api.NewEnrollmentFlowDelivery(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return testEnrollmentDelivery{module: module}
}

func (a testEnrollmentDelivery) CountRelatedEmails(ctx context.Context, relatedType string, relatedID int64) (int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := a.module.Statuses(ctx, tenantID.Int64(), deliveryContract.TransportEmail, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID})
	return len(rows), err
}

func (a testEnrollmentDelivery) CancelRelatedEmails(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	return a.module.Cancel(ctx, tenantID.Int64(), deliveryContract.TransportEmail, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

func enqueueTestEnrollmentEmail(t *testing.T, db *orm.DB, tenantID, requestID int64, key string, payload map[string]any) deliveryContract.Enqueued {
	t.Helper()
	capability := newTestEnrollmentDelivery(t, db)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	var result deliveryContract.Enqueued
	err = testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ orm.Tx) error {
		var enqueueErr error
		result, enqueueErr = enqueueTestEnrollmentEmailInContext(ctx, capability, tenantID, requestID, key, encoded)
		return enqueueErr
	})
	require.NoError(t, err)
	return result
}

func enqueueTestEnrollmentEmailInContext(ctx context.Context, capability testEnrollmentDelivery, tenantID, requestID int64, key string, payload json.RawMessage) (deliveryContract.Enqueued, error) {
	return capability.module.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		TenantID: tenantID, Template: "enrollment_test", Recipient: deliveryContract.EmailRecipient{Address: "guardian@example.invalid"},
		Payload: payload, IdempotencyKey: key, Related: deliveryContract.RelatedEntity{Type: "enrollment_request", ID: requestID},
	})
}
