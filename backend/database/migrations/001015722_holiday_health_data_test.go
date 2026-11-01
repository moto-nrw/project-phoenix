package migrations

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func testHolidayHealthCipher(t *testing.T) holidayFieldCipher {
	t.Helper()
	cipher, err := securityRuntimeCompose.NewFieldCipher("test:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	require.NoError(t, err)
	return cipher
}

func holidayColumnExists(t *testing.T, db *bun.DB, table, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'enrollment' AND table_name = ? AND column_name = ?)`, table, column).Scan(context.Background(), &exists))
	return exists
}

// The questions of the sealed registration's version: a health question per
// child and per family, a question without health information, and one
// without a marker, which counts as health information.
const holidayHealthVersion = `[
	{"key": "tetanus_impfschutz", "label": "Tetanus-Impfschutz", "type": "select", "applies_to_child": true, "health": true, "share_with_host": true,
		"options": [{"label": "Ja", "value": "ja"}]},
	{"key": "darf_allein_nach_hause", "label": "Darf allein nach Hause gehen", "type": "boolean", "applies_to_child": true, "health": false, "share_with_host": true},
	{"key": "ohne_marker", "label": "Ohne Marker", "type": "text", "applies_to_child": true},
	{"key": "familie_gesundheit", "label": "Krankenkasse", "type": "text", "health": true},
	{"key": "familie_frei", "label": "Anreise", "type": "text", "health": false}
]`

// TestHolidayHealthDataSealsEveryPlaintextValue runs the migration of #4722
// over registrations written before it: afterwards no allergy, medication,
// health note or answer to a health question stays in plaintext, each sealed
// value opens only with the context of its row, the answers without health
// information stay where they were, and the down path restores the plaintext.
func TestHolidayHealthDataSealsEveryPlaintextValue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	cipher := testHolidayHealthCipher(t)
	require.NoError(t, holidayHealthDataDownWith(ctx, db, cipher))
	require.True(t, holidayColumnExists(t, db, "holiday_registration_children", "allergies"))

	f := newHolidaySharedAnswersFixture(t, db)
	f.insertVersion(t, f.own, f.period, 2, holidayHealthVersion)
	var registration, child, quietChild int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, health_consent_at, status, form_version, custom_data)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), NOW(), 'confirmed', 2,
			'{"familie_gesundheit": "AOK Rheinland", "familie_frei": "Mit dem Rad"}'::jsonb) RETURNING id`,
		f.own, f.period).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date, home_school_id, allergies, medication, health_notes, custom_data)
		VALUES (?, ?, ?, 1, 'Lia', 'Muster', '2018-03-01', ?, 'Erdnüsse', 'Asthmaspray', 'Trägt eine Brille',
			'{"tetanus_impfschutz": "ja", "darf_allein_nach_hause": true, "ohne_marker": "Geheimnis"}'::jsonb) RETURNING id`,
		f.own, f.period, registration, f.home).Scan(ctx, &child))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date, home_school_id, custom_data)
		VALUES (?, ?, ?, 2, 'Ben', 'Muster', '2016-03-01', ?, '{"darf_allein_nach_hause": false}'::jsonb) RETURNING id`,
		f.own, f.period, registration, f.home).Scan(ctx, &quietChild))

	require.Error(t, holidayHealthDataUpWith(ctx, db, nil), "health information to seal needs the key ring")
	require.NoError(t, holidayHealthDataUpWith(ctx, db, cipher))

	for _, column := range []string{"allergies", "medication", "health_notes"} {
		assert.False(t, holidayColumnExists(t, db, "holiday_registration_children", column), "%s leaves the table", column)
	}
	var whole string
	require.NoError(t, db.NewRaw(`SELECT (SELECT jsonb_agg(to_jsonb(child))::text FROM enrollment.holiday_registration_children AS child
			WHERE child.holiday_registration_id = ?) || (SELECT to_jsonb(registration)::text FROM enrollment.holiday_registrations AS registration
			WHERE registration.id = ?)`, registration, registration).Scan(ctx, &whole))
	for _, plaintext := range []string{"Erdnüsse", "Asthmaspray", "Brille", "Geheimnis", "AOK", "tetanus"} {
		assert.NotContains(t, whole, plaintext, "no plaintext health information stays in the tables")
	}
	assert.Contains(t, whole, "Mit dem Rad", "answers without health information stay")

	var rows []struct {
		ID         int64           `bun:"id"`
		HealthData []byte          `bun:"health_data"`
		CustomData json.RawMessage `bun:"custom_data,type:jsonb"`
	}
	require.NoError(t, db.NewRaw(`SELECT id, health_data, custom_data FROM enrollment.holiday_registration_children
		WHERE holiday_registration_id = ? ORDER BY position`, registration).Scan(ctx, &rows))
	require.Len(t, rows, 2)
	assert.Nil(t, rows[1].HealthData, "a child without health information gets no sealed value")
	assert.JSONEq(t, `{"darf_allein_nach_hause": true}`, string(rows[0].CustomData))
	opened, err := cipher.Open(rows[0].HealthData, holidayChildHealthContext(f.own, registration, 1))
	require.NoError(t, err)
	assert.JSONEq(t, `{"allergies": "Erdnüsse", "medication": "Asthmaspray", "health_notes": "Trägt eine Brille",
		"answers": {"tetanus_impfschutz": "ja", "ohne_marker": "Geheimnis"}}`, string(opened))
	_, err = cipher.Open(rows[0].HealthData, holidayChildHealthContext(f.own, registration, 2))
	assert.Error(t, err, "a sealed value opens only for its own row")

	var family []byte
	require.NoError(t, db.NewRaw(`SELECT health_data FROM enrollment.holiday_registrations WHERE id = ?`, registration).Scan(ctx, &family))
	opened, err = cipher.Open(family, holidayFamilyHealthContext(f.own, registration))
	require.NoError(t, err)
	assert.JSONEq(t, `{"answers": {"familie_gesundheit": "AOK Rheinland"}}`, string(opened))

	require.NoError(t, holidayHealthDataDownWith(ctx, db, cipher))
	var restored struct {
		Allergies  string          `bun:"allergies"`
		CustomData json.RawMessage `bun:"custom_data,type:jsonb"`
	}
	require.NoError(t, db.NewRaw(`SELECT allergies, custom_data FROM enrollment.holiday_registration_children WHERE id = ?`, child).Scan(ctx, &restored))
	assert.Equal(t, "Erdnüsse", restored.Allergies)
	assert.JSONEq(t, `{"tetanus_impfschutz": "ja", "darf_allein_nach_hause": true, "ohne_marker": "Geheimnis"}`, string(restored.CustomData))
	require.NoError(t, holidayHealthDataUpWith(ctx, db, cipher))
}

