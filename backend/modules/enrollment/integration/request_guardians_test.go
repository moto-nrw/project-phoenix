package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRequestGuardiansIsolationRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	var schools, requests, guardians, profiles, phases, children []int64
	for _, school := range []string{"first", "second"} {
		t.Run(school, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			schoolID := testpkg.Tenant(t)
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			profile := testpkg.CreateTestGuardianProfile(t, db, "guardian-owner")
			request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: fmt.Sprintf("guardian-owner-%d", schoolID)}
			require.NoError(t, module.InsertRequest(ctx, request))
			requestID := request.ID
			require.Equal(t, schoolID, request.TenantID)
			child := &enrollment.RequestChild{RequestID: requestID, FirstName: "Child", LastName: "Last", DateOfBirth: "2020-03-29"}
			require.NoError(t, module.InsertChild(ctx, child))
			require.Equal(t, schoolID, child.TenantID)
			children = append(children, child.ID)
			guardian := &enrollment.RequestGuardian{RequestID: requestID, FirstName: "Co", LastName: "Guardian"}
			require.NoError(t, module.CreateRequestGuardian(ctx, guardian))
			require.Equal(t, schoolID, guardian.TenantID)
			schools = append(schools, schoolID)
			requests = append(requests, requestID)
			guardians = append(guardians, guardian.ID)
			profiles = append(profiles, profile.ID)
			phases = append(phases, phase.ID)
		})
	}
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schools[0])
	localChild, err := module.ChildByID(ctx, children[0])
	require.NoError(t, err)
	require.Equal(t, enrollment.Date("2020-03-29"), localChild.DateOfBirth)
	_, err = module.ChildByID(ctx, children[1])
	require.ErrorIs(t, err, sql.ErrNoRows)
	childRows, err := module.ChildrenForRequests(ctx, requests)
	require.NoError(t, err)
	require.Len(t, childRows, 1)
	require.Equal(t, children[0], childRows[0].ID)
	foreignChild := &enrollment.RequestChild{RequestID: requests[1], FirstName: "Wrong", LastName: "School", DateOfBirth: "2020-03-29"}
	require.ErrorIs(t, module.InsertChild(ctx, foreignChild), sql.ErrNoRows)
	childFailure := errors.New("after child data update")
	err = testpkg.WithTenantTx(t, ctx, db, schools[0], func(txCtx context.Context, _ bun.Tx) error {
		localChild.FirstName = "Changed"
		require.NoError(t, module.UpdateChildData(txCtx, localChild))
		return childFailure
	})
	require.ErrorIs(t, err, childFailure)
	localChild, err = module.ChildByID(ctx, children[0])
	require.NoError(t, err)
	require.Equal(t, "Child", localChild.FirstName)
	_, err = module.RequestByID(ctx, requests[1], false)
	require.ErrorIs(t, err, sql.ErrNoRows)
	listed, err := module.RequestsByID(ctx, requests)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, requests[0], listed[0].ID)
	adminRows, err := module.AdminRequests(ctx, enrollment.RequestListFilters{})
	require.NoError(t, err)
	require.Len(t, adminRows, 1)
	foreignCreate := &enrollment.Request{PhaseID: phases[1], GuardianFirstName: "Wrong", GuardianLastName: "School", GuardianEmail: "wrong@example.test", StatusToken: "foreign-request"}
	require.ErrorIs(t, module.InsertRequest(ctx, foreignCreate), sql.ErrNoRows)
	err = tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		request, err := module.RequestByToken(adminCtx, fmt.Sprintf("guardian-owner-%d", schools[1]), false)
		require.NoError(t, err)
		require.Equal(t, requests[1], request.ID, "authorized token discovery can identify another school")
		_, err = module.RequestByID(testpkg.ContextForTenant(adminCtx, schools[0]), requests[1], false)
		require.ErrorIs(t, err, sql.ErrNoRows, "ID lookup stays scoped even when RLS is bypassed")
		return nil
	})
	require.NoError(t, err)
	updateFailure := errors.New("after guardian edit")
	err = testpkg.WithTenantTx(t, ctx, db, schools[0], func(txCtx context.Context, _ bun.Tx) error {
		request, err := module.RequestByID(txCtx, requests[0], true)
		require.NoError(t, err)
		request.GuardianFirstName = "Changed"
		require.NoError(t, module.UpdateRequestGuardian(txCtx, request, false))
		return updateFailure
	})
	require.ErrorIs(t, err, updateFailure)
	unchanged, err := module.RequestByID(ctx, requests[0], false)
	require.NoError(t, err)
	require.Equal(t, "First", unchanged.GuardianFirstName)
	_, err = module.PinDecisionNotificationMode(ctx, requests[1], "digest")
	require.Error(t, err, "a foreign request cannot be pinned")
	_, err = module.PinDecisionNotificationMode(ctx, requests[0], "invalid")
	require.ErrorContains(t, err, "invalid enrollment decision notification mode")
	rollbackPin := errors.New("after notification pin")
	err = testpkg.WithTenantTx(t, ctx, db, schools[0], func(txCtx context.Context, _ bun.Tx) error {
		mode, pinErr := module.PinDecisionNotificationMode(txCtx, requests[0], "digest")
		require.NoError(t, pinErr)
		require.Equal(t, "digest", mode)
		return rollbackPin
	})
	require.ErrorIs(t, err, rollbackPin)
	mode, err := module.PinDecisionNotificationMode(ctx, requests[0], "immediate")
	require.NoError(t, err)
	require.Equal(t, "immediate", mode, "a rolled-back pin must not constrain the retry")
	mode, err = module.PinDecisionNotificationMode(ctx, requests[0], "digest")
	require.NoError(t, err)
	require.Equal(t, "immediate", mode, "the first committed pin wins")
	rows, err := module.RequestGuardians(ctx, requests)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, guardians[0], rows[0].ID)
	require.Error(t, module.CreateRequestGuardian(ctx, &enrollment.RequestGuardian{RequestID: requests[1], FirstName: "Wrong", LastName: "School"}))
	require.Error(t, module.StampRequestGuardianProfile(ctx, guardians[1], profiles[0]))
	require.NoError(t, module.DeleteRequestGuardians(ctx, requests[1]))
	failure := errors.New("after guardian write")
	for _, operation := range []string{"create", "stamp", "delete"} {
		t.Run(operation, func(t *testing.T) {
			err := testpkg.WithTenantTx(t, ctx, db, schools[0], func(txCtx context.Context, _ bun.Tx) error {
				switch operation {
				case "create":
					require.NoError(t, module.CreateRequestGuardian(txCtx, &enrollment.RequestGuardian{RequestID: requests[0], FirstName: "Rollback", LastName: "Guardian"}))
				case "stamp":
					require.NoError(t, module.StampRequestGuardianProfile(txCtx, guardians[0], profiles[0]))
				case "delete":
					require.NoError(t, module.DeleteRequestGuardians(txCtx, requests[0]))
				}
				return failure
			})
			require.ErrorIs(t, err, failure)
			rows, err := module.RequestGuardians(ctx, requests)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Nil(t, rows[0].GuardianProfileID)
		})
	}
	require.NoError(t, module.StampRequestGuardianProfile(ctx, guardians[0], profiles[0]))
	rows, err = module.RequestGuardians(ctx, requests)
	require.NoError(t, err)
	require.Equal(t, &profiles[0], rows[0].GuardianProfileID)
	require.NoError(t, module.DeleteRequestGuardians(ctx, requests[0]))
	rows, err = module.RequestGuardians(ctx, requests)
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = module.RequestGuardians(testpkg.ContextForTenant(testpkg.Ctx(t), schools[1]), requests)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, guardians[1], rows[0].ID)

	withdrawnAt := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	readWithdrawal := func(schoolID, requestID int64) *time.Time {
		var at *time.Time
		readCtx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolID)
		err := testpkg.WithTenantTx(t, readCtx, db, schoolID, func(txCtx context.Context, tx bun.Tx) error {
			return tx.NewSelect().TableExpr("enrollment.requests").Column("withdrawn_at").Where("id = ?", requestID).Scan(txCtx, &at)
		})
		require.NoError(t, err)
		return at
	}
	require.NoError(t, module.SetRequestWithdrawal(ctx, requests[1], &withdrawnAt))
	require.Nil(t, readWithdrawal(schools[1], requests[1]))
	err = testpkg.WithTenantTx(t, ctx, db, schools[0], func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.SetRequestWithdrawal(txCtx, requests[0], &withdrawnAt))
		return failure
	})
	require.ErrorIs(t, err, failure)
	require.Nil(t, readWithdrawal(schools[0], requests[0]))
	require.NoError(t, module.SetRequestWithdrawal(ctx, requests[0], &withdrawnAt))
	require.WithinDuration(t, withdrawnAt, *readWithdrawal(schools[0], requests[0]), 0)
	err = testpkg.WithTenantTx(t, ctx, db, schools[0], func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.SetRequestWithdrawal(txCtx, requests[0], nil))
		return failure
	})
	require.ErrorIs(t, err, failure)
	require.WithinDuration(t, withdrawnAt, *readWithdrawal(schools[0], requests[0]), 0)
	require.NoError(t, module.SetRequestWithdrawal(ctx, requests[0], nil))
	require.Nil(t, readWithdrawal(schools[0], requests[0]))
	count, err := module.CountPhaseRequests(ctx, phases[1])
	require.NoError(t, err)
	require.Zero(t, count)
	require.Error(t, module.DeleteRequest(ctx, requests[1]))
	count, err = module.DeletePhaseRequests(ctx, phases[1])
	require.NoError(t, err)
	require.Zero(t, count)
	for _, operation := range []string{"single request", "phase requests"} {
		t.Run(operation, func(t *testing.T) {
			err := testpkg.WithTenantTx(t, ctx, db, schools[0], func(txCtx context.Context, _ bun.Tx) error {
				if operation == "single request" {
					require.NoError(t, module.DeleteRequest(txCtx, requests[0]))
				} else {
					deleted, deleteErr := module.DeletePhaseRequests(txCtx, phases[0])
					require.NoError(t, deleteErr)
					require.Equal(t, 1, deleted)
				}
				return failure
			})
			require.ErrorIs(t, err, failure)
			count, err := module.CountPhaseRequests(ctx, phases[0])
			require.NoError(t, err)
			require.Equal(t, 1, count, "request deletion must roll back with its caller")
		})
	}
	count, err = module.DeletePhaseRequests(ctx, phases[0])
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = module.CountPhaseRequests(testpkg.ContextForTenant(testpkg.Ctx(t), schools[1]), phases[1])
	require.NoError(t, err)
	require.Equal(t, 1, count, "another school's request must survive deletion")
}
