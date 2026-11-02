package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// holidaySharedAnswersFixture is a published period of a Träger with an open
// week and an ended week, both hosted by the OGS Nord, and the form versions
// of #5198 that the hosting school's functions read.
type holidaySharedAnswersFixture struct {
	db                        *bun.DB
	own, host, home           int64
	period, open, openModel   int64
	ended, endedModel         int64
	foreign, foreignSchool    int64
	foreignPeriod             int64
	foreignWeek, foreignModel int64
}

func newHolidaySharedAnswersFixture(t *testing.T, db *bun.DB) holidaySharedAnswersFixture {
	t.Helper()
	newTraeger := func(name string) int64 {
		slug := fmt.Sprintf("traeger-freigabe-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		return organization
	}
	f := holidaySharedAnswersFixture{db: db}
	f.own = newTraeger("Träger Freigabe Eigen")
	f.host = testpkg.CreateTestSchoolInOrganization(t, db, f.own, "Träger Freigabe OGS Nord")
	f.home = testpkg.CreateTestSchoolInOrganization(t, db, f.own, "Träger Freigabe OGS Süd")
	f.foreign = newTraeger("Träger Freigabe Fremd")
	f.foreignSchool = testpkg.CreateTestSchoolInOrganization(t, db, f.foreign, "Träger Freigabe Fremd OGS")
	f.period = f.insertPeriod(t, f.own)
	f.open, f.openModel = f.insertWeek(t, f.own, f.period, f.host, "2099-07-06", "2099-07-10")
	f.ended, f.endedModel = f.insertWeek(t, f.own, f.period, f.host, "2020-07-06", "2020-07-10")
	f.foreignPeriod = f.insertPeriod(t, f.foreign)
	f.foreignWeek, f.foreignModel = f.insertWeek(t, f.foreign, f.foreignPeriod, f.foreignSchool, "2099-07-06", "2099-07-10")
	return f
}

func (f holidaySharedAnswersFixture) insertPeriod(t *testing.T, organization int64) int64 {
	t.Helper()
	var period int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at)
		VALUES (?, 'Sommerferien', '2020-01-01', '2099-06-30', 'published', NOW()) RETURNING id`, organization).Scan(context.Background(), &period))
	return period
}

func (f holidaySharedAnswersFixture) insertWeek(t *testing.T, organization, period, host int64, startsOn, endsOn string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var week, model int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, ?, ?, ?) RETURNING id`, organization, period, startsOn, endsOn, host).Scan(ctx, &week))
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'bis 13 Uhr', '08:00', '13:00', 4500) RETURNING id`, organization, week).Scan(ctx, &model))
	return week, model
}

// insertVersion writes a form version with the given questions as JSON, so a
// question can lack its markers like no writer of the application would.
func (f holidaySharedAnswersFixture) insertVersion(t *testing.T, organization, period int64, version int, fields string) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO enrollment.holiday_period_forms (organization_id, holiday_period_id, version, fields)
		VALUES (?, ?, ?, ?::jsonb)`, organization, period, version, fields)
	require.NoError(t, err)
}

type holidaySharedRegistration struct {
	organization, period int64
	version              int
	status               string
	homeSchool           any
	secondGuardian       bool
	family, child        string
	bookings             [][2]int64
}

