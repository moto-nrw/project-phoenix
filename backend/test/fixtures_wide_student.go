package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// FillWideStudentFields writes the personal/administrative columns a real
// production row carries. Without them omitempty hides most of the full
// projection, and a payload guard that asserts a field is absent would
// measure nothing.
//
// bus_days / pickup_days / departure_days / allowed_departure_modes are
// `scanonly` on the model, so they are written with raw SQL here exactly like
// the repository reads them.
func FillWideStudentFields(tb testing.TB, db *bun.DB, studentID, personID int64) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	guardian := CreateTestGuardianProfileNamed(tb, db, "Erika", "Mustermann", "erika.mustermann@example.com")
	CreateTestStudentGuardianLink(tb, db, studentID, guardian.ID, "parent")
	_, err := db.NewRaw(`UPDATE users.student_profiles SET
			address_street = 'Musterstrasse 12a',
			address_city = 'Koeln',
			address_postal_code = '50667',
			extra_info = 'Geschwisterkind in Klasse 2b, kommt meist mit dem Rad',
			agb_accepted_at = now(),
			data_processing_accepted_at = now(),
			email_contact_accepted_at = now()
		WHERE id = ?`, studentID).Exec(ctx)
	require.NoError(tb, err, "failed to fill wide profile fields")
	_, err = db.NewRaw(`UPDATE users.student_care_profiles SET
			health_info = 'Laktoseintoleranz, Notfallmedikament im Gruppenraum',
			pickup_status = 'Wird abgeholt',
			bus_days = '{"mon":true,"tue":true,"wed":true,"thu":true,"fri":true}'::jsonb,
			pickup_days = '{"mon":true,"tue":true,"wed":true,"thu":true,"fri":true}'::jsonb,
			departure_days = '{"mon":"pickup","tue":"pickup","wed":"pickup","thu":"pickup","fri":"pickup"}'::jsonb,
			allowed_departure_modes = '{"mon":["pickup"],"tue":["pickup"],"wed":["pickup"],"thu":["pickup"],"fri":["pickup"]}'::jsonb
		WHERE membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)`, studentID).Exec(ctx)
	require.NoError(tb, err, "failed to fill wide student fields")

	_, err = db.NewRaw(`UPDATE users.persons SET birthday = DATE '2019-02-02' WHERE id = ?`, personID).Exec(ctx)
	require.NoError(tb, err, "failed to set person birthday")
}
