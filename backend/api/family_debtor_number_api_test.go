// Router-level tests for the debtor number of a family (#6692): a
// confirmation and a change of a holiday registration remember the number
// at the family of the Träger, the next registration of the family reads it
// as a proposal, a different number needs the office's choice, a sibling
// group writes each of its families once, the list of families shows and
// changes it also for a family without a care contract, and no other Träger
// and no school reads it.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const billingFamilyDebtorNumberPath = "/traeger/billing-families/debtor-number"

func familyDebtorNumberPath(periodID, registrationID int64) string {
	return holidayContributionPath(periodID, registrationID) + "/family-debtor-number"
}

// submitFamily sends a registration of the fixture's family under email.
func (f reviewFixture) submitFamily(t *testing.T, email string) int64 {
	t.Helper()
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), siblingsRegistration(f.offer, email))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
}

func (f reviewFixture) familyProposal(t *testing.T, token string, id int64) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodGet, familyDebtorNumberPath(f.period.ID, id), nil)
}

func (f reviewFixture) proposedNumber(t *testing.T, id int64) string {
	t.Helper()
	rec := f.familyProposal(t, f.office.token, id)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		DebtorNumber string `json:"debtor_number"`
	}](t, rec.Body.Bytes()).DebtorNumber
}

// familyNumber reads the number the family of email has at the fixture's
// Träger, "-" without a row.
func (f reviewFixture) familyNumber(t *testing.T, email string) string {
	t.Helper()
	var numbers []*string
	require.NoError(t, f.db.NewRaw(`SELECT debtor_number FROM parent_billing.family_billing WHERE organization_id = ? AND family_email = ?`,
		f.office.organization, email).Scan(context.Background(), &numbers))
	if len(numbers) == 0 {
		return "-"
	}
	if numbers[0] == nil {
		return ""
	}
	return *numbers[0]
}

