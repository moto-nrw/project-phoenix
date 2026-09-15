package enrollment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type approvalOutboxFunc func(context.Context, platformModels.OutboxEnqueueRequest) error

type accountBeforeRosterGroups struct {
	activitiesModels.GroupRepository
	check func(context.Context) error
}

func (r accountBeforeRosterGroups) FindTemplatesWithOfferingSource(ctx context.Context) ([]*activitiesModels.Group, error) {
	if err := r.check(ctx); err != nil {
		return nil, err
	}
	return r.GroupRepository.FindTemplatesWithOfferingSource(ctx)
}

func TestExistingStudentApprovalGrantsAccountBeforeClassRosterResync(t *testing.T) {
	t.Parallel()
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, env.db, "existing-student-lock-order")
	existing := testpkg.CreateTestStudent(t, env.db, "Existing", "Child", "1a")
	requestID, childID := submitOneChild(t, env, account.Email, "Existing", "Child")
	matchChildToExistingStudent(t, env, childID, existing.ID)
	_, err := env.db.NewRaw("UPDATE enrollment.request_children SET target_school_class = '2a' WHERE id = ? AND tenant_id = ?", childID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	var txDB bun.IDB
	checked := false
	env.repos.ActivityGroup = accountBeforeRosterGroups{GroupRepository: env.repos.ActivityGroup, check: func(txCtx context.Context) error {
		checked = true
		var granted bool
		if err := txDB.NewRaw("SELECT EXISTS (SELECT 1 FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id WHERE ar.account_id = ? AND ar.tenant_id = ? AND LOWER(r.name) = 'guardian')", account.ID, testpkg.Tenant(t)).Scan(txCtx, &granted); err != nil {
			return err
		}
		if !granted {
			return errors.New("class roster resync reached before the account-first guardian grant")
		}
		return nil
	}}
	decision := newDecisionServiceForTest(env.rolloverTestEnv, nil, nil)
	err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		txDB = tx
		_, err := decision.Decide(txCtx, enrollmentService.DecideInput{RequestID: requestID, ChildID: childID, Status: enrollmentService.DecisionApproved, ReviewedBy: env.creatorID})
		return err
	})
	require.True(t, checked, "the changed class must reach roster resynchronization")
	require.NoError(t, err)
}

func (f approvalOutboxFunc) EnqueueOutbox(ctx context.Context, request platformModels.OutboxEnqueueRequest) error {
	return f(ctx, request)
}

