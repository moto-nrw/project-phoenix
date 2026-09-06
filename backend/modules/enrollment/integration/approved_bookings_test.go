package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestApprovedBookingsTenantScopeNullStudentAndReadFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	var schoolIDs, studentIDs, phaseIDs, requestIDs []int64
	for _, school := range []string{"first", "second"} {
		t.Run(school, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			student := testpkg.CreateTestStudent(t, db, "Approved", "Booking", "3a")
			schoolID := testpkg.Tenant(t)
			schoolIDs = append(schoolIDs, schoolID)
			studentIDs = append(studentIDs, student.ID)
			phaseIDs = append(phaseIDs, phase.ID)
			err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, schoolID, func(ctx context.Context, tx bun.Tx) error {
				var requestID int64
				err := tx.NewRaw(`INSERT INTO enrollment.requests (tenant_id,phase_id,guardian_first_name,guardian_last_name,guardian_email,status_token,consent_flags,custom_data)
				 VALUES (?,?,'First','Last','audit@example.test',?,'{}'::jsonb,'{}'::jsonb) RETURNING id`, schoolID, phase.ID, fmt.Sprintf("approved-bookings-%d", schoolID)).Scan(ctx, &requestID)
				if err != nil {
					return err
				}
				requestIDs = append(requestIDs, requestID)
				_, err = tx.NewRaw(`INSERT INTO enrollment.request_children
				 (tenant_id,request_id,first_name,last_name,date_of_birth,status,activation_mode,sort_order,custom_data,matched_student_id)
				 VALUES (?,?,'First','Last','2018-04-15','approved','scheduled',0,'{}'::jsonb,NULL),
				 (?,?,'Second','Last','2018-04-15','approved','scheduled',1,'{}'::jsonb,?),
				 (?,?,'Pending','Last','2018-04-15','submitted','scheduled',2,'{}'::jsonb,NULL)`, schoolID, requestID, schoolID, requestID, student.ID, schoolID, requestID).Exec(ctx)
				return err
			})
			require.NoError(t, err)
		})
	}
	for i, schoolID := range schoolIDs {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolID)
		duplicates, err := module.ActiveDuplicateChildren(ctx, phaseIDs[i], "  AUDIT@EXAMPLE.TEST  ", []enrollment.DuplicateChildKey{{FirstName: " FIRST ", LastName: " LAST "}}, 0)
		require.NoError(t, err)
		require.Equal(t, []enrollment.DuplicateChildKey{{FirstName: "first", LastName: "last"}}, duplicates)
		duplicates, err = module.ActiveDuplicateChildren(ctx, phaseIDs[i], "audit@example.test", []enrollment.DuplicateChildKey{{FirstName: "First", LastName: "Last"}}, requestIDs[i])
		require.NoError(t, err)
		require.Empty(t, duplicates, "editing a request must not match itself")
		duplicates, err = module.ActiveDuplicateChildren(ctx, phaseIDs[1-i], "audit@example.test", []enrollment.DuplicateChildKey{{FirstName: "First", LastName: "Last"}}, 0)
		require.NoError(t, err)
		require.Empty(t, duplicates, "another school's submissions cannot match")
		matched, err := module.HasActiveRequestForMatchedStudent(ctx, phaseIDs[i], studentIDs[i], 0)
		require.NoError(t, err)
		require.True(t, matched, "approved children remain active duplicates")
		matched, err = module.HasActiveRequestForMatchedStudent(ctx, phaseIDs[1-i], studentIDs[1-i], 0)
		require.NoError(t, err)
		require.False(t, matched)
		count, err := module.CountStudentReferences(ctx, studentIDs[i])
		require.NoError(t, err)
		require.Equal(t, 1, count)
		createdIDs, err := module.CreatedStudentRequestChildIDs(ctx, studentIDs)
		require.NoError(t, err)
		require.Empty(t, createdIDs, "matched-only links are not care-exit source ownership")
		periods, err := module.StudentCarePeriods(ctx, studentIDs[i])
		require.NoError(t, err)
		require.Empty(t, periods, "matched-only links are not created-student care periods")
		links, err := module.CareExitApplicationLinks(ctx, studentIDs)
		require.NoError(t, err)
		require.Len(t, links, 1, "selected-student projection includes matched links but excludes foreign schools")
		require.Nil(t, links[0].CreatedStudentID)
		require.Equal(t, &studentIDs[i], links[0].MatchedStudentID)
		require.Equal(t, "approved", links[0].Status)
		activationFailure := errors.New("after activation plan")
		err = testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, tx bun.Tx) error {
			for _, date := range []enrollment.Date{"2030-03-31", "2030-10-27"} {
				require.NoError(t, module.UpdateChildActivationPlan(txCtx, links[0].ID, "scheduled", &date))
				var stored string
				require.NoError(t, tx.NewRaw("SELECT activate_on::text FROM enrollment.request_children WHERE id = ?", links[0].ID).Scan(txCtx, &stored))
				require.Equal(t, string(date), stored, "calendar dates must not shift across DST boundaries")
			}
			require.NoError(t, module.UpdateChildActivationPlan(txCtx, links[0].ID, "scheduled", nil))
			var isNull bool
			require.NoError(t, tx.NewRaw("SELECT activate_on IS NULL FROM enrollment.request_children WHERE id = ?", links[0].ID).Scan(txCtx, &isNull))
			require.True(t, isNull)
			return activationFailure
		})
		require.ErrorIs(t, err, activationFailure)
		matched, err = module.HasActiveRequestForMatchedStudent(ctx, phaseIDs[i], studentIDs[i], links[0].ID)
		require.NoError(t, err)
		require.False(t, matched, "rechecking a child must exclude its own pin")
		transitionFailure := errors.New("after bulk restore")
		err = testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			changed, err := module.TransitionPhaseChildren(txCtx, phaseIDs[1-i], "approved", "withdrawn")
			require.NoError(t, err)
			require.Zero(t, changed)
			changed, err = module.TransitionPhaseChildren(txCtx, phaseIDs[i], "approved", "withdrawn")
			require.NoError(t, err)
			require.Equal(t, 2, changed)
			restored, err := module.RestoreWithdrawnChildren(txCtx, requestIDs[i], []int64{links[0].ID})
			require.NoError(t, err)
			require.Len(t, restored, 2)
			return transitionFailure
		})
		require.ErrorIs(t, err, transitionFailure)
		afterRestore, err := module.ApprovedBookings(ctx)
		require.NoError(t, err)
		require.Len(t, afterRestore, 2, "bulk transitions and both restore writes join the caller's transaction")
		for _, operation := range []string{"status", "rollover review"} {
			failure := errors.New("after child review write")
			err := testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
				if operation == "status" {
					require.NoError(t, module.UpdateChildStatus(txCtx, links[0].ID, "rejected", nil, 0))
				} else {
					require.NoError(t, module.ReviewRolloverChild(txCtx, links[0].ID, "rejected", nil, nil, 0))
				}
				return failure
			})
			require.ErrorIs(t, err, failure)
			after, err := module.CareExitApplicationLinks(ctx, studentIDs)
			require.NoError(t, err)
			require.Len(t, after, 1)
			require.Equal(t, "approved", after[0].Status, "a failed workflow must not persist its child review")
		}
		allLinks, err := module.CareExitApplicationLinks(ctx, nil)
		require.NoError(t, err)
		require.Len(t, allLinks, 3, "scheduled expiry projection retains all local application statuses")
		for _, link := range allLinks {
			require.Equal(t, schoolID, link.TenantID)
		}
		count, err = module.CountStudentReferences(ctx, studentIDs[1-i])
		require.NoError(t, err)
		require.Zero(t, count, "another school's child references are not visible")
		linkFailure := errors.New("after student link writes")
		err = testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			require.NoError(t, module.LinkCreatedStudent(txCtx, links[0].ID, studentIDs[i]))
			require.NoError(t, module.UpdateMatchedStudent(txCtx, links[0].ID, nil))
			return linkFailure
		})
		require.ErrorIs(t, err, linkFailure)
		afterLinks, err := module.CareExitApplicationLinks(ctx, studentIDs)
		require.NoError(t, err)
		require.Len(t, afterLinks, 1)
		require.Nil(t, afterLinks[0].CreatedStudentID)
		require.Equal(t, &studentIDs[i], afterLinks[0].MatchedStudentID)
		foreignCtx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[1-i])
		require.Error(t, module.LinkCreatedStudent(foreignCtx, links[0].ID, studentIDs[1-i]))
		require.Error(t, module.UpdateMatchedStudent(foreignCtx, links[0].ID, nil))
		require.NoError(t, module.LinkCreatedStudent(ctx, links[0].ID, studentIDs[i]))
		periods, err = module.StudentCarePeriods(ctx, studentIDs[i])
		require.NoError(t, err)
		require.Len(t, periods, 1)
		require.Equal(t, phaseIDs[i], periods[0].PhaseID)
		phase, err := module.Phase(ctx, phaseIDs[i])
		require.NoError(t, err)
		require.Equal(t, phase.ServiceStartDate, periods[0].ServiceStartDate)
		require.Equal(t, phase.ServiceEndDate, periods[0].ServiceEndDate)
		foreignPeriods, err := module.StudentCarePeriods(foreignCtx, studentIDs[i])
		require.NoError(t, err)
		require.Empty(t, foreignPeriods)
		count, err = module.CountStudentReferences(ctx, studentIDs[i])
		require.NoError(t, err)
		require.Equal(t, 1, count, "one child with both links is counted once")
		createdIDs, err = module.CreatedStudentRequestChildIDs(ctx, append(studentIDs, studentIDs[i]))
		require.NoError(t, err)
		require.Len(t, createdIDs, 1, "the owner filters foreign schools and duplicate input IDs")
		rows, err := module.ApprovedBookings(ctx)
		require.NoError(t, err)
		require.Len(t, rows, 2, "approved rows without a student must remain available for missing-offering audits")
		require.Nil(t, rows[0].StudentID)
		require.Equal(t, &studentIDs[i], rows[1].StudentID)
		for _, row := range rows {
			require.Equal(t, schoolID, row.TenantID)
			require.NotEmpty(t, row.ServiceStartDate)
			require.NotEmpty(t, row.ServiceEndDate)
		}
		err = tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
			_, err := module.ApprovedBookings(adminCtx)
			require.ErrorContains(t, err, "tenant ID is required")
			rows, err = module.ApprovedBookings(testpkg.ContextForTenant(adminCtx, schoolID))
			require.NoError(t, err)
			require.Len(t, rows, 2)
			for _, row := range rows {
				require.Equal(t, schoolID, row.TenantID)
			}
			return nil
		})
		require.NoError(t, err)
	}
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[0])
	err := testpkg.WithTenantTx(t, ctx, db, schoolIDs[0], func(txCtx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw("SELECT 1 / 0").Exec(txCtx)
		require.Error(t, err)
		_, countErr := module.CountStudentReferences(txCtx, studentIDs[0])
		require.ErrorContains(t, countErr, "count enrollment student references")
		_, sourceErr := module.CreatedStudentRequestChildIDs(txCtx, studentIDs)
		require.ErrorContains(t, sourceErr, "list enrollment children created as students")
		_, linksErr := module.CareExitApplicationLinks(txCtx, studentIDs)
		require.ErrorContains(t, linksErr, "list care-exit application links")
		_, err = module.ApprovedBookings(txCtx)
		require.ErrorContains(t, err, "list approved enrollment bookings")
		return err
	})
	require.Error(t, err)
	rows, err := module.ApprovedBookings(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2, "retry succeeds after the failed transaction rolls back")
	count, err := module.CountCreatedStudentsByPhase(ctx, phaseIDs[0])
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = module.CountCreatedStudentsByPhase(ctx, phaseIDs[1])
	require.NoError(t, err)
	require.Zero(t, count)
	require.NoError(t, module.DeleteRequestChildren(ctx, requestIDs[1]))
	failure := errors.New("after child deletion")
	err = testpkg.WithTenantTx(t, ctx, db, schoolIDs[0], func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.DeleteRequestChildren(txCtx, requestIDs[0]))
		return failure
	})
	require.ErrorIs(t, err, failure)
	count, err = module.CountCreatedStudentsByPhase(ctx, phaseIDs[0])
	require.NoError(t, err)
	require.Equal(t, 1, count, "rollback restores the child and its student reference")
	require.NoError(t, module.DeleteRequestChildren(ctx, requestIDs[0]))
	count, err = module.CountCreatedStudentsByPhase(ctx, phaseIDs[0])
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = module.CountCreatedStudentsByPhase(testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[1]), phaseIDs[1])
	require.NoError(t, err)
	require.Equal(t, 1, count, "another school's children remain")
}
