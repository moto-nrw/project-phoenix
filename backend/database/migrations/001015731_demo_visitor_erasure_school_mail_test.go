package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A visitor the function of 1.15.646 erased has no demo order any more, so
// the function never reaches its hidden schools again. 1.15.731 removes the
// mail left there once, found through the anonymized visitor account its
// persons and guardian profiles still link. Other soft-deleted schools and
// open schools keep their mail, and a second run changes nothing (#5521).
func TestDemoVisitorErasureSchoolMailClearsTheMailOfErasedVisitors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	byPersonID, _ := testpkg.CreateTestTenant(t, db)
	byGuardianID, _ := testpkg.CreateTestTenant(t, db)
	otherDeletedID, _ := testpkg.CreateTestTenant(t, db)
	openID, _ := testpkg.CreateTestTenant(t, db)

	caregiver := testpkg.CreateTestAccount(t, db, "betreuer@example.org")
	parent := testpkg.CreateTestAccount(t, db, "eltern@example.org")
	_, err := db.NewRaw(`UPDATE auth.accounts SET email = 'demo-besuch-' || id || '@example.invalid', active = FALSE
		WHERE id IN (?, ?)`, caregiver.ID, parent.ID).Exec(ctx)
	require.NoError(t, err)
	person := testpkg.CreateTestPersonForTenant(t, db, byPersonID, "Demo", "Besuch")
	_, err = db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE id = ?`, caregiver.ID, person.ID).Exec(ctx)
	require.NoError(t, err)
	for _, schoolID := range []int64{byGuardianID, openID} {
		guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, schoolID, "Demo", "Besuch", "eltern@example.org")
		_, err = db.NewRaw(`UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, parent.ID, guardian.ID).Exec(ctx)
		require.NoError(t, err)
	}
	_, err = db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id IN (?)`,
		bun.List([]int64{byPersonID, byGuardianID, otherDeletedID})).Exec(ctx)
	require.NoError(t, err)

	mail := func(schoolID int64) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (tenant_id, kind, recipient, payload, status)
			VALUES (?, 'transactional_mail', jsonb_build_object('name', 'Zyxwolda Quorbelt', 'address', 'julia.klein@demo-k3m9x1.moto-ogs.de'),
				jsonb_build_object('FirstName', 'Zyxwolda'), 'pending') RETURNING id`, schoolID).Scan(ctx, &id))
		_, err := db.NewRaw(`INSERT INTO platform.email_delivery (tenant_id, related_entity_type, related_entity_id, outbox_id, recipient_email)
			VALUES (?, 'staff_qualification_reminder', ?, ?, 'julia.klein@demo-k3m9x1.moto-ogs.de')`, schoolID, id, id).Exec(ctx)
		require.NoError(t, err)
		return id
	}
	byPerson, byGuardian := mail(byPersonID), mail(byGuardianID)
	otherDeleted, open := mail(otherDeletedID), mail(openID)
	ids := []int64{byPerson, byGuardian, otherDeleted, open}
	schools := []int64{byPersonID, byGuardianID, otherDeletedID, openID}

	remaining := func() (mails []int64, deliveries []int64) {
		require.NoError(t, db.NewRaw(`SELECT id FROM platform.email_outbox WHERE id IN (?) ORDER BY id`, bun.List(ids)).Scan(ctx, &mails))
		require.NoError(t, db.NewRaw(`SELECT tenant_id FROM platform.email_delivery WHERE tenant_id IN (?) ORDER BY tenant_id`,
			bun.List(schools)).Scan(ctx, &deliveries))
		return mails, deliveries
	}

	require.NoError(t, demoVisitorErasureSchoolMailUp(ctx, db))
	mails, deliveries := remaining()
	assert.Equal(t, []int64{otherDeleted, open}, mails,
		"the hidden schools of erased visitors lose their mail, other deleted and open schools keep it")
	assert.Equal(t, []int64{otherDeletedID, openID}, deliveries,
		"the hidden schools of erased visitors lose their delivery records")

	require.NoError(t, demoVisitorErasureSchoolMailUp(ctx, db))
	again, deliveriesAgain := remaining()
	assert.Equal(t, mails, again, "a second run removes no further mail")
	assert.Equal(t, deliveries, deliveriesAgain, "a second run removes no further delivery record")
}
