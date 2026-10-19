package requestreviewhttp_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// correctionFixture is the approved enrollment child a booking correction acts
// on: phase, request, child linked to an enrolled student, plus two offerings
// the child is booked into.
type correctionFixture struct {
	phase    *capability.Phase
	child    *capability.RequestChild
	ganztag  *testpkg.CareOffering
	mittag   *testpkg.CareOffering
	tenantID int64
}

func setupCorrectionFixture(t *testing.T, tc *testContext, studentID, tenantID int64, lastName string) *correctionFixture {
	t.Helper()
	// The phase has to cover the day the services see as today.
	var phase *capability.Phase
	if tc.clock != nil {
		phase = testpkg.CreateTestEnrollmentPhaseAround(t, tc.db, calendar.DateFromTime(tc.clock()))
	} else {
		phase = testpkg.CreateTestEnrollmentPhase(t, tc.db)
	}
	ganztag := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "Ganztag")
	mittag := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "Mittagessen")

	request := &capability.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Erzieh",
		GuardianLastName:  "Ungsberechtigt",
		GuardianEmail:     fmt.Sprintf("korrektur-%d@example.test", time.Now().UnixNano()),
		StatusToken:       fmt.Sprintf("tok-%d", testpkg.UniqueSuffix()),
	}
	request.TenantID = tenantID
	owner := api.NewEnrollmentBookingFixture()
	ownerCtx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), tc.db)
	require.NoError(t, testpkg.TenantCommand1(t, ownerCtx, testpkg.SetupTestDB(t), owner.InsertRequest, request))

	child := &capability.RequestChild{
		RequestID:        request.ID,
		FirstName:        "Zkorrektur",
		LastName:         lastName,
		DateOfBirth:      capability.Date(studentsTestToday.AddDays(-2500)),
		Status:           capability.ChildStatusApproved,
		CreatedStudentID: &studentID,
	}
	child.TenantID = tenantID
	require.NoError(t, testpkg.TenantCommand1(t, ownerCtx, testpkg.SetupTestDB(t), owner.InsertChild, child))

	for _, offering := range []*testpkg.CareOffering{ganztag, mittag} {
		link := &capability.RequestChildOffering{
			RequestChildID: child.ID,
			CareOfferingID: offering.ID,
		}
		link.TenantID = tenantID
		require.NoError(t, owner.InsertRequestChildOffering(ownerCtx, link))
	}

	return &correctionFixture{phase: phase, child: child, ganztag: ganztag, mittag: mittag, tenantID: tenantID}
}

// insertPendingOfferingChangeRequest stores the parent's submission. Creating
// it is the parents portal's job, out of this router's scope; the decision is
// what this test drives through the production route.
func insertPendingOfferingChangeRequest(
	t *testing.T,
	tc *testContext,
	fixture *correctionFixture,
	studentID, submittedBy int64,
) *careplan.OfferingChangeRequest {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"offerings": []any{
			map[string]any{"offering_id": fixture.ganztag.ID},
			map[string]any{"offering_id": fixture.mittag.ID},
		},
	})
	require.NoError(t, err)
	row := &careplan.OfferingChangeRequest{
		StudentID:      studentID,
		RequestChildID: fixture.child.ID,
		SubmittedBy:    submittedBy,
		Payload:        payload,
		EffectiveFrom:  string(studentsTestToday.AddDays(30)),
		Status:         careplan.OfferingChangePending,
	}
	row.TenantID = fixture.tenantID
	err = tc.db.NewRaw(`INSERT INTO enrollment.offering_change_requests
		(tenant_id, student_id, request_child_id, submitted_by, payload, effective_from, status)
		VALUES (?, ?, ?, ?, ?::jsonb, ?, ?) RETURNING id, created_at, updated_at`,
		row.TenantID, row.StudentID, row.RequestChildID, row.SubmittedBy,
		string(payload), row.EffectiveFrom, row.Status).Scan(t.Context(), row)
	require.NoError(t, err)
	return row
}
