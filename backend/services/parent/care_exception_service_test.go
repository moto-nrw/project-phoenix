package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// errBoom is the injected repository failure used by the error-propagation
// tests below.
var errBoom = errors.New("boom: repository unavailable")

// stubPickupRepo wraps the real pickup-exception repository and forces a chosen
// read to fail, so the care-exception service's error paths can be exercised
// without a broken database. Embedding the interface means every method not
// overridden here delegates to the real repo.
type stubPickupRepo struct {
	scheduleModels.StudentPickupExceptionRepository
	findErr  error
	rangeErr error
}

func (s stubPickupRepo) FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) (*scheduleModels.StudentPickupException, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.StudentPickupExceptionRepository.FindByStudentIDAndDate(ctx, studentID, date)
}

func (s stubPickupRepo) FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModels.StudentPickupException, error) {
	if s.rangeErr != nil {
		return nil, s.rangeErr
	}
	return s.StudentPickupExceptionRepository.FindByStudentIDAndDateRange(ctx, studentID, from, to)
}

// stubArrivalRepo mirrors stubPickupRepo for the arrival leg.
type stubArrivalRepo struct {
	scheduleModels.StudentArrivalExceptionRepository
	findErr  error
	rangeErr error
}

func (s stubArrivalRepo) FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) (*scheduleModels.StudentArrivalException, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.StudentArrivalExceptionRepository.FindByStudentIDAndDate(ctx, studentID, date)
}

func (s stubArrivalRepo) FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModels.StudentArrivalException, error) {
	if s.rangeErr != nil {
		return nil, s.rangeErr
	}
	return s.StudentArrivalExceptionRepository.FindByStudentIDAndDateRange(ctx, studentID, from, to)
}

// careRepoWrap optionally wraps each exception repository for fault injection.
type careRepoWrap struct {
	pickup  func(scheduleModels.StudentPickupExceptionRepository) scheduleModels.StudentPickupExceptionRepository
	arrival func(scheduleModels.StudentArrivalExceptionRepository) scheduleModels.StudentArrivalExceptionRepository
}

// buildCareServiceWithRepos builds the care service, optionally wrapping the
// pickup and/or arrival repositories so individual reads can be forced to fail.
func buildCareServiceWithRepos(t *testing.T, w careRepoWrap) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	pickup := repos.StudentPickupException
	if w.pickup != nil {
		pickup = w.pickup(repos.StudentPickupException)
	}
	arrival := repos.StudentArrivalException
	if w.arrival != nil {
		arrival = w.arrival(repos.StudentArrivalException)
	}
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  pickup,
		ArrivalExceptionRepo: arrival,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{configModels.KeyParentPickupChangeEnabled: true},
		},
		Broadcaster: testpkg.NewRecordingBroadcaster(),
		DB:          db,
		Logger:      slog.Default(),
	})
	return svc, db
}

// buildCareServiceWithPickupRepo builds the care service with a wrapped pickup
// repository, used to inject read failures.
func buildCareServiceWithPickupRepo(t *testing.T, wrap func(scheduleModels.StudentPickupExceptionRepository) scheduleModels.StudentPickupExceptionRepository) (parentService.Service, *bun.DB) {
	return buildCareServiceWithRepos(t, careRepoWrap{pickup: wrap})
}

func buildCareService(t *testing.T, pickupChangeEnabled bool) (parentService.Service, *testpkg.RecordingBroadcaster, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	bc := testpkg.NewRecordingBroadcaster()
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  repos.StudentPickupException,
		ArrivalExceptionRepo: repos.StudentArrivalException,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{configModels.KeyParentPickupChangeEnabled: pickupChangeEnabled},
		},
		Broadcaster: bc,
		DB:          db,
		Logger:      slog.Default(),
	})
	return svc, bc, db
}

// wallClock builds a reference-date-anchored wall-clock instant matching the
// HH:MM value stored in the TIME column.
func wallClock(h, m int) *time.Time {
	t := time.Date(2000, 1, 1, h, m, 0, 0, time.UTC)
	return &t
}

