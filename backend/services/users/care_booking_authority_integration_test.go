package users_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func bookingAuthorityService(t *testing.T, db *bun.DB, authoritative bool) userService.CareLifecycleService {
	t.Helper()
	// RFID tag release runs through the People Directory composition (#2661).
	repos, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	return userService.NewCareLifecycleService(userService.CareLifecycleDependencies{
		StudentRepo: repos.Student, PersonRepo: repos.Person, CareExitRepo: repos.CareExit,
		CleanupRepo: repos.CareExitCleanup, WithdrawalRepo: repos.CareWithdrawal,
		TagReleaser:           repos.GradeTransition,
		AuditService:          userService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default()),
		LockCareBookingWrites: func(context.Context) error { return nil },
		BookingsAuthoritative: func(context.Context) (bool, error) { return authoritative, nil },
		DB:                    db, Logger: slog.Default(),
	})
}

func lockedBookingAuthorityService(t *testing.T, db *bun.DB) userService.CareLifecycleService {
	t.Helper()
	// RFID tag release runs through the People Directory composition (#2661).
	repos, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	return userService.NewCareLifecycleService(userService.CareLifecycleDependencies{
		StudentRepo: repos.Student, PersonRepo: repos.Person, CareExitRepo: repos.CareExit,
		CleanupRepo: repos.CareExitCleanup, WithdrawalRepo: repos.CareWithdrawal,
		TagReleaser:  repos.GradeTransition,
		AuditService: userService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default()),
		LockCareBookingWrites: func(ctx context.Context) error {
			return scheduleService.LockTenantRecurrenceWrites(ctx, db)
		},
		BookingsAuthoritative: func(context.Context) (bool, error) { return true, nil },
		DB:                    db, Logger: slog.Default(),
	})
}

func createCareBooking(
	t *testing.T, db *bun.DB, scope testpkg.TenantScope, studentID int64,
	key string, validFrom, validUntil *timezone.Date,
) *enrollmentModels.RequestChild {
	t.Helper()
	offering := createCareBookingOffering(t, db, scope, studentID, key)
	child := createCareBookingSource(t, db, scope, offering.PhaseID, studentID, key)
	link := &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID, CareOfferingID: offering.ID,
		ValidFrom: validFrom, ValidUntil: validUntil,
	}
	link.TenantID = scope.TenantID
	_, err := db.NewInsert().Model(link).ModelTableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).Exec(scope.Context())
	require.NoError(t, err)
	return child
}

func createCareBookingOffering(
	t *testing.T, db *bun.DB, scope testpkg.TenantScope, studentID int64, key string,
) *enrollmentModels.CareOffering {
	t.Helper()
	phase := &enrollmentModels.Phase{
		Name: fmt.Sprintf("Buchungsprüfung-%d-%s", studentID, key), Kind: "school_year",
		ServiceStartDate: timezone.TodayDate().AddDays(-30),
		ServiceEndDate:   timezone.TodayDate().AddDays(300),
		CareOverflowMode: "waitlist", CareOfferingSelectionMode: "optional", IsActive: true,
	}
	phase.TenantID = scope.TenantID
	_, err := db.NewInsert().Model(phase).ModelTableExpr(`enrollment.phases AS "phase"`).Exec(scope.Context())
	require.NoError(t, err)
	offering := &enrollmentModels.CareOffering{
		PhaseID: phase.ID, Name: fmt.Sprintf("Betreuung-%d-%s", studentID, key),
		DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon", "tue", "wed", "thu", "fri"},
		AutoAddGradeLevels: []int{}, IsActive: true, CountsAsCare: true,
	}
	offering.TenantID = scope.TenantID
	_, err = db.NewInsert().Model(offering).ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).Exec(scope.Context())
	require.NoError(t, err)
	return offering
}

