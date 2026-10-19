package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
)

// NewEnrollmentAudience is the Enrollment owner the retained parent
// announcement store reads its pending-enrollment audience from. The People
// Directory contract suite may not import Enrollment test support, so it
// takes the owner from here.
var NewEnrollmentAudience = enrollmentOwner.New

// CreateTestUnstampedEnrollmentApplicant submits one enrollment request for
// guardianEmail that no account is stamped on (guardian_account_id NULL), with
// one submitted child, in an inactive phase of tenantID. It is the
// invite-accept-backfill-failed edge case the pending-enrollment audience
// still reaches by e-mail.
func CreateTestUnstampedEnrollmentApplicant(tb testing.TB, db *bun.DB, tenantID int64, guardianEmail string) {
	tb.Helper()
	owner := enrollmentOwner.New()
	ctx := WithTenantRuntime(tb, TenantContext(tenantID), db)

	phase := &enrollmentOwner.Phase{
		Name:                      "Testphase",
		ServiceStartDate:          "2026-09-01",
		ServiceEndDate:            "2027-07-31",
		CareOverflowMode:          "waitlist",
		CareOfferingSelectionMode: "optional",
	}
	phase.TenantID = tenantID
	require.NoError(tb, TenantCommand1(tb, ctx, db, owner.InsertPhase, phase))

	request := &enrollmentOwner.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Sabine",
		GuardianLastName:  "Schneider",
		GuardianEmail:     guardianEmail,
		GuardianAccountID: nil,
		StatusToken:       fmt.Sprintf("tok-%d", time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	require.NoError(tb, TenantCommand1(tb, ctx, db, owner.InsertRequest, request))

	require.NoError(tb, TenantCommand1(tb, ctx, db, owner.InsertChild, &enrollmentOwner.RequestChild{
		RequestID:   request.ID,
		FirstName:   "Felix",
		LastName:    "Schneider",
		DateOfBirth: "2019-03-03",
		Status:      enrollmentOwner.ChildStatusSubmitted,
	}))
}
