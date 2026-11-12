package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestCareContractConclusionIsGuardedByTheDatabase pins the conclusion of
// #4680 under the request role: a contract is active only with the instant
// and method of its conclusion, both come together and only with a known
// method, a version moves only along ADR 0088, and the trail knows the
// action conclude.
func TestCareContractConclusionIsGuardedByTheDatabase(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-abschluss-%d", testpkg.UniqueTestTenantID(t))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Abschluss", Slug: slug}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Träger Abschluss OGS")

	var contract, version int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status, sent_at)
		VALUES (?, ?, 21, 'versendet', NOW()) RETURNING id`, organization, school).Scan(ctx, &contract))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, '2027-08-01', 'Lina', 'Muster', 'OGS', 'open_ended', 1, 'month_end', false, '', false, '', '', '')
		RETURNING id`, organization, contract).Scan(ctx, &version))

	asTraeger := func(statement string, args ...any) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(organization)); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, statement, args...)
			return err
		})
	}

	assert.ErrorContains(t, asTraeger(`UPDATE care_contract.contracts SET status = 'aktiv' WHERE id = ?`, contract),
		"chk_contracts_active_concluded", "no active contract without its conclusion")
	assert.ErrorContains(t, asTraeger(`UPDATE care_contract.contracts SET concluded_at = NOW() WHERE id = ?`, contract),
		"chk_contracts_concluded", "the instant needs its method")
	assert.ErrorContains(t, asTraeger(`UPDATE care_contract.contracts SET concluded_at = NOW(), conclusion_method = 'handschlag'
		WHERE id = ?`, contract), "chk_contracts_conclusion_method", "only the methods of ADR 0088 section 3")
	require.NoError(t, asTraeger(`UPDATE care_contract.contracts SET status = 'aktiv', concluded_at = NOW(),
		conclusion_method = 'mit_anmeldung' WHERE id = ?`, contract), "the request role concludes a contract")

	assert.ErrorContains(t, asTraeger(`UPDATE care_contract.contract_versions SET status = 'abgeloest' WHERE id = ?`, version),
		"cannot move from beantragt to abgeloest", "a requested version is not superseded")
	require.NoError(t, asTraeger(`UPDATE care_contract.contract_versions SET status = 'wirksam' WHERE id = ?`, version),
		"the request role makes a requested version effective")
	assert.ErrorContains(t, asTraeger(`UPDATE care_contract.contract_versions SET status = 'beantragt' WHERE id = ?`, version),
		"cannot move from wirksam to beantragt", "an effective version never goes back")
	assert.Equal(t, "42501", testpkg.PostgresSQLState(asTraeger(`UPDATE care_contract.contract_versions SET child_first_name = 'Ben' WHERE id = ?`, version)), "the content of a version stays frozen")

	require.NoError(t, asTraeger(`INSERT INTO care_contract.contract_events (organization_id, contract_id, school_id, action)
		VALUES (?, ?, ?, 'conclude')`, organization, contract, school), "the trail records the conclusion")
}
