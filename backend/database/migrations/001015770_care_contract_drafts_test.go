package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestCareContractsStayWithTheirTraeger pins ADR 0153 on the tables of the
// Betreuungsvertrag: under the request role a Träger reads only its own
// contracts, creates a contract only as a draft, never changes a version,
// a document or a send, and removes only a draft; a school transaction sees
// nothing.
func TestCareContractsStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-vertrag-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerCareContracts(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Vertrag Eigen")
	foreign, _ := newTraeger("Träger Vertrag Fremd")

	insertContract := func(child int64, status string) (int64, int64) {
		var contract, version int64
		require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id,
				status, sent_at) VALUES (?, ?, ?, ?, CASE WHEN ? = 'entwurf' THEN NULL ELSE NOW() END) RETURNING id`,
			own, ownSchool, child, status, status).Scan(ctx, &contract))
		require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
				effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
				special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
			VALUES (?, ?, 1, '2027-08-01', 'Lina', 'Muster', 'OGS', 'open_ended', 1, 'month_end', false, '', false, '', '', '')
			RETURNING id`, own, contract).Scan(ctx, &version))
		_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
				weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, 'mon', 7, 'Bis 14 Uhr', '14:00'), (?, ?, 'tue', 8, 'Bis 16:30 Uhr', '16:30')`,
			own, version, own, version)
		require.NoError(t, err)
		return contract, version
	}
	draft, draftVersion := insertContract(11, "entwurf")
	sent, sentVersion := insertContract(12, "versendet")
	var document int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_documents (organization_id, contract_id, contract_version_id,
			template_version, filename, pdf) VALUES (?, ?, ?, 0, 'betreuungsvertrag.pdf', ?) RETURNING id`,
		own, sent, sentVersion, []byte("%PDF-1.4 Vertrag")).Scan(ctx, &document))

	asRequestRole := func(setting, value, statement string, args ...any) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, statement, args...)
			return err
		})
	}
	ownOrg := fmt.Sprint(own)

	count := func(setting, value, table string) int {
		var visible int
		require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE organization_id = ?`, own).Scan(ctx, &visible)
		}))
		return visible
	}
	// The school reads the booked care of its sent contract, never the draft
	// and never a document (#4683, migration 1.15.760).
	schoolReads := map[string]int{"care_contract.contracts": 1, "care_contract.contract_versions": 1,
		"care_contract.contract_version_services": 2, "care_contract.contract_documents": 0}
	for _, table := range []string{"care_contract.contracts", "care_contract.contract_versions", "care_contract.contract_version_services", "care_contract.contract_documents"} {
		assert.Positive(t, count("app.current_org_id", ownOrg, table), "the Träger reads its own %s", table)
		assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign), table), "another Träger reads none of %s", table)
		assert.Equal(t, schoolReads[table], count("app.current_tenant_id", fmt.Sprint(ownSchool), table),
			"a school transaction reads only the sent contract of %s", table)
		assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(ownSchool+1_000_000), table), "another school reads none of %s", table)
	}

	assert.ErrorContains(t, asRequestRole("app.current_org_id", ownOrg, `INSERT INTO care_contract.contracts
			(organization_id, school_id, request_child_id, status, sent_at) VALUES (?, ?, 13, 'versendet', NOW())`,
		own, ownSchool), "row-level security", "a contract starts as a draft")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", ownOrg, `INSERT INTO care_contract.contracts
			(organization_id, school_id, request_child_id) VALUES (?, ?, 11)`,
		own, ownSchool), "uq_contracts_application", "one contract per application child")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", fmt.Sprint(foreign), `INSERT INTO care_contract.contracts
			(organization_id, school_id, request_child_id) VALUES (?, ?, 14)`,
		own, ownSchool), "row-level security", "another Träger adds no contract to this Träger")

	for _, statement := range []string{
		`UPDATE care_contract.contract_versions SET child_first_name = 'Ben' WHERE organization_id = ?`,
		`DELETE FROM care_contract.contract_versions WHERE organization_id = ?`,
		`UPDATE care_contract.contract_version_services SET ends_at = '18:00' WHERE organization_id = ?`,
		`DELETE FROM care_contract.contract_version_services WHERE organization_id = ?`,
		`UPDATE care_contract.contract_documents SET filename = 'anders.pdf' WHERE organization_id = ?`,
		`UPDATE care_contract.contracts SET school_id = 1 WHERE organization_id = ?`,
	} {
		assert.ErrorContains(t, asRequestRole("app.current_org_id", ownOrg, statement, own), "permission denied",
			"the request role changes no frozen row: %s", statement)
	}

	// Only a draft goes; a sent contract stays for every role but the owner's.
	require.NoError(t, asRequestRole("app.current_org_id", ownOrg, `DELETE FROM care_contract.contracts WHERE organization_id = ? AND id = ?`, own, sent))
	require.NoError(t, asRequestRole("app.current_org_id", ownOrg, `DELETE FROM care_contract.contracts WHERE organization_id = ? AND id = ?`, own, draft))
	var left []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM care_contract.contracts WHERE organization_id = ? ORDER BY id`, own).Scan(ctx, &left))
	assert.Equal(t, []int64{sent}, left, "the draft went, the sent contract stayed")
	var draftServices int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_version_services WHERE contract_version_id = ?`,
		draftVersion).Scan(ctx, &draftServices))
	assert.Zero(t, draftServices, "the services of a discarded draft go with it")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", ownOrg, `INSERT INTO care_contract.contract_version_services
			(organization_id, contract_version_id, weekday, care_offering_id, module_name) VALUES (?, ?, 'xyz', 7, 'Bis 14 Uhr')`,
		own, sentVersion), "chk_contract_version_services_weekday", "a service names a weekday")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", ownOrg, `UPDATE care_contract.contracts SET status = 'gekuendigt' WHERE id = ?`, sent),
		"chk_contracts_status", "a contract takes only the states of ADR 0088")
}
