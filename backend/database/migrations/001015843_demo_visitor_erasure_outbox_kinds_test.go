package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"platform.email_outbox": {"care_contract", "care_contract_notice", "holiday_registration_received", "parent_billing_letter"},
		"platform.push_outbox":  {"care_contract_document", "parent_billing_document", "traeger_document_release", "traeger_document_release_account"},
	} {
		constraint := "chk_" + strings.TrimPrefix(table, "platform.") + "_traeger_kind"
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
