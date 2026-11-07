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

// TestCareContractNoticeEndDecisionKeepsItsShape pins 1.15.800 (#4682)
// under the request role: only an extraordinary termination carries a
// decision, a decided end has its day and a decided day has its decision,
// the notice is no blank text, the request role may write the decision, and
// the trail knows the refusal of the reason.
func TestCareContractNoticeEndDecisionKeepsItsShape(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Entscheidung", Slug: fmt.Sprintf("traeger-entscheidung-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Entscheidung OGS")
	as := func(statements ...string) error {
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
	contract := func(child int) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status, sent_at, concluded_at, conclusion_method)
			VALUES (?, ?, ?, 'gekuendigt', NOW(), NOW(), 'mit_anmeldung') RETURNING id`, organization, school, child).Scan(ctx, &id))
		return id
	}
	notice := func(contractID int64, noticeKind string, reason any) int64 {
		var id int64
		endsOn := "NULL"
		if noticeKind == "ordentlich" {
			endsOn = "'2026-12-31'"
		}
		require.NoError(t, db.NewRaw(fmt.Sprintf(`INSERT INTO care_contract.contract_notices (organization_id, contract_id, kind, notice_kind,
			channel, declared_at, received_on, ends_on, reason, recipient_email, confirmation_text)
			VALUES (?, ?, 'kuendigung', ?, 'traeger_buero', NOW(), '2026-11-10', %s, ?, 'erika@example.org', 'Bestätigung') RETURNING id`, endsOn),
			organization, contractID, noticeKind, reason).Scan(ctx, &id))
		return id
	}
	extraordinary := notice(contract(41), "ausserordentlich", "Umzug")
	ordinary := notice(contract(42), "ordentlich", nil)

	decide := func(id int64, set string) string {
		return fmt.Sprintf(`UPDATE care_contract.contract_notices SET %s WHERE id = %d`, set, id)
	}
	assert.ErrorContains(t, as(decide(extraordinary, `end_decision = 'umgedeutet'`)), "chk_contract_notices_end_decision",
		"a decision has its day")
	assert.ErrorContains(t, as(decide(extraordinary, `ends_on = '2026-12-31'`)), "chk_contract_notices_end_decision",
		"an end of an extraordinary termination has its decision")
	assert.ErrorContains(t, as(decide(extraordinary, `ends_on = '2026-12-31', end_decision = 'abgelehnt'`)), "chk_contract_notices_end_decision",
		"only the two decisions")
	assert.ErrorContains(t, as(decide(ordinary, `end_decision = 'festgelegt'`)), "chk_contract_notices_end_decision",
		"an ordinary termination has no decision")
	assert.ErrorContains(t, as(decide(extraordinary, `ends_on = '2026-12-31', end_decision = 'umgedeutet', end_notice_text = ' '`)),
		"chk_contract_notices_end_notice_text", "the notice is no blank text")
	require.NoError(t, as(decide(extraordinary, `ends_on = '2026-12-31', end_decision = 'umgedeutet', end_decided_at = NOW(),
		end_notice_text = 'Ihre Kündigung gilt als ordentliche Kündigung.', end_notice_outbox_id = 7`)),
		"the request role writes the decision")
	require.NoError(t, as(fmt.Sprintf(`INSERT INTO care_contract.contract_events (organization_id, contract_id, school_id, action)
		SELECT organization_id, contract_id, %d, 'reject_reason' FROM care_contract.contract_notices WHERE id = %d`, school, extraordinary)),
		"the trail records the refusal of the reason")
}