func createCareBookingSource(
	t *testing.T, db *bun.DB, scope testpkg.TenantScope, phaseID, studentID int64, key string,
) *enrollmentModels.RequestChild {
	t.Helper()
	request := &enrollmentModels.Request{
		GuardianFirstName: "Test", GuardianLastName: "Person",
		GuardianEmail: fmt.Sprintf("booking-%d-%s@example.test", studentID, key),
		StatusToken:   fmt.Sprintf("booking-%d-%s", studentID, key),
	}
	request.PhaseID = phaseID
	request.TenantID = scope.TenantID
	_, err := db.NewInsert().Model(request).ModelTableExpr(`enrollment.requests AS "request"`).Exec(scope.Context())
	require.NoError(t, err)
	child := &enrollmentModels.RequestChild{
		RequestID: request.ID, FirstName: "Test", LastName: "Kind",
		DateOfBirth: timezone.TodayDate().AddDays(-2500), Status: enrollmentModels.ChildStatusApproved,
		CreatedStudentID: &studentID,
	}
	child.TenantID = scope.TenantID
	_, err = db.NewInsert().Model(child).ModelTableExpr(`enrollment.request_children AS "request_child"`).Exec(scope.Context())
	require.NoError(t, err)
	return child
}

func createImpactStudents(
	t *testing.T, db *bun.DB, scope testpkg.TenantScope, gap timezone.Date,
) (blocker, planned *userModels.Student) {
	t.Helper()
	blocker = testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Ohne", "Buchung", "1a")
	planned = testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Mit", "Ende", "1b")
	ordinary := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Reguläres", "Ende", "1c")
	createCareBooking(t, db, scope, planned.ID, "planned", nil, &gap)
	createCareBooking(t, db, scope, ordinary.ID, "ordinary", nil, &gap)
	_, err := db.NewUpdate().TableExpr("users.students").Set("enrolled_until = ?", gap.AddDays(-1)).
		Where("tenant_id = ? AND id = ?", scope.TenantID, ordinary.ID).Exec(scope.Context())
	require.NoError(t, err)
	return blocker, planned
}

func TestBookingAuthorityImpactAndActivationUseTheSameEvaluation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	ctx := scope.Context()
	today := timezone.TodayDate()
	plannedGap := today.AddDays(7)

	blocker, planned := createImpactStudents(t, db, scope, plannedGap)

	svc := bookingAuthorityService(t, db, true)
	impact, err := svc.PreviewBookingAuthorityImpact(ctx, today)
	require.NoError(t, err)
	require.Len(t, impact.BlockingChildren, 1)
	assert.Equal(t, fmt.Sprintf("%d", blocker.ID), impact.BlockingChildren[0].StudentID)
	require.Len(t, impact.PlannedCompletions, 1)
	assert.Equal(t, fmt.Sprintf("%d", planned.ID), impact.PlannedCompletions[0].StudentID)

	_, err = svc.ApplyBookingAuthoritySetting(ctx, today, true)
	require.ErrorIs(t, err, userService.ErrBookingAuthorityBlocked)
	rows, total, err := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, rows)

	createCareBooking(t, db, scope, blocker.ID, "blocker-fixed", nil, nil)
	for range 2 {
		_, err = svc.ApplyBookingAuthoritySetting(ctx, today, true)
		require.NoError(t, err)
	}
	rows, total, err = repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, planned.ID, *rows[0].StudentID)
}

