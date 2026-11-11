//go:build testhooks

package enrollmenthttp_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	api "github.com/moto-nrw/project-phoenix/api"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Real Intake fixture shared by the fault schedules below. The trusted school
// eligibility flag is not a complete HTTP/session authorization test.
func restoreMatchFailureFixture(t *testing.T) (*decisionTestEnv, int64, []int64, func(context.Context, ...int) error) {
	t.Helper()
	matchedCount := 3
	testpkg.OwnTenant(t)
	db := testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	require.Same(t, db, env.db)
	ctx := testpkg.Ctx(t)
	input := validSubmission(t, env.sourcePhase.ID)
	input.GuardianEmail = fmt.Sprintf("restore-match-%d@example.test", matchedCount)
	childCount := matchedCount
	if childCount == 0 {
		childCount = 1
	}
	input.Children = make([]enrollmentAPI.SubmitChild, childCount)
	for i := range input.Children {
		input.Children[i] = enrollmentAPI.SubmitChild{
			FirstName: "Lina" + string(rune('A'+i)), LastName: "Lockprobe",
			DateOfBirth:      validSubmission(t, env.sourcePhase.ID).Children[0].DateOfBirth,
			TargetGradeLevel: testpkg.Int16Ptr(2),
		}
	}
	expectedStudentByName := make(map[string]int64, matchedCount)
	if matchedCount > 0 {
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		input.GuardianAccountID = &chain.AccountID
		input.GuardianEmail = chain.Email
		input.GuardianFirstName, input.GuardianLastName = "Sabine", "Schneider"
		input.Children[0].FirstName, input.Children[0].LastName = "Felix", "Schneider"
		expectedStudentByName["Felix\x00Schneider"] = chain.StudentID
		personIDs := make(map[string]int64, matchedCount)
		personIDs["Felix\x00Schneider"] = chain.PersonID
		for i := 1; i < matchedCount; i++ {
			child := input.Children[i]
			student := testpkg.CreateTestStudent(t, db, child.FirstName, child.LastName, "2b")
			testpkg.CreateTestStudentGuardianLink(t, db, student.ID, chain.GuardianProfileID, "primary_guardian")
			key := child.FirstName + "\x00" + child.LastName
			expectedStudentByName[key], personIDs[key] = student.ID, student.PersonID
		}
		env.sourcePhase.Audience = enrollment.PhaseAudienceExistingStudents
		eligible := false
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			if err := env.repos.Enrollment().UpdatePhase(txCtx, env.sourcePhase); err != nil {
				return err
			}
			for _, child := range input.Children {
				key := child.FirstName + "\x00" + child.LastName
				if err := setRestoreMatchBirthday(txCtx, env, personIDs[key], child.DateOfBirth); err != nil {
					return err
				}
				granted, err := env.repos.StudentGuardian.AccountHasStudentPermission(txCtx, chain.AccountID, expectedStudentByName[key], testpkg.Tenant(t), "parent_portal.enrollment.submit")
				if err != nil {
					return err
				}
				if !granted {
					return fmt.Errorf("fixture guardian lacks actual enrollment.submit for child %s", child.FirstName)
				}
				eligible = eligible || granted
			}
			return nil
		}))
		require.True(t, eligible)
		input.GuardianSubmitEligible = eligible
	}
	// Real student matching and per-child guardian authorization are wired;
	// settings/outbox remain the existing explicit suite collaborators.
	requestSvc := newTestRequestService(owners.EnrollmentIntakeSources{
		Bookings: requestTestBookingCommands(), Requests: env.repos.Enrollment(), Children: env.repos.Enrollment(),
		Guardians: env.repos.Enrollment(), CareOfferingRepo: enrollmentCompose.NewCareOfferingRecords(env.reposCare.CarePlan),
		Catalog: env.repos.Enrollment(), StudentRepo: env.repos.Student, GuardianAuthorizer: env.repos.StudentGuardian, GuardianHold: testGuardianHold(env.db, env.repos),
		SchoolRepo: factorySchools{repos: env.reposSchools}, RateLimitRepo: env.repos.Enrollment(),
		LateInviteRepo: env.repos.Enrollment(), Uploads: env.repos.Enrollment(), OutboxEnqueuer: env.outbox,
		Settings: env.settings, FrontendURL: "http://localhost:3000", ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
	})
	submitted, err := requestSvc.Submit(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, submitted)
	require.NotNil(t, submitted.Request)
	require.NoError(t, requestSvc.Withdraw(ctx, submitted.Request.StatusToken, 0))
	children, err := enrollmentAPI.ReadOwnerRequestChildrenForTest(ctx, env.repos.Enrollment(), submitted.Request.ID)
	require.NoError(t, err)
	require.Len(t, children, childCount)
	expectedIDs := make([]int64, 0, childCount)
	expectedPins := make(map[int64]int64, matchedCount)
	for _, child := range children {
		require.Equal(t, enrollment.ChildStatusWithdrawn, child.Status)
		expectedIDs = append(expectedIDs, child.ID)
		if want, ok := expectedStudentByName[child.FirstName+"\x00"+child.LastName]; ok {
			require.NotNil(t, child.MatchedStudentID, "real existing_students Submit must generate the pin")
			require.Equal(t, want, *child.MatchedStudentID)
			expectedPins[child.ID] = want
		} else {
			require.Nil(t, child.MatchedStudentID)
		}
	}
	require.Len(t, expectedPins, matchedCount)
	input.GuardianEmail = "restore-competing-guardian@example.test"
	return env, submitted.Request.ID, expectedIDs, func(ctx context.Context, indexes ...int) error {
		competing := input
		if len(indexes) > 0 {
			competing.Children = make([]enrollmentAPI.SubmitChild, 0, len(indexes))
			for _, index := range indexes {
				competing.Children = append(competing.Children, input.Children[index])
			}
		}
		_, err := requestSvc.Submit(ctx, competing)
		return err
	}
}