func approvalOwnerSnapshot(t *testing.T, ctx context.Context, db bun.IDB, tenantID int64) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, table := range []string{
		"enrollment.requests", "enrollment.request_children", "enrollment.request_child_offerings",
		"users.persons", "users.students", "users.guardian_profiles", "users.students_guardians",
		"users.class_list_entries", "activities.student_enrollments", "schedule.instance_students",
		"schedule.student_pickup_schedules", "schedule.student_arrival_schedules",
		"auth.account_tenants", "auth.account_roles",
	} {
		var rows string
		require.NoError(t, db.NewRaw("SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY id), '[]'::jsonb)::text FROM "+table+" r WHERE tenant_id = ?", tenantID).Scan(ctx, &rows))
		result[table] = rows
	}
	return result
}
func TestDecisionService_ApprovalRollsBackEveryOwnerAfterMaterialization(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	otherTenant, _ := testpkg.CreateTestTenant(t, env.db)
	testpkg.OwnTenantRows(t, env.db, otherTenant)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, env.db, otherTenant, "Other", "School", "3a")
	foreignBefore := approvalOwnerSnapshot(t, ctx, env.db, otherTenant)
	_, readErr := testStudentEnrollment(env.db).ReadEnrollmentStudent(ctx, foreignStudent.ID, "")
	require.Error(t, readErr, "an approval must not resolve another school's student")

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Fixed-Days")
	room := testpkg.CreateTestRoom(t, env.db, "Decision-Fixed-Days")
	group := &activitiesModels.Group{
		Name:            "Decision Fixed Days",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      true,
		PlannedRoomID:   &room.ID,
	}
	group.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, group))
	period := createCareOfferingTestPeriod(t, env.db, "decision-fixed-days",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayTuesday, &period.ID)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayThursday, &period.ID)
	defer func() {
		_, _ = env.db.NewDelete().
			TableExpr("activities.schedules").
			Where("activity_group_id = ?", group.ID).
			Exec(ctx)
	}()

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Fixed Tue Thu",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"tue", "thu"},
		IsActive:        true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	account := testpkg.CreateTestAccount(t, env.db, "approval-rollback")
	_, err := env.db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ? AND tenant_id = ?", account.ID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	req := enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Fixed",
		GuardianEmail:     account.Email,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Fina",
				LastName:         "Fixed",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
			},
		},
	}
	submitted, err := env.requestSvc.Submit(ctx, req)
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	instance := testpkg.CreateTestActivityInstance(t, env.db, timezone.NewDate(2026, 9, 15), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &group.ID, CalendarPeriodID: &period.ID,
	})
	registerSourcedInstanceCleanup(t, env, instance.ID)
	before := approvalOwnerSnapshot(t, ctx, env.db, testpkg.Tenant(t))
	var accountBefore string
	require.NoError(t, env.db.NewRaw("SELECT to_jsonb(a)::text FROM auth.accounts a WHERE id = ?", account.ID).Scan(ctx, &accountBefore))
	injected := errors.New("late decision notification failure")
	reached := false
	var txDB bun.IDB
	failing := newDecisionServiceForTestWithDependencies(env.rolloverTestEnv, nil, nil, nil, nil,
		approvalOutboxFunc(func(txCtx context.Context, _ platformModels.OutboxEnqueueRequest) error {
			reached = true
			pending := approvalOwnerSnapshot(t, txCtx, txDB, testpkg.Tenant(t))
			for _, table := range []string{"enrollment.request_children", "users.students", "users.persons", "users.students_guardians", "activities.student_enrollments", "schedule.instance_students", "auth.account_tenants", "auth.account_roles"} {
				require.NotEqual(t, before[table], pending[table], "the late failure must follow a real write to %s", table)
			}
			return injected
		}))
	err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		txDB = tx
		_, decisionErr := failing.Decide(txCtx, enrollmentService.DecideInput{
			RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
			Status: enrollmentService.DecisionApproved, ReviewedBy: env.creatorID,
		})
		return decisionErr
	})
	require.True(t, reached, "failure injection must reach the final notification")
	require.ErrorIs(t, err, injected)
	require.Equal(t, before, approvalOwnerSnapshot(t, ctx, env.db, testpkg.Tenant(t)), "every participating owner must roll back")
	require.Equal(t, foreignBefore, approvalOwnerSnapshot(t, ctx, env.db, otherTenant))
	var accountAfter string
	require.NoError(t, env.db.NewRaw("SELECT to_jsonb(a)::text FROM auth.accounts a WHERE id = ?", account.ID).Scan(ctx, &accountAfter))
	require.JSONEq(t, accountBefore, accountAfter, "approval must not mutate global account credentials")
	var outcome *enrollmentService.DecideOutcome
	err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var decisionErr error
		outcome, decisionErr = env.decision.Decide(txCtx, enrollmentService.DecideInput{
			RequestID:  submitted.Request.ID,
			ChildID:    submitted.Children[0].ID,
			Status:     enrollmentService.DecisionApproved,
			ReviewedBy: env.creatorID,
		})
		return decisionErr
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	var rows []activitiesModels.StudentEnrollment
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, *outcome.Child.CreatedStudentID).
		Where(`"student_enrollment".activity_group_id = ?`, group.ID).
		Scan(ctx))
	require.Len(t, rows, 1)
	assert.Equal(t, []int{2, 4}, rows[0].SelectedWeekdays,
		"fixed offering approval must constrain enrollment to available_days")
	require.NotNil(t, rows[0].CalendarPeriodID)
	assert.Equal(t, period.ID, *rows[0].CalendarPeriodID)
	stable := approvalOwnerSnapshot(t, ctx, env.db, testpkg.Tenant(t))
	err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		again, retryErr := env.decision.Decide(txCtx, enrollmentService.DecideInput{
			RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
			Status: enrollmentService.DecisionApproved, ReviewedBy: env.creatorID,
		})
		if retryErr == nil {
			require.Equal(t, outcome.Child.CreatedStudentID, again.Child.CreatedStudentID)
		}
		return retryErr
	})
	require.NoError(t, err)
	afterRetry := approvalOwnerSnapshot(t, ctx, env.db, testpkg.Tenant(t))
	for _, table := range []string{"users.students", "users.students_guardians", "activities.student_enrollments", "schedule.instance_students", "auth.account_tenants", "auth.account_roles"} {
		require.Equal(t, stable[table], afterRetry[table], "repeating approval must not duplicate or rewrite %s", table)
	}
	require.Equal(t, foreignBefore, approvalOwnerSnapshot(t, ctx, env.db, otherTenant))
}