// TestHolidayHostParticipantsShareHealthOnlyWithTheNewConsent pins the
// hosting school's function after #4722: the sealed values and the shared
// health questions leave only for a registration whose parents agreed to the
// health information under consent texts of version 2; never a plaintext
// value, never a question the version does not share, never to another school
// and never after the week.
func TestHolidayHostParticipantsShareHealthOnlyWithTheNewConsent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	f := newHolidaySharedAnswersFixture(t, db)
	f.insertVersion(t, f.own, f.period, 2, holidayHealthVersion)
	register := func(consentVersion any, health bool) int64 {
		child := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 2, status: "confirmed",
			homeSchool: f.home, bookings: [][2]int64{{f.open, f.openModel}, {f.ended, f.endedModel}}})
		_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations AS registration
			SET consent_version = ?, health_consent_at = CASE WHEN ? THEN NOW() END, health_data = '\x01'::bytea
			FROM enrollment.holiday_registration_children AS child
			WHERE child.id = ? AND registration.id = child.holiday_registration_id`, consentVersion, health, child)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registration_children SET health_data = '\x02'::bytea WHERE id = ?`, child)
		require.NoError(t, err)
		return child
	}
	shared := register(2, true)
	before := register(nil, true)
	refused := register(2, false)

	type row struct {
		ChildID      int64           `bun:"child_id"`
		Health       []byte          `bun:"health_data"`
		FamilyHealth []byte          `bun:"family_health_data"`
		HealthFields json.RawMessage `bun:"health_fields,type:jsonb"`
	}
	read := func(school, week int64) map[int64]row {
		var rows []row
		f.asSchool(t, school, func(ctx context.Context, tx bun.Tx) error {
			return tx.NewRaw(`SELECT child_id, health_data, family_health_data, health_fields
				FROM enrollment.holiday_host_participants(?, ?)`, f.own, week).Scan(ctx, &rows)
		})
		byChild := map[int64]row{}
		for _, r := range rows {
			byChild[r.ChildID] = r
		}
		return byChild
	}
	open := read(f.host, f.open)
	require.Contains(t, open, shared)
	assert.Equal(t, []byte{2}, open[shared].Health)
	assert.Equal(t, []byte{1}, open[shared].FamilyHealth)
	var fields []map[string]any
	require.NoError(t, json.Unmarshal(open[shared].HealthFields, &fields))
	require.Len(t, fields, 1, "only the shared health question of the version: not the unshared one without marker, not a question without health information")
	assert.Equal(t, "tetanus_impfschutz", fields[0]["key"])
	for _, child := range []int64{before, refused} {
		require.Contains(t, open, child)
		assert.Nil(t, open[child].Health, "no sealed value without the consent of version 2")
		assert.Nil(t, open[child].FamilyHealth)
		assert.Nil(t, open[child].HealthFields)
	}
	assert.Empty(t, read(f.home, f.open), "another school of the Träger sees nothing")
	assert.Empty(t, read(f.host, f.ended), "the access ends after the last day of the week")

	var whole string
	f.asSchool(t, f.host, func(ctx context.Context, tx bun.Tx) error {
		return tx.NewRaw(`SELECT COALESCE(jsonb_agg(to_jsonb(participant))::text, '[]')
			FROM enrollment.holiday_host_participants(?, ?) AS participant`, f.own, f.open).Scan(ctx, &whole)
	})
	assert.False(t, strings.Contains(whole, "eva@example.org"), "no e-mail address leaves")
}