func TestBookingAuthorityIncludesImmediatelyActivatedStudents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Sofort", "Aktiv", "1a")
	futureStart := timezone.TodayDate().AddDays(14)
	_, err := db.NewUpdate().TableExpr("users.students").
		Set("enrolled_from = ?", futureStart).
		Where("tenant_id = ? AND id = ?", scope.TenantID, student.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	impact, err := bookingAuthorityService(t, db, true).PreviewBookingAuthorityImpact(scope.Context(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, impact.BlockingChildren, 1)
	assert.Equal(t, fmt.Sprintf("%d", student.ID), impact.BlockingChildren[0].StudentID)
}

func TestBookingAuthorityExcludesLegacyInactiveStudentWithoutEnrollmentBounds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Alt", "Inaktiv", "1a")

	_, err := db.NewUpdate().TableExpr("users.students").
		Set("status = ?", userModels.StudentStatusInactive).
		Set("enrolled_from = NULL, enrolled_until = NULL").
		Where("tenant_id = ? AND id = ?", scope.TenantID, student.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	impact, err := bookingAuthorityService(t, db, true).PreviewBookingAuthorityImpact(scope.Context(), timezone.TodayDate())
	require.NoError(t, err)
	assert.Empty(t, impact.BlockingChildren)
}

func TestBookingAuthorityIncludesBookingsFromExistingStudentApproval(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Bestehend", "Gebucht", "1a")
	gap := timezone.TodayDate().AddDays(7)
	child := createCareBooking(t, db, scope, student.ID, "matched", nil, &gap)

	_, err := db.NewUpdate().TableExpr("enrollment.request_children").
		Set("created_student_id = NULL").Set("matched_student_id = ?", student.ID).
		Where("id = ?", child.ID).Exec(scope.Context())
	require.NoError(t, err)

	impact, err := bookingAuthorityService(t, db, true).PreviewBookingAuthorityImpact(scope.Context(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, impact.PlannedCompletions, 1)
	assert.Equal(t, fmt.Sprintf("%d", student.ID), impact.PlannedCompletions[0].StudentID)
}

func TestBookingAuthorityExcludesUnapprovedBookingForExistingStudent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Bestehend", "Unbestätigt", "1a")
	child := createCareBooking(t, db, scope, student.ID, "submitted", nil, nil)

	_, err := db.NewUpdate().TableExpr("enrollment.request_children").
		Set("created_student_id = NULL").
		Set("matched_student_id = ?", student.ID).
		Set("status = ?", enrollmentModels.ChildStatusSubmitted).
		Where("id = ?", child.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	impact, err := bookingAuthorityService(t, db, true).PreviewBookingAuthorityImpact(scope.Context(), timezone.TodayDate())
	require.NoError(t, err)
	assert.Empty(t, impact.PlannedCompletions)
	require.Len(t, impact.BlockingChildren, 1)
	assert.Equal(t, fmt.Sprintf("%d", student.ID), impact.BlockingChildren[0].StudentID)
}

func TestBookingParticipationBoundaryKeepsActualPresenceVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	ctx := scope.Context()
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Live", "Sicher", "2a")
	studentID := student.ID
	firstGap := timezone.TodayDate().AddDays(2)
	require.NoError(t, repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: firstGap,
		Trigger:                 userModels.CareWithdrawalTriggerBookingExpired,
		WithdrawalConfirmedRole: "system", WithdrawalConfirmedAt: time.Now(),
	}))
	svc := bookingAuthorityService(t, db, true)

	before, err := svc.ParticipatingStudentIDs(ctx, []int64{student.ID}, firstGap.AddDays(-1), nil)
	require.NoError(t, err)
	assert.True(t, before[student.ID])
	onGap, err := svc.ParticipatingStudentIDs(ctx, []int64{student.ID}, firstGap, nil)
	require.NoError(t, err)
	assert.False(t, onGap[student.ID])
	present, err := svc.ParticipatingStudentIDs(ctx, []int64{student.ID}, firstGap, map[int64]bool{student.ID: true})
	require.NoError(t, err)
	assert.True(t, present[student.ID])
}

func TestDirectWithdrawalBoundaryDoesNotDependOnBookingAuthority(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Direkt", "Abgemeldet", "2b")
	studentID := student.ID
	firstGap := timezone.TodayDate()
	// RFID tag release runs through the People Directory composition (#2661).
	repos, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	require.NoError(t, repos.CareWithdrawal.UpsertPending(scope.Context(), &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: firstGap,
		Trigger:                 userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}))

	participating, err := bookingAuthorityService(t, db, false).ParticipatingStudentIDs(
		scope.Context(), []int64{student.ID}, firstGap, nil,
	)
	require.NoError(t, err)
	assert.False(t, participating[student.ID])
}

func TestBookingParticipationRangeExcludesAlumniWithoutDateBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Bereits", "Ausgetreten", "2c")
	_, err := db.NewUpdate().TableExpr("users.students").
		Set("status = ?", userModels.StudentStatusAlumnus).
		Set("enrolled_from = NULL, enrolled_until = NULL").
		Where("tenant_id = ? AND id = ?", scope.TenantID, student.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	today := timezone.NewDate(2026, 8, 24)
	participating, err := bookingAuthorityService(t, db, false).ParticipatingStudentIDsByDate(
		scope.Context(), []int64{student.ID}, today, today.AddDays(2),
	)
	require.NoError(t, err)
	for day := today; !day.After(today.AddDays(2)); day = day.AddDays(1) {
		assert.False(t, participating[day][student.ID])
	}
}

