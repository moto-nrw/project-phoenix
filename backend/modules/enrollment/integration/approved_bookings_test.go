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
		duplicates, err := testpkg.TenantQuery4(t, ctx, db, module.ActiveDuplicateChildren, phaseIDs[i], "  AUDIT@EXAMPLE.TEST  ", []enrollment.DuplicateChildKey{{FirstName: " FIRST ", LastName: " LAST "}}, 0)
		require.NoError(t, err)
		require.Equal(t, []enrollment.DuplicateChildKey{{FirstName: "first", LastName: "last"}}, duplicates)
		duplicates, err = testpkg.TenantQuery4(t, ctx, db, module.ActiveDuplicateChildren, phaseIDs[i], "audit@example.test", []enrollment.DuplicateChildKey{{FirstName: "First", LastName: "Last"}}, requestIDs[i])
		require.NoError(t, err)
		require.Empty(t, duplicates, "editing a request must not match itself")
		duplicates, err = testpkg.TenantQuery4(t, ctx, db, module.ActiveDuplicateChildren, phaseIDs[1-i], "audit@example.test", []enrollment.DuplicateChildKey{{FirstName: "First", LastName: "Last"}}, 0)
		require.NoError(t, err)
		require.Empty(t, duplicates, "another school's submissions cannot match")
		matched, err := testpkg.TenantQuery3(t, ctx, db, module.HasActiveRequestForMatchedStudent, phaseIDs[i], studentIDs[i], 0)
		require.NoError(t, err)
		require.True(t, matched, "approved children remain active duplicates")
		matched, err = testpkg.TenantQuery3(t, ctx, db, module.HasActiveRequestForMatchedStudent, phaseIDs[1-i], studentIDs[1-i], 0)
		require.NoError(t, err)
		require.False(t, matched)
		count, _, err := testpkg.TenantQueryPair1(t, ctx, db, module.CountStudentReferencesForDeletion, studentIDs[i])
		require.NoError(t, err)
		require.Equal(t, 1, count)
		createdIDs, err := testpkg.TenantQuery1(t, ctx, db, module.CreatedStudentRequestChildIDs, studentIDs)
		require.NoError(t, err)
		require.Empty(t, createdIDs, "matched-only links are not care-exit source ownership")
		periods, err := testpkg.TenantQuery1(t, ctx, db, module.StudentCarePeriods, studentIDs[i])
		require.NoError(t, err)
		require.Empty(t, periods, "matched-only links are not created-student care periods")
		links, err := testpkg.TenantQuery1(t, ctx, db, module.CareExitApplicationLinks, studentIDs)
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
		matched, err = testpkg.TenantQuery3(t, ctx, db, module.HasActiveRequestForMatchedStudent, phaseIDs[i], studentIDs[i], links[0].ID)
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
		afterRestore, err := testpkg.TenantQuery0(t, ctx, db, module.ApprovedBookings)
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
			after, err := testpkg.TenantQuery1(t, ctx, db, module.CareExitApplicationLinks, studentIDs)
			require.NoError(t, err)
			require.Len(t, after, 1)
			require.Equal(t, "approved", after[0].Status, "a failed workflow must not persist its child review")
		}
		allLinks, err := testpkg.TenantQuery1(t, ctx, db, module.CareExitApplicationLinks, nil)
		require.NoError(t, err)
		require.Len(t, allLinks, 3, "scheduled expiry projection retains all local application statuses")
		for _, link := range allLinks {
			require.Equal(t, schoolID, link.TenantID)
		}
		count, _, err = testpkg.TenantQueryPair1(t, ctx, db, module.CountStudentReferencesForDeletion, studentIDs[1-i])
		require.NoError(t, err)
		require.Zero(t, count, "another school's child references are not visible")
		linkFailure := errors.New("after student link writes")
		err = testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
			require.NoError(t, module.LinkCreatedStudent(txCtx, links[0].ID, studentIDs[i]))
			require.NoError(t, module.UpdateMatchedStudent(txCtx, links[0].ID, nil))
			return linkFailure
		})
		require.ErrorIs(t, err, linkFailure)
		afterLinks, err := testpkg.TenantQuery1(t, ctx, db, module.CareExitApplicationLinks, studentIDs)
		require.NoError(t, err)
		require.Len(t, afterLinks, 1)
		require.Nil(t, afterLinks[0].CreatedStudentID)
		require.Equal(t, &studentIDs[i], afterLinks[0].MatchedStudentID)
		foreignCtx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[1-i])
		require.Error(t, testpkg.TenantCommand2(t, foreignCtx, db, module.LinkCreatedStudent, links[0].ID, studentIDs[1-i]))
		require.Error(t, testpkg.TenantCommand2(t, foreignCtx, db, module.UpdateMatchedStudent, links[0].ID, nil))
		require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.LinkCreatedStudent, links[0].ID, studentIDs[i]))
		periods, err = testpkg.TenantQuery1(t, ctx, db, module.StudentCarePeriods, studentIDs[i])
		require.NoError(t, err)
		require.Len(t, periods, 1)
		require.Equal(t, phaseIDs[i], periods[0].PhaseID)
		phase, err := testpkg.TenantQuery1(t, ctx, db, module.Phase, phaseIDs[i])
		require.NoError(t, err)
		require.Equal(t, phase.ServiceStartDate, periods[0].ServiceStartDate)
		require.Equal(t, phase.ServiceEndDate, periods[0].ServiceEndDate)
		foreignPeriods, err := testpkg.TenantQuery1(t, foreignCtx, db, module.StudentCarePeriods, studentIDs[i])
		require.NoError(t, err)
		require.Empty(t, foreignPeriods)
		count, _, err = testpkg.TenantQueryPair1(t, ctx, db, module.CountStudentReferencesForDeletion, studentIDs[i])
		require.NoError(t, err)
		require.Equal(t, 1, count, "one child with both links is counted once")
		createdIDs, err = testpkg.TenantQuery1(t, ctx, db, module.CreatedStudentRequestChildIDs, append(studentIDs, studentIDs[i]))
		require.NoError(t, err)
		require.Len(t, createdIDs, 1, "the owner filters foreign schools and duplicate input IDs")
		rows, err := testpkg.TenantQuery0(t, ctx, db, module.ApprovedBookings)
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
			_, err := testpkg.TenantQuery0(t, adminCtx, db, module.ApprovedBookings)
			require.ErrorContains(t, err, "tenant ID is required")
			rows, err = testpkg.TenantQuery0(t, testpkg.ContextForTenant(adminCtx, schoolID), db, module.ApprovedBookings)
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
		_, _, countErr := module.CountStudentReferencesForDeletion(txCtx, studentIDs[0])
		require.ErrorContains(t, countErr, "count enrollment student references for deletion")
		_, sourceErr := module.CreatedStudentRequestChildIDs(txCtx, studentIDs)
		require.ErrorContains(t, sourceErr, "list enrollment children created as students")
		_, linksErr := module.CareExitApplicationLinks(txCtx, studentIDs)
		require.ErrorContains(t, linksErr, "list care-exit application links")
		_, err = module.ApprovedBookings(txCtx)
		require.ErrorContains(t, err, "list approved enrollment bookings")
		return err
	})
	require.Error(t, err)
	rows, err := testpkg.TenantQuery0(t, ctx, db, module.ApprovedBookings)
	require.NoError(t, err)
	require.Len(t, rows, 2, "retry succeeds after the failed transaction rolls back")
	count, err := testpkg.TenantQuery1(t, ctx, db, module.CountCreatedStudentsByPhase, phaseIDs[0])
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = testpkg.TenantQuery1(t, ctx, db, module.CountCreatedStudentsByPhase, phaseIDs[1])
	require.NoError(t, err)
	require.Zero(t, count)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.DeleteRequestChildren, requestIDs[1]))
	failure := errors.New("after child deletion")
	err = testpkg.WithTenantTx(t, ctx, db, schoolIDs[0], func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.DeleteRequestChildren(txCtx, requestIDs[0]))
		return failure
	})
	require.ErrorIs(t, err, failure)
	count, err = testpkg.TenantQuery1(t, ctx, db, module.CountCreatedStudentsByPhase, phaseIDs[0])
	require.NoError(t, err)
	require.Equal(t, 1, count, "rollback restores the child and its student reference")
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.DeleteRequestChildren, requestIDs[0]))
	count, err = testpkg.TenantQuery1(t, ctx, db, module.CountCreatedStudentsByPhase, phaseIDs[0])
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = testpkg.TenantQuery1(t, testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[1]), db, module.CountCreatedStudentsByPhase, phaseIDs[1])
	require.NoError(t, err)
	require.Equal(t, 1, count, "another school's children remain")
}

