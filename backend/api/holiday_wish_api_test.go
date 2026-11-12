package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// approveWithHolidayWish approves an enrollment of student at school in a
// school-year phase around the first day of the period with the parents'
// holiday care wish (#4703).
func approveWithHolidayWish(t *testing.T, db *bun.DB, school, student int64, wish bool) {
	t.Helper()
	ctx := context.Background()
	suffix := testpkg.UniqueTestTenantID(t)
	var phase, request int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.phases
		(tenant_id, name, kind, service_start_date, service_end_date, care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, ?, 'school_year', '2026-08-01', '2027-07-31', 'waitlist', 'optional', true) RETURNING id`,
		school, fmt.Sprintf("Schuljahr %d", suffix)).Scan(ctx, &phase))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.requests
		(tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token, submitted_at, submission_source)
		VALUES (?, ?, 'Eva', 'Muster', ?, ?, NOW(), 'public') RETURNING id`,
		school, phase, fmt.Sprintf("eva-%d@example.test", suffix), fmt.Sprintf("wish-%d", suffix)).Scan(ctx, &request))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.request_children
		(tenant_id, request_id, first_name, last_name, date_of_birth, status, sort_order, created_student_id, holiday_care_wish)
		VALUES (?, ?, 'Lia', 'Muster', '2018-03-01', 'approved', 0, ?, ?)`, school, request, student, wish)
	require.NoError(t, err)
}

// holidayWishProofFor is the statement the prefill gives the parent of the
// child (#4703, ADR 0113): read in the school's tenant transaction and
// sealed with the root's field cipher.
func holidayWishProofFor(t *testing.T, f reviewFixture, school, student int64) enrollment.HolidayWishStatement {
	t.Helper()
	id, err := tenant.NewTenantID(school)
	require.NoError(t, err)
	var statement enrollment.HolidayWishStatement
	proofs, _ := enrollmentCompose.NewHolidayWishes(testHealthCipher())
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	require.NoError(t, tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		statement, err = proofs.HolidayWishProof(txCtx, enrollment.HolidayWishClaim{
			OrganizationID: f.office.organization, PeriodID: f.period.ID, SchoolID: school, StudentID: student,
			FirstName: "Lia", LastName: "Muster", BirthDate: calendar.NewDate(2018, 3, 1), On: calendar.NewDate(2027, 1, 4),
		})
		return err
	}))
	return statement
}

// switchOnHolidayWish turns the rule of the period on through the Träger
// portal, as the office does in the period editor.
func switchOnHolidayWish(t *testing.T, f reviewFixture) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPeriodPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[map[string]any](t, rec.Body.Bytes())
	assert.Equal(t, false, period["holiday_wish_rule"], "the rule is off by default")
	period["holiday_wish_rule"] = true
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), period)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, true, decodeSchoolPortalData[map[string]any](t, rec.Body.Bytes())["holiday_wish_rule"])
}

func wishRegistration(offer holidayOfferBody, email, proof string) map[string]any {
	body := siblingsRegistration(offer, email)
	children := body["children"].([]map[string]any)
	lia := children[0]
	if proof != "" {
		lia["wish_proof"] = proof
	}
	body["children"] = []map[string]any{lia}
	return body
}

type wishChildBody struct {
	ID                int64  `json:"id"`
	FirstName         string `json:"first_name"`
	HolidayWish       string `json:"holiday_wish"`
	HolidayWishSource string `json:"holiday_wish_source"`
	HolidayWishCheck  bool   `json:"holiday_wish_check"`
}

type wishRegistrationBody struct {
	ID       int64           `json:"id"`
	Children []wishChildBody `json:"children"`
	History  []struct {
		Action string `json:"action"`
	} `json:"history"`
}

// TestHolidayWishRoundTrip pins the invariants of #4703 end to end:
//   - a parent with the relationship right brings the child's wish as a
//     sealed statement; with "wish_only" a stated no refuses the form with
//     the hint code, with "priority" it is stored as no;
//   - the same child without a statement is accepted with an unknown wish,
//     so the public answer never tells whether a school knows the child;
//   - a wish a caller puts in the body is dropped;
//   - only the office sees the wish and the mark "check", filters by it and
//     decides it, with a history entry; another Träger gets 404, a role
//     without the right 403; the receipt names no wish.
func TestHolidayWishRoundTrip(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Ferienwunsch")
	db := testpkg.SetupTestDB(t)
	school := f.office.schools[0]
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Lia", "Muster", "2a")
	approveWithHolidayWish(t, db, school, student.ID, false)
	switchOnHolidayWish(t, f)

	offer := publicJSON(t, f.public, http.MethodGet, registrationPath(f.period.PublicToken), nil)
	require.Equal(t, http.StatusOK, offer.Code, offer.Body.String())
	assert.Equal(t, enrollment.HolidayWishModePriority, decodeSchoolPortalData[map[string]any](t, offer.Body.Bytes())["holiday_wish_mode"],
		"precedence is the default of the Träger setting")

	statement := holidayWishProofFor(t, f, school, student.ID)
	require.Equal(t, enrollment.HolidayWishNo, statement.Wish)
	require.NotEmpty(t, statement.Proof)

	submit := func(body map[string]any) *http.Response {
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), body)
		return rec.Result()
	}
	stated := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), wishRegistration(f.offer, "stated@example.org", statement.Proof))
	require.Equal(t, http.StatusCreated, stated.Code, stated.Body.String())
	assert.NotContains(t, stated.Body.String(), "holiday_wish", "the receipt names no wish")
	statedID := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, stated.Body.Bytes()).ID

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut,
		"/traeger/settings/values/"+configModels.KeyHolidayRegistrationWishMode, map[string]any{"value": enrollment.HolidayWishModeWishOnly})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	refused := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), wishRegistration(f.offer, "refused@example.org", statement.Proof))
	require.Equal(t, http.StatusConflict, refused.Code, refused.Body.String())
	code, _ := registrationRefusalOf(t, refused)
	assert.Equal(t, enrollment.CodeHolidayWishMissing, code)

	forged := wishRegistration(f.offer, "unknown@example.org", "")
	forged["children"].([]map[string]any)[0]["holiday_wish"] = "yes"
	forged["children"].([]map[string]any)[0]["holiday_wish_source"] = "office"
	unknown := submit(forged)
	require.Equal(t, http.StatusCreated, unknown.StatusCode, "without a statement nothing is looked up and nothing refused")
	var unknownBody struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(unknown.Body).Decode(&unknownBody))
	unknownID := unknownBody.Data.ID

	detail := func(id int64) wishRegistrationBody {
		t.Helper()
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[wishRegistrationBody](t, rec.Body.Bytes())
	}
	statedChild := detail(statedID).Children[0]
	assert.Equal(t, enrollment.HolidayWishNo, statedChild.HolidayWish)
	assert.Equal(t, enrollment.HolidayWishSourceAccount, statedChild.HolidayWishSource)
	assert.False(t, statedChild.HolidayWishCheck)
	unknownChild := detail(unknownID).Children[0]
	assert.Empty(t, unknownChild.HolidayWish, "a wish the caller sets is dropped")
	assert.True(t, unknownChild.HolidayWishCheck, "the office checks an unknown wish")
	assert.False(t, detail(f.siblings).Children[1].HolidayWishCheck, "a holiday guest is never checked")

	checks := f.list(t, "?wish=check")
	ids := []int64{}
	for _, registration := range checks.Registrations {
		ids = append(ids, registration.ID)
	}
	assert.Contains(t, ids, unknownID)
	assert.NotContains(t, ids, statedID, "a stated wish needs no check")

	wishPath := f.registrationPath(unknownID, fmt.Sprintf("/children/%d/holiday-wish", unknownChild.ID))
	other := newTraegerOffice(t, f.db, f.traeger, "Fremder Träger Ferienwunsch", identityaccess.TraegerRoleAdministration)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodPut, wishPath, map[string]any{"wish": "yes"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger reaches no registration: %s", rec.Body.String())
	readOnly := newTraegerOffice(t, f.db, f.traeger, "Lesend Ferienwunsch", identityaccess.TraegerRoleReadOnly)
	rec = traegerJSON(t, f.traeger, readOnly.token, http.MethodPut, wishPath, map[string]any{"wish": "yes"})
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, wishPath, map[string]any{"wish": "maybe"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, wishPath, map[string]any{"wish": "yes"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	decided := decodeSchoolPortalData[wishRegistrationBody](t, rec.Body.Bytes())
	assert.Equal(t, enrollment.HolidayWishYes, decided.Children[0].HolidayWish)
	assert.Equal(t, enrollment.HolidayWishSourceOffice, decided.Children[0].HolidayWishSource)
	assert.False(t, decided.Children[0].HolidayWishCheck)
	assert.Equal(t, enrollment.HolidayAuditWish, decided.History[len(decided.History)-1].Action)

	guestPath := f.registrationPath(f.siblings, fmt.Sprintf("/children/%d/holiday-wish", detail(f.siblings).Children[1].ID))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, guestPath, map[string]any{"wish": "yes"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a holiday guest has no wish: %s", rec.Body.String())
}

// TestHolidayWishStaysWithTheOffice pins that the hosting OGS never sees the
// wish of a registered child (#4703): its participant list has no such
// column. It goes red when the list starts to carry the wish.
func TestHolidayWishStaysWithTheOffice(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Ferienwunsch Gastgeber")
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(f.office.schools[0]))
	require.NoError(t, err)
	rows, err := tx.QueryContext(ctx, `SELECT * FROM enrollment.holiday_host_participants(?, ?)`, f.office.organization, f.offer.Weeks[0].ID)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	columns, err := rows.Columns()
	require.NoError(t, err)
	require.Contains(t, columns, "first_name", "the participant list names its children")
	for _, column := range columns {
		assert.NotContains(t, column, "wish", "the hosting OGS sees no holiday care wish")
	}
}