type restoreMatchSchedule struct {
	enrollmentCompose.DecisionRequests
	locks, checks int
	beforeLock    func(context.Context, int) error
	beforeCheck   func(context.Context, int) error
	afterCheck    func(context.Context, int) error
}

func (s *restoreMatchSchedule) AcquireExistingStudentMatchLock(ctx context.Context, phaseID int64) error {
	s.locks++
	if s.beforeLock != nil {
		if err := s.beforeLock(ctx, s.locks); err != nil {
			return err
		}
	}
	return s.DecisionRequests.AcquireExistingStudentMatchLock(ctx, phaseID)
}

func (s *restoreMatchSchedule) HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludedChildID int64) (bool, error) {
	s.checks++
	if s.beforeCheck != nil {
		if err := s.beforeCheck(ctx, s.checks); err != nil {
			return false, err
		}
	}
	has, err := s.DecisionRequests.HasActiveRequestForMatchedStudent(ctx, phaseID, studentID, excludedChildID)
	if err == nil && s.afterCheck != nil {
		if hookErr := s.afterCheck(ctx, s.checks); hookErr != nil {
			return false, hookErr
		}
	}
	return has, err
}

func restoreMatchDecision(env *decisionTestEnv, requests enrollmentCompose.DecisionRequests) enrollmentAPI.DecisionService {
	return newTestDecisionService(enrollmentCompose.DecisionDependencies{
		Requests: requests, Children: env.repos.Enrollment(), Phases: env.repos.Enrollment(),
		Restorations: owners.EnrollmentDecisionRestorations(env.reposAudit.EnrollmentRestorationAudit),
	}, nil)
}

