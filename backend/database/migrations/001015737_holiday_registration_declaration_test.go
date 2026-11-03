package migrations

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestHolidayRegistrationDeclarationKeepsEarlierRegistrations runs the
// migration of #5626 over registrations written before it: they keep every
// value and their consent version and get no custody and no name. A
// registration under version 3 must carry both. Down and Up again keep the
// registrations.
func TestHolidayRegistrationDeclarationKeepsEarlierRegistrations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, holidayRegistrationDeclarationDown(ctx, db))
	require.False(t, holidayColumnExists(t, db, "holiday_registrations", "declaration_name"))

	f := newHolidaySharedAnswersFixture(t, db)
	f.insertVersion(t, f.own, f.period, 2, `[]`)
	insert := func(consentVersion any) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
				guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, health_consent_at, status, form_version, consent_version)
			VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), NOW(), 'confirmed', 2, ?) RETURNING id`,
			f.own, f.period, consentVersion).Scan(ctx, &id))
		return id
	}
	legacy, hostHealth := insert(nil), insert(2)

	require.NoError(t, holidayRegistrationDeclarationUp(ctx, db))

	type row struct {
		Status         string  `bun:"status"`
		ConsentVersion *int    `bun:"consent_version"`
		Health         bool    `bun:"health"`
		Custody        bool    `bun:"custody"`
		Name           *string `bun:"declaration_name"`
	}
	read := func(id int64) row {
		var r row
		require.NoError(t, db.NewRaw(`SELECT status, consent_version, health_consent_at IS NOT NULL AS health,
				custody_confirmed_at IS NOT NULL AS custody, declaration_name
			FROM enrollment.holiday_registrations WHERE id = ?`, id).Scan(ctx, &r))
		return r
	}
	assert.Equal(t, row{Status: "confirmed", Health: true}, read(legacy), "a registration before #4722 stays as it was")
	version := 2
	assert.Equal(t, row{Status: "confirmed", ConsentVersion: &version, Health: true}, read(hostHealth),
		"a registration under version 2 keeps its version and gets no declaration")

	require.NoError(t, holidayRegistrationDeclarationDown(ctx, db))
	assert.False(t, holidayColumnExists(t, db, "holiday_registrations", "custody_confirmed_at"))
	require.NoError(t, holidayRegistrationDeclarationUp(ctx, db))
	assert.Equal(t, row{Status: "confirmed", Health: true}, read(legacy), "Up again keeps the registrations")

	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET consent_version = 3 WHERE id = ?`, hostHealth)
	require.Error(t, err, "version 3 requires the custody and the name")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations
		SET consent_version = 3, custody_confirmed_at = NOW(), declaration_name = ? WHERE id = ?`, strings.Repeat("a", 101), hostHealth)
	require.Error(t, err, "the name is bounded like the name of the parent")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations
		SET consent_version = 3, custody_confirmed_at = NOW(), declaration_name = 'Eva Muster' WHERE id = ?`, hostHealth)
	require.NoError(t, err)
}
