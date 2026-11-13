package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestCareContractVersionsFormOneTimeline pins the invariants of #4681 on
// the versions of a contract under the request role: the effective versions
// never overlap (one wirksam version, the newest; no shared start), a change
// may go in front of a planned version, a version replaced on its first day
// becomes ersetzt and never holds again, and a follow-up version never takes
// effect in the past.
func TestCareContractVersionsFormOneTimeline(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Vertragsversion", Slug: fmt.Sprintf("traeger-version-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Vertragsversion OGS")

	var contract, first int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id,
			status, sent_at, concluded_at, conclusion_method) VALUES (?, ?, 21, 'aktiv', NOW(), NOW(), 'mit_anmeldung') RETURNING id`,
		organization, school).Scan(ctx, &contract))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			status, effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, 'wirksam', '2026-08-01', 'Lina', 'Muster', 'OGS', 'open_ended', 1, 'month_end', false, '', false, '', '', '')
		RETURNING id`, organization, contract).Scan(ctx, &first))

	asTraeger := func(statements ...string) error {
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
	version := func(number int, status, from string) string {
		return fmt.Sprintf(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			status, effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner, source)
			VALUES (%d, %d, %d, '%s', '%s', 'Lina', 'Muster', 'OGS', 'open_ended', 1, 'month_end', false, '', false, '', '', '',
				'buchungsaenderung')`, organization, contract, number, status, from)
	}
	supersede := fmt.Sprintf(`UPDATE care_contract.contract_versions SET status = 'abgeloest' WHERE id = %d`, first)

	require.ErrorContains(t, asTraeger(version(2, "wirksam", "2099-03-01")), "uq_contract_versions_effective",
		"two wirksam versions at once: the predecessor must be superseded first")
	require.ErrorContains(t, asTraeger(supersede, version(2, "wirksam", "2026-08-01")), "must start after every effective version",
		"a version sharing the start of an effective one would overlap it")
	require.ErrorContains(t, asTraeger(supersede, version(2, "wirksam", "2026-07-01")), "must start after every effective version",
		"a version starting before an effective one would cut into the timeline")
	require.ErrorContains(t, asTraeger(supersede, version(2, "wirksam", "2026-09-01")), "cannot take effect in the past",
		"a follow-up version never takes effect in the past")
	require.NoError(t, asTraeger(supersede, version(2, "wirksam", "2099-03-01")),
		"a superseded predecessor and a later start make a valid timeline")
	require.NoError(t, asTraeger(version(4, "abgeloest", "2099-02-01")),
		"a change before the planned version goes in front of it as superseded")
	require.ErrorContains(t, asTraeger(version(5, "abgeloest", "2099-05-01")), "needs a later effective version",
		"a superseded version without a later one would leave two newest versions")
	require.ErrorContains(t, asTraeger(version(5, "abgeloest", "2026-07-15")), "cannot take effect in the past",
		"a version before the first one is fine only ahead of today")
	require.ErrorContains(t, asTraeger(version(5, "abgeloest", "2099-02-01")), "uq_contract_versions_timeline",
		"two versions never start on the same day")
	replace := fmt.Sprintf(`UPDATE care_contract.contract_versions SET status = 'ersetzt' WHERE contract_id = %d AND version_number = 4`, contract)
	require.NoError(t, asTraeger(replace, version(6, "abgeloest", "2099-02-01")),
		"a change on the first day of a version replaces it: the old one becomes ersetzt, the new one takes its day")
	require.ErrorContains(t, asTraeger(fmt.Sprintf(`UPDATE care_contract.contract_versions SET status = 'wirksam'
		WHERE contract_id = %d AND version_number = 4`, contract)), "cannot move from ersetzt",
		"a replaced version never holds again")
	require.ErrorContains(t, asTraeger(version(3, "beantragt", "2099-04-01"),
		fmt.Sprintf(`UPDATE care_contract.contract_versions SET status = 'wirksam' WHERE contract_id = %d AND version_number = 3`, contract)), "uq_contract_versions_effective",
		"a requested version becomes effective only after its predecessor is superseded")
}
