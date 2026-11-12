// Router-level tests for siblings from separate holiday registrations
// (#5628, ADR 0115 addendum): the Träger-Büro merges registrations of one
// period as siblings, the sibling rule ranks their children together, a
// confirmed registration gets a new version of its frozen contribution, a
// split restores it, an issued invoice locks the group, the group shares
// one debtor number, the audit trail of every registration records the
// change, and no other period, Träger or right reaches it.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

type siblingGroupBody struct {
	ID              int64   `json:"id"`
	RegistrationIDs []int64 `json:"registration_ids"`
}

type siblingContributionBody struct {
	contributionBody
	Siblings []struct {
		RegistrationID int64  `json:"registration_id"`
		Status         string `json:"status"`
		Children       []struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"children"`
	} `json:"siblings"`
	SiblingDebtorNumber string `json:"sibling_debtor_number"`
	SiblingsInvoiced    bool   `json:"siblings_invoiced"`
}

func siblingGroupsPath(periodID int64) string {
	return fmt.Sprintf("/traeger/holiday-contributions/%d/sibling-groups", periodID)
}

func siblingSplitPath(periodID, registrationID int64) string {
	return fmt.Sprintf("%s/registrations/%d", siblingGroupsPath(periodID), registrationID)
}

// submitChild sends a registration of one child of the family name who
// books the first week, "bis 13 Uhr" (45,00 €) or "bis 16 Uhr" (67,50 €).
func (f reviewFixture) submitChild(t *testing.T, period traegerPeriodLink, offer holidayOfferBody, email, child string, model int) int64 {
	t.Helper()
	body := siblingsRegistration(offer, email)
	first := body["children"].([]map[string]any)[0]
	first["first_name"] = child
	first["bookings"] = []map[string]any{{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[model].ID}}
	body["children"] = []map[string]any{first}
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
}

func (f reviewFixture) merge(t *testing.T, token string, ids ...int64) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodPost, siblingGroupsPath(f.period.ID), map[string]any{"registration_ids": ids})
}

func (f reviewFixture) split(t *testing.T, token string, id int64) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodDelete, siblingSplitPath(f.period.ID, id), nil)
}

func (f reviewFixture) siblingContribution(t *testing.T, id int64) siblingContributionBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, id), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[siblingContributionBody](t, rec.Body.Bytes())
}

func (f reviewFixture) siblingGroups(t *testing.T) []siblingGroupBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, siblingGroupsPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]siblingGroupBody](t, rec.Body.Bytes())
}

// contributionVersions lists the total of every frozen version of a
// registration, oldest first.
func (f reviewFixture) contributionVersions(t *testing.T, id int64) []int64 {
	t.Helper()
	var totals []int64
	require.NoError(t, f.db.NewRaw(`SELECT sum(total_cents) FROM parent_billing.holiday_contributions
		WHERE holiday_registration_id = ? GROUP BY version ORDER BY version`, id).Scan(context.Background(), &totals))
	return totals
}

func assertRefusedWith(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
	got, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, code, got)
}

func TestSiblingsFromTwoRegistrationsRankTogether(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Rang")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	first := f.submitChild(t, f.period, f.offer, "mama.rang@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.rang@example.org", "Tom", 0)
	assert.Equal(t, int64(4500), f.contribution(t, second).TotalCents, "alone each child pays in full")
	assert.Empty(t, f.siblingGroups(t))

	rec := f.merge(t, f.office.token, second, first, first)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	group := decodeSchoolPortalData[siblingGroupBody](t, rec.Body.Bytes())
	assert.Equal(t, []int64{first, second}, group.RegistrationIDs)
	assert.Equal(t, []siblingGroupBody{group}, f.siblingGroups(t))

	earlier := f.siblingContribution(t, first)
	later := f.siblingContribution(t, second)
	assert.Equal(t, int64(4500), earlier.TotalCents, "on a tie the child of the registration sent earlier is the first child")
	assert.Equal(t, int64(2250), later.TotalCents, "the other child pays half")
	require.Len(t, later.Children, 1, "a registration shows only its own children")
	assert.Equal(t, []string{"base:4500", "sibling:-2250"}, lineKinds(later.Children[0].Lines))
	assert.Equal(t, 2, *later.Children[0].Lines[1].ChildRank)
	require.Len(t, later.Siblings, 1)
	assert.Equal(t, first, later.Siblings[0].RegistrationID)
	assert.Equal(t, "Mia", later.Siblings[0].Children[0].FirstName, "the registration names the children of the other one")

	rec = f.confirmWith(t, second, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(2250), decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes()).Contribution.TotalCents,
		"the confirmation freezes the half price")

	// A third registration with the dearer child joins the group; it now
	// ranks first and the frozen registration gets a new version.
	third := f.submitChild(t, f.period, f.offer, "oma.rang@example.org", "Ida", 1)
	rec = f.merge(t, f.office.token, third, second)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{first, second, third}, decodeSchoolPortalData[siblingGroupBody](t, rec.Body.Bytes()).RegistrationIDs,
		"a merge with a member takes the whole group")
	assert.Len(t, f.siblingGroups(t), 1)
	assert.Equal(t, int64(6750), f.contribution(t, third).TotalCents)
	assert.Equal(t, int64(2250), f.contribution(t, first).TotalCents)
	assert.Equal(t, []int64{2250, 2250}, f.contributionVersions(t, second),
		"the rank of the frozen child moves from 2 to 3, its amount stays: a second version with the new rank")
}

func TestMergeWithAConfirmedRegistrationAddsAVersionAndSplitRestoresIt(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Fassung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	first := f.submitChild(t, f.period, f.offer, "mama.fassung@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.fassung@example.org", "Tom", 1)
	require.Equal(t, http.StatusOK, f.confirmWith(t, first, map[string]any{"debtor_number": "10 4711"}).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, second, nil).Code)
	assert.Equal(t, []int64{4500}, f.contributionVersions(t, first))

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := startBillingRun(t, f, billing, f.period.ID)
	totals := func() map[int64]int64 {
		out := map[int64]int64{}
		for _, document := range billingRun(t, f, billing, run.ID).Documents {
			out[document.RegistrationID] = document.TotalCents
		}
		return out
	}
	assert.Equal(t, int64(4500), totals()[first])

	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second).Code)
	assert.Equal(t, []int64{4500, 2250}, f.contributionVersions(t, first), "a new version; the old one stays")
	assert.Equal(t, []int64{6750}, f.contributionVersions(t, second), "the dearer child keeps its contribution")
	frozen := f.siblingContribution(t, first)
	assert.True(t, frozen.Frozen)
	assert.Equal(t, int64(2250), frozen.TotalCents)
	require.NotNil(t, frozen.ConfirmedAt, "the version keeps the time of the confirmation")
	assert.Equal(t, map[int64]int64{first: 2250, second: 6750}, totals(), "the open run shows the new amount")
	assert.Equal(t, "10 4711", f.debtorNumberDetail(t, second).DebtorNumber, "the group pays with the number of the first registration")

	history := f.debtorNumberDetail(t, first).History
	require.NotEmpty(t, history)
	merged := history[len(history)-1]
	assert.Equal(t, "sibling_merge", merged.Action)
	assert.Equal(t, []any{float64(second)}, merged.Details["siblings"], "the history names the other registration")

	rec := f.split(t, f.office.token, second)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Empty(t, f.siblingGroups(t), "a group of one registration goes")
	assert.Equal(t, []int64{4500, 2250, 4500}, f.contributionVersions(t, first), "the split restores the contribution without the group")
	assert.Equal(t, map[int64]int64{first: 4500, second: 6750}, totals())
	assert.Equal(t, "sibling_split", f.debtorNumberDetail(t, second).History[len(f.debtorNumberDetail(t, second).History)-1].Action)
	assertRefusedWith(t, f.split(t, f.office.token, second), http.StatusNotFound, common.CodeParentBillingSiblingGroupNotFound)
}

func TestSiblingsLockOnceAnInvoiceIsIssued(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Rechnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	first := f.submitChild(t, f.period, f.offer, "mama.rechnung@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.rechnung@example.org", "Tom", 0)
	third := f.submitChild(t, f.period, f.offer, "oma.rechnung@example.org", "Ida", 0)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, first, nil).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, second, nil).Code)

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assertRefusedWith(t, f.split(t, f.office.token, second), http.StatusConflict, common.CodeParentBillingSiblingGroupInvoiced)
	assert.True(t, f.siblingContribution(t, first).SiblingsInvoiced, "the page learns that the group is locked")
	assert.True(t, f.siblingContribution(t, second).SiblingsInvoiced)
	assertRefusedWith(t, f.merge(t, f.office.token, third, first), http.StatusConflict, common.CodeParentBillingSiblingGroupInvoiced)
	assert.Len(t, f.siblingGroups(t), 1, "the refusals change nothing")
	assert.Equal(t, []int64{2250}, f.contributionVersions(t, second))
}

func TestMergeRefusesWhatIsNoSiblingGroup(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Ablehnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Geschwister Ablehnung Fremd", identityaccess.TraegerRoleAdministration)
	cancelled := f.submitChild(t, f.period, f.offer, "storno.ablehnung@example.org", "Mia", 0)
	require.Equal(t, http.StatusOK, f.act(t, cancelled, "cancel").Code)
	otherPeriod := openHolidays(t, f.holidayRegistrationFixture, f.office)
	elsewhere := f.submitChild(t, otherPeriod, holidayOffer(t, f.holidayRegistrationFixture, otherPeriod.PublicToken),
		"anders.ablehnung@example.org", "Tom", 0)

	assertRefusedWith(t, f.merge(t, f.office.token, f.siblings), http.StatusBadRequest, common.CodeParentBillingSiblingGroupInvalid)
	assertRefusedWith(t, f.merge(t, f.office.token, f.siblings, f.siblings), http.StatusBadRequest, common.CodeParentBillingSiblingGroupInvalid)
	assertRefusedWith(t, f.merge(t, f.office.token, f.siblings, cancelled), http.StatusConflict, common.CodeEnrollmentHolidayRegistrationStateConflict)
	assertRefusedWith(t, f.merge(t, f.office.token, f.siblings, elsewhere), http.StatusNotFound, common.CodeEnrollmentHolidayRegistrationEntryNotFound)
	assertRefusedWith(t, f.split(t, f.office.token, f.siblings), http.StatusNotFound, common.CodeParentBillingSiblingGroupNotFound)

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, siblingGroupsPath(f.period.ID)},
		{http.MethodPost, siblingGroupsPath(f.period.ID)},
		{http.MethodDelete, siblingSplitPath(f.period.ID, f.siblings)},
	} {
		body := map[string]any{"registration_ids": []int64{f.siblings, f.again}}
		rec := traegerJSON(t, f.traeger, billing, request.method, request.path, body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}
	rec := f.merge(t, foreign.token, f.siblings, f.again)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger merges nothing: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, siblingGroupsPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]siblingGroupBody](t, rec.Body.Bytes()))
	assert.Empty(t, f.siblingGroups(t), "no refusal left a group")
}

func TestDeletedRegistrationLeavesItsSiblingGroup(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Löschung")
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, f.siblings, f.again).Code)
	_, err := f.db.ExecContext(t.Context(), `DELETE FROM enrollment.holiday_registrations WHERE id = ?`, f.again)
	require.NoError(t, err)
	var members, groups int
	require.NoError(t, f.db.NewRaw(`SELECT
		(SELECT count(*) FROM parent_billing.holiday_sibling_group_members WHERE organization_id = ?),
		(SELECT count(*) FROM parent_billing.holiday_sibling_groups WHERE organization_id = ?)`,
		f.office.organization, f.office.organization).Scan(context.Background(), &members, &groups))
	assert.Zero(t, members, "the deleted registration and the last one left go out of the group")
	assert.Zero(t, groups, "a group of one registration goes")
}

func TestSiblingGroupListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Budget")
	list := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, siblingGroupsPath(f.period.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, f.siblings, f.again).Code)
	one := list()
	first := f.submitChild(t, f.period, f.offer, "mama.budget@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.budget@example.org", "Tom", 0)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second).Code)
	assert.Len(t, list().Statements, len(one.Statements), "the list reads the same statements for any number of groups")
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_sibling_groups", one)
}

// TestConfirmationRanksSiblingsInTheOrderTheyWereSent pins the tie-break of
// a confirmation (#5628, review of #5655): two children of the same price
// in two merged registrations. The child of the registration sent earlier is
// the first child, whichever registration the Träger-Büro confirms first,
// and the confirmation freezes what the preview showed.
func TestConfirmationRanksSiblingsInTheOrderTheyWereSent(t *testing.T) {
	t.Parallel()
	for name, earlierFirst := range map[string]bool{"earlier confirmed first": true, "later confirmed first": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newReviewFixture(t, "Geschwister Reihenfolge "+name)
			billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
			createContributionRule(t, f, billing, siblingHalf())
			earlier := f.submitChild(t, f.period, f.offer, "mama.reihe@example.org", "Mia", 0)
			later := f.submitChild(t, f.period, f.offer, "papa.reihe@example.org", "Tom", 0)
			require.Equal(t, http.StatusOK, f.merge(t, f.office.token, earlier, later).Code)
			assert.Equal(t, int64(4500), f.contribution(t, earlier).TotalCents)
			assert.Equal(t, int64(2250), f.contribution(t, later).TotalCents)

			order := []int64{earlier, later}
			if !earlierFirst {
				order = []int64{later, earlier}
			}
			for _, id := range order {
				rec := f.confirmWith(t, id, nil)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			}
			assert.Equal(t, []int64{4500}, f.contributionVersions(t, earlier),
				"the child of the registration sent earlier pays in full, frozen once")
			assert.Equal(t, []int64{2250}, f.contributionVersions(t, later),
				"the child of the later registration pays half, frozen once")
		})
	}
}

// TestRemovedDebtorNumberLeavesTheWholeSiblingGroup pins the review of
// #5655: where the Träger does not ask for the debtor number, removing it
// from one registration removes it from the other confirmed registrations
// of the group, and a later confirmation in the group inherits none.
func TestRemovedDebtorNumberLeavesTheWholeSiblingGroup(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Debitor entfernen")
	first := f.submitChild(t, f.period, f.offer, "mama.debitor@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.debitor@example.org", "Tom", 0)
	third := f.submitChild(t, f.period, f.offer, "oma.debitor@example.org", "Ida", 0)
	require.Equal(t, http.StatusOK, f.confirmWith(t, first, map[string]any{"debtor_number": "10 4711"}).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, second, nil).Code)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second, third).Code)
	require.Equal(t, "10 4711", f.debtorNumberDetail(t, second).DebtorNumber, "the group pays with one number")

	rec := f.changeDebtorNumber(t, f.office.token, first, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.debtorNumberDetail(t, first).DebtorNumber)
	assert.Empty(t, f.debtorNumberDetail(t, second).DebtorNumber, "the removal reaches the confirmed sibling")
	assert.Empty(t, f.siblingContribution(t, third).SiblingDebtorNumber, "the group proposes no removed number")
	require.Equal(t, http.StatusOK, f.confirmWith(t, third, nil).Code)
	assert.Empty(t, f.debtorNumberDetail(t, third).DebtorNumber, "a later confirmation inherits no removed number")
}

// TestMergeWaitsForACancellationOfTheSamePeriod pins the review of #5655:
// a merge holds the period in Enrollment before it reads the group, so a
// cancellation that is still running finishes first and the merge refuses
// the cancelled registration instead of grouping it.
func TestMergeWaitsForACancellationOfTheSamePeriod(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Sperre")
	first := f.submitChild(t, f.period, f.offer, "mama.sperre@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.sperre@example.org", "Tom", 0)

	// A cancellation holds the period, as Enrollment's change does, and
	// cancels the registration the merge reads first before it commits.
	cancellation, err := f.db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = cancellation.Rollback() }()
	_, err = cancellation.ExecContext(t.Context(), `SELECT id FROM enrollment.holiday_periods WHERE id = ? FOR UPDATE`, f.period.ID)
	require.NoError(t, err)
	_, err = cancellation.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations SET status = 'cancelled' WHERE id = ?`, first)
	require.NoError(t, err)

	merged := make(chan *httptest.ResponseRecorder, 1)
	go func() { merged <- f.merge(t, f.office.token, first, second) }()
	select {
	case rec := <-merged:
		t.Fatalf("the merge did not wait for the period: %d %s", rec.Code, rec.Body.String())
	case <-time.After(500 * time.Millisecond):
	}
	require.NoError(t, cancellation.Commit())

	rec := <-merged
	assertRefusedWith(t, rec, http.StatusConflict, common.CodeEnrollmentHolidayRegistrationStateConflict)
	assert.Empty(t, f.siblingGroups(t), "no group with the cancelled registration")
}

