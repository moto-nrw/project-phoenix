// Router-level tests of the holiday allocation workflow (ADR 0206, #6555):
// a rebooking, a new capacity and the Büro's decision by criteria change the
// places in Enrollment and the frozen contributions in Parent Billing in one
// Träger transaction. A refused contribution rolls back everything the
// command wrote, and a retry runs as a new unit.
package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// refuseHolidayContributions makes every new contribution version fail, as
// a refusal of Parent Billing at the last step of the workflow would. The
// trigger is global, so its test owns an isolated database.
func refuseHolidayContributions(t *testing.T, f reviewFixture) func() {
	t.Helper()
	_, err := f.db.ExecContext(t.Context(), `CREATE FUNCTION parent_billing.reject_workflow_contribution() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'injected contribution failure'; END $$;
 CREATE TRIGGER reject_workflow_contribution BEFORE INSERT ON parent_billing.holiday_contributions
 FOR EACH ROW EXECUTE FUNCTION parent_billing.reject_workflow_contribution()`)
	require.NoError(t, err)
	return func() {
		_, err := f.db.ExecContext(t.Context(), `DROP TRIGGER reject_workflow_contribution ON parent_billing.holiday_contributions;
 DROP FUNCTION parent_billing.reject_workflow_contribution()`)
		require.NoError(t, err)
	}
}

// traegerOutbox counts every mail the Träger queued.
func (f reviewFixture) traegerOutbox(t *testing.T) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE organization_id = ?`, f.office.organization).
		Scan(context.Background(), &count))
	return count
}

// ADR 0206, acceptance criterion 2: a contribution Parent Billing refuses
// rolls back each way completely. For the rebooking and the capacity the
// confirmed registration that moves up keeps its waiting booking, the week
// its capacity, Ben his place; for the decision a direct place that changes
// the discount of a confirmed sibling stays pending. No contribution version
// and no mail is written. Once Parent Billing accepts again, each request
// runs as a new unit. Both families share one isolated database, because
// the refusing trigger is global.
func TestHolidayAllocationWorkflowRollsBackOnARefusedContribution(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := newReviewFixture(t, "Träger Workflow Rückabwicklung")
	first, second := f.offer.Weeks[0], f.offer.Weeks[1]
	f.setCapacity(t, first.ID, 2)
	tom := f.submitPlaces(t, "tom.workflow@example.org", map[string][]int{"Tom": {0}})
	require.Equal(t, "waitlisted", tom.Placements[0].Placement)
	require.Equal(t, http.StatusOK, f.confirmWith(t, tom.ID, nil).Code)
	frozen := f.contributionVersions(t, tom.ID)
	require.Len(t, frozen, 1)
	ben := f.list(t, "").Registrations[0].Children[1]
	rebook := map[string]any{"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}}}
	a, sibling, candidate := refusedDecisionFixture(t)
	outbox, decisionOutbox := f.traegerOutbox(t), a.traegerOutbox(t)
	allow := refuseHolidayContributions(t, f)

	assertUnchanged := func(t *testing.T, what string) {
		t.Helper()
		week := f.places(t)[first.ID]
		require.NotNil(t, week.Capacity, what)
		assert.Equal(t, 2, *week.Capacity, "%s: the capacity rolls back", what)
		assert.Equal(t, 2, week.Placed, "%s: Ben keeps his place", what)
		require.Len(t, week.Waitlist, 1, what)
		assert.Equal(t, tom.ID, week.Waitlist[0].RegistrationID, "%s: Tom stays on the waiting list", what)
		assert.Equal(t, frozen, f.contributionVersions(t, tom.ID), "%s: no contribution version", what)
		assert.Equal(t, outbox, f.traegerOutbox(t), "%s: no mail is queued", what)
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, f.siblings, ben.ID), rebook)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assertUnchanged(t, "rebooking")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayCapacityPath(f.period.ID, first.ID), map[string]any{"capacity": 3})
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assertUnchanged(t, "capacity")
	rec = a.decide(t, decision(candidate, "placed", ""))
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	board := a.board(t)
	require.Len(t, board.Weeks[0].Pending, 1, "the decision rolls back")
	assert.Equal(t, 1, board.Weeks[0].Placed, "the place stays free")
	assert.Equal(t, []int64{4500}, a.contributionVersions(t, sibling), "the sibling keeps its contribution")
	assert.Equal(t, decisionOutbox, a.traegerOutbox(t), "no decision mail is queued")

	allow()
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, f.siblings, ben.ID), rebook)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.places(t)[first.ID].Waitlist, "the retry moves Tom up")
	assert.Equal(t, 1, f.mails(t, "holiday_place_granted", tom.ID))
	assert.Len(t, f.contributionVersions(t, tom.ID), 2, "the retry freezes Tom's new contribution")
	assert.Equal(t, first.TimeModels[0].PriceCents, f.contributionTotal(t, tom.ID))
	rec = a.decide(t, decision(candidate, "placed", ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{4500, 2250}, a.contributionVersions(t, sibling), "the retry refreezes the confirmed sibling")
	assert.Equal(t, 1, a.mails(t, "holiday_place_granted", candidate.RegistrationID))
}

// refusedDecisionFixture is a period by criteria with a confirmed child that
// has a place and a pending sibling whose direct place halves the confirmed
// child's contribution. It returns the sibling with the place and the
// pending candidate.
func refusedDecisionFixture(t *testing.T) (allocationFixture, int64, allocationCandidateBody) {
	t.Helper()
	a := newAllocationFixture(t, "Träger Workflow Entscheidung")
	billing := traegerColleague(t, a.reviewFixture, identityaccess.TraegerRoleBilling)
	createContributionRule(t, a.reviewFixture, billing, siblingHalf())
	first := a.submitCriteria(t, "erstes.workflow@example.org", false, false)
	rec := a.decide(t, decision(a.board(t).Weeks[0].Pending[0], "placed", ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, http.StatusOK, a.act(t, first.ID, "confirm").Code)
	require.Equal(t, []int64{4500}, a.contributionVersions(t, first.ID))
	body := placesRegistration(a.reviewFixture, "zweites.workflow@example.org", map[string][]int{"Ida": {0}})
	body["criteria"] = map[string]bool{fmt.Sprint(a.working): true}
	child := body["children"].([]map[string]any)[0]
	child["criteria"] = map[string]bool{fmt.Sprint(a.sibling): true}
	child["bookings"].([]map[string]any)[0]["time_model_id"] = a.offer.Weeks[0].TimeModels[1].ID
	rec = publicJSON(t, a.public, http.MethodPost, registrationPath(a.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	second := decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
	rec = a.merge(t, a.office.token, first.ID, second.ID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	a.setCapacity(t, a.offer.Weeks[0].ID, 2)
	candidate := a.board(t).Weeks[0].Pending[0]
	require.Equal(t, second.ID, candidate.RegistrationID)
	return a, first.ID, candidate
}
