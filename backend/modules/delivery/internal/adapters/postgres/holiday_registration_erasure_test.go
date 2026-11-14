package postgres_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestHolidayRegistrationErasureTakesEveryMailOfTheRegistration (#7000): the
// mail kinds come from the outbox CHECK of the Träger kinds, so a new holiday
// mail is in this test the moment the database accepts it.
func TestHolidayRegistrationErasureTakesEveryMailOfTheRegistration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferienanmeldung löschen", Slug: fmt.Sprintf("holiday-erasure-%d", testpkg.Tenant(t)),
	}).ID

	var check string
	require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
  WHERE conname = 'chk_email_outbox_traeger_kind' AND conrelid = 'platform.email_outbox'::regclass`).Scan(ctx, &check))
	kinds := regexp.MustCompile(`'(holiday_[a-z_]+)'`).FindAllStringSubmatch(check, -1)
	require.NotEmpty(t, kinds, "the CHECK names the holiday mail kinds")
	assert.Contains(t, check, "'holiday_parent_account_offer'")

	const erased, kept int64 = 7_000_001, 7_000_002
	insert := func(kind, key string, related int64, payload string) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO platform.email_outbox (organization_id, kind, idempotency_key,
   recipient, payload, related_entity_type, related_entity_id)
   VALUES (?, ?, ?, '{"address":"familie@example.org"}', ?::jsonb, 'holiday_registration', ?)`,
			organization, kind, key, payload, related)
		require.NoError(t, err)
	}
	for _, match := range kinds {
		insert(match[1], "erased:"+match[1], erased, `{}`)
		insert(match[1], "kept:"+match[1], kept, `{}`)
	}
	// A closure notice bundles the registrations of one family; it hangs at the
	// first and goes with any of them.
	insert("holiday_closure_notice", "bundled", kept, fmt.Sprintf(`{"registration_ids":[%d,%d]}`, kept, erased))

	store := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	var deleted int
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(ctx context.Context, _ bun.Tx) error {
		var err error
		deleted, err = store.EraseHolidayRegistrationMails(ctx, []int64{organization}, []int64{erased})
		return err
	}))

	var left []string
	require.NoError(t, db.NewRaw(`SELECT idempotency_key FROM platform.email_outbox WHERE organization_id = ?
  ORDER BY idempotency_key`, organization).Scan(ctx, &left))
	want := make([]string, 0, len(kinds))
	for _, match := range kinds {
		want = append(want, "kept:"+match[1])
	}
	assert.ElementsMatch(t, want, left, "every mail of the erased registration goes, the other family's stay")
	assert.Equal(t, len(kinds)+1, deleted)
}