// TestCancelledSiblingPassesOnNoDebtorNumber pins the review of #5655: a
// cancelled registration keeps the number it had. When the group changes
// its number, a later confirmation takes the group's current number, not
// the cancelled registration's old one.
func TestCancelledSiblingPassesOnNoDebtorNumber(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Debitor Storno")
	first := f.submitChild(t, f.period, f.offer, "mama.storno@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.storno@example.org", "Tom", 0)
	third := f.submitChild(t, f.period, f.offer, "oma.storno@example.org", "Ida", 0)
	require.Equal(t, http.StatusOK, f.confirmWith(t, first, map[string]any{"debtor_number": "10 4711"}).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, second, nil).Code)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second, third).Code)
	require.Equal(t, http.StatusOK, f.act(t, first, "cancel").Code)

	rec := f.changeDebtorNumber(t, f.office.token, second, "20 0815")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "10 4711", f.debtorNumberDetail(t, first).DebtorNumber, "the cancelled registration keeps its number")
	assert.Equal(t, "20 0815", f.siblingContribution(t, third).SiblingDebtorNumber, "the group proposes its current number")

	require.Equal(t, http.StatusOK, f.confirmWith(t, third, nil).Code)
	assert.Equal(t, "20 0815", f.debtorNumberDetail(t, third).DebtorNumber)
	assert.Equal(t, "20 0815", f.debtorNumberDetail(t, second).DebtorNumber, "the confirmation brings back no old number")
}

