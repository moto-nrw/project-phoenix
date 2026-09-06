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
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestIntakeCreationRollsBackEachAuthoritativeWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := enrollmentCompose.New()
	for _, stage := range []string{"phase", "request", "child"} {
		t.Run(stage, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			failure := errors.New("after " + stage + " creation")
			create := func(inject bool) error {
				return testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
					phase := &enrollment.Phase{Name: "Atomic intake", ServiceStartDate: "2030-08-01", ServiceEndDate: "2031-07-31"}
					if err := owner.InsertPhase(txCtx, phase); err != nil {
						return err
					}
					if inject && stage == "phase" {
						return failure
					}
					request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "Fixture", GuardianLastName: "Guardian",
						GuardianEmail: "atomic@example.test", StatusToken: "atomic-intake-" + stage}
					if err := owner.InsertRequest(txCtx, request); err != nil {
						return err
					}
					if inject && stage == "request" {
						return failure
					}
					child := &enrollment.RequestChild{RequestID: request.ID, FirstName: "Fixture", LastName: "Child", DateOfBirth: "2023-03-26"}
					if err := owner.InsertChild(txCtx, child); err != nil {
						return err
					}
					if inject {
						return failure
					}
					return nil
				})
			}
			require.ErrorIs(t, create(true), failure)
			assertCounts := func(expected int) {
				for _, table := range []string{"enrollment.phases", "enrollment.requests", "enrollment.request_children"} {
					count, err := db.NewSelect().Table(table).Where("tenant_id = ?", testpkg.Tenant(t)).Count(ctx)
					require.NoError(t, err)
					require.Equal(t, expected, count, table)
				}
			}
			assertCounts(0)
			require.NoError(t, create(false), "retry reuses the status token without a partial prior submission")
			assertCounts(1)
		})
	}
}

func TestIntakeCommandsAndRLSPreserveTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	var requests []*enrollment.Request
	var children []*enrollment.RequestChild
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phase := &enrollment.Phase{Name: name, ServiceStartDate: "2030-08-01", ServiceEndDate: "2031-07-31"}
			require.NoError(t, module.InsertPhase(testpkg.Ctx(t), phase))
			request := &enrollment.Request{
				PhaseID: phase.ID, GuardianFirstName: "Fixture", GuardianLastName: "Guardian",
				GuardianEmail: "intake@example.test", StatusToken: fmt.Sprintf("isolation-%d", phase.ID),
			}
			require.NoError(t, module.InsertRequest(testpkg.Ctx(t), request))
			child := &enrollment.RequestChild{RequestID: request.ID, FirstName: "Fixture", LastName: "Child", DateOfBirth: "2023-03-26"}
			require.NoError(t, module.InsertChild(testpkg.Ctx(t), child))
			require.Equal(t, testpkg.Tenant(t), request.TenantID)
			require.Equal(t, testpkg.Tenant(t), child.TenantID)
			requests = append(requests, request)
			children = append(children, child)
		})
	}
	for index, request := range requests {
		foreign := requests[1-index]
		foreignChild := children[1-index]
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), request.TenantID)
		_, err := module.RequestByID(ctx, foreign.ID, false)
		require.ErrorIs(t, err, sql.ErrNoRows)
		_, err = module.ChildByID(ctx, foreignChild.ID)
		require.ErrorIs(t, err, sql.ErrNoRows)
		rows, err := module.RequestsByID(ctx, []int64{request.ID, foreign.ID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, request.ID, rows[0].ID)
		changed := *foreignChild
		changed.FirstName = "must not change"
		require.Error(t, module.UpdateChildData(ctx, &changed))
		changedRequest := *foreign
		changedRequest.GuardianFirstName = "must not change"
		require.ErrorIs(t, module.UpdateRequestGuardian(ctx, &changedRequest, false), sql.ErrNoRows)
		crossTenantChild := &enrollment.RequestChild{RequestID: foreign.ID, FirstName: "Fixture", LastName: "Child", DateOfBirth: "2023-03-26"}
		require.ErrorIs(t, module.InsertChild(ctx, crossTenantChild), sql.ErrNoRows)
		crossTenantRequest := &enrollment.Request{PhaseID: foreign.PhaseID}
		require.ErrorIs(t, module.InsertRequest(ctx, crossTenantRequest), sql.ErrNoRows)

		err = testpkg.WithTenantTx(t, ctx, db, request.TenantID, func(txCtx context.Context, tx bun.Tx) error {
			var bypass bool
			require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
			require.False(t, bypass, "RLS evidence must use a non-bypass role")
			var ids []int64
			require.NoError(t, tx.NewRaw("SELECT id FROM enrollment.phases WHERE id IN (?, ?)", request.PhaseID, foreign.PhaseID).Scan(txCtx, &ids))
			require.Equal(t, []int64{request.PhaseID}, ids)
			ids = nil
			require.NoError(t, tx.NewRaw("SELECT id FROM enrollment.requests WHERE id IN (?, ?)", request.ID, foreign.ID).Scan(txCtx, &ids))
			require.Equal(t, []int64{request.ID}, ids, "RLS must filter without an application tenant predicate")
			ids = nil
			require.NoError(t, tx.NewRaw("SELECT id FROM enrollment.request_children WHERE id IN (?, ?)", children[index].ID, foreignChild.ID).Scan(txCtx, &ids))
			require.Equal(t, []int64{children[index].ID}, ids)
			return nil
		})
		require.NoError(t, err)
		current, err := module.ChildByID(ctx, children[index].ID)
		require.NoError(t, err)
		require.Equal(t, "Fixture", current.FirstName)
		require.Equal(t, enrollment.Date("2023-03-26"), current.DateOfBirth)
		currentRequest, err := module.RequestByID(ctx, request.ID, false)
		require.NoError(t, err)
		require.Equal(t, "Fixture", currentRequest.GuardianFirstName)
	}
}

