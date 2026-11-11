package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/migrate"
)

// TestOutboxKindsOfATraegerAreDeclared pins the list the outbox guard of
// #6342 reads: a row of a Träger takes only a declared kind, so a producer
// of a new kind fails until a migration declares it (and the guard then
// asks for its erasure). Rows of a school keep any kind.
func TestOutboxKindsOfATraegerAreDeclared(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Arten", Slug: fmt.Sprintf("traeger-arten-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Träger Arten OGS")
	insert := func(table string, tenantID, organizationID any, kind string) error {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		_, err = tx.ExecContext(ctx, `INSERT INTO `+table+` (tenant_id, organization_id, kind, recipient, payload)
			VALUES (?, ?, ?, '{}'::jsonb, '{}'::jsonb)`, tenantID, organizationID, kind)
		return err
	}
	for table, kinds := range map[string][]string{
		"platform.email_outbox": holidayAcceptanceEmailOutboxTraegerKinds, // extended by 1.15.878 (#4699), 1.15.887 (#6434) and 1.15.927 (#6672)
		"platform.push_outbox":  pushOutboxTraegerKinds,
	} {
		constraint := "chk_" + strings.TrimPrefix(table, "platform.") + "_traeger_kind"
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
			WHERE conrelid = ?::regclass AND conname = ?`, table, constraint).Scan(ctx, &definition))
		require.Equal(t, kinds, traegerKindsOfCheck(definition), "the preflight checks the kinds %s declares", constraint)
		for _, kind := range kinds {
			assert.NoError(t, insert(table, nil, organization, kind), "%s takes the declared kind %s of a Träger", table, kind)
		}
		assert.ErrorContains(t, insert(table, nil, organization, "traeger_new_notice"), constraint,
			"%s refuses an undeclared kind of a Träger", table)
		assert.NoError(t, insert(table, school, nil, "traeger_new_notice"), "%s keeps any kind of a school", table)
	}
}

// TestOutboxKindErasureExtendsTheErasureOf823 keeps the erasure the one of
// 1.15.823 with the statements of #6342 and nothing else.
func TestOutboxKindErasureExtendsTheErasureOf823(t *testing.T) {
	t.Parallel()
	// Every line of 1.15.823 stays, in its order; the new lines are the
	// statements of #6342 and the kinds the old ones now name.
	old, current := strings.Split(demoVisitorErasureCareNoticesSQL, "\n"), strings.Split(demoVisitorErasureOutboxKindsSQL, "\n")
	var added []string
	i := 0
	for _, line := range current {
		if i < len(old) && line == old[i] {
			i++
			continue
		}
		added = append(added, strings.TrimSpace(line))
	}
	require.Equal(t, len(old), i, "every line of the erasure of 1.15.823 stays, in its order")
	for _, line := range []string{
		"DELETE FROM platform.push_outbox",
		"AND push.kind IN ('traeger_document_release', 'traeger_document_release_account')",
		"AND outbox.kind = 'holiday_registration_received'",
		"AND kind = 'traeger_document_release_account'",
		"AND outbox.kind IN ('care_contract', 'care_contract_notice')",
		"AND push.kind = 'care_contract_document'",
		"AND outbox.kind = 'parent_billing_letter'",
		"AND push.kind = 'parent_billing_document'",
	} {
		assert.Contains(t, added, line, "the erasure adds %q", line)
	}
	holidayMail := strings.Index(demoVisitorErasureOutboxKindsSQL, "AND outbox.kind = 'holiday_registration_received'")
	registrations := strings.Index(demoVisitorErasureOutboxKindsSQL, "DELETE FROM enrollment.holiday_registrations")
	require.Positive(t, holidayMail)
	assert.Less(t, holidayMail, registrations, "the receipt goes while the registration still names its school")
}

// The preflight answers before a release stops the application. A fresh
// database, whose outbox tables get their rows of a Träger in the same
// release, has nothing to check.
func TestOutboxKindsPreflightSkipsPendingTraegerRows(t *testing.T) {
	t.Parallel()
	ctx := withPendingMigrations(t.Context(), migrate.MigrationSlice{
		{Name: "001015545"}, // holidayRegistrationsVersion
		{Name: "001015684"}, // parentBillingDocumentInboxVersion
	})
	require.NoError(t, demoVisitorErasureOutboxKindsPrecondition(ctx, nil))
}

// A row of a Träger with a kind the CHECK would refuse stops the release in
// the preflight with its kind and count; without one the preflight passes.
// Up adds each CHECK NOT VALID and validates it apart: a failed VALIDATE
// leaves the migration repeatable, and the CHECKs end validated.
func TestOutboxKindsPreflightReportsUndeclaredKindsAndUpValidates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	require.NoError(t, demoVisitorErasureOutboxKindsDown(ctx, db), "the state before 1.15.843")
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Altarten", Slug: fmt.Sprintf("traeger-altarten-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Träger Altarten OGS")
	require.NoError(t, demoVisitorErasureOutboxKindsPrecondition(ctx, db), "no row of a Träger yet")

	var oldPush int64
	var oldMails []int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.push_outbox (organization_id, kind, recipient, payload)
		VALUES (?, 'traeger_old_notice', '{}'::jsonb, '{}'::jsonb) RETURNING id`, organization).Scan(ctx, &oldPush))
	require.NoError(t, db.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload)
		VALUES (?, 'traeger_old_mail', '{}'::jsonb, '{}'::jsonb), (?, 'traeger_old_mail', '{}'::jsonb, '{}'::jsonb)
		RETURNING id`, organization, organization).Scan(ctx, &oldMails))
	require.Len(t, oldMails, 2)
	_, err := db.ExecContext(ctx, `INSERT INTO platform.push_outbox (tenant_id, kind, recipient, payload)
		VALUES (?, 'traeger_old_notice', '{}'::jsonb, '{}'::jsonb)`, school)
	require.NoError(t, err, "a row of a school keeps any kind")

	// The registered preflight of the migration, as `migrate preflight` asks it.
	err = MigrationRegistry[demoVisitorErasureOutboxKindsVersion].Precondition(ctx, db)
	require.ErrorIs(t, err, errUndeclaredOutboxTraegerKind)
	assert.ErrorContains(t, err, "platform.email_outbox traeger_old_mail: 2")
	assert.ErrorContains(t, err, "platform.push_outbox traeger_old_notice: 1")
	require.Error(t, demoVisitorErasureOutboxKindsUp(ctx, db), "VALIDATE refuses the old row inside the migration as well")

	_, err = db.ExecContext(ctx, `DELETE FROM platform.email_outbox WHERE organization_id = ? AND kind = 'traeger_old_mail'`, organization)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM platform.push_outbox WHERE id = ?`, oldPush)
	require.NoError(t, err)
	require.NoError(t, demoVisitorErasureOutboxKindsPrecondition(ctx, db), "without the old rows the preflight passes")
	require.NoError(t, demoVisitorErasureOutboxKindsUp(ctx, db), "the corrected release runs the migration again")
	var validated []bool
	require.NoError(t, db.NewRaw(`SELECT convalidated FROM pg_constraint
		WHERE conname IN ('chk_email_outbox_traeger_kind', 'chk_push_outbox_traeger_kind') ORDER BY conname`).Scan(ctx, &validated))
	assert.Equal(t, []bool{true, true}, validated, "both CHECKs are validated")
}
