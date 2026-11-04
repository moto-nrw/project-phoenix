// Router-level tests for a cancellation in a sibling group (#5662, ADR 0115
// addendum): the cancelled registration leaves its group at once, the rest
// of the group gets its frozen contribution computed again in the same
// transaction, a registration an issued invoice bills keeps its own and
// shows that a correction may be needed, every registration of the group
// records the change, and the cancellation holds the period like a merge.
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

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type cancelledContributionBody struct {
	siblingContributionBody
	CorrectionNeeded bool `json:"correction_needed"`
}

func (f reviewFixture) cancel(t *testing.T, token string, id int64) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodPost, holidayCancelPath(f.period.ID, id), nil)
}

func (f reviewFixture) cancelledContribution(t *testing.T, id int64) cancelledContributionBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, id), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[cancelledContributionBody](t, rec.Body.Bytes())
}

// lastHistory is the newest change in the history of a registration.
func (f reviewFixture) lastHistory(t *testing.T, id int64) (string, any) {
	t.Helper()
	history := f.debtorNumberDetail(t, id).History
	require.NotEmpty(t, history)
	last := history[len(history)-1]
	return last.Action, last.Details["siblings"]
}

func TestCancellationComputesTheRestOfTheSiblingGroupAgain(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Storno Neuberechnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	first := f.submitChild(t, f.period, f.offer, "mama.neu@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.neu@example.org", "Tom", 0)
	require.Equal(t, http.StatusOK, f.confirmWith(t, first, nil).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, second, nil).Code)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second).Code)
	require.Equal(t, []int64{4500, 2250}, f.contributionVersions(t, second), "in the group the second child pays half")

	rec := f.cancel(t, f.office.token, first)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cancelled := decodeSchoolPortalData[struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, first, cancelled.ID)
	assert.Equal(t, "cancelled", cancelled.Status)

	assert.Equal(t, []int64{4500, 2250, 4500}, f.contributionVersions(t, second),
		"without its sibling the second child is the first child and pays in full, as a new version")
	assert.Equal(t, int64(4500), f.cancelledContribution(t, second).TotalCents)
	assert.Empty(t, f.cancelledContribution(t, second).Siblings, "the cancelled registration left the group")
	assert.False(t, f.cancelledContribution(t, second).CorrectionNeeded, "no invoice, nothing to correct")
	assert.Empty(t, f.siblingGroups(t), "a group of one registration goes")
	assert.Equal(t, []int64{4500}, f.contributionVersions(t, first), "the cancelled registration keeps its contribution")

	action, siblings := f.lastHistory(t, first)
	assert.Equal(t, "sibling_cancel", action)
	assert.Equal(t, []any{float64(second)}, siblings)
	action, siblings = f.lastHistory(t, second)
	assert.Equal(t, "sibling_cancel", action)
	assert.Equal(t, []any{float64(first)}, siblings, "the history of the sibling names the cancelled registration")
}

func TestCancellationKeepsTheRestOfALargerGroupTogether(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Storno Rest")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	first := f.submitChild(t, f.period, f.offer, "mama.rest@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.rest@example.org", "Tom", 0)
	third := f.submitChild(t, f.period, f.offer, "oma.rest@example.org", "Ida", 0)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second, third).Code)
	for _, id := range []int64{first, second, third} {
		require.Equal(t, http.StatusOK, f.confirmWith(t, id, nil).Code)
	}
	require.Equal(t, http.StatusOK, f.cancel(t, f.office.token, first).Code)

	groups := f.siblingGroups(t)
	require.Len(t, groups, 1)
	assert.Equal(t, []int64{second, third}, groups[0].RegistrationIDs)
	assert.Equal(t, int64(4500), f.cancelledContribution(t, second).TotalCents, "the second child moves up to the first")
	assert.Equal(t, int64(2250), f.cancelledContribution(t, third).TotalCents)
	action, siblings := f.lastHistory(t, first)
	assert.Equal(t, "sibling_cancel", action)
	assert.Equal(t, []any{float64(second), float64(third)}, siblings, "the cancelled registration names the rest of its group")
}

func TestCancellationKeepsAnInvoicedSiblingAndShowsTheCorrection(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Storno Rechnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	first := f.submitChild(t, f.period, f.offer, "mama.korrektur@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.korrektur@example.org", "Tom", 0)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, first, second).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, first, nil).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, second, nil).Code)

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, f.cancelledContribution(t, second).CorrectionNeeded, "the invoice bills what the group ranks")

	rec = f.cancel(t, f.office.token, first)
	require.Equal(t, http.StatusOK, rec.Code, "an issued invoice does not stop a cancellation: %s", rec.Body.String())
	assert.Equal(t, []int64{2250}, f.contributionVersions(t, second), "the invoiced registration keeps its contribution")
	invoiced := f.cancelledContribution(t, second)
	assert.True(t, invoiced.Invoiced)
	assert.Equal(t, int64(2250), invoiced.TotalCents)
	assert.True(t, invoiced.CorrectionNeeded, "the page shows that the invoice may need a correction")
	assert.Empty(t, f.siblingGroups(t))
	action, _ := f.lastHistory(t, second)
	assert.Equal(t, "sibling_cancel", action)
}

func TestCancellationRouteKeepsItsRightAndTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Storno Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Geschwister Storno Recht Fremd", identityaccess.TraegerRoleAdministration)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, f.siblings, f.again).Code)

	assert.Equal(t, http.StatusForbidden, f.cancel(t, billing, f.siblings).Code, "the right Abrechnung alone cancels nothing")
	assertRefusedWith(t, f.cancel(t, foreign.token, f.siblings), http.StatusNotFound, common.CodeEnrollmentHolidayPeriodNotFound)
	assert.Len(t, f.siblingGroups(t), 1, "no refusal changed the group")
	assert.Equal(t, "received", f.debtorNumberDetail(t, f.siblings).Status)

	require.Equal(t, http.StatusOK, f.cancel(t, f.office.token, f.siblings).Code)
	assertRefusedWith(t, f.cancel(t, f.office.token, f.siblings), http.StatusConflict, common.CodeEnrollmentHolidayRegistrationStateConflict)
}

// TestCancellationWaitsForTheHeldPeriod pins the lock of #5662: the
// cancellation holds the period before it reads the sibling group, so a
// merge still running finishes first and the cancellation takes the
// registration out of the group the merge made.
func TestCancellationWaitsForTheHeldPeriod(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Storno Sperre")
	first := f.submitChild(t, f.period, f.offer, "mama.sperre.storno@example.org", "Mia", 0)
	second := f.submitChild(t, f.period, f.offer, "papa.sperre.storno@example.org", "Tom", 0)

	// A merge holds the period and groups the two registrations before it
	// commits.
	merge, err := f.db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = merge.Rollback() }()
	_, err = merge.ExecContext(t.Context(), `SELECT id FROM enrollment.holiday_periods WHERE id = ? FOR UPDATE`, f.period.ID)
	require.NoError(t, err)
	var group int64
	require.NoError(t, merge.NewRaw(`INSERT INTO parent_billing.holiday_sibling_groups
		(organization_id, holiday_period_id) VALUES (?, ?) RETURNING id`, f.office.organization, f.period.ID).Scan(t.Context(), &group))
	_, err = merge.ExecContext(t.Context(), `INSERT INTO parent_billing.holiday_sibling_group_members
		(organization_id, holiday_period_id, sibling_group_id, holiday_registration_id) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		f.office.organization, f.period.ID, group, first, f.office.organization, f.period.ID, group, second)
	require.NoError(t, err)

	cancelled := make(chan *httptest.ResponseRecorder, 1)
	go func() { cancelled <- f.cancel(t, f.office.token, first) }()
	select {
	case rec := <-cancelled:
		t.Fatalf("the cancellation did not wait for the period: %d %s", rec.Code, rec.Body.String())
	case <-time.After(500 * time.Millisecond):
	}
	require.NoError(t, merge.Commit())

	rec := <-cancelled
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.siblingGroups(t), "the cancellation saw the group and took the registration out")
	action, _ := f.lastHistory(t, second)
	assert.Equal(t, "sibling_cancel", action)
	var members int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.holiday_sibling_group_members WHERE holiday_registration_id = ?`,
		first).Scan(context.Background(), &members))
	assert.Zero(t, members)
}

// TestRuleChangeRaisesNoCorrectionHintWithoutSiblings pins the review of
// #5662: the hint names siblings, so an invoiced registration that never
// ranked with siblings shows none, even when a later rule change would
// compute it differently.
func TestRuleChangeRaisesNoCorrectionHintWithoutSiblings(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Geschwister Storno Regel")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	exemption := createContributionRule(t, f, billing, benefitsExemption())
	alone := f.submitChild(t, f.period, f.offer, "mama.regel@example.org", "Mia", 0)
	require.Equal(t, http.StatusOK, f.confirmWith(t, alone, map[string]any{"benefits_exemption": true}).Code)

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, fmt.Sprintf("%s/%d", contributionRulesPath, exemption.ID), nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	invoiced := f.cancelledContribution(t, alone)
	assert.True(t, invoiced.Invoiced)
	assert.True(t, invoiced.BenefitsExemption, "the frozen contribution keeps its exemption")
	assert.False(t, invoiced.CorrectionNeeded, "no sibling, no hint about siblings")
}