// TestSiblingInvoiceLocksTheWholeGroup pins the review of #5655: when an
// issued invoice bills only another registration of the group, the page of
// this registration learns that the group is locked, and a split is refused.
func TestSiblingInvoiceLocksTheWholeGroup(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Rechnung Gruppe")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	first := f.submitChild(t, f.period, f.offer, "mama.gruppe@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.gruppe@example.org", "Tom", 0)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second).Code)
	assert.False(t, f.siblingContribution(t, second).SiblingsInvoiced, "no invoice yet")
	require.Equal(t, http.StatusOK, f.confirmWith(t, first, nil).Code)

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	other := f.siblingContribution(t, second)
	assert.False(t, other.Invoiced, "no invoice bills this registration")
	assert.True(t, other.SiblingsInvoiced, "an invoice of the sibling locks the group")
	assertRefusedWith(t, f.split(t, f.office.token, second), http.StatusConflict, common.CodeParentBillingSiblingGroupInvoiced)
}

// TestCostTakeoverOfOneSiblingRegistration pins the combination of #5628
// and #5627 through the routes: the later registration of a group is
// confirmed with a cost bearer. The sibling discount comes first, the cost
// bearer takes over what is left, the family pays nothing for it, and the
// earlier registration keeps its full price.
func TestCostTakeoverOfOneSiblingRegistration(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Kostenträger")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	bearer := createCostBearer(t, f, billing, youthOffice())
	earlier := f.submitChild(t, f.period, f.offer, "mama.kosten@example.org", "Mia", 0)
	later := f.submitChild(t, f.period, f.offer, "papa.kosten@example.org", "Tom", 0)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, earlier, later).Code)

	rec := f.confirmWith(t, later, map[string]any{"cost_bearer_id": bearer.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	frozen := f.siblingContribution(t, later)
	assert.Equal(t, int64(0), frozen.TotalCents, "the family pays nothing for the registration with the cost takeover")
	require.Len(t, frozen.Children, 1)
	assert.Equal(t, []string{"base:4500", "sibling:-2250", "cost_takeover:-2250"}, lineKinds(frozen.Children[0].Lines),
		"the cost bearer gets the amount after the sibling discount")
	assert.Equal(t, int64(4500), f.contribution(t, earlier).TotalCents, "the earlier registration keeps its full price")

	require.Equal(t, http.StatusOK, f.confirmWith(t, earlier, nil).Code)
	assert.Equal(t, []int64{4500}, f.contributionVersions(t, earlier))
	assert.Equal(t, []int64{0}, f.contributionVersions(t, later), "confirming the sibling keeps the cost takeover as it was")
}