func TestCountStudentReferencesForDeletionSeparatesRetainedChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	schoolID := testpkg.Tenant(t)
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolID)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	student := testpkg.CreateTestStudent(t, db, "Reference", "Count", "3a")

	err := testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, tx bun.Tx) error {
		var requestID int64
		err := tx.NewRaw(`INSERT INTO enrollment.requests (tenant_id,phase_id,guardian_first_name,guardian_last_name,guardian_email,status_token,consent_flags,custom_data)
			VALUES (?,?,'First','Last','count@example.test',?,'{}'::jsonb,'{}'::jsonb) RETURNING id`, schoolID, phase.ID, fmt.Sprintf("reference-count-%d", schoolID)).Scan(txCtx, &requestID)
		if err != nil {
			return err
		}
		_, err = tx.NewRaw(`INSERT INTO enrollment.request_children
			(tenant_id,request_id,first_name,last_name,date_of_birth,status,activation_mode,sort_order,custom_data,created_student_id,matched_student_id)
			VALUES (?,?,'Approved','Child','2018-04-15',?,'scheduled',0,'{}'::jsonb,?,NULL),
			 (?,?,'Open','Child','2018-04-15',?,'scheduled',1,'{}'::jsonb,NULL,?),
			 (?,?,'Rejected','Child','2018-04-15',?,'scheduled',2,'{}'::jsonb,?,NULL),
			 (?,?,'Withdrawn','Child','2018-04-15',?,'scheduled',3,'{}'::jsonb,NULL,?)`,
			schoolID, requestID, enrollment.ChildStatusApproved, student.ID,
			schoolID, requestID, enrollment.ChildStatusSubmitted, student.ID,
			schoolID, requestID, enrollment.ChildStatusRejected, student.ID,
			schoolID, requestID, enrollment.ChildStatusWithdrawn, student.ID).Exec(txCtx)
		return err
	})
	require.NoError(t, err)

	approved, total, err := testpkg.TenantQueryPair1(t, ctx, db, module.CountStudentReferencesForDeletion, student.ID)
	require.NoError(t, err)
	require.Equal(t, 1, approved, "only the approved enrollment child leaves with the student")
	require.Equal(t, 4, total, "retained enrollment children still require the locked recheck")
}