func TestSubmitCareException_PersistsGuardianRowWithNullCreatedBy(t *testing.T) {
	svc, bc, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	date := timezone.TodayDate().AddDays(1)
	result, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID, date, wallClock(14, 30), wallClock(8, 15))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.PickupTime)
	require.NotNil(t, result.ArrivalTime)
	assert.Equal(t, "14:30", result.PickupTime.Format("15:04"))
	assert.Equal(t, "08:15", result.ArrivalTime.Format("15:04"))
	assert.Equal(t, scheduleModels.ExceptionSourceGuardian, result.Source)
	assert.Contains(t, tenantBroadcastIDs(bc), chain.TenantID, "SSE broadcast must fire")

	// The load-bearing assertion: a guardian row stores created_by as NULL
	// (nullzero) and references the account via created_by_guardian. NULL is
	// what lets the row past the single-column created_by → users.staff(id) FK,
	// which migration 1.15.136 made nullable for exactly this case.
	var createdBy *int64
	var createdByGuardian *int64
	var source string
	require.NoError(t, db.NewSelect().
		ColumnExpr("created_by").ColumnExpr("created_by_guardian").ColumnExpr("source").
		TableExpr("schedule.student_pickup_exceptions").
		Where("student_id = ?", chain.StudentID).
		Where("exception_date = ?", date).
		Scan(context.Background(), &createdBy, &createdByGuardian, &source))
	assert.Nil(t, createdBy, "guardian-authored row must store created_by as NULL")
	require.NotNil(t, createdByGuardian)
	assert.Equal(t, chain.AccountID, *createdByGuardian)
	assert.Equal(t, scheduleModels.ExceptionSourceGuardian, source)
}

func TestSubmitCareException_FeatureDisabled(t *testing.T) {
	svc, _, db := buildCareService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(1), wallClock(15, 0), nil)
	assert.ErrorIs(t, err, parentService.ErrPickupChangeDisabled)
}

func TestSubmitCareException_NoTimes(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(1), nil, nil)
	assert.ErrorIs(t, err, parentService.ErrNoCareException)
}

func TestSubmitCareException_PastDate(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(-1), wallClock(15, 0), nil)
	assert.ErrorIs(t, err, parentService.ErrPastCareDate)
}

func TestSubmitCareException_TooFarDate(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// One day past the two-calendar-month cap is rejected; the boundary itself
	// stays allowed.
	today := timezone.TodayDate()
	maxDate := timezone.NewDate(today.Year, today.Month+2, today.Day)

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		maxDate.AddDays(1), wallClock(15, 0), nil)
	assert.ErrorIs(t, err, parentService.ErrCareDateTooFar)

	_, err = svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		maxDate, wallClock(15, 0), nil)
	assert.NoError(t, err)
}

func TestSubmitCareException_NotOwnedChild(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, other.ID,
		timezone.TodayDate().AddDays(1), wallClock(15, 0), nil)
	assert.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