// register writes a registration with one child, its answers, an emergency
// contact and its bookings, and returns the child.
func (f holidaySharedAnswersFixture) register(t *testing.T, r holidaySharedRegistration) int64 {
	t.Helper()
	ctx := context.Background()
	if r.family == "" {
		r.family = "{}"
	}
	if r.child == "" {
		r.child = "{}"
	}
	var second struct{ first, last, email, phone any }
	if r.secondGuardian {
		second.first, second.last, second.email, second.phone = "Tom", "Zweit", "zweit@example.org", "0170 222"
	}
	var registration, child int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, health_consent_at, status, form_version, custom_data,
			second_guardian_first_name, second_guardian_last_name, second_guardian_email, second_guardian_phone)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), NOW(), ?, ?, ?::jsonb, ?, ?, ?, ?) RETURNING id`,
		r.organization, r.period, r.status, r.version, r.family, second.first, second.last, second.email, second.phone).Scan(ctx, &registration))
	require.NoError(t, f.db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date, home_school_id, health_data, custom_data)
		VALUES (?, ?, ?, 1, 'Lia', 'Muster', '2018-03-01', ?, convert_to('Erdnüsse', 'UTF8'), ?::jsonb) RETURNING id`,
		r.organization, r.period, registration, r.homeSchool, r.child).Scan(ctx, &child))
	_, err := f.db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_contacts (organization_id, holiday_registration_id, kind, position, name, phone, relationship)
		VALUES (?, ?, 'emergency', 1, 'Oma Muster', '05402 1', 'Großmutter')`, r.organization, registration)
	require.NoError(t, err)
	for _, booking := range r.bookings {
		_, err := f.db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
				holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
			r.organization, r.period, child, booking[0], booking[1])
		require.NoError(t, err)
	}
	return child
}

func (f holidaySharedAnswersFixture) asSchool(t *testing.T, school int64, fn func(context.Context, bun.Tx) error) {
	t.Helper()
	require.NoError(t, f.db.RunInTx(context.Background(), &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(school)); err != nil {
			return err
		}
		return fn(ctx, tx)
	}))
}

type sharedParticipantRow struct {
	ChildID             int64           `bun:"child_id"`
	SecondGuardianName  *string         `bun:"second_guardian_name"`
	SecondGuardianPhone *string         `bun:"second_guardian_phone"`
	Answers             json.RawMessage `bun:"answers,type:jsonb"`
}

// participants reads the hosting school's function and returns its rows by
// child, and the whole answer as text for the absence checks.
func (f holidaySharedAnswersFixture) participants(t *testing.T, school, organization, week int64) (map[int64]sharedParticipantRow, string) {
	t.Helper()
	var rows []sharedParticipantRow
	var whole string
	f.asSchool(t, school, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewRaw(`SELECT child_id, second_guardian_name, second_guardian_phone, answers
			FROM enrollment.holiday_host_participants(?, ?)`, organization, week).Scan(ctx, &rows); err != nil {
			return err
		}
		return tx.NewRaw(`SELECT COALESCE(jsonb_agg(to_jsonb(participant))::text, '[]')
			FROM enrollment.holiday_host_participants(?, ?) AS participant`, organization, week).Scan(ctx, &whole)
	})
	byChild := make(map[int64]sharedParticipantRow, len(rows))
	for _, row := range rows {
		byChild[row.ChildID] = row
	}
	return byChild, whole
}

// The questions of version 2: what the hosting school sees is shared and
// explicitly no health information, of a type with an answer, under a key
// no version of the period marks otherwise.
const holidaySharedVersionTwo = `[
	{"key": "darf_allein_nach_hause", "label": "Darf allein nach Hause gehen", "type": "boolean", "applies_to_child": true, "health": false, "share_with_host": true},
	{"key": "tetanus_impfschutz", "label": "Tetanus-Impfschutz", "type": "select", "applies_to_child": true, "health": true, "share_with_host": true,
		"options": [{"label": "Ja", "value": "ja"}]},
	{"key": "notiz", "label": "Notiz", "type": "text", "applies_to_child": true, "health": false, "share_with_host": false},
	{"key": "abholung", "label": "Abholung", "type": "select", "health": false, "share_with_host": true,
		"options": [{"label": "Mit dem Bus", "value": "bus"}]},
	{"key": "ohne_marker", "label": "Ohne Marker", "type": "text", "applies_to_child": true, "share_with_host": true},
	{"key": "foerderbedarf", "label": "Förderbedarf", "type": "textarea", "applies_to_child": true, "health": false, "share_with_host": true},
	{"key": "ausflug_ab", "label": "Ausflug ab", "type": "date", "applies_to_child": true, "health": false, "share_with_host": true},
	{"key": "hinweis", "label": "Hinweis", "type": "information", "content": "Bitte lesen", "applies_to_child": true, "health": false, "share_with_host": true},
	{"key": "taschengeld", "label": "Taschengeld", "type": "text", "applies_to_child": true, "health": false, "share_with_host": false},
	{"key": "lieblingsfarbe", "label": "Lieblingsfarbe", "type": "text", "applies_to_child": true, "health": "false", "share_with_host": true}
]`

// Version 3 marks Förderbedarf as health information and shares Taschengeld.
const holidaySharedVersionThree = `[
	{"key": "foerderbedarf", "label": "Förderbedarf", "type": "textarea", "applies_to_child": true, "health": true, "share_with_host": false},
	{"key": "taschengeld", "label": "Taschengeld", "type": "text", "applies_to_child": true, "health": false, "share_with_host": true}
]`

// TestHolidayHostParticipantsShareOnlyAnswersWithoutHealthInformation pins
// the invariants of #5202 on enrollment.holiday_host_participants and
// enrollment.holiday_host_guest_contacts:
//
//  1. No health question reaches the hosting school: a question leaves only
//     when the registration's version shares it and marks it health = false
//     as a JSON boolean, and no version of the period marks its key
//     otherwise; allergies, a missing marker and an information block stay
//     back as well.
//  2. Nothing of another week, another school or another Träger reaches it,
//     and nothing after the last day of the week.
//  3. A registration of before, without answers or second guardian, keeps
//     its row.
//  4. An answer of an older version without the share stays back, even when
//     a newer version shares the question.
//  5. No e-mail address leaves; the second guardian leaves with name and
//     phone, in the list and as an emergency contact of a holiday guest.
func TestHolidayHostParticipantsShareOnlyAnswersWithoutHealthInformation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newHolidaySharedAnswersFixture(t, db)
	f.insertVersion(t, f.own, f.period, 2, holidaySharedVersionTwo)
	f.insertVersion(t, f.own, f.period, 3, holidaySharedVersionThree)
	open := [2]int64{f.open, f.openModel}
	answersOfVersionTwo := `{"darf_allein_nach_hause": true, "tetanus_impfschutz": "ja", "notiz": "Geheimnis", "ohne_marker": "Unmarkiert",
		"foerderbedarf": "Förderstunden", "ausflug_ab": "2099-07-07", "hinweis": "x", "taschengeld": "Fünf Euro", "lieblingsfarbe": "Blau"}`

	shared := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 2, status: "confirmed",
		homeSchool: f.home, secondGuardian: true, family: `{"abholung": "bus"}`, child: answersOfVersionTwo,
		bookings: [][2]int64{open, {f.ended, f.endedModel}}})
	newer := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 3, status: "confirmed",
		child: `{"taschengeld": "Zehn Euro", "foerderbedarf": "Förderstunden drei"}`, bookings: [][2]int64{open}})
	before := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 1, status: "confirmed",
		homeSchool: f.home, bookings: [][2]int64{open}})
	f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 2, status: "received",
		homeSchool: f.home, secondGuardian: true, child: `{"darf_allein_nach_hause": false, "ausflug_ab": "2099-07-09"}`, bookings: [][2]int64{open}})
	f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 2, status: "confirmed",
		homeSchool: f.home, child: `{"darf_allein_nach_hause": false, "ausflug_ab": "2020-07-08"}`, bookings: [][2]int64{{f.ended, f.endedModel}}})
	f.insertVersion(t, f.foreign, f.foreignPeriod, 2, holidaySharedVersionTwo)
	foreignChild := f.register(t, holidaySharedRegistration{organization: f.foreign, period: f.foreignPeriod, version: 2, status: "confirmed",
		secondGuardian: true, child: `{"darf_allein_nach_hause": false, "ausflug_ab": "2099-07-08"}`,
		bookings: [][2]int64{{f.foreignWeek, f.foreignModel}}})

	rows, whole := f.participants(t, f.host, f.own, f.open)
	require.Len(t, rows, 3, "the confirmed children of the open week, the one received stays back")
	assert.JSONEq(t, `[
		{"key": "darf_allein_nach_hause", "label": "Darf allein nach Hause gehen", "type": "boolean", "value": true},
		{"key": "abholung", "label": "Abholung", "type": "select", "value": "Mit dem Bus"},
		{"key": "ausflug_ab", "label": "Ausflug ab", "type": "date", "value": "2099-07-07"}
	]`, string(rows[shared].Answers), "only shared questions without health information leave, a selection with its label")
	require.NotNil(t, rows[shared].SecondGuardianName)
	assert.Equal(t, "Tom Zweit", *rows[shared].SecondGuardianName)
	assert.Equal(t, "0170 222", *rows[shared].SecondGuardianPhone)
	assert.JSONEq(t, `[{"key": "taschengeld", "label": "Taschengeld", "type": "text", "value": "Zehn Euro"}]`, string(rows[newer].Answers),
		"version 3 shares Taschengeld; Förderbedarf stays back because version 3 marks it as health information")
	assert.Nil(t, rows[newer].SecondGuardianName, "a registration without a second guardian names none")
	assert.JSONEq(t, `[]`, string(rows[before].Answers), "a registration of before keeps its row without answers")
	assert.Nil(t, rows[before].SecondGuardianName)
	for _, private := range []string{
		"ja", "Geheimnis", "Unmarkiert", "Förderstunden", "Fünf Euro", "Blau", "Erdnüsse", "Bitte lesen",
		"eva@example.org", "zweit@example.org", "2099-07-09", "2020-07-08", "2099-07-08",
	} {
		assert.NotContains(t, whole, `"`+private, "%q must not reach the hosting school", private)
	}

	ended, _ := f.participants(t, f.host, f.own, f.ended)
	assert.Empty(t, ended, "after the last day of the week the function answers nothing")
	other, _ := f.participants(t, f.home, f.own, f.open)
	assert.Empty(t, other, "another school of the Träger reads no answer of a week it does not host")
	foreignRead, _ := f.participants(t, f.foreignSchool, f.own, f.open)
	assert.Empty(t, foreignRead, "a school of another Träger reads no answer")
	crossed, _ := f.participants(t, f.host, f.foreign, f.foreignWeek)
	assert.Empty(t, crossed, "the hosting school reads nothing of another Träger's week")
	foreignOwn, _ := f.participants(t, f.foreignSchool, f.foreign, f.foreignWeek)
	require.Len(t, foreignOwn, 1)
	assert.Contains(t, string(foreignOwn[foreignChild].Answers), "2099-07-08", "the other Träger's host reads only its own child")

	var contacts []string
	f.asSchool(t, f.host, func(ctx context.Context, tx bun.Tx) error {
		return tx.NewRaw(`SELECT child_id::text || ':' || kind || ':' || name || ':' || phone || ':' || relationship
			FROM enrollment.holiday_host_guest_contacts(?, ?) ORDER BY child_id, kind, position`, f.own, f.open).Scan(ctx, &contacts)
	})
	assert.Equal(t, []string{
		fmt.Sprintf("%d:emergency:Eva Muster:0123:Eltern", shared),
		fmt.Sprintf("%d:emergency:Tom Zweit:0170 222:Eltern", shared),
		fmt.Sprintf("%d:emergency:Oma Muster:05402 1:Großmutter", shared),
		fmt.Sprintf("%d:emergency:Eva Muster:0123:Eltern", newer),
		fmt.Sprintf("%d:emergency:Oma Muster:05402 1:Großmutter", newer),
		fmt.Sprintf("%d:emergency:Eva Muster:0123:Eltern", before),
		fmt.Sprintf("%d:emergency:Oma Muster:05402 1:Großmutter", before),
	}, contacts, "the second guardian follows the registering parent as an emergency contact, without an e-mail address")
}