func TestNaturalBookingEndSchedulerIsIdempotent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	ctx := tenant.WithTenantID(scope.Context(), scope.TenantID)
	planned := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Geplant", "Ende", "3a")
	overdue := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Überfällig", "Ende", "3b")
	plannedGap := timezone.NewDate(2026, 8, 24).AddDays(5)
	overdueGap := timezone.NewDate(2026, 8, 24).AddDays(-3)
	createCareBooking(t, db, scope, planned.ID, "planned-scheduler", nil, &plannedGap)
	createCareBooking(t, db, scope, overdue.ID, "overdue-scheduler", nil, &overdueGap)
	svc := bookingAuthorityService(t, db, true)

	_, err := svc.ApplyDueEffects(ctx, timezone.NewDate(2026, 8, 24))
	require.NoError(t, err)
	rows, total, err := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	updatedAt := map[int64]time.Time{*rows[0].StudentID: rows[0].UpdatedAt, *rows[1].StudentID: rows[1].UpdatedAt}
	_, err = svc.ApplyDueEffects(ctx, timezone.NewDate(2026, 8, 24))
	require.NoError(t, err)
	rows, total, err = repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	assert.ElementsMatch(t, []timezone.Date{plannedGap, overdueGap}, []timezone.Date{
		rows[0].FirstBookinglessDay,
		rows[1].FirstBookinglessDay,
	})
	for _, row := range rows {
		assert.Equal(t, updatedAt[*row.StudentID], row.UpdatedAt, "an idempotent scheduler run must not rewrite the task")
	}
}

func TestNaturalSchedulerPreservesConfirmedWithdrawalAudit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Audit", "Bleibt", "3c")
	actor := testpkg.CreateTestAccount(t, db, "booking-audit@example.test")
	gap := timezone.TodayDate().AddDays(5)
	createCareBooking(t, db, scope, student.ID, "audit", nil, &gap)
	studentID := student.ID
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	require.NoError(t, repo.UpsertPending(scope.Context(), &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: gap, Trigger: userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy: &actor.ID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now().Add(-time.Hour),
	}))
	before := requirePendingCompletion(t, repo, scope.Context(), student.ID)

	_, err := bookingAuthorityService(t, db, true).ApplyDueEffects(scope.Context(), timezone.TodayDate())
	require.NoError(t, err)
	after := requirePendingCompletion(t, repo, scope.Context(), student.ID)
	assert.Equal(t, before.Trigger, after.Trigger)
	assert.Equal(t, before.WithdrawalConfirmedBy, after.WithdrawalConfirmedBy)
	assert.Equal(t, before.WithdrawalConfirmedRole, after.WithdrawalConfirmedRole)
	assert.Equal(t, before.WithdrawalConfirmedAt, after.WithdrawalConfirmedAt)
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt)
}

func TestFiniteRebookingKeepsCompletionEventHistory(t *testing.T) {
	t.Parallel()
	t.Run("later gap", func(t *testing.T) {
		assertFiniteRebookingHistory(t, 4, 10)
	})
	t.Run("earlier gap", func(t *testing.T) {
		assertFiniteRebookingHistory(t, 10, 4)
	})
}

func TestOverdueRebookingReplacesTheStaleCompletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Nachträglich", "Gebucht", "3d")
	oldGap := timezone.NewDate(2026, 8, 24).AddDays(-2)
	newGap := timezone.NewDate(2026, 8, 24).AddDays(5)
	createCareBooking(t, db, scope, student.ID, "renewed", &oldGap, &newGap)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	studentID := student.ID
	require.NoError(t, repo.UpsertPending(scope.Context(), &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: oldGap,
		Trigger: userModels.CareWithdrawalTriggerBookingExpired, WithdrawalConfirmedRole: "system", WithdrawalConfirmedAt: time.Now(),
	}))

	require.NoError(t, bookingAuthorityService(t, db, true).ReconcileAuthoritativeBookingChange(
		scope.Context(), userModels.CareWithdrawalBookingChange{StudentID: student.ID, FirstBookinglessDay: oldGap},
	))
	assert.Equal(t, newGap, requirePendingCompletion(t, repo, scope.Context(), student.ID).FirstBookinglessDay)
}

