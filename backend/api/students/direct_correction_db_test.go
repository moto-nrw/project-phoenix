package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/uptrace/bun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// correctionFixture is the approved enrollment child a booking correction acts
// on: phase, request, child linked to an enrolled student, plus two offerings
// the child is booked into.
type correctionFixture struct {
	phase    *enrollmentModels.Phase
	child    *enrollmentModels.RequestChild
	ganztag  *enrollmentModels.CareOffering
	mittag   *enrollmentModels.CareOffering
	tenantID int64
}

func setupCorrectionFixture(t *testing.T, tc *testContext, studentID, tenantID int64, lastName string) *correctionFixture {
	t.Helper()
	ctx := t.Context()
	phase := testpkg.CreateTestEnrollmentPhase(t, tc.db)
	ganztag := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "Ganztag")
	mittag := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "Mittagessen")

	request := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Erzieh",
		GuardianLastName:  "Ungsberechtigt",
		GuardianEmail:     fmt.Sprintf("korrektur-%d@example.test", time.Now().UnixNano()),
		StatusToken:       fmt.Sprintf("tok-%d", time.Now().UnixNano()),
	}
	request.TenantID = tenantID
	_, err := tc.db.NewInsert().Model(request).ModelTableExpr(`enrollment.requests AS "request"`).Exec(ctx)
	require.NoError(t, err)

	child := &enrollmentModels.RequestChild{
		RequestID:        request.ID,
		FirstName:        "Zkorrektur",
		LastName:         lastName,
		DateOfBirth:      timezone.TodayDate().AddDays(-2500),
		Status:           enrollmentModels.ChildStatusApproved,
		CreatedStudentID: &studentID,
	}
	child.TenantID = tenantID
	_, err = tc.db.NewInsert().Model(child).ModelTableExpr(`enrollment.request_children AS "request_child"`).Exec(ctx)
	require.NoError(t, err)

	for _, offering := range []*enrollmentModels.CareOffering{ganztag, mittag} {
		link := &enrollmentModels.RequestChildOffering{
			RequestChildID: child.ID,
			CareOfferingID: offering.ID,
		}
		link.TenantID = tenantID
		_, err = tc.db.NewInsert().Model(link).
			ModelTableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).Exec(ctx)
		require.NoError(t, err)
	}

	return &correctionFixture{phase: phase, child: child, ganztag: ganztag, mittag: mittag, tenantID: tenantID}
}

// A correction the office makes itself shows up in the central history as its
// own row kind. The booking change an approved parent request writes into the
// very same append-only log does not: that one is already in the list as the
// decided request, and showing both would double it (#2436).
func TestAggregatedChangeRequests_RouterDirectCorrections(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Agg", "CorrectionReviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AggCorrectionGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Zkorrektur", "Kindlein", "AL3")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	fixture := setupCorrectionFixture(t, tc, student.ID, student.TenantID, "Kindlein")
	claims := testutil.AdminTestClaims(int(account.ID))
	perms := []string{"users:read", "users:update"}

	fetch := func(t *testing.T, query string) aggListEnvelope {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?"+query, nil), claims, perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var env aggListEnvelope
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		return env
	}

	// ARRANGE 1 — the office corrects the booking itself, through the very
	// service the admin route calls: the child stays in Ganztag and is taken
	// out of Mittagessen. The frozen before/after snapshots must show that.
	err := testpkg.WithTenantTx(t, t.Context(), tc.db, student.TenantID, func(ctx context.Context, _ bun.Tx) error {
		_, updateErr := tc.resource.EnrollmentDecision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
			RequestID:      fixture.child.RequestID,
			ChildID:        fixture.child.ID,
			Offerings:      []enrollmentService.OfferingAdjustmentSelection{{OfferingID: fixture.ganztag.ID}},
			Reason:         "Telefonisch gemeldet",
			ActorAccountID: account.ID,
			ActorRole:      "admin",
		})
		return updateErr
	})
	require.NoError(t, err)

	// ASSERT 1 — the correction is in the history, as its own row kind.
	history := fetch(t, "view=history&search=Zkorrektur")
	require.Len(t, history.Data.Items, 1)
	assert.Equal(t, "direct_correction", history.Data.Items[0].RequestType)

	var row struct {
		ID            string `json:"id"`
		StudentName   string `json:"student_name"`
		ChangedByName string `json:"changed_by_name"`
		Reason        string `json:"reason"`
		Diff          []struct {
			Label string `json:"label"`
			Old   string `json:"old"`
			New   string `json:"new"`
		} `json:"diff"`
	}
	require.NoError(t, json.Unmarshal(history.Data.Items[0].Data, &row))
	assert.Equal(t, "Zkorrektur Kindlein", row.StudentName)
	assert.Equal(t, "Agg CorrectionReviewer", row.ChangedByName)
	assert.Equal(t, "Telefonisch gemeldet", row.Reason)
	require.NotEmpty(t, row.Diff)
	byLabel := map[string]string{}
	for _, line := range row.Diff {
		byLabel[line.Label] = line.Old + " → " + line.New
	}
	// Only what actually changed: Ganztag stayed, so it carries no line.
	assert.Equal(t, map[string]string{
		fixture.mittag.Name: "alle Betreuungstage → abgemeldet",
	}, byLabel)

	// ARRANGE 2 — a parent asks for a change and the office approves it through
	// the production decide route. That apply writes into the same log.
	pending := insertPendingOfferingChangeRequest(t, tc, fixture, student.ID, account.ID)
	// Eine Freigabe trägt seit #2484 immer das Datum, das die OGS bestätigt —
	// hier das der Anfrage.
	body := strings.NewReader(fmt.Sprintf(
		`{"approve":true,"reason":"Passt","effective_from":%q}`, pending.EffectiveFrom.String()))
	rr := authExec(t, tc,
		testutil.NewRequest("POST", fmt.Sprintf("/offering-change-requests/%d/decide", pending.ID), body),
		claims, perms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// ASSERT 2 — the approval appears exactly once, as the decided request; its
	// side effect on the booking log stays out of the correction feed.
	after := fetch(t, "view=history&search=Zkorrektur")
	types := make([]string, 0, len(after.Data.Items))
	for _, item := range after.Data.Items {
		types = append(types, item.RequestType)
	}
	assert.Equal(t, []string{"offering", "direct_correction"}, types)

	var sources []string
	require.NoError(t, tc.db.NewSelect().TableExpr("audit.enrollment_offering_adjustments").
		Column("source").Where("request_child_id = ?", fixture.child.ID).
		OrderExpr("id").Scan(t.Context(), &sources))
	assert.Equal(t, []string{
		auditModels.OfferingAdjustmentSourceDirect,
		auditModels.OfferingAdjustmentSourceRequest,
	}, sources, "the two write paths must stamp different sources")

	// The working list never shows corrections, not even when asked for them.
	assert.Empty(t, fetch(t, "types=direct_correction&search=Zkorrektur").Data.Items)
	openTypes := fetch(t, "search=Zkorrektur").Data.Items
	for _, item := range openTypes {
		assert.NotEqual(t, "direct_correction", item.RequestType)
	}
}

