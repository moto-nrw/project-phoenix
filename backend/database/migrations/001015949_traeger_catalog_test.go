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
	"github.com/uptrace/bun/dialect/pgdialect"
)

type catalogTraeger struct {
	organization int64
	school       int64
}

func newCatalogTraeger(t *testing.T, db *bun.DB, name string) catalogTraeger {
	t.Helper()
	slug := fmt.Sprintf("traeger-katalog-%d", testpkg.UniqueTestTenantID(t))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
	testpkg.OwnTraegerCatalog(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	testpkg.OwnTenantRows(t, db, school)
	return catalogTraeger{organization: organization, school: school}
}

func asCatalogRequestRole(ctx context.Context, db *bun.DB, setting, value string, fn func(bun.Tx) error) error {
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

// TestTraegerCatalogFollowsTheTraegerPolicy pins ADR 0195 section 2 on the six
// Träger tables: a Träger transaction sees and writes only its own rows, a
// school transaction sees none, only an undeleted school of the row's Träger
// is selected, and a released version never changes.
func TestTraegerCatalogFollowsTheTraegerPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	own := newCatalogTraeger(t, db, "Katalog Eigen")
	foreign := newCatalogTraeger(t, db, "Katalog Fremd")

	var phase, offering int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.traeger_phases (organization_id, draft) VALUES (?, '{}') RETURNING id`,
		own.organization).Scan(ctx, &phase))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.traeger_care_offerings (organization_id, traeger_phase_id, draft)
		VALUES (?, ?, '{}') RETURNING id`, own.organization, phase).Scan(ctx, &offering))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.traeger_phase_schools (organization_id, traeger_phase_id, school_id)
		VALUES (?, ?, ?)`, own.organization, phase, own.school)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.traeger_phase_schools (organization_id, traeger_phase_id, school_id)
		VALUES (?, ?, ?)`, own.organization, phase, foreign.school)
	assert.ErrorContains(t, err, "is not a school of Träger", "a foreign school is never selected")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.traeger_care_offering_schools (organization_id, traeger_care_offering_id, school_id)
		VALUES (?, ?, ?)`, own.organization, offering, foreign.school)
	assert.ErrorContains(t, err, "is not a school of Träger", "a foreign school gets no offering")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.traeger_care_offerings (organization_id, traeger_phase_id, draft)
		VALUES (?, ?, '{}')`, foreign.organization, phase)
	assert.Error(t, err, "an offering cannot join the phase of another Träger")

	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.traeger_phase_versions (organization_id, traeger_phase_id, version, fields)
		VALUES (?, ?, 1, '{"name":"Schuljahr"}')`, own.organization, phase)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE enrollment.traeger_phase_versions SET fields = '{}' WHERE traeger_phase_id = ?`, phase)
	assert.ErrorContains(t, err, "never changes", "a released version is immutable")

	count := func(setting, value string) int {
		var visible int
		require.NoError(t, asCatalogRequestRole(ctx, db, setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT
				(SELECT COUNT(*) FROM enrollment.traeger_phases WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM enrollment.traeger_phase_versions WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM enrollment.traeger_phase_schools WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM enrollment.traeger_care_offerings WHERE organization_id = ?)`,
				own.organization, own.organization, own.organization, own.organization).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 4, count("app.current_org_id", fmt.Sprint(own.organization)), "the Träger sees its rows")
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign.organization)), "another Träger sees nothing")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(own.school)), "a school transaction sees nothing of its Träger's catalog")

	err = asCatalogRequestRole(ctx, db, "app.current_org_id", fmt.Sprint(foreign.organization), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.traeger_phases (organization_id, draft) VALUES (?, '{}')`, own.organization)
		return err
	})
	assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a Träger cannot write a phase for another Träger")
}

