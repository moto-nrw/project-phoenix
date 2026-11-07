package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestCareContractNoticesKeepTheirTraegerAndTheirStates pins 1.15.792
// (#4682) under the request role: a terminated contract has its last day,
// only a concluded contract ends, the conclusion records the withdrawal
// notice, and a Träger never sees another Träger's declarations.
func TestCareContractNoticesKeepTheirTraegerAndTheirStates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	traeger := func(name string) int64 {
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("traeger-kuendigung-%d", testpkg.UniqueTestTenantID(t)),
		}).ID
		testpkg.OwnTraegerCareContracts(t, db, organization)
		return organization
	}
	own, other := traeger("Träger Kündigung"), traeger("Träger Kündigung Fremd")
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Kündigung OGS")

	var sent, active int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status, sent_at)
		VALUES (?, ?, 31, 'versendet', NOW()) RETURNING id`, own, school).Scan(ctx, &sent))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status, sent_at)
		VALUES (?, ?, 32, 'versendet', NOW()) RETURNING id`, own, school).Scan(ctx, &active))

	as := func(organization int64, statements ...string) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(organization)); err != nil {
				return err
			}
			for _, statement := range statements {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return err
				}
			}
			return nil
		})
	}
	require.NoError(t, as(own, fmt.Sprintf(`UPDATE care_contract.contracts SET status = 'aktiv', concluded_at = NOW(),
		conclusion_method = 'mit_anmeldung' WHERE id = %d`, active)))
	var informed bool
	require.NoError(t, db.NewRaw(`SELECT withdrawal_informed_at IS NULL FROM care_contract.contracts WHERE id = ?`, active).Scan(ctx, &informed))
	assert.True(t, informed, "conclusion alone proves no notice provision")

	assert.ErrorContains(t, as(own, fmt.Sprintf(`UPDATE care_contract.contracts SET status = 'gekuendigt' WHERE id = %d`, active)),
		"chk_contracts_terminated", "a terminated contract names its last day")
	assert.ErrorContains(t, as(own, fmt.Sprintf(`UPDATE care_contract.contracts SET status = 'widerrufen' WHERE id = %d`, sent)),
		"chk_contracts_notice_concluded", "only a concluded contract is withdrawn")
	assert.ErrorContains(t, as(own, fmt.Sprintf(`UPDATE care_contract.contracts SET status = 'aktiv', ends_on = '2026-12-31' WHERE id = %d`, active)),
		"chk_contracts_terminated", "an active contract has no last day")

	notice := func(organization int64, kind, noticeKind, endsOn string) string {
		return fmt.Sprintf(`INSERT INTO care_contract.contract_notices (organization_id, contract_id, kind, notice_kind, channel,
			declared_at, received_on, ends_on, recipient_email, confirmation_text)
			VALUES (%d, %d, '%s', %s, 'traeger_buero', NOW(), '2026-11-10', %s, 'erika@example.org', 'Bestätigung')`,
			organization, active, kind, noticeKind, endsOn)
	}
	assert.ErrorContains(t, as(own, notice(own, "kuendigung", "NULL", "'2026-12-31'")), "chk_contract_notices_notice_kind",
		"a termination names its kind")
	assert.ErrorContains(t, as(own, notice(own, "widerruf", "NULL", "'2026-12-31'")), "chk_contract_notices_notice_kind",
		"a withdrawal has no last day")
	assert.Error(t, as(other, notice(own, "widerruf", "NULL", "NULL")), "another Träger writes no declaration here")
	require.NoError(t, as(own, notice(own, "kuendigung", "'ordentlich'", "'2026-12-31'"),
		fmt.Sprintf(`UPDATE care_contract.contracts SET status = 'gekuendigt', ends_on = '2026-12-31' WHERE id = %d`, active)))

	count := func(organization int64) int {
		var visible int
		require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(organization)); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_notices`).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 1, count(own))
	assert.Zero(t, count(other), "RLS keeps the declarations with their Träger")
}

func TestParentContractNoticeSurvivesHardAccountDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger aufbewahrte Erklärung", Slug: fmt.Sprintf("notice-retained-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("notice-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Erklärung")
	var contract, notice int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id)
  VALUES (?, ?, ?) RETURNING id`, organization, school, testpkg.UniqueTestTenantID(t)).Scan(ctx, &contract))
	insert := `INSERT INTO care_contract.contract_notices (organization_id, contract_id, kind, channel, account_id,
  declared_at, received_on, recipient_email, confirmation_text) VALUES (?, ?, 'widerruf', 'eltern_app', ?, NOW(), '2026-11-07', 'eltern@example.org', 'Bestätigung') RETURNING id`
	assert.Error(t, db.NewRaw(insert, organization, contract, nil).Scan(ctx, &notice), "new parent declarations require attribution")
	require.NoError(t, db.NewRaw(insert, organization, contract, account.ID).Scan(ctx, &notice))
	_, err := db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	require.NoError(t, err, "account deletion must not delete or invalidate the retained declaration")
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT account_id IS NULL AND confirmation_text = 'Bestätigung' FROM care_contract.contract_notices WHERE id = ?`, notice).Scan(ctx, &retained))
	assert.True(t, retained)
}