// Deliberately NOT parallel: the tenant-wide settings cache is process-global state.
func TestOfferingWithdrawalApprovalRequiresUpdateButNotDeletePermission(t *testing.T) {
	tc := setupStudentsRoute(t)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Withdrawal", "Reviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "WithdrawalReviewGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Komplett", "Abmeldung", "WA1")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	require.NoError(t, tc.resource.SettingsService.SetValue(
		testpkg.Ctx(t), configModel.KeyEnrollmentBookingsAuthoritative, true, nil, nil,
	))
	t.Cleanup(func() {
		require.NoError(t, tc.resource.SettingsService.ResetValue(
			testpkg.Ctx(t), configModel.KeyEnrollmentBookingsAuthoritative, nil, nil,
		))
	})

	fixture := setupCorrectionFixture(t, tc, student.ID, student.TenantID, "Abmeldung")
	pending := insertPendingOfferingChangeRequest(t, tc, fixture, student.ID, account.ID)
	_, err := tc.db.NewUpdate().TableExpr("enrollment.offering_change_requests").
		Set(`payload = '{"offerings":[]}'::jsonb`).
		Set("complete_withdrawal_confirmed = TRUE").
		Set("withdrawal_confirmed_by = ?", account.ID).
		Set("withdrawal_confirmed_at = ?", time.Now()).
		Where("id = ?", pending.ID).Exec(t.Context())
	require.NoError(t, err)

	// #2267: reason policy defaults to "both"
	body := strings.NewReader(fmt.Sprintf(
		`{"approve":true,"reason":"Passt so","effective_from":%q,"complete_withdrawal_confirmed":true}`,
		pending.EffectiveFrom.String(),
	))
	response := authExec(t, tc,
		testutil.NewRequest("POST", fmt.Sprintf("/offering-change-requests/%d/decide", pending.ID), body),
		testutil.AdminTestClaims(int(account.ID)), []string{"users:update"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
}

// insertPendingOfferingChangeRequest stores the parent's submission. Creating
// it is the parents portal's job, out of this router's scope; the decision is
// what this test drives through the production route.
func insertPendingOfferingChangeRequest(
	t *testing.T,
	tc *testContext,
	fixture *correctionFixture,
	studentID, submittedBy int64,
) *enrollmentModels.OfferingChangeRequest {
	t.Helper()
	row := &enrollmentModels.OfferingChangeRequest{
		StudentID:      studentID,
		RequestChildID: fixture.child.ID,
		SubmittedBy:    submittedBy,
		Payload: map[string]any{
			"offerings": []any{
				map[string]any{"offering_id": fixture.ganztag.ID},
				map[string]any{"offering_id": fixture.mittag.ID},
			},
		},
		EffectiveFrom: timezone.TodayDate().AddDays(30),
		Status:        enrollmentModels.OfferingChangeStatusPending,
	}
	row.TenantID = fixture.tenantID
	_, err := tc.db.NewInsert().Model(row).
		ModelTableExpr(`enrollment.offering_change_requests AS "offering_change_request"`).Exec(t.Context())
	require.NoError(t, err)
	return row
}
