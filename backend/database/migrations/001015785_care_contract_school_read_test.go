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

// TestCareContractSchoolReadIsReadOnly pins migration 1.15.785 (#4683, ADR
// 0156): a school reads the booked care of its sent contracts but changes
// nothing; every write stays with the Träger policy.
func TestCareContractSchoolReadIsReadOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-schule-%d", testpkg.UniqueTestTenantID(t))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Schule", Slug: slug}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Träger Schule OGS")
	contract := testpkg.CreateTestCareContract(t, db, organization, school, 77, "versendet", "2027-08-01",
		testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "14:00"})

	asSchool := func(statement string, args ...any) (int64, error) {
		var affected int64
		err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(school)); err != nil {
				return err
			}
			result, err := tx.ExecContext(ctx, statement, args...)
			if err == nil {
				affected, _ = result.RowsAffected()
			}
			return err
		})
		return affected, err
	}
	for _, statement := range []string{
		`UPDATE care_contract.contracts SET status = 'beendet' WHERE id = ?`,
		`DELETE FROM care_contract.contracts WHERE id = ?`,
	} {
		affected, err := asSchool(statement, contract)
		if err == nil {
			assert.Zero(t, affected, "the school changes no contract: %s", statement)
		}
	}
	_, err := asSchool(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id) VALUES (?, ?, 78)`,
		organization, school)
	assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a school adds no contract")
	var status string
	require.NoError(t, db.NewRaw(`SELECT status FROM care_contract.contracts WHERE id = ?`, contract).Scan(ctx, &status))
	assert.Equal(t, "versendet", status)
}