// The callbacks arrange real transaction/driver failures, never fabricated DB
// errors. In particular ctx.Err()==nil does not imply an ambient tx is usable.
func TestRestoreMatchedStudentFailureAttribution(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"cancel_after_first_exists", "closed_tx_after_first_exists", "aborted_tx_after_first_exists", "terminated_connection_after_first_exists", "first_lock_failure", "exists_failure_1", "exists_failure_2", "exists_failure_3"} {
		t.Run(mode, func(t *testing.T) {
			env, requestID, _, _ := restoreMatchFailureFixture(t)
			baseCtx := testpkg.Ctx(t)
			ctx, cancel := context.WithCancel(baseCtx)
			defer cancel()
			schedule := &restoreMatchSchedule{DecisionRequests: env.repos.Enrollment()}
			decision := restoreMatchDecision(env, schedule)
			counter := testpkg.CaptureQueries(t, env.db)
			defer counter.Stop()
			var callbackErr error
			var outcome *enrollmentAPI.RestoreOutcome
			var operationErr error
			callErr := testpkg.WithTenantTx(t, counter.Context(ctx), env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				abort := func() error {
					_, err := tx.NewRaw("SELECT 1 / 0").Exec(txCtx)
					callbackErr = err
					if err == nil {
						return errors.New("division by zero unexpectedly succeeded")
					}
					return nil
				}
				if mode == "first_lock_failure" {
					schedule.beforeLock = func(_ context.Context, n int) error {
						if n == 1 {
							return abort()
						}
						return nil
					}
				}
				if strings.HasPrefix(mode, "exists_failure_") {
					target := int(mode[len(mode)-1] - '0')
					schedule.beforeCheck = func(_ context.Context, n int) error {
						if n == target {
							return abort()
						}
						return nil
					}
				}
				schedule.afterCheck = func(_ context.Context, n int) error {
					if n != 1 {
						return nil
					}
					switch mode {
					case "cancel_after_first_exists":
						cancel()
					case "closed_tx_after_first_exists":
						callbackErr = tx.Rollback()
						return callbackErr
					case "aborted_tx_after_first_exists":
						return abort()
					case "terminated_connection_after_first_exists":
						var pid int
						if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
							return err
						}
						var terminated bool
						callbackErr = env.db.NewRaw("SELECT pg_terminate_backend(?)", pid).Scan(baseCtx, &terminated)
						if callbackErr != nil {
							return callbackErr
						}
						if !terminated {
							return errors.New("own transaction backend was not terminated")
						}
					}
					return nil
				}
				var err error
				outcome, err = decision.RestoreWithdrawn(txCtx, requestID, env.creatorID)
				operationErr = err
				return err
			})
			counter.Stop()
			children, childrenErr := enrollmentAPI.ReadOwnerRequestChildrenForTest(baseCtx, env.repos.Enrollment(), requestID)
			request, requestErr := enrollmentAPI.ReadOwnerRequestForTest(baseCtx, env.repos.Enrollment(), requestID)
			var audit []*api.EnrollmentFlowRestorationAudit
			auditErr := testpkg.WithTenantTx(t, baseCtx, env.db, testpkg.Tenant(t), func(readCtx context.Context, tx bun.Tx) error {
				var err error
				audit, err = readRestoreMatchAudit(readCtx, tx, testpkg.Tenant(t), requestID)
				return err
			})
			snapshot, encodeErr := json.Marshal(struct {
				Children any
				Request  any
				Audit    any
				SQL      []string
				Outcome  any
			}{children, request, audit, counter.Queries(), outcome})
			t.Logf("operation_error=%v", operationErr)
			t.Logf("fault=%s locks=%d exists=%d error=%v callback_error=%v ctx_error=%v children_error=%v request_error=%v audit_error=%v encode_error=%v state=%s", mode, schedule.locks, schedule.checks, callErr, callbackErr, ctx.Err(), childrenErr, requestErr, auditErr, encodeErr, snapshot)
			require.NoError(t, childrenErr)
			require.NoError(t, requestErr)
			require.NoError(t, auditErr)
			require.NoError(t, encodeErr)
			require.Nil(t, outcome)
			require.Error(t, callErr)
			require.ErrorIs(t, callErr, operationErr)
			// E4 (#5696): errors after the first duplicate read now arise from
			// the next fresh duplicate read, not a reentrant lock acquisition.
			prefix := "restore: matched-student duplicate check: "
			if mode == "first_lock_failure" {
				prefix = "restore: acquire existing-student match lock: "
			}
			require.True(t, strings.HasPrefix(callErr.Error(), prefix), "public error attribution: %v", callErr)
			switch mode {
			case "cancel_after_first_exists":
				require.ErrorIs(t, callErr, context.Canceled)
				require.EqualError(t, operationErr, prefix+"failed to check active request for matched student: context canceled")
			case "closed_tx_after_first_exists":
				require.NoError(t, callbackErr)
				require.NoError(t, ctx.Err())
				require.ErrorIs(t, callErr, sql.ErrTxDone)
				require.EqualError(t, operationErr, prefix+"failed to check active request for matched student: "+sql.ErrTxDone.Error())
			case "terminated_connection_after_first_exists":
				require.NoError(t, callbackErr)
				var state pgdriver.Error
				require.ErrorAs(t, callErr, &state)
				require.Equal(t, "57P01", state.Field('C'))
				require.EqualError(t, operationErr, prefix+"failed to check active request for matched student: "+state.Error())
			default:
				require.Error(t, callbackErr)
				var state pgdriver.Error
				require.ErrorAs(t, callErr, &state)
				require.Equal(t, "25P02", state.Field('C'))
				detail := "failed to check active request for matched student: "
				if mode == "first_lock_failure" {
					detail = "failed to acquire existing-student match lock: "
				}
				require.EqualError(t, operationErr, prefix+detail+state.Error())
			}
			require.NotNil(t, request.WithdrawnAt)
			require.Empty(t, audit)
			require.Len(t, children, 3)
			for _, child := range children {
				require.Equal(t, enrollment.ChildStatusWithdrawn, child.Status)
				require.NotNil(t, child.MatchedStudentID)
			}
		})
	}
}
