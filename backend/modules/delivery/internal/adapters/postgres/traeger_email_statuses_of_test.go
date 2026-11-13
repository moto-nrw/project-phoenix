package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/adapters/postgres"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// #6976: a Träger reads its mails of the related entities and with the keys
// in one statement. Another Träger's mail with the same related ID or the
// same key stays out, also through the Träger's own transaction.
func TestTraegerEmailStatusesOfStaysWithTheTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	organization := func(name string) int64 {
		return testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("statuses-of-%s-%d", name, testpkg.Tenant(t)),
		}).ID
	}
	own, other := organization("own"), organization("other")
	mail := func(organizationID int64, kind, relatedType string, relatedID int64, key string) {
		_, err := db.ExecContext(ctx, `INSERT INTO platform.email_outbox
			(organization_id, kind, recipient, payload, related_entity_type, related_entity_id, idempotency_key)
			VALUES (?, ?, '{"address":"family@example.org"}', '{}', ?, ?, ?)`,
			organizationID, kind, relatedType, relatedID, key)
		require.NoError(t, err)
	}
	mail(own, "holiday_registration_received", "holiday_registration", 41, "received:41")
	mail(own, "holiday_closure_notice", "holiday_registration", 40, "closure:family")
	mail(own, "holiday_registration_received", "holiday_registration", 42, "received:42")
	mail(own, "holiday_place_granted", "care_contract", 41, "contract:41")
	mail(other, "holiday_registration_received", "holiday_registration", 41, "other:41")
	mail(other, "holiday_closure_notice", "holiday_registration", 99, "closure:family")

	raw := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		rows, err := raw.TraegerEmailStatusesOf(ctx, own, "holiday_registration", []int64{41}, []string{"closure:family"})
		require.NoError(t, err)
		keys := make([]string, 0, len(rows))
		for _, row := range rows {
			assert.Equal(t, own, row.OrganizationID)
			assert.False(t, row.CreatedAt.IsZero(), "each mail names when it was enqueued")
			keys = append(keys, *row.IdempotencyKey)
		}
		assert.ElementsMatch(t, []string{"received:41", "closure:family"}, keys,
			"the related mail and the keyed mail, without another entity type, another registration or another Träger")
		return nil
	}))
}
