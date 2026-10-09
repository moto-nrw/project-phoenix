package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Approving next school year's re-enrollment of a child the school still cares
// for keeps the running class and enrollment start until the new year begins;
// a confirmed edit only re-plans the switch, and the switch applies on the
// first day of the new year (#3917).
func TestDecisionService_Decide_ExistingStudentRenewalKeepsRunningClassUntilYearStarts(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	newYearStart := timezone.NewDate(2030, 8, 1)
	newYearEnd := timezone.NewDate(2031, 7, 31)
	env.sourcePhase.Kind = enrollmentModels.PhaseKindSchoolYear
	env.sourcePhase.ServiceStartDate = capability.Date(newYearStart)
	env.sourcePhase.ServiceEndDate = capability.Date(newYearEnd)
	require.NoError(t, env.repos.Enrollment().UpdatePhase(ctx, enrollmentAPI.OwnerPhaseForTest(env.sourcePhase)))

	runningFrom := timezone.NewDate(2026, 8, 1)
	runningUntil := timezone.NewDate(2027, 7, 31)
	existing := testpkg.CreateTestStudent(t, env.db, "Ida", "Laufend", "1a")
	existing.Status = usersModels.StudentStatusActive
	existing.EnrolledFrom = &runningFrom
	existing.EnrolledUntil = &runningUntil
	require.NoError(t, env.repos.Student.Update(ctx, existing))

	requestID, childID := submitReEnrollment(t, env, "Eltern", "Laufend", "renewal-running@example.com", nil,
		"Ida", "Laufend", map[string]any{"agb": true, "data_processing": true, "email_contact": true, "photo": true})
	matchChildToExistingStudent(t, env, childID, existing.ID)

	_, err := env.decision.Decide(ctx, enrollmentAPI.DecideInput{
		RequestID: requestID, ChildID: childID, Status: capability.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	student, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusActive, student.Status)
	assert.Equal(t, "1a", student.SchoolClass, "the running class stays until the new school year")
	require.NotNil(t, student.EnrolledFrom)
	assert.Equal(t, runningFrom, *student.EnrolledFrom, "the running enrollment start stays")
	require.NotNil(t, student.EnrolledUntil)
	assert.Equal(t, newYearEnd, *student.EnrolledUntil, "the window's end reaches into the new school year")

	// A confirmed edit moves the child to grade 3: the plan follows, the
	// running class does not.
	child, err := enrollmentAPI.ReadOwnerChildForTest(ctx, env.repos.Enrollment(), childID)
	require.NoError(t, err)
	grade := int16(3)
	child.TargetGradeLevel = &grade
	require.NoError(t, enrollmentAPI.UpdateOwnerChildForTest(ctx, env.repos.Enrollment(), child))
	_, err = changeRequestApplierForTest(t, env).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: env.creatorID,
	})
	require.NoError(t, err)
	student, err = env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, "1a", student.SchoolClass, "an edit before the new school year keeps the running class")

	owner := decisionOwner(t, env.decision)
	applied, err := owner.ApplyDueClassSwitches(ctx, newYearStart.AddDays(-1))
	require.NoError(t, err)
	assert.Equal(t, 0, applied, "nothing switches before the new school year")

	applied, err = owner.ApplyDueClassSwitches(ctx, newYearStart)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	student, err = env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, "3", student.SchoolClass, "the edited target applies when the new school year starts")
	assert.Equal(t, runningFrom, *student.EnrolledFrom)
}

// A child the school does not care for right now gets next year's class and
// window at once, as before: it is pending until the new year starts anyway.
func TestDecisionService_Decide_InactiveExistingStudentRenewalSwitchesAtOnce(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	newYearStart := timezone.NewDate(2030, 8, 1)
	newYearEnd := timezone.NewDate(2031, 7, 31)
	env.sourcePhase.Kind = enrollmentModels.PhaseKindSchoolYear
	env.sourcePhase.ServiceStartDate = capability.Date(newYearStart)
	env.sourcePhase.ServiceEndDate = capability.Date(newYearEnd)
	require.NoError(t, env.repos.Enrollment().UpdatePhase(ctx, enrollmentAPI.OwnerPhaseForTest(env.sourcePhase)))

	existing := testpkg.CreateTestStudent(t, env.db, "Ole", "Pause", "1a")
	existing.Status = usersModels.StudentStatusInactive
	require.NoError(t, env.repos.Student.Update(ctx, existing))

	requestID, childID := submitReEnrollment(t, env, "Eltern", "Pause", "renewal-inactive@example.com", nil,
		"Ole", "Pause", map[string]any{"agb": true, "data_processing": true, "email_contact": true, "photo": true})
	matchChildToExistingStudent(t, env, childID, existing.ID)

	_, err := env.decision.Decide(ctx, enrollmentAPI.DecideInput{
		RequestID: requestID, ChildID: childID, Status: capability.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	student, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusPending, student.Status)
	assert.Equal(t, "2", student.SchoolClass)
	require.NotNil(t, student.EnrolledFrom)
	assert.Equal(t, newYearStart, *student.EnrolledFrom)

	applied, err := decisionOwner(t, env.decision).ApplyDueClassSwitches(ctx, newYearStart)
	require.NoError(t, err)
	assert.Equal(t, 0, applied, "nothing was planned for a child switched at once")
}