func TestSubmitCareException_ConflictWithStaffException(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	staff := testpkg.CreateTestStaff(t, db, "Team", "Mitglied")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_exceptions WHERE student_id = ?`, chain.StudentID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staff.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, staff.PersonID)
	}()

	date := timezone.TodayDate().AddDays(2)
	staffTime := wallClock(16, 0)
	staffEx := &scheduleModels.StudentPickupException{
		StudentID:     chain.StudentID,
		ExceptionDate: date,
		PickupTime:    staffTime,
		CreatedBy:     staff.ID,
	}
	staffEx.SetTenantID(chain.TenantID)
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.StudentPickupException.Create(context.Background(), staffEx))

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		date, wallClock(14, 0), nil)
	assert.ErrorIs(t, err, parentService.ErrCareExceptionConflict)
}

// TestSubmitCareException_ConflictWhenStaffOwnsOtherLeg guards the day-level
// staff check: staff own only the pickup leg, the parent submits only an
// arrival. The whole day is staff-owned, so the arrival write must be refused
// rather than silently persisting while the day still renders as staff-set.
func TestSubmitCareException_ConflictWhenStaffOwnsOtherLeg(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	staff := testpkg.CreateTestStaff(t, db, "Team", "Mitglied")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_exceptions WHERE student_id = ?`, chain.StudentID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_arrival_exceptions WHERE student_id = ?`, chain.StudentID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staff.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, staff.PersonID)
	}()

	date := timezone.TodayDate().AddDays(2)
	staffEx := &scheduleModels.StudentPickupException{
		StudentID:     chain.StudentID,
		ExceptionDate: date,
		PickupTime:    wallClock(16, 0),
		CreatedBy:     staff.ID,
	}
	staffEx.SetTenantID(chain.TenantID)
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.StudentPickupException.Create(context.Background(), staffEx))

	// Parent submits ONLY an arrival; the staff-owned pickup leg must still block it.
	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		date, nil, wallClock(8, 0))
	assert.ErrorIs(t, err, parentService.ErrCareExceptionConflict)

	// And no guardian arrival row leaked through.
	var count int
	require.NoError(t, db.NewSelect().
		ColumnExpr("COUNT(*)").
		TableExpr("schedule.student_arrival_exceptions").
		Where("student_id = ?", chain.StudentID).
		Scan(context.Background(), &count))
	assert.Equal(t, 0, count, "arrival write must not persist when the day is staff-owned")
}

// TestSubmitCareException_ClearingLegRemovesIt locks in the full-replacement
// contract: the submitted times are the complete override for the day, so
// re-submitting with one leg nil clears that leg's guardian row instead of
// silently keeping the previously-saved value (the parents-portal modal sends
// null for an emptied field).
func TestSubmitCareException_ClearingLegRemovesIt(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()

	date := timezone.TodayDate().AddDays(1)

	// Both legs set first.
	_, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, wallClock(14, 30), wallClock(8, 15))
	require.NoError(t, err)

	// Re-submit with the arrival leg cleared (nil) and a changed pickup.
	result, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, wallClock(14, 0), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.PickupTime)
	assert.Equal(t, "14:00", result.PickupTime.Format("15:04"))
	assert.Nil(t, result.ArrivalTime, "cleared arrival leg must not linger in the merged result")

	// The arrival guardian row must be gone, the pickup row updated in place.
	var arrivalCount int
	require.NoError(t, db.NewSelect().
		ColumnExpr("COUNT(*)").
		TableExpr("schedule.student_arrival_exceptions").
		Where("student_id = ?", chain.StudentID).
		Scan(ctx, &arrivalCount))
	assert.Equal(t, 0, arrivalCount, "emptying the arrival field must delete the guardian row")

	list, err := svc.ListCareExceptions(ctx, chain.AccountID, chain.StudentID, timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].PickupTime)
	assert.Equal(t, "14:00", list[0].PickupTime.Format("15:04"))
	assert.Nil(t, list[0].ArrivalTime)
}

func TestListAndDeleteCareException(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()

	date := timezone.TodayDate().AddDays(3)
	_, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, wallClock(14, 45), nil)
	require.NoError(t, err)

	from := timezone.TodayDate()
	to := timezone.TodayDate().AddDays(30)
	list, err := svc.ListCareExceptions(ctx, chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].PickupTime)
	assert.Equal(t, "14:45", list[0].PickupTime.Format("15:04"))
	assert.Equal(t, scheduleModels.ExceptionSourceGuardian, list[0].Source)

	require.NoError(t, svc.DeleteCareException(ctx, chain.AccountID, chain.StudentID, date))

	after, err := svc.ListCareExceptions(ctx, chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	assert.Empty(t, after, "delete must revert the day to the standard plan")
}

// TestGuardianExceptionSurvivesAccountDeletion guards migration 1.15.136: a
// parent-authored exception must NOT vanish when the guardian account is deleted
// (the old ON DELETE CASCADE silently erased future care instructions). The FK
// is now ON DELETE SET NULL and chk_exception_author tolerates the orphaned
// guardian row, so the time staff rely on stays put — only the author link clears.
func TestGuardianExceptionSurvivesAccountDeletion(t *testing.T) {
	_, _, db := buildCareService(t, true)
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "Orphan", "Care", "3c")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// A standalone account nothing else references, so deleting it exercises only
	// the exception's created_by_guardian FK.
	account := testpkg.CreateTestAccount(t, db, "orphan-guardian")

	date := timezone.TodayDate().AddDays(5)
	guardianID := account.ID
	exception := &scheduleModels.StudentPickupException{
		StudentID:         student.ID,
		ExceptionDate:     date,
		PickupTime:        wallClock(13, 30),
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &guardianID,
	}
	exception.SetTenantID(1)
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.StudentPickupException.Create(ctx, exception))
	defer func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM schedule.student_pickup_exceptions WHERE student_id = ?`, student.ID)
	}()

	// Hard-delete the guardian account.
	_, err := db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	require.NoError(t, err)

	// The care instruction survives; only the author link is cleared.
	persisted, err := repos.StudentPickupException.FindByStudentIDAndDate(ctx, student.ID, date)
	require.NoError(t, err)
	require.NotNil(t, persisted, "exception must survive guardian account deletion (no CASCADE)")
	assert.Nil(t, persisted.CreatedByGuardian, "author link is cleared to NULL")
	assert.Equal(t, scheduleModels.ExceptionSourceGuardian, persisted.Source, "row stays a guardian-sourced override")
	require.NotNil(t, persisted.PickupTime)
	assert.Equal(t, "13:30", persisted.PickupTime.Format("15:04"))
}