func TestIntakeTimestampDefaults(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := enrollmentCompose.New()
	phase := &enrollment.Phase{Name: "Timestamp defaults", ServiceStartDate: "2030-08-01", ServiceEndDate: "2031-07-31"}
	require.NoError(t, owner.InsertPhase(ctx, phase))
	before := time.Now().Add(-time.Second)
	request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "Fixture", GuardianLastName: "Guardian", GuardianEmail: "timestamps@example.test", StatusToken: "timestamp-defaults"}
	require.NoError(t, owner.InsertRequest(ctx, request))
	child := &enrollment.RequestChild{RequestID: request.ID, FirstName: "Fixture", LastName: "Child", DateOfBirth: "2023-03-26"}
	require.NoError(t, owner.InsertChild(ctx, child))
	guardian := &enrollment.RequestGuardian{RequestID: request.ID, FirstName: "Second", LastName: "Guardian"}
	require.NoError(t, owner.CreateRequestGuardian(ctx, guardian))
	storedRequest, err := owner.RequestByID(ctx, request.ID, false)
	require.NoError(t, err)
	storedChild, err := owner.ChildByID(ctx, child.ID)
	require.NoError(t, err)
	storedGuardians, err := owner.RequestGuardians(ctx, []int64{request.ID})
	require.NoError(t, err)
	require.Len(t, storedGuardians, 1)
	after := time.Now().Add(time.Second)
	for name, timestamps := range map[string][]time.Time{
		"request":  {request.CreatedAt, request.UpdatedAt, storedRequest.CreatedAt, storedRequest.UpdatedAt},
		"child":    {child.CreatedAt, child.UpdatedAt, storedChild.CreatedAt, storedChild.UpdatedAt},
		"guardian": {guardian.CreatedAt, guardian.UpdatedAt, storedGuardians[0].CreatedAt, storedGuardians[0].UpdatedAt},
	} {
		for _, timestamp := range timestamps {
			require.True(t, timestamp.After(before) && timestamp.Before(after), "%s timestamp %s must fall within the insertion window", name, timestamp)
		}
	}
}
