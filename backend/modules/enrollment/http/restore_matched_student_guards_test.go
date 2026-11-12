//go:build testhooks

package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	api "github.com/moto-ogs/project-phoenix/api"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRestoreMatchedStudentOrderedDuplicateAndNullGuards(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		duplicate int
		nullFirst bool
	}{
		{"first_duplicate", 0, false}, {"last_duplicate", 2, false}, {"null_before_last_duplicate", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, requestID, childIDs, submitAgain := restoreMatchFailureFixture(t)
			require.Same(t, testpkg.SetupIsolatedTestDB(t), env.db)
			ctx := testpkg.Ctx(t)
			if tc.nullFirst {
				require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
					return env.repos.Enrollment().UpdateMatchedStudent(txCtx, childIDs[0], nil)
				}))
			}
			require.NoError(t, submitAgain(ctx, tc.duplicate))
			schedule := &restoreMatchSchedule{DecisionRequests: env.repos.Enrollment()}
			counter := testpkg.CaptureQueries(t, env.db)
			defer counter.Stop()
			outcome, callErr := restoreMatchDecision(env, schedule).RestoreWithdrawn(ctx, requestID, env.creatorID)
			counter.Stop()
			t.Logf("guard=%s outcome=%+v error=%v SQL=%q", tc.name, outcome, callErr, counter.Queries())
			require.Nil(t, outcome)
			require.ErrorIs(t, callErr, enrollment.ErrRestoreDuplicateActive)
			wantChecks := tc.duplicate + 1
			start := 0
			if tc.nullFirst {
				wantChecks--
				start++
			}
			require.Equal(t, wantChecks, schedule.checks)
			queries := counter.Matching(func(q string) bool { return containsMatchExists(q) })
			require.Len(t, queries, wantChecks)
			for i, query := range queries {
				// Check the ordered child exclusions in the real EXISTS statements.
				require.Contains(t, query, fmt.Sprintf("rc.id <> %d", childIDs[start+i]))
			}
			logRestoreMatchPhysicalState(t, env, requestID)
		})
	}
}

func containsMatchExists(q string) bool {
	return strings.Contains(q, "enrollment.request_children") && strings.Contains(q, "rc.matched_student_id =")
}

func TestRestoreMatchedStudentInactivePhaseDoesNotAcquire(t *testing.T) {
	t.Parallel()
	env, requestID, _, _ := restoreMatchFailureFixture(t)
	ctx := testpkg.Ctx(t)
	env.sourcePhase.IsActive = false
	require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		return env.repos.Enrollment().UpdatePhase(txCtx, env.sourcePhase)
	}))
	schedule := &restoreMatchSchedule{DecisionRequests: env.repos.Enrollment()}
	outcome, callErr := restoreMatchDecision(env, schedule).RestoreWithdrawn(ctx, requestID, env.creatorID)
	t.Logf("inactive_phase outcome=%+v error=%v", outcome, callErr)
	require.Nil(t, outcome)
	require.ErrorIs(t, callErr, enrollment.ErrRestorePhaseInactive)
	require.Zero(t, schedule.locks)
	require.Zero(t, schedule.checks)
	logRestoreMatchPhysicalState(t, env, requestID)
}

type restoreAfterAuditFailure struct {
	enrollmentCompose.RestorationAudit
	failure  error
	recorded bool
}

func (a *restoreAfterAuditFailure) RecordRestoration(ctx context.Context, restoration enrollmentCompose.Restoration) error {
	if err := a.RestorationAudit.RecordRestoration(ctx, restoration); err != nil {
		return err
	}
	a.recorded = true
	return a.failure
}

func TestRestoreMatchedStudentAuditFailureRollsBack(t *testing.T) {
	t.Parallel()
	env, requestID, _, _ := restoreMatchFailureFixture(t)
	// This is an audit-port failure after a real append, not a fabricated DB
	// error or evidence that the removed SQL could safely be reused.
	failure := errors.New("audit acknowledgement failed")
	audit := &restoreAfterAuditFailure{RestorationAudit: owners.EnrollmentDecisionRestorations(env.reposAudit.EnrollmentRestorationAudit), failure: failure}
	decision := newTestDecisionService(enrollmentCompose.DecisionDependencies{
		Requests: env.repos.Enrollment(), Children: env.repos.Enrollment(), Phases: env.repos.Enrollment(), Restorations: audit,
	}, nil)
	counter := testpkg.CaptureQueries(t, env.db)
	defer counter.Stop()
	outcome, callErr := decision.RestoreWithdrawn(testpkg.Ctx(t), requestID, env.creatorID)
	counter.Stop()
	t.Logf("audit_failure outcome=%+v error=%v SQL=%q driver_DML_rows=%d", outcome, callErr, counter.Queries(), counter.WriteRows())
	require.True(t, audit.recorded)
	require.Nil(t, outcome)
	require.ErrorIs(t, callErr, failure)
	require.EqualError(t, callErr, "restore: write audit event: "+failure.Error())
	require.Positive(t, counter.WriteRows(), "writes occurred before rollback; never claim zero DML")
	logRestoreMatchPhysicalState(t, env, requestID)
}

func logRestoreMatchPhysicalState(t *testing.T, env *decisionTestEnv, requestID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	children, childrenErr := enrollmentAPI.ReadOwnerRequestChildrenForTest(ctx, env.repos.Enrollment(), requestID)
	request, requestErr := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), requestID)
	var audit []*api.EnrollmentFlowRestorationAudit
	auditErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(readCtx context.Context, tx bun.Tx) error {
		var err error
		audit, err = readRestoreMatchAudit(readCtx, tx, testpkg.Tenant(t), requestID)
		return err
	})
	state, encodeErr := json.Marshal(struct {
		Children any
		Request  any
		Audit    any
	}{children, request, audit})
	t.Logf("physical_state=%s children_error=%v request_error=%v audit_error=%v encode_error=%v", state, childrenErr, requestErr, auditErr, encodeErr)
	require.NoError(t, childrenErr)
	require.NoError(t, requestErr)
	require.NoError(t, auditErr)
	require.NoError(t, encodeErr)
	require.Len(t, children, 3)
	for _, child := range children {
		require.Equal(t, enrollment.ChildStatusWithdrawn, child.Status)
	}
	require.NotNil(t, request.WithdrawnAt)
	require.Empty(t, audit)
}

// Read the same audit projection as listRestorationAuditRows, scoped to the
// ambient transaction so the lock/rollback probes can inspect uncommitted state.
func readRestoreMatchAudit(ctx context.Context, db bun.IDB, tenantID, requestID int64) ([]*api.EnrollmentFlowRestorationAudit, error) {
	if tenantID <= 0 || requestID <= 0 {
		return nil, errors.New("restoration probe requires tenant and request")
	}
	rows := []*api.EnrollmentFlowRestorationAudit{}
	err := db.NewSelect().Model(&rows).
		ModelTableExpr(`audit.enrollment_restorations AS "enrollment_restoration"`).
		Where("enrollment_restoration.tenant_id = ? AND enrollment_restoration.request_id = ?", tenantID, requestID).
		OrderExpr("enrollment_restoration.id ASC").Scan(ctx)
	return rows, err
}

// Prepare the birthday through the existing typed root's People Directory
// port before Intake resolves the real student and guardian relationship.
func setRestoreMatchBirthday(ctx context.Context, env *decisionTestEnv, personID int64, day calendar.Date) error {
	person, err := env.repos.Person.FindByID(ctx, personID)
	if err != nil {
		return err
	}
	person.Birthday = &day
	return env.repos.Person.Update(ctx, person)
}