// familyTrail lists the actions of the family's trail, oldest first.
func (f reviewFixture) familyTrail(t *testing.T, email string) []string {
	t.Helper()
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT event.action FROM parent_billing.family_billing_events AS event
		JOIN parent_billing.family_billing AS family ON family.id = event.family_billing_id
		WHERE family.organization_id = ? AND family.family_email = ? ORDER BY event.id`, f.office.organization, email).
		Scan(context.Background(), &actions))
	return actions
}

func TestConfirmationRemembersTheNumberAtTheFamilyAndProposesIt(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Merken")
	later := f.submitFamily(t, "  EVA.Muster@Example.org ")
	assert.Empty(t, f.proposedNumber(t, later), "no number before the first confirmation")

	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": " 10 4711 "}).Code)
	assert.Equal(t, "10 4711", f.familyNumber(t, "eva.muster@example.org"), "the confirmation remembers the trimmed number at the family")
	assert.Equal(t, "-", f.familyNumber(t, "papa.muster@example.org"), "another address is another family")

	assert.Equal(t, "10 4711", f.proposedNumber(t, later), "the next registration of the family, in any case, reads it")
	assert.Empty(t, f.proposedNumber(t, f.again), "another family reads none")
	assert.Equal(t, []string{"save", "read"}, f.familyTrail(t, "eva.muster@example.org"), "the write and the read are in the trail")
	assert.Equal(t, "received", f.debtorNumberDetail(t, later).Status, "the proposal confirms nothing")
	assert.Empty(t, f.debtorNumberDetail(t, later).DebtorNumber, "the proposal fills nothing in")

	rec := f.confirmWith(t, later, map[string]any{"debtor_number": "10 4711"})
	require.Equal(t, http.StatusOK, rec.Code, "the same number needs no choice: %s", rec.Body.String())
	assert.Equal(t, "10 4711", f.debtorNumberDetail(t, later).DebtorNumber)
}

func TestDifferentNumberNeedsTheChoiceOfTheOffice(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Konflikt")
	later := f.submitFamily(t, "eva.muster@example.org")
	third := f.submitFamily(t, "eva.muster@example.org")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)

	rec := f.confirmWith(t, later, map[string]any{"debtor_number": "99"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	refusal := decodeProblem(t, rec)
	assert.Equal(t, common.CodeParentBillingFamilyDebtorNumberConflict, refusal.Code)
	assert.Equal(t, map[string]any{"family_debtor_number": "10 4711", "debtor_number": "99", "family_email": "eva.muster@example.org"},
		refusal.Details, "the refusal names both numbers and the family")
	assert.Equal(t, "received", f.debtorNumberDetail(t, later).Status, "nothing is confirmed without the choice")
	assert.Zero(t, f.frozenContributions(t, later), "and nothing frozen")
	assert.Equal(t, "10 4711", f.familyNumber(t, "eva.muster@example.org"))

	rec = f.confirmWith(t, later, map[string]any{"debtor_number": "99", "family_debtor_number": "sometimes"})
	require.Equal(t, http.StatusBadRequest, rec.Code, "an unknown choice is refused: %s", rec.Body.String())

	rec = f.confirmWith(t, later, map[string]any{"debtor_number": "99", "family_debtor_number": "keep"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "99", f.debtorNumberDetail(t, later).DebtorNumber, "keep gives the registration the new number")
	assert.Equal(t, "10 4711", f.familyNumber(t, "eva.muster@example.org"), "and leaves the family's")

	rec = f.changeDebtorNumber(t, f.office.token, later, "55")
	require.Equal(t, http.StatusConflict, rec.Code, "a change asks too: %s", rec.Body.String())
	assert.Equal(t, "99", f.debtorNumberDetail(t, later).DebtorNumber, "the refused change rolls back")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayDebtorNumberPath(f.period.ID, later),
		map[string]any{"debtor_number": "55", "family_debtor_number": "replace"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "55", f.familyNumber(t, "eva.muster@example.org"), "replace makes it the family's number")
	assert.Equal(t, "55", f.proposedNumber(t, third), "the next registration reads the new one")
	assert.Equal(t, "10 4711", f.debtorNumberDetail(t, f.siblings).DebtorNumber, "a registration keeps its own number")
}

// A sibling group pays with one number (#5628); each family of the group
// remembers it once, so the registrations of one family confirmed together
// (#6674) resolve to the same number without typing it again.
func TestSiblingGroupWritesEachFamilyOnce(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Geschwister")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, f.siblings, f.again).Code)

	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)
	assert.Equal(t, "10 4711", f.debtorNumberDetail(t, f.again).DebtorNumber, "the group takes the number")
	assert.Equal(t, "10 4711", f.familyNumber(t, "eva.muster@example.org"))
	assert.Equal(t, "10 4711", f.familyNumber(t, "papa.muster@example.org"), "the family of the sibling remembers it too")
	assert.Equal(t, []string{"save", "save"}, f.familyTrail(t, "papa.muster@example.org"),
		"the confirmation without a number and the group's number, one entry each")
	assert.Equal(t, []string{"save"}, f.familyTrail(t, "eva.muster@example.org"), "one write for the family")
}

func TestFamiliesListShowsAndSetsTheNumberOfAHolidayFamily(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Liste")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)

	type listedFamily struct {
		ID                 int64  `json:"id"`
		RecipientFirstName string `json:"recipient_first_name"`
		RecipientLastName  string `json:"recipient_last_name"`
		FamilyEmail        string `json:"family_email"`
		FamilyDebtorNumber string `json:"family_debtor_number"`
		HolidayOnly        bool   `json:"holiday_only"`
	}
	list := func() []listedFamily {
		rec := traegerJSON(t, f.traeger, billing, http.MethodGet, "/traeger/billing-families", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[[]listedFamily](t, rec.Body.Bytes())
	}
	assert.ElementsMatch(t, []listedFamily{
		{RecipientFirstName: "Eva", RecipientLastName: "Muster", FamilyEmail: "eva.muster@example.org", FamilyDebtorNumber: "10 4711", HolidayOnly: true},
		{RecipientFirstName: "Eva", RecipientLastName: "Muster", FamilyEmail: "papa.muster@example.org", HolidayOnly: true},
	}, list(), "both families of the holiday care are listed before any invoice")

	rec := traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": " PAPA.muster@example.org", "debtor_number": " 20 815 "})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"family_email":"papa.muster@example.org","debtor_number":"20 815"}`, string(decodeSchoolPortalRaw(t, rec.Body.Bytes())))
	assert.Equal(t, "20 815", f.proposedNumber(t, f.submitFamily(t, "papa.muster@example.org")), "a new registration reads the number set in the list")

	assertDebtorNumberRefused(t, traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "papa.muster@example.org", "debtor_number": "10\n4711"}))
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "fremd@example.org", "debtor_number": "1"})
	require.Equal(t, http.StatusNotFound, rec.Code, "an address that is no family of the Träger: %s", rec.Body.String())
	assert.Equal(t, "-", f.familyNumber(t, "fremd@example.org"), "and gets no row")
	assert.Contains(t, f.familyTrail(t, "eva.muster@example.org"), "read", "the list's read is in the trail")
}

func TestFamilyNumberStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Grenze")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)
	later := f.submitFamily(t, "eva.muster@example.org")

	foreign := newTraegerOffice(t, f.db, f.traeger, "Familie Grenze Fremd", identityaccess.TraegerRoleAdministration)
	rec := f.familyProposal(t, foreign.token, later)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger does not read the proposal")
	assert.NotContains(t, rec.Body.String(), "4711")
	foreignBilling := newTraegerOffice(t, f.db, f.traeger, "Familie Grenze Fremd Abrechnung", identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, foreignBilling.token, http.MethodGet, "/traeger/billing-families", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "4711", "nor sees the family in its list")
	rec = traegerJSON(t, f.traeger, foreignBilling.token, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "eva.muster@example.org", "debtor_number": "1"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "nor sets its number: %s", rec.Body.String())

	readOnly := traegerColleague(t, f, identityaccess.TraegerRoleReadOnly)
	assert.Equal(t, http.StatusForbidden, f.familyProposal(t, readOnly, later).Code, "a role without the right gets 403")
	school := testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: 1, Sub: "schule@example.org", TenantID: f.office.schools[0],
		Roles: []string{"admin"}, Permissions: []string{"*"}})
	rec = f.familyProposal(t, school, later)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "a school session does not reach the Träger routes: %s", rec.Body.String())
	assert.Equal(t, "10 4711", f.familyNumber(t, "eva.muster@example.org"))
	assert.Equal(t, "10 4711", f.proposedNumber(t, later))
}

type familyProblem struct {
	Code    string         `json:"code"`
	Details map[string]any `json:"details"`
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) familyProblem {
	t.Helper()
	var problem familyProblem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem), rec.Body.String())
	return problem
}

// The number of a family outlives the deletion of its registrations: after
// the registration deletion run the family still has it and the next
// registration reads it. Without a document of the family it goes on
// 1 January of the fourth year after the last booked day, with its trail.
func TestFamilyNumberOutlivesTheRegistrationAndHasItsOwnPeriod(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := newReviewFixture(t, "Familie Frist")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)

	summer := januaryHolidays(f.office)
	summer.Name, summer.RegistrationOpensOn, summer.RegistrationClosesOn = "Sommerferien 2027", "2020-01-01", "2099-12-31"
	summer.Weeks[0].StartsOn, summer.Weeks[0].EndsOn = "2027-07-05", "2027-07-09"
	summer.Weeks[1].StartsOn, summer.Weeks[1].EndsOn = "2027-07-12", "2027-07-16"
	created := createHolidayPeriod(t, f.traeger, f.office, summer)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	link := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(link.PublicToken),
		siblingsRegistration(holidayOffer(t, f.holidayRegistrationFixture, link.PublicToken), "eva.muster@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	next := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID

	ctx := context.Background()
	admin := func(run func(adminCtx context.Context) error) {
		t.Helper()
		require.NoError(t, tenant.WithinAdmin(tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, f.db)), run))
	}
	retain := func(today calendar.Date) {
		t.Helper()
		admin(func(adminCtx context.Context) error {
			_, err := enrollmentCompose.NewHolidayRegistrationRetention(func() calendar.Date { return today }).DeleteExpiredHolidayRegistrations(adminCtx)
			return err
		})
		admin(func(adminCtx context.Context) error {
			_, err := owners.NewBelegRetention(func() calendar.Date { return today }).ApplyBelegRetention(adminCtx)
			return err
		})
	}
	registrations := func() int {
		var count int
		require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registrations WHERE id = ?`, f.siblings).Scan(ctx, &count))
		return count
	}

	// The winter period ends on 15.01.2027; its registrations go the day
	// after the retention of the Träger (#6691).
	afterWinter := calendar.NewDate(2027, time.January, 15).AddDays(holidayDefaultRetentionDays + 1)
	retain(afterWinter)
	require.Zero(t, registrations(), "the deletion run deleted the confirmed registration")
	assert.Equal(t, "10 4711", f.familyNumber(t, "eva.muster@example.org"), "the family keeps its number")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, familyDebtorNumberPath(created.ID, next), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"debtor_number":"10 4711"}`, string(decodeSchoolPortalRaw(t, rec.Body.Bytes())),
		"the next registration of the family reads it")

	// The number was needed until 15.01.2027, so it stays until 31.12.2030.
	retain(calendar.NewDate(2030, time.December, 31))
	assert.Equal(t, "10 4711", f.familyNumber(t, "eva.muster@example.org"), "three years after the end of the year it is still kept")
	retain(calendar.NewDate(2031, time.January, 1))
	assert.Equal(t, "-", f.familyNumber(t, "eva.muster@example.org"), "then the family without a document goes")
	var trail int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing_events
		WHERE organization_id = ? AND care_contract_id IS NULL`, f.office.organization).Scan(ctx, &trail))
	assert.Zero(t, trail, "and its trail with it")
}