// TestTraegerCopyTracksDeviationsAndDetachesOnTraegerChange pins ADR 0195
// sections 4 and 8 on the copy columns: every school write recomputes the
// deviations from the row, a change back to the Träger's value removes it,
// a second copy of the same source is refused, and a school that changes its
// Träger keeps its copies as ordinary rows.
func TestTraegerCopyTracksDeviationsAndDetachesOnTraegerChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	own := newCatalogTraeger(t, db, "Kopie Eigen")
	other := newCatalogTraeger(t, db, "Kopie Neu")

	var phase, offering int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.phases (tenant_id, name, kind, service_start_date, service_end_date,
			care_overflow_mode, care_offering_selection_mode, is_active, available_school_classes, eligible_school_classes,
			eligible_grade_levels, translations, traeger_source_id, traeger_source_version, traeger_values)
		VALUES (?, 'Schuljahr', 'school_year', '2027-08-01', '2028-07-31', 'waitlist', 'optional', true, '[]', '[]', '[]', '{}',
			41, 1, '{"name":"Schuljahr","service_start_date":"2027-08-01","is_active":true}') RETURNING id`, own.school).Scan(ctx, &phase))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.care_offerings (tenant_id, phase_id, name, days_of_week_mode, available_days,
			auto_add_grade_levels, is_active, counts_as_care, capacity, traeger_source_id, traeger_source_version, traeger_values)
		VALUES (?, ?, 'Frühbetreuung', 'fixed', '["mon","tue"]', '[]', true, true, 20, 42, 1,
			'{"name":"Frühbetreuung","capacity":20,"is_active":true,"available_days":["mon","tue"]}') RETURNING id`,
		own.school, phase).Scan(ctx, &offering))

	deviations := func(table string, id int64) []string {
		var fields []string
		require.NoError(t, db.NewRaw(`SELECT traeger_deviations FROM `+table+` WHERE id = ?`, id).Scan(ctx, pgdialect.Array(&fields)))
		return fields
	}
	assert.Empty(t, deviations("enrollment.care_offerings", offering), "a fresh copy carries the Träger's values")

	_, err := db.ExecContext(ctx, `UPDATE enrollment.care_offerings SET capacity = 25, is_active = false WHERE id = ?`, offering)
	require.NoError(t, err)
	assert.Equal(t, []string{"capacity", "is_active"}, deviations("enrollment.care_offerings", offering),
		"a school change and a deactivation both count as deviations")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.care_offerings SET capacity = 20 WHERE id = ?`, offering)
	require.NoError(t, err)
	assert.Equal(t, []string{"is_active"}, deviations("enrollment.care_offerings", offering),
		"a change back to the Träger's value removes the deviation")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.phases SET service_start_date = '2027-08-15' WHERE id = ?`, phase)
	require.NoError(t, err)
	assert.Equal(t, []string{"service_start_date"}, deviations("enrollment.phases", phase), "a date counts in its canonical form")

	_, err = db.ExecContext(ctx, `UPDATE enrollment.care_offerings SET traeger_values = '{"tenant_id":1}' WHERE id = ?`, offering)
	assert.Error(t, err, "traeger_values carries only the Träger's fields")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.care_offerings (tenant_id, phase_id, name, days_of_week_mode, available_days,
			auto_add_grade_levels, is_active, counts_as_care, traeger_source_id, traeger_source_version, traeger_values)
		VALUES (?, ?, 'Doppelt', 'fixed', '["mon"]', '[]', true, true, 42, 1, '{}')`, own.school, phase)
	assert.Equal(t, "23505", testpkg.PostgresSQLState(err), "a second copy of the same source is refused")

	_, err = db.ExecContext(ctx, `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, other.organization, own.school)
	require.NoError(t, err)
	var linked int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT COUNT(*) FROM enrollment.phases WHERE tenant_id = ? AND (traeger_source_id IS NOT NULL OR traeger_values IS NOT NULL OR traeger_deviations <> '{}'))
		+ (SELECT COUNT(*) FROM enrollment.care_offerings WHERE tenant_id = ? AND (traeger_source_id IS NOT NULL OR traeger_values IS NOT NULL OR traeger_deviations <> '{}'))`,
		own.school, own.school).Scan(ctx, &linked))
	assert.Zero(t, linked, "a school that changes its Träger keeps its copies as ordinary rows")
	var active bool
	require.NoError(t, db.NewRaw(`SELECT is_active FROM enrollment.care_offerings WHERE id = ?`, offering).Scan(ctx, &active))
	assert.False(t, active, "the school's own values stay")
}
