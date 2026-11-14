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

// TestTraegerStaffRecordsStayWithTheirTraeger pins the policies of the
// Personalstamm (#4705, ADR 0089 section 3): under the request role a Träger
// reads and writes only its own records, links and trail; another Träger sees
// none of them; a school sees only the links of its own employments and no
// record; and a record exists only while its Träger holds the release.
func TestTraegerStaffRecordsStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) int64 {
		slug := fmt.Sprintf("traeger-personal-%d", testpkg.UniqueTestTenantID(t))
		return testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
	}
	own, other := newTraeger("Träger Personal Eigen"), newTraeger("Träger Personal Fremd")
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Grundschule Personal A")
	neighbour := testpkg.CreateTestSchoolInOrganization(t, db, own, "Grundschule Personal B")

	as := func(setting, value string, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	asTraeger := func(organization int64, fn func(bun.Tx) error) error {
		return as("app.current_org_id", fmt.Sprint(organization), fn)
	}
	asSchool := func(schoolID int64, fn func(bun.Tx) error) error {
		return as("app.current_tenant_id", fmt.Sprint(schoolID), fn)
	}
	insertRecord := func(organization, forTraeger int64, number string) (int64, error) {
		var id int64
		err := asTraeger(organization, func(tx bun.Tx) error {
			return tx.NewRaw(`INSERT INTO workforce.staff_records (organization_id, first_name, last_name, personnel_number)
				VALUES (?, 'Mira', 'Sommer', ?) RETURNING id`, forTraeger, number).Scan(ctx, &id)
		})
		return id, err
	}
	count := func(run func(func(bun.Tx) error) error, table string) int {
		var n int
		require.NoError(t, run(func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM `+table).Scan(ctx, &n)
		}))
		return n
	}

	_, err := insertRecord(own, own, "70001")
	require.Error(t, err, "without the release the Träger keeps no record")

	_, err = db.ExecContext(ctx, `INSERT INTO workforce.staff_releases (organization_id, representation, released_by_operator_id) VALUES (?, 'agreement', 1)`, own)
	require.NoError(t, err)
	var releases int
	require.NoError(t, asTraeger(own, func(tx bun.Tx) error {
		return tx.NewRaw(`SELECT COUNT(*) FROM workforce.staff_releases`).Scan(ctx, &releases)
	}))
	assert.Equal(t, 1, releases, "the Träger reads its own release")
	assert.Error(t, asTraeger(own, func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM workforce.staff_releases WHERE organization_id = ?`, own)
		return err
	}), "only the moto team withdraws a release")

	record, err := insertRecord(own, own, "70001")
	require.NoError(t, err, "the released Träger keeps a record")
	_, err = insertRecord(own, own, "70001")
	require.Error(t, err, "the personnel number is unique per Träger")
	_, err = insertRecord(other, own, "70002")
	require.Error(t, err, "another Träger cannot write a record in this Träger's name")

	require.NoError(t, asTraeger(own, func(tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workforce.staff_record_links (organization_id, staff_record_id, school_id, membership_id, linked_by_account_id)
			VALUES (?, ?, ?, 101, 1), (?, ?, ?, 202, 1)`, own, record, school, own, record, neighbour); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO workforce.staff_record_changes (organization_id, staff_record_id, kind, field, value_text, school_id, account_id)
			VALUES (?, ?, 'not_chosen', 'last_name', 'Sommer-Berg', ?, 1)`, own, record, neighbour)
		return err
	}))
	assert.Error(t, asTraeger(own, func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE workforce.staff_record_changes SET value_text = 'x' WHERE staff_record_id = ?`, record)
		return err
	}), "the trail is append-only")

	inOwn := func(fn func(bun.Tx) error) error { return asTraeger(own, fn) }
	inOther := func(fn func(bun.Tx) error) error { return asTraeger(other, fn) }
	inSchool := func(fn func(bun.Tx) error) error { return asSchool(school, fn) }
	for _, table := range []string{"workforce.staff_records", "workforce.staff_record_links", "workforce.staff_record_changes"} {
		assert.NotZero(t, count(inOwn, table), "the Träger reads its own %s", table)
		assert.Zero(t, count(inOther, table), "another Träger sees no %s", table)
	}
	assert.Zero(t, count(inSchool, "workforce.staff_records"), "a school reads no record")
	assert.Zero(t, count(inSchool, "workforce.staff_record_changes"), "a school reads no trail")
	assert.Zero(t, count(inSchool, "workforce.staff_releases"), "a school reads no release")
	var schools []int64
	require.NoError(t, inSchool(func(tx bun.Tx) error {
		return tx.NewRaw(`SELECT school_id FROM workforce.staff_record_links`).Scan(ctx, &schools)
	}))
	assert.Equal(t, []int64{school}, schools, "a school reads only the links of its own employments")
	assert.Error(t, inSchool(func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO workforce.staff_record_links (organization_id, staff_record_id, school_id, membership_id, linked_by_account_id)
			VALUES (?, ?, ?, 303, 1)`, own, record, school)
		return err
	}), "a school never writes a link")

	_, err = db.ExecContext(ctx, `DELETE FROM workforce.staff_releases WHERE organization_id = ?`, own)
	require.Error(t, err, "the release is not withdrawn while the Träger keeps records")
}