func TestDeleteCareException_PastDate(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()

	date := timezone.TodayDate().AddDays(-1)
	guardianID := chain.AccountID
	exception := &scheduleModels.StudentPickupException{
		StudentID:         chain.StudentID,
		ExceptionDate:     date,
		PickupTime:        wallClock(14, 45),
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &guardianID,
	}
	exception.SetTenantID(chain.TenantID)
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.StudentPickupException.Create(ctx, exception))

	err := svc.DeleteCareException(ctx, chain.AccountID, chain.StudentID, date)
	assert.ErrorIs(t, err, parentService.ErrPastCareDate)

	persisted, err := repos.StudentPickupException.FindByStudentIDAndDate(ctx, chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, persisted, "past guardian row must remain for audit/history")
	assert.Equal(t, exception.ID, persisted.ID)
}

// TestSubmitCareException_ArrivalOnlyThenUpdateInPlace exercises the arrival
// leg in isolation: an arrival-only submit (pickup stays nil, so no pickup row
// is created), followed by a second arrival-only submit that updates the same
// guardian row in place rather than inserting a duplicate. The pickup-leg
// equivalents are covered elsewhere; this is the arrival mirror.
func TestSubmitCareException_ArrivalOnlyThenUpdateInPlace(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()
	repos := repositories.NewFactory(db)

	date := timezone.TodayDate().AddDays(1)

	first, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, nil, wallClock(8, 0))
	require.NoError(t, err)
	require.NotNil(t, first.ArrivalTime)
	assert.Nil(t, first.PickupTime, "arrival-only submit must not create a pickup row")
	assert.Equal(t, "08:00", first.ArrivalTime.Format("15:04"))

	original, err := repos.StudentArrivalException.FindByStudentIDAndDate(ctx, chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, original)

	// Second arrival-only submit: same date, new time. Must update the existing
	// guardian row, keeping its id, not insert a second row.
	second, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, nil, wallClock(9, 30))
	require.NoError(t, err)
	require.NotNil(t, second.ArrivalTime)
	assert.Equal(t, "09:30", second.ArrivalTime.Format("15:04"))

	updated, err := repos.StudentArrivalException.FindByStudentIDAndDate(ctx, chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, original.ID, updated.ID, "arrival leg must be updated in place, not duplicated")
	assert.Equal(t, scheduleModels.ExceptionSourceGuardian, updated.Source)

	var pickupCount int
	require.NoError(t, db.NewSelect().
		ColumnExpr("COUNT(*)").
		TableExpr("schedule.student_pickup_exceptions").
		Where("student_id = ?", chain.StudentID).
		Scan(ctx, &pickupCount))
	assert.Equal(t, 0, pickupCount, "arrival-only flow must never touch the pickup table")
}

