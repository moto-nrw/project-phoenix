package students_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestCreateStudent_WithSchedules verifies a student and its weekly arrival and
// pickup schedules are created together in one atomic request (issue #1502),
// mirroring the existing atomic guardian-creation path.
func TestCreateStudent_WithSchedules(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// The schedule path stamps CreatedBy from the JWT, so the acting account
	// must resolve to a staff record (account → person → staff).
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Schedule", "Creator")

	body := map[string]interface{}{
		"first_name":   "Scheduled",
		"last_name":    "Child",
		"school_class": "1a",
		"arrival_schedules": []map[string]interface{}{
			{"weekday": 1, "expected_arrival": "07:30", "notes": "Kommt mit Bus"},
			{"weekday": 3, "expected_arrival": "08:00"},
		},
		"pickup_schedules": []map[string]interface{}{
			{"weekday": 1, "pickup_time": "15:00"},
			{"weekday": 3, "pickup_time": "16:30", "notes": "Oma holt ab"},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID, "expected a student id in the response")

	// Schedules FK to the student with ON DELETE CASCADE, so deleting the
	// student (via the shared guardian cleanup) also removes them.

	ctx := context.Background()

	arrivalCount, err := tc.db.NewSelect().
		Model((*scheduleModel.StudentArrivalSchedule)(nil)).
		ModelTableExpr("schedule.student_arrival_schedules").
		Where("student_id = ?", resp.Data.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, arrivalCount, "expected two weekly arrival schedules")

	pickupCount, err := tc.db.NewSelect().
		Model((*scheduleModel.StudentPickupSchedule)(nil)).
		ModelTableExpr("schedule.student_pickup_schedules").
		Where("student_id = ?", resp.Data.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, pickupCount, "expected two weekly pickup schedules")

	// Spot-check the persisted values: the time and notes mapped through, not
	// silently dropped.
	var arrival struct {
		ExpectedArrival string  `bun:"expected_arrival"`
		Notes           *string `bun:"notes"`
	}
	require.NoError(t, tc.db.NewSelect().
		ColumnExpr("expected_arrival, notes").
		TableExpr("schedule.student_arrival_schedules").
		Where("student_id = ? AND weekday = ?", resp.Data.ID, 1).
		Scan(ctx, &arrival))
	assert.Contains(t, arrival.ExpectedArrival, "07:30", "Monday arrival time should persist")
	require.NotNil(t, arrival.Notes)
	assert.Equal(t, "Kommt mit Bus", *arrival.Notes)
}

// TestCreateStudent_InvalidScheduleTimeRejected verifies a malformed schedule
// time is a client error (400) caught at Bind — before any row is written — so
// no orphaned person/student survives.
func TestCreateStudent_InvalidScheduleTimeRejected(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BadTimeSchedule", "Creator")

	const firstName = "BadTime"
	const lastName = "Schedule"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1b",
		"arrival_schedules": []map[string]interface{}{
			{"weekday": 1, "expected_arrival": "invalid"},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"invalid arrival time must return 400. Body: %s", rr.Body.String())

	ctx := context.Background()

	// Safety net in case a regressed validation let a row through.
	defer func() {
		var personIDs []int64
		if err := tc.db.NewSelect().
			Table("users.persons").
			Column("id").
			Where("first_name = ? AND last_name = ?", firstName, lastName).
			Scan(ctx, &personIDs); err == nil {
			for _, pid := range personIDs {
				if _, err := tc.db.NewDelete().Table("users.students").Where("person_id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup students: %v", err)
				}
				if _, err := tc.db.NewDelete().Table("users.persons").Where("id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup persons: %v", err)
				}
			}
		}
	}()

	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount, "person must not persist when schedule validation fails (Bind rejects before any write)")
}

// TestCreateStudent_WithSchedulesRequiresUsersUpdate pins the permission split:
// users:create is enough for creating the student itself, but attached weekly
// care schedules are Betreuungszeiten writes and must require the same
// users:update permission as the standalone update endpoints.
func TestCreateStudent_WithSchedulesRequiresUsersUpdate(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ScheduleCreateOnly", "Creator")

	const firstName = "CreateOnly"
	const lastName = "ScheduleDenied"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1c",
		"arrival_schedules": []map[string]interface{}{
			{"weekday": 1, "expected_arrival": "07:30"},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"users:create"})

	assert.Equal(t, http.StatusForbidden, rr.Code,
		"create-only users must not create Betreuungszeiten. Body: %s", rr.Body.String())

	ctx := context.Background()
	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount, "student/person must not persist when schedule permission is missing")
}