func assertFiniteRebookingHistory(t *testing.T, oldOffset, newOffset int) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Neu", "Gebucht", "3d")
	oldGap := timezone.TodayDate().AddDays(oldOffset)
	newGap := timezone.TodayDate().AddDays(newOffset)
	createCareBooking(t, db, scope, student.ID, "old-window", nil, &oldGap)
	svc := bookingAuthorityService(t, db, true)
	require.NoError(t, svc.ReconcileAuthoritativeBookingChange(scope.Context(), userModels.CareWithdrawalBookingChange{StudentID: student.ID}))
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	oldTask := requirePendingCompletion(t, repo, scope.Context(), student.ID)

	if newGap.Before(oldGap) {
		_, err := db.NewUpdate().TableExpr("enrollment.request_child_offerings AS rco").
			Set("valid_until = ?", newGap).
			Where("request_child_id IN (SELECT id FROM enrollment.request_children WHERE created_student_id = ?)", student.ID).
			Exec(scope.Context())
		require.NoError(t, err)
	} else {
		createCareBooking(t, db, scope, student.ID, "extension", &oldGap, &newGap)
	}
	require.NoError(t, svc.ReconcileAuthoritativeBookingChange(scope.Context(), userModels.CareWithdrawalBookingChange{StudentID: student.ID}))
	newTask := requirePendingCompletion(t, repo, scope.Context(), student.ID)
	assert.NotEqual(t, oldTask.ID, newTask.ID)
	assert.Equal(t, newGap, newTask.FirstBookinglessDay)
	oldTask, err := repo.FindByID(scope.Context(), oldTask.ID)
	require.NoError(t, err)
	require.NotNil(t, oldTask)
	assert.Equal(t, userModels.CareWithdrawalStateObsolete, oldTask.State)
	require.NotNil(t, oldTask.ObsoleteReason)
	assert.Equal(t, userModels.CareWithdrawalObsoleteRebooked, *oldTask.ObsoleteReason)
}

func requirePendingCompletion(
	t *testing.T, repo userModels.CareWithdrawalCompletionRepository, ctx context.Context, studentID int64,
) *userModels.CareWithdrawalCompletion {
	t.Helper()
	rows, total, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: studentID, Page: 1, PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	return rows[0]
}

func TestBookingMutationPlansFutureNaturalEndImmediately(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Direkt", "Geplant", "3c")
	gap := timezone.NewDate(2026, 8, 24).AddDays(9)
	createCareBooking(t, db, scope, student.ID, "direct-future", nil, &gap)

	err := bookingAuthorityService(t, db, true).ReconcileAuthoritativeBookingChange(
		scope.Context(), userModels.CareWithdrawalBookingChange{StudentID: student.ID, ConfirmedRole: "admin"},
	)
	require.NoError(t, err)
	rows, total, err := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.ListPending(
		scope.Context(), userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20},
	)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, gap, rows[0].FirstBookinglessDay)
}

func TestConfirmedWithdrawalCannotBypassBookingAuthority(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Bewusst", "Abgemeldet", "3e")
	gap := timezone.TodayDate().AddDays(4)

	err := bookingAuthorityService(t, db, false).ReconcileAuthoritativeBookingChange(
		scope.Context(), userModels.CareWithdrawalBookingChange{
			StudentID: student.ID, FirstBookinglessDay: gap,
			WasCompleteWithdrawal: true, ConfirmedRole: "admin",
		},
	)
	require.NoError(t, err)
	rows, total, err := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.ListPending(
		scope.Context(), userModels.CareWithdrawalCompletionFilter{StudentID: student.ID, Page: 1, PageSize: 1},
	)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, rows)
}

func TestNonCareMutationKeepsConfirmedWithdrawal(t *testing.T) {
	t.Parallel()
	for _, authoritative := range []bool{false, true} {
		t.Run(fmt.Sprintf("authoritative=%t", authoritative), func(t *testing.T) {
			assertNonCareMutationKeepsConfirmedWithdrawal(t, authoritative)
		})
	}
}