// TestSubmitCareException_ClearPickupKeepsArrival locks in the per-leg
// full-replacement contract from the opposite side of
// TestSubmitCareException_ClearingLegRemovesIt: clearing the pickup leg deletes
// its guardian row while the arrival leg is updated in the same submit.
func TestSubmitCareException_ClearPickupKeepsArrival(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()

	date := timezone.TodayDate().AddDays(1)

	_, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, wallClock(14, 30), wallClock(8, 15))
	require.NoError(t, err)

	// Clear the pickup leg (nil) and change the arrival leg in one submit.
	result, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, nil, wallClock(8, 45))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.PickupTime, "cleared pickup leg must not linger")
	require.NotNil(t, result.ArrivalTime)
	assert.Equal(t, "08:45", result.ArrivalTime.Format("15:04"))

	var pickupCount int
	require.NoError(t, db.NewSelect().
		ColumnExpr("COUNT(*)").
		TableExpr("schedule.student_pickup_exceptions").
		Where("student_id = ?", chain.StudentID).
		Scan(ctx, &pickupCount))
	assert.Equal(t, 0, pickupCount, "emptying the pickup field must delete its guardian row")
}

// TestListCareExceptions_MergesBothLegsAndFlagsStaffSource covers the merge
// projection across multiple days: a day with BOTH guardian legs (the second
// leg must reuse the same merged entry), plus staff-authored pickup and arrival
// days that must surface with Source = "staff" so the portal renders them
// read-only.
func TestListCareExceptions_MergesBothLegsAndFlagsStaffSource(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	staff := testpkg.CreateTestStaff(t, db, "Team", "Mitglied")
	ctx := context.Background()
	repos := repositories.NewFactory(db)
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_exceptions WHERE student_id = ?`, chain.StudentID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_arrival_exceptions WHERE student_id = ?`, chain.StudentID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staff.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, staff.PersonID)
	}()

	guardianDay := timezone.TodayDate().AddDays(1)
	staffPickupDay := timezone.TodayDate().AddDays(2)
	staffArrivalDay := timezone.TodayDate().AddDays(3)

	// Day 1: both guardian legs (forces the merge to fold two rows into one entry).
	_, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, guardianDay, wallClock(15, 0), wallClock(8, 0))
	require.NoError(t, err)

	// Day 2: a staff-authored pickup row.
	staffPickup := &scheduleModels.StudentPickupException{
		StudentID:     chain.StudentID,
		ExceptionDate: staffPickupDay,
		PickupTime:    wallClock(16, 0),
		CreatedBy:     staff.ID,
	}
	staffPickup.SetTenantID(chain.TenantID)
	require.NoError(t, repos.StudentPickupException.Create(ctx, staffPickup))

	// Day 3: a staff-authored arrival row.
	staffArrival := &scheduleModels.StudentArrivalException{
		StudentID:       chain.StudentID,
		ExceptionDate:   staffArrivalDay,
		ExpectedArrival: wallClock(7, 45),
		CreatedBy:       staff.ID,
	}
	staffArrival.SetTenantID(chain.TenantID)
	require.NoError(t, repos.StudentArrivalException.Create(ctx, staffArrival))

	list, err := svc.ListCareExceptions(ctx, chain.AccountID, chain.StudentID,
		timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	require.NoError(t, err)
	require.Len(t, list, 3, "each distinct date is one merged entry")

	byDate := map[timezone.Date]*parentService.CareException{}
	for _, ce := range list {
		byDate[ce.Date] = ce
	}

	merged := byDate[guardianDay]
	require.NotNil(t, merged)
	require.NotNil(t, merged.PickupTime)
	require.NotNil(t, merged.ArrivalTime)
	assert.Equal(t, "15:00", merged.PickupTime.Format("15:04"))
	assert.Equal(t, "08:00", merged.ArrivalTime.Format("15:04"))
	assert.Equal(t, scheduleModels.ExceptionSourceGuardian, merged.Source)

	require.NotNil(t, byDate[staffPickupDay])
	assert.Equal(t, scheduleModels.ExceptionSourceStaff, byDate[staffPickupDay].Source,
		"a staff pickup row must mark the day staff-owned")
	require.NotNil(t, byDate[staffArrivalDay])
	assert.Equal(t, scheduleModels.ExceptionSourceStaff, byDate[staffArrivalDay].Source,
		"a staff arrival row must mark the day staff-owned")
}