// TestHolidayHostSharedAnswersDownRestoresTheFunctionsOfBefore pins the down
// path of #5202: the functions get back the signatures of #4665 and #4666,
// the guest copy loses its answers, and the up path brings both back.
func TestHolidayHostSharedAnswersDownRestoresTheFunctionsOfBefore(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	// The health data migration of #4722 replaces the participant function
	// again; step back to the state of #5202 first.
	require.NoError(t, holidayHealthDataDownWith(ctx, db, nil))
	columns := func(function string) []string {
		var names []string
		require.NoError(t, db.NewRaw(`SELECT unnest(proargnames) FROM pg_proc
			WHERE pronamespace = 'enrollment'::regnamespace AND proname = ?`, function).Scan(ctx, &names))
		return names
	}
	answersColumn := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'enrollment' AND table_name = 'holiday_guests' AND column_name = 'answers')`).Scan(ctx, &exists))
		return exists
	}
	participantColumns := []string{"p_organization_id", "p_week_id", "child_id", "first_name", "last_name", "birth_date",
		"home_school_id", "guest_school_name", "time_model_id"}
	contactColumns := []string{"p_organization_id", "p_week_id", "child_id", "kind", "position", "name", "phone", "relationship"}

	assert.Equal(t, append(append([]string{}, participantColumns...), "second_guardian_name", "second_guardian_phone", "answers"),
		columns("holiday_host_participants"))
	assert.True(t, answersColumn())
	require.NoError(t, holidayHostSharedAnswersDown(ctx, db))
	assert.Equal(t, participantColumns, columns("holiday_host_participants"), "the down path restores the signature of #4665")
	assert.Equal(t, contactColumns, columns("holiday_host_guest_contacts"), "the down path restores the signature of #4666")
	assert.False(t, answersColumn(), "the guest copy loses its answers")
	var executable bool
	require.NoError(t, db.NewRaw(`SELECT has_function_privilege('phoenix_tenant', 'enrollment.holiday_host_participants(bigint, bigint)', 'EXECUTE')
		AND NOT has_function_privilege('public', 'enrollment.holiday_host_participants(bigint, bigint)', 'EXECUTE')`).Scan(ctx, &executable))
	assert.True(t, executable, "the restored function keeps its grants")
	require.NoError(t, holidayHostSharedAnswersUp(ctx, db))
	assert.True(t, answersColumn())
	assert.Equal(t, contactColumns, columns("holiday_host_guest_contacts"))
}