func assertNonCareMutationKeepsConfirmedWithdrawal(t *testing.T, authoritative bool) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Direkt", "Bleibt", "3f")
	studentID := student.ID
	gap := timezone.TodayDate().AddDays(4)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal
	require.NoError(t, repo.UpsertPending(scope.Context(), &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: gap, Trigger: userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: time.Now(),
	}))

	err := bookingAuthorityService(t, db, authoritative).ReconcileAuthoritativeBookingChange(
		scope.Context(), userModels.CareWithdrawalBookingChange{StudentID: student.ID, FirstBookinglessDay: gap},
	)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalTriggerDirectSchool,
		requirePendingCompletion(t, repo, scope.Context(), student.ID).Trigger)
}

func TestConcurrentBookingAuthorityActivationCreatesOneCompletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Parallel", "Aktiviert", "3d")
	gap := timezone.TodayDate().AddDays(11)
	createCareBooking(t, db, scope, student.ID, "concurrent-enable", nil, &gap)
	svc := lockedBookingAuthorityService(t, db)

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- testpkg.WithTenantTx(t, context.Background(), db, scope.TenantID, func(ctx context.Context, _ bun.Tx) error {
				_, err := svc.ApplyBookingAuthoritySetting(ctx, timezone.TodayDate(), true)
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	_, total, err := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareWithdrawal.ListPending(
		scope.Context(), userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestBookingAuthorityReconciliationQueryCountIsIndependentOfStudentCount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	small := testpkg.NewTenantScope(t, db)
	large := testpkg.NewTenantScope(t, db)
	createContinuousBookingStudents(t, db, small, 1)
	createContinuousBookingStudents(t, db, large, 8)
	counter := testpkg.NewQueryCounter()
	countedDB := db.WithQueryHook(counter)

	_, err := bookingAuthorityService(t, countedDB, true).ApplyBookingAuthoritySetting(small.Context(), timezone.TodayDate(), true)
	require.NoError(t, err)
	smallCount := counter.Total()
	counter.Reset()
	_, err = bookingAuthorityService(t, countedDB, true).ApplyBookingAuthoritySetting(large.Context(), timezone.TodayDate(), true)
	require.NoError(t, err)
	assert.Equal(t, smallCount, counter.Total(), "reconciliation must use fixed batch queries")
}

func createContinuousBookingStudents(t *testing.T, db *bun.DB, scope testpkg.TenantScope, count int) {
	t.Helper()
	for i := range count {
		student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Batch", fmt.Sprintf("Kind%d", i), "4a")
		createCareBooking(t, db, scope, student.ID, fmt.Sprintf("batch-%d", i), nil, nil)
	}
}

func TestDisablingBookingAuthorityObsoletesOnlyOpenBookingCompletions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	ctx := scope.Context()
	// RFID tag release runs through the People Directory composition (#2661).
	repos, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	today := timezone.TodayDate()

	openStudent := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Offen", "Aufgabe", "4a")
	resolvedStudent := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Erledigt", "Aufgabe", "4b")
	for _, id := range []int64{openStudent.ID, resolvedStudent.ID} {
		studentID := id
		require.NoError(t, repos.CareWithdrawal.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
			StudentID: &studentID, FirstBookinglessDay: today,
			Trigger:                 userModels.CareWithdrawalTriggerBookingExpired,
			WithdrawalConfirmedRole: "system", WithdrawalConfirmedAt: time.Now(),
		}))
	}
	resolvedRows, _, err := repos.CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{
		StudentID: resolvedStudent.ID, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Len(t, resolvedRows, 1)
	actorID := testpkg.CreateTestAccount(t, db, "booking-authority-disable").ID
	changed, err := repos.CareWithdrawal.MarkResolved(ctx, resolvedRows[0].ID, actorID, time.Now())
	require.NoError(t, err)
	require.True(t, changed)

	_, err = bookingAuthorityService(t, db, false).ApplyBookingAuthoritySetting(ctx, today, false)
	require.NoError(t, err)
	_, pendingTotal, err := repos.CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Zero(t, pendingTotal)
	_, resolvedTotal, err := repos.CareWithdrawal.ListResolved(ctx, userModels.CareWithdrawalCompletionFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, 1, resolvedTotal)
}