// TestListCareExceptions_FlagsAbsentPickupRow proves a staff pickup exception
// with NO time (StudentPickupException.IsAbsent — "not coming today") surfaces as
// PickupAbsent so the parent tile resolves it as an absence rather than falling
// back to the base-plan pickup. Such a row creates no status day, so the
// care-schedule today_absent signal alone would miss it (#1725 review).
func TestListCareExceptions_FlagsAbsentPickupRow(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	staff := testpkg.CreateTestStaff(t, db, "Team", "Mitglied")
	ctx := context.Background()
	repos := repositories.NewFactory(db)
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_pickup_exceptions WHERE student_id = ?`, chain.StudentID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staff.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, staff.PersonID)
	}()

	absentDay := timezone.TodayDate().AddDays(1)
	// A staff pickup row with no time — the "child is absent today" marker.
	absent := &scheduleModels.StudentPickupException{
		StudentID:     chain.StudentID,
		ExceptionDate: absentDay,
		PickupTime:    nil,
		CreatedBy:     staff.ID,
	}
	absent.SetTenantID(chain.TenantID)
	require.NoError(t, repos.StudentPickupException.Create(ctx, absent))

	list, err := svc.ListCareExceptions(ctx, chain.AccountID, chain.StudentID,
		timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, absentDay, list[0].Date)
	assert.Nil(t, list[0].PickupTime, "an absent pickup row carries no time")
	assert.True(t, list[0].PickupAbsent, "a timeless pickup row must flag the day absent")
	assert.False(t, list[0].ArrivalAbsent, "no arrival row exists, so the arrival leg is not absent")
	assert.Equal(t, scheduleModels.ExceptionSourceStaff, list[0].Source)
}

// TestListCareExceptions_FlagsAbsentArrivalRow is the arrival-leg twin of
// TestListCareExceptions_FlagsAbsentPickupRow: a staff arrival exception with NO
// expected time (StudentArrivalException.IsAbsent — "not coming today") surfaces
// as ArrivalAbsent. Like a timeless pickup row it creates no status day, so the
// today_absent signal alone would miss it and the parent tile would wrongly fall
// back to the base-plan pickup for a child who is not coming (#1725 review).
func TestListCareExceptions_FlagsAbsentArrivalRow(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	staff := testpkg.CreateTestStaff(t, db, "Team", "Mitglied")
	ctx := context.Background()
	repos := repositories.NewFactory(db)
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.student_arrival_exceptions WHERE student_id = ?`, chain.StudentID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.staff WHERE id = ?`, staff.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, staff.PersonID)
	}()

	absentDay := timezone.TodayDate().AddDays(1)
	// A staff arrival row with no expected time — the "child is absent today" marker.
	absent := &scheduleModels.StudentArrivalException{
		StudentID:       chain.StudentID,
		ExceptionDate:   absentDay,
		ExpectedArrival: nil,
		CreatedBy:       staff.ID,
	}
	absent.SetTenantID(chain.TenantID)
	require.NoError(t, repos.StudentArrivalException.Create(ctx, absent))

	list, err := svc.ListCareExceptions(ctx, chain.AccountID, chain.StudentID,
		timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, absentDay, list[0].Date)
	assert.Nil(t, list[0].ArrivalTime, "an absent arrival row carries no time")
	assert.True(t, list[0].ArrivalAbsent, "a timeless arrival row must flag the day absent")
	assert.False(t, list[0].PickupAbsent, "no pickup row exists, so the pickup leg is not absent")
	assert.Equal(t, scheduleModels.ExceptionSourceStaff, list[0].Source)
}

// TestDeleteCareException_RemovesBothLegs covers the arrival side of the delete
// path (the pickup-only delete is covered by TestListAndDeleteCareException):
// a day with both guardian legs must have both rows removed and broadcast once.
func TestDeleteCareException_RemovesBothLegs(t *testing.T) {
	svc, bc, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()

	date := timezone.TodayDate().AddDays(2)
	_, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, wallClock(15, 30), wallClock(8, 30))
	require.NoError(t, err)

	require.NoError(t, svc.DeleteCareException(ctx, chain.AccountID, chain.StudentID, date))
	assert.Contains(t, tenantBroadcastIDs(bc), chain.TenantID, "deleting guardian rows must broadcast a student update")

	for _, table := range []string{"schedule.student_pickup_exceptions", "schedule.student_arrival_exceptions"} {
		var count int
		require.NoError(t, db.NewSelect().
			ColumnExpr("COUNT(*)").
			TableExpr(table).
			Where("student_id = ?", chain.StudentID).
			Scan(ctx, &count))
		assert.Equalf(t, 0, count, "%s guardian row must be deleted", table)
	}

	after, err := svc.ListCareExceptions(ctx, chain.AccountID, chain.StudentID,
		timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	require.NoError(t, err)
	assert.Empty(t, after)
}

// TestSubmitCareException_RepoErrorSurfaces verifies a read failure inside the
// transaction is wrapped and returned (not swallowed or misreported as a
// success/conflict), and that nothing is persisted. Guards against the service
// silently absorbing a DB outage during the staff-ownership check.
func TestSubmitCareException_RepoErrorSurfaces(t *testing.T) {
	svc, db := buildCareServiceWithPickupRepo(t, func(r scheduleModels.StudentPickupExceptionRepository) scheduleModels.StudentPickupExceptionRepository {
		return stubPickupRepo{StudentPickupExceptionRepository: r, findErr: errBoom}
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()

	_, err := svc.SubmitCareException(ctx, chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(1), wallClock(15, 0), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom, "DB error must propagate, not be swallowed")
	assert.NotErrorIs(t, err, parentService.ErrCareExceptionConflict, "a read failure is not a staff conflict")

	var count int
	require.NoError(t, db.NewSelect().
		ColumnExpr("COUNT(*)").
		TableExpr("schedule.student_pickup_exceptions").
		Where("student_id = ?", chain.StudentID).
		Scan(ctx, &count))
	assert.Equal(t, 0, count, "failed submit must roll back")
}

// TestListCareExceptions_RepoErrorSurfaces verifies a range-read failure is
// wrapped and returned rather than yielding a silently empty list (which the UI
// would render as "no overrides", hiding real ones).
func TestListCareExceptions_RepoErrorSurfaces(t *testing.T) {
	svc, db := buildCareServiceWithPickupRepo(t, func(r scheduleModels.StudentPickupExceptionRepository) scheduleModels.StudentPickupExceptionRepository {
		return stubPickupRepo{StudentPickupExceptionRepository: r, rangeErr: errBoom}
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	rows, err := svc.ListCareExceptions(context.Background(), chain.AccountID, chain.StudentID,
		timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
	assert.Nil(t, rows, "no partial list on error")
}

// TestDeleteCareException_RepoErrorSurfaces verifies a read failure during the
// delete transaction is wrapped and returned, so the caller learns the day was
// not reverted instead of seeing a false success.
func TestDeleteCareException_RepoErrorSurfaces(t *testing.T) {
	svc, db := buildCareServiceWithPickupRepo(t, func(r scheduleModels.StudentPickupExceptionRepository) scheduleModels.StudentPickupExceptionRepository {
		return stubPickupRepo{StudentPickupExceptionRepository: r, findErr: errBoom}
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	err := svc.DeleteCareException(context.Background(), chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(1))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
}

// TestSubmitCareException_ArrivalRepoErrorSurfaces is the arrival-leg mirror of
// TestSubmitCareException_RepoErrorSurfaces: a failure reading the arrival leg
// during the staff-ownership check must propagate, not be swallowed.
func TestSubmitCareException_ArrivalRepoErrorSurfaces(t *testing.T) {
	svc, db := buildCareServiceWithRepos(t, careRepoWrap{
		arrival: func(r scheduleModels.StudentArrivalExceptionRepository) scheduleModels.StudentArrivalExceptionRepository {
			return stubArrivalRepo{StudentArrivalExceptionRepository: r, findErr: errBoom}
		},
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitCareException(context.Background(), chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(1), nil, wallClock(8, 0))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
}

// TestListCareExceptions_ArrivalRepoErrorSurfaces mirrors the pickup range-read
// failure for the arrival leg.
func TestListCareExceptions_ArrivalRepoErrorSurfaces(t *testing.T) {
	svc, db := buildCareServiceWithRepos(t, careRepoWrap{
		arrival: func(r scheduleModels.StudentArrivalExceptionRepository) scheduleModels.StudentArrivalExceptionRepository {
			return stubArrivalRepo{StudentArrivalExceptionRepository: r, rangeErr: errBoom}
		},
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	rows, err := svc.ListCareExceptions(context.Background(), chain.AccountID, chain.StudentID,
		timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)
	assert.Nil(t, rows)
}

// TestDeleteCareException_ArrivalFindErrorSurfaces covers the arrival read leg
// of the delete transaction: a guardian pickup row is removed, then the arrival
// lookup fails and the whole delete must error (and roll back the pickup
// delete) rather than reporting a partial success.
func TestDeleteCareException_ArrivalFindErrorSurfaces(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	ctx := context.Background()
	date := timezone.TodayDate().AddDays(1)

	// Seed a guardian pickup row with the real service.
	realSvc, _, _ := buildCareService(t, true)
	_, err := realSvc.SubmitCareException(ctx, chain.AccountID, chain.StudentID, date, wallClock(15, 0), nil)
	require.NoError(t, err)

	// Delete via a service whose arrival reads fail.
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  repos.StudentPickupException,
		ArrivalExceptionRepo: stubArrivalRepo{StudentArrivalExceptionRepository: repos.StudentArrivalException, findErr: errBoom},
		Settings: parentSettingsStub{
			boolValues: map[string]bool{configModels.KeyParentPickupChangeEnabled: true},
		},
		Broadcaster: testpkg.NewRecordingBroadcaster(),
		DB:          db,
		Logger:      slog.Default(),
	})

	err = svc.DeleteCareException(ctx, chain.AccountID, chain.StudentID, date)
	require.Error(t, err)
	assert.ErrorIs(t, err, errBoom)

	// The pickup delete must have rolled back with the failed transaction.
	persisted, err := repos.StudentPickupException.FindByStudentIDAndDate(ctx, chain.StudentID, date)
	require.NoError(t, err)
	assert.NotNil(t, persisted, "failed delete tx must roll back the pickup delete")
}

// TestListCareExceptions_NotOwnedChild and TestDeleteCareException_NotOwnedChild
// guard authorization on the read/clear paths: both must reject a child the
// account does not guard, mirroring the submit-path ownership check.
func TestListCareExceptions_NotOwnedChild(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.ListCareExceptions(context.Background(), chain.AccountID, other.ID,
		timezone.TodayDate(), timezone.TodayDate().AddDays(30))
	assert.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

func TestDeleteCareException_NotOwnedChild(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	err := svc.DeleteCareException(context.Background(), chain.AccountID, other.ID,
		timezone.TodayDate().AddDays(1))
	assert.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

func TestSubmitCareExceptionWithReasonPersistsReason(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	date := timezone.TodayDate().AddDays(1)
	result, err := svc.SubmitCareExceptionWithReason(
		context.Background(),
		chain.AccountID,
		chain.StudentID,
		date,
		wallClock(14, 30),
		nil,
		"  Arzttermin  ",
	)
	require.NoError(t, err)
	require.NotNil(t, result.Reason)
	assert.Equal(t, "Arzttermin", *result.Reason)

	var reason string
	require.NoError(t, db.NewSelect().
		Column("reason").
		TableExpr("schedule.student_pickup_exceptions").
		Where("student_id = ?", chain.StudentID).
		Where("exception_date = ?", date).
		Scan(context.Background(), &reason))
	assert.Equal(t, "Arzttermin", reason)
}

func TestSubmitCareExceptionWithReasonValidatesInput(t *testing.T) {
	svc, _, db := buildCareService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	date := timezone.TodayDate().AddDays(1)
	_, err := svc.SubmitCareExceptionWithReason(
		context.Background(),
		chain.AccountID,
		chain.StudentID,
		date,
		wallClock(14, 30),
		nil,
		"   ",
	)
	assert.ErrorIs(t, err, parentService.ErrCareExceptionReasonRequired)

	_, err = svc.SubmitCareExceptionWithReason(
		context.Background(),
		chain.AccountID,
		chain.StudentID,
		date,
		wallClock(14, 30),
		nil,
		strings.Repeat("a", 256),
	)
	assert.ErrorIs(t, err, parentService.ErrCareExceptionReasonTooLong)
}
