package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// insertAdjustment writes one row of the append-only booking log the office's
// corrections land in, with a controlled instant so the keyset order is
// deterministic.
func insertAdjustment(
	t *testing.T,
	tc *testContext,
	child *enrollmentModels.RequestChild,
	studentID, tenantID, accountID int64,
	source, reason string,
	changedAt time.Time,
) *auditModels.EnrollmentOfferingAdjustment {
	t.Helper()
	actor := "Olga Office"
	entry := &auditModels.EnrollmentOfferingAdjustment{
		RequestID:         child.RequestID,
		RequestChildID:    child.ID,
		StudentID:         studentID,
		ActorAccountID:    accountID,
		ActorRole:         "admin",
		ActorNameSnapshot: &actor,
		Reason:            reason,
		Source:            source,
		Before:            json.RawMessage(`[{"offering_id":"1","offering_name":"Mittagessen","days_of_week_mode":"fixed","selected_days":["mon"]}]`),
		After:             json.RawMessage(`[]`),
		ChangedAt:         changedAt,
	}
	entry.TenantID = tenantID
	_, err := tc.db.NewInsert().Model(entry).
		ModelTableExpr(`audit.enrollment_offering_adjustments AS "enrollment_offering_adjustment"`).
		Exec(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		// context.Background(), not t.Context(): the test context is already
		// canceled when cleanups run.
		_, cleanupErr := tc.db.NewDelete().TableExpr("audit.enrollment_offering_adjustments").
			Where("id = ?", entry.ID).Exec(context.Background())
		if cleanupErr != nil {
			t.Logf("cleanup of offering adjustment %d failed: %v", entry.ID, cleanupErr)
		}
	})
	return entry
}

// insertApprovedRequestChild creates the enrollment request and approved child
// the booking log hangs off, linked to an already enrolled student.
func insertApprovedRequestChild(t *testing.T, tc *testContext, studentID, tenantID int64, lastName string) *enrollmentModels.RequestChild {
	t.Helper()
	phase := testpkg.CreateTestEnrollmentPhase(t, tc.db)
	request := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Erzieh",
		GuardianLastName:  "Ungsberechtigt",
		GuardianEmail:     fmt.Sprintf("korrektur-%d@example.test", time.Now().UnixNano()),
		StatusToken:       fmt.Sprintf("tok-%d", time.Now().UnixNano()),
	}
	request.TenantID = tenantID
	_, err := tc.db.NewInsert().Model(request).ModelTableExpr(`enrollment.requests AS "request"`).Exec(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().TableExpr("enrollment.requests").Where("id = ?", request.ID).Exec(context.Background())
	})

	child := &enrollmentModels.RequestChild{
		RequestID:        request.ID,
		FirstName:        "Zkorrektur",
		LastName:         lastName,
		DateOfBirth:      timezone.TodayDate().AddDays(-2500),
		Status:           enrollmentModels.ChildStatusApproved,
		CreatedStudentID: &studentID,
	}
	child.TenantID = tenantID
	_, err = tc.db.NewInsert().Model(child).ModelTableExpr(`enrollment.request_children AS "request_child"`).Exec(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().TableExpr("enrollment.request_children").Where("id = ?", child.ID).Exec(context.Background())
	})
	return child
}

// A correction the office made itself shows up in the central history as its
// own row kind, while the adjustment an approved parent request writes on the
// side does not — that one is already in the list as the decided request
// (#2436).
func TestAggregatedChangeRequests_RouterDirectCorrections(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Agg", "CorrectionReviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AggCorrectionGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Zkorrektur", "Kindlein", "AL3")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	child := insertApprovedRequestChild(t, tc, student.ID, student.TenantID, "Kindlein")
	base := time.Now().UTC().Add(-time.Hour)
	direct := insertAdjustment(t, tc, child, student.ID, student.TenantID, account.ID,
		auditModels.OfferingAdjustmentSourceDirect, "Telefonisch gemeldet", base)
	insertAdjustment(t, tc, child, student.ID, student.TenantID, account.ID,
		auditModels.OfferingAdjustmentSourceRequest, "Elternanfrage #1 freigegeben", base.Add(-time.Minute))

	claims := testutil.TeacherTestClaims(int(account.ID))
	perms := []string{"users:read", "users:update"}

	fetch := func(t *testing.T, query string) aggListEnvelope {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?"+query, nil), claims, perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var env aggListEnvelope
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		return env
	}

	history := fetch(t, "view=history&search=Zkorrektur")
	require.Len(t, history.Data.Items, 1, "only the direct correction belongs in the history")
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
	assert.Equal(t, strconv.FormatInt(direct.ID, 10), row.ID)
	assert.Equal(t, "Zkorrektur Kindlein", row.StudentName)
	assert.Equal(t, "Olga Office", row.ChangedByName)
	assert.Equal(t, "Telefonisch gemeldet", row.Reason)
	require.Len(t, row.Diff, 1)
	assert.Equal(t, "Mittagessen", row.Diff[0].Label)
	assert.Equal(t, "Mo", row.Diff[0].Old)
	assert.Equal(t, "abgemeldet", row.Diff[0].New)

	// The working list never shows corrections, not even when asked for them.
	assert.Empty(t, fetch(t, "search=Zkorrektur").Data.Items)
	assert.Empty(t, fetch(t, "types=direct_correction&search=Zkorrektur").Data.Items)
}
