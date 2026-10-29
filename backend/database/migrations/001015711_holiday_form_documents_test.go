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

func TestHolidayLegalMigrationPreservesExistingPeriodsAndIsolatesPDFs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger PDF Migration", Slug: fmt.Sprintf("pdf-%d", testpkg.UniqueTestTenantID(t))}).ID
	foreign := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger PDF Fremd", Slug: fmt.Sprintf("pdf-fremd-%d", testpkg.UniqueTestTenantID(t))}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, own)
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "PDF OGS")
	require.NoError(t, holidayFormDocumentsDown(ctx, db))
	var period, registration int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods(organization_id,name,registration_opens_on,registration_closes_on,status,published_at)
 VALUES (?,'Bestand','2020-01-01','2099-12-31','published',NOW()) RETURNING id`, own).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations(organization_id,holiday_period_id,guardian_first_name,guardian_last_name,guardian_email,guardian_phone,privacy_consent_at)
 VALUES (?,?,'Eva','Muster','pdf@example.org','0123',NOW()) RETURNING id`, own, period).Scan(ctx, &registration))
	require.NoError(t, holidayFormDocumentsUp(ctx, db))
	var blocks, accepted string
	require.NoError(t, db.NewRaw(`SELECT legal_blocks::text FROM enrollment.holiday_period_forms WHERE holiday_period_id=?`, period).Scan(ctx, &blocks))
	require.NoError(t, db.NewRaw(`SELECT legal_acceptances::text FROM enrollment.holiday_registrations WHERE id=?`, registration).Scan(ctx, &accepted))
	assert.JSONEq(t, `[]`, blocks)
	assert.JSONEq(t, `{}`, accepted)
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_form_documents(organization_id,holiday_period_id,filename,content,size) VALUES (?,?,'Hinweis.pdf',convert_to('%PDF-1.4','UTF8'),8)`, own, period)
	require.NoError(t, err)
	asRole := func(setting string, value int64, fn func(context.Context, bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?,?,true)`, setting, fmt.Sprint(value)); err != nil {
				return err
			}
			return fn(ctx, tx)
		})
	}
	for _, tc := range []struct {
		setting string
		value   int64
		want    int
	}{
		{"app.current_org_id", own, 1}, {"app.current_org_id", foreign, 0}, {"app.current_tenant_id", school, 0},
	} {
		var count int
		require.NoError(t, asRole(tc.setting, tc.value, func(ctx context.Context, tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_form_documents WHERE holiday_period_id=?`, period).Scan(ctx, &count)
		}))
		assert.Equal(t, tc.want, count, "scope %s=%d", tc.setting, tc.value)
	}
	err = asRole("app.current_org_id", own, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE enrollment.holiday_form_documents SET filename='neu.pdf' WHERE holiday_period_id=?`, period)
		return err
	})
	assert.ErrorContains(t, err, "permission denied", "PDFs are immutable under the request role")
	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_periods WHERE id=?`, period)
	require.NoError(t, err)
	var count int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_form_documents WHERE holiday_period_id=?`, period).Scan(ctx, &count))
	assert.Zero(t, count, "PDFs go with their period, independently of registration retention")
}