// TestCreateStudent_GuardianFailureRollsBackSchedules verifies the combined
// student+guardian+schedule create is one atomic unit: when the request carries
// valid weekly schedules but an invalid guardian, the whole transaction rolls
// back and NO schedule rows leak (mirrors the #1500 guardian-rollback test).
//
// Note on coverage: the schedule write is the terminal step of the create
// transaction, every schedule field is validated at Bind, and CreatedBy is
// guaranteed > 0 by getStaffIDFromJWT — so the schedule insert itself cannot be
// made to fail from a black-box handler test without mocking the service. This
// test therefore exercises the realistic atomic-failure path (a bad guardian)
// and locks the contract that schedules never survive a rolled-back create. The
// happy-path persistence of schedules inside the same transaction is covered by
// TestCreateStudent_WithSchedules above.
func TestCreateStudent_GuardianFailureRollsBackSchedules(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// The schedule path resolves the acting staff from the JWT, so the account
	// must map to a staff record (account → person → staff).
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ScheduleRollback", "Creator")

	const firstName = "ScheduleAtomic"
	const lastName = "Orphan"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1d",
		// Valid schedules — these must not survive the rollback triggered below.
		"arrival_schedules": []map[string]interface{}{
			{"weekday": 1, "expected_arrival": "07:45"},
		},
		"pickup_schedules": []map[string]interface{}{
			{"weekday": 1, "pickup_time": "15:30"},
		},
		// Invalid guardian (bad email) aborts the transaction.
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Invalid",
				"last_name":          "Email",
				"email":              "not-a-valid-email",
				"relationship_type":  "parent",
				"emergency_priority": 1,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"invalid guardian email must return 400 even with schedules attached. Body: %s", rr.Body.String())

	ctx := context.Background()

	// Safety net in case the rollback regressed and left residue behind.
	defer func() {
		var personIDs []int64
		if err := tc.db.NewSelect().
			Table("users.persons").
			Column("id").
			Where("first_name = ? AND last_name = ?", firstName, lastName).
			Scan(ctx, &personIDs); err == nil {
			for _, pid := range personIDs {
				if _, err := tc.db.NewDelete().Table("users.students").Where("person_id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup students: %v", err)
				}
				if _, err := tc.db.NewDelete().Table("users.persons").Where("id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup persons: %v", err)
				}
			}
		}
	}()

	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount, "person must not persist when the guardian fails (transaction must roll back)")

	// Schedules FK to the student; assert none leaked by joining back to the
	// would-be person name. Catches any regression where schedules are written
	// outside the create transaction.
	studentSubquery := tc.db.NewSelect().
		Table("users.students").
		Column("id").
		Where("person_id IN (SELECT id FROM users.persons WHERE first_name = ? AND last_name = ?)", firstName, lastName)

	arrivalCount, err := tc.db.NewSelect().
		Model((*scheduleModel.StudentArrivalSchedule)(nil)).
		ModelTableExpr("schedule.student_arrival_schedules").
		Where("student_id IN (?)", studentSubquery).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, arrivalCount, "arrival schedules must not survive a rolled-back create")

	pickupCount, err := tc.db.NewSelect().
		Model((*scheduleModel.StudentPickupSchedule)(nil)).
		ModelTableExpr("schedule.student_pickup_schedules").
		Where("student_id IN (?)", studentSubquery).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, pickupCount, "pickup schedules must not survive a rolled-back create")
}

// TestCreateStudent_InvalidPickupTimeRejected is the pickup counterpart to
// TestCreateStudent_InvalidScheduleTimeRejected: a malformed pickup time must be
// caught at Bind (400) before any row is written. The arrival list is empty here
// so this exercises the pickup validation branch specifically — the same
// validatePickupScheduleItems rules the bulk-update endpoint enforces.
func TestCreateStudent_InvalidPickupTimeRejected(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BadPickupSchedule", "Creator")

	const firstName = "BadPickup"
	const lastName = "Schedule"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1e",
		"pickup_schedules": []map[string]interface{}{
			{"weekday": 2, "pickup_time": "25:99"},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"invalid pickup time must return 400. Body: %s", rr.Body.String())

	ctx := context.Background()

	// Safety net in case a regressed validation let a row through.
	defer func() {
		var personIDs []int64
		if err := tc.db.NewSelect().
			Table("users.persons").
			Column("id").
			Where("first_name = ? AND last_name = ?", firstName, lastName).
			Scan(ctx, &personIDs); err == nil {
			for _, pid := range personIDs {
				if _, err := tc.db.NewDelete().Table("users.students").Where("person_id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup students: %v", err)
				}
				if _, err := tc.db.NewDelete().Table("users.persons").Where("id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup persons: %v", err)
				}
			}
		}
	}()

	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount, "person must not persist when pickup validation fails (Bind rejects before any write)")
}

// TestCreateStudent_SchedulesNonStaffAccountForbidden pins the staff-resolution
// contract: schedule rows are stamped with CreatedBy, so the acting JWT account
// must resolve to a staff record. An account that has users:update but maps to a
// non-staff person (e.g. a guardian-only or bare person account) must be refused
// with 403 — and no student/person may be created. Covers the getStaffIDFromJWT
// failure branch of the atomic create path.
func TestCreateStudent_SchedulesNonStaffAccountForbidden(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Person + account WITHOUT a staff record — getStaffIDFromJWT must fail.
	_, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "NonStaff", "Scheduler")

	const firstName = "NonStaffCreated"
	const lastName = "Child"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1f",
		"arrival_schedules": []map[string]interface{}{
			{"weekday": 1, "expected_arrival": "07:30"},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	// users:update passes the permission gate so the staff-resolution branch is
	// the thing under test, not the permission check.
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"users:create", "users:update"})

	assert.Equal(t, http.StatusForbidden, rr.Code,
		"non-staff account must not create Betreuungszeiten. Body: %s", rr.Body.String())

	ctx := context.Background()
	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount, "student/person must not persist when staff resolution fails")
}
