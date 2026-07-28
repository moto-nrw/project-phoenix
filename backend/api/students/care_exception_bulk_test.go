package students_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Bulk Care Exception Tests (#893)
//
// The care plan editor applies one override to a stretch of days. These cover
// the endpoints that back the "gilt für mehrere Tage" scope: one transaction,
// existing days overwritten rather than rejected, guardian days still guarded.
// =============================================================================

func TestBulkUpsertStudentPickupExceptions(t *testing.T) {
	tc := setupTestContext(t)

	cleanupPickupExceptions := func(t *testing.T, studentID int64) {
		t.Helper()
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Where("student_id = ?", studentID).
			Exec(context.Background())
	}

	t.Run("writes_every_listed_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkPickup", "Test", "BPU1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Bulk", "PickupTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)
		defer cleanupPickupExceptions(t, student.ID)

		body := map[string]any{
			"dates":       []string{"2026-03-16", "2026-03-17", "2026-03-18"},
			"pickup_time": "13:30",
			"reason":      "Projektwoche",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions/bulk", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		var count int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("COUNT(*)").
			Where("student_id = ?", student.ID).
			Scan(context.Background(), &count))
		assert.Equal(t, 3, count, "every listed date must get a row")
	})

	t.Run("empty_pickup_time_means_no_pickup", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkNoPickup", "Test", "BPU2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkNone", "PickupTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)
		defer cleanupPickupExceptions(t, student.ID)

		body := map[string]any{
			"dates":  []string{"2026-03-23", "2026-03-24"},
			"reason": "Ferienbetreuung entfällt",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions/bulk", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		var withTime int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("COUNT(*)").
			Where("student_id = ?", student.ID).
			Where("pickup_time IS NOT NULL").
			Scan(context.Background(), &withTime))
		assert.Equal(t, 0, withTime, "an omitted time must persist as 'no pickup', not as a zero clock")
	})

	t.Run("overwrites_an_existing_staff_day_instead_of_conflicting", func(t *testing.T) {
		// The single-date endpoint answers 409 here, on purpose: it protects a
		// day-level edit against a concurrent writer. A range edit explicitly
		// asks for every covered day, so one pre-existing override must not
		// fail the whole batch.
		student := testpkg.CreateTestStudent(t, tc.db, "BulkOverwrite", "Test", "BPU3")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkOver", "PickupTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)
		defer cleanupPickupExceptions(t, student.ID)

		first := map[string]any{
			"exception_date": "2026-03-30",
			"pickup_time":    "15:00",
			"reason":         "Erste Fassung",
		}
		firstReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), first)
		firstRR := authExec(t, tc, firstReq, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
		require.Equal(t, http.StatusCreated, firstRR.Code, "setup create failed: %s", firstRR.Body.String())

		body := map[string]any{
			"dates":       []string{"2026-03-30", "2026-03-31"},
			"pickup_time": "12:15",
			"reason":      "Neue Fassung",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions/bulk", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		var count int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("COUNT(*)").
			Where("student_id = ?", student.ID).
			Scan(context.Background(), &count))
		assert.Equal(t, 2, count, "the overwritten day must not be duplicated")

		var reason string
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("reason").
			Where("student_id = ?", student.ID).
			Where("exception_date = ?", timezone.NewDate(2026, 3, 30)).
			Scan(context.Background(), &reason))
		assert.Equal(t, "Neue Fassung", reason)
	})

	t.Run("reclaims_a_guardian_day_for_staff", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
		defer testpkg.CleanupParentGuardianChain(t, tc.db, chain)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkReclaim", "PickupTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)
		defer cleanupPickupExceptions(t, chain.StudentID)

		exceptionDate := timezone.NewDate(2026, 4, 20)
		parentReason := "Elternangabe"
		guardianRow := &scheduleModel.StudentPickupException{
			StudentID:         chain.StudentID,
			ExceptionDate:     exceptionDate,
			Reason:            &parentReason,
			Source:            scheduleModel.ExceptionSourceGuardian,
			CreatedByGuardian: &chain.AccountID,
		}
		guardianRow.SetTenantID(chain.TenantID)
		_, err := tc.db.NewInsert().Model(guardianRow).
			ModelTableExpr("schedule.student_pickup_exceptions").
			Returning("id").
			Exec(context.Background())
		require.NoError(t, err)

		body := map[string]any{
			"dates":       []string{"2026-04-20", "2026-04-21"},
			"pickup_time": "14:00",
			"reason":      "Von der Einrichtung geändert",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions/bulk", chain.StudentID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		var source string
		var createdByGuardian *int64
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_pickup_exceptions").
			ColumnExpr("source").
			ColumnExpr("created_by_guardian").
			Where("student_id = ?", chain.StudentID).
			Where("exception_date = ?", exceptionDate).
			Scan(context.Background(), &source, &createdByGuardian))
		assert.Equal(t, scheduleModel.ExceptionSourceStaff, source)
		assert.Nil(t, createdByGuardian, "the guardian link must be dropped once staff take the day over")
	})

	t.Run("rejects_an_empty_or_malformed_date_list", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkInvalid", "Test", "BPU4")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkBad", "PickupTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		cases := []struct {
			name string
			body map[string]any
		}{
			{"empty", map[string]any{"dates": []string{}, "pickup_time": "14:00"}},
			{"bad_format", map[string]any{"dates": []string{"15.03.2026"}, "pickup_time": "14:00"}},
			{"duplicate", map[string]any{"dates": []string{"2026-03-16", "2026-03-16"}}},
			{"bad_time", map[string]any{"dates": []string{"2026-03-16"}, "pickup_time": "25:00"}},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions/bulk", student.ID), tt.body)
				rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
				assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
			})
		}
	})

	t.Run("rejects_more_dates_than_the_batch_cap", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkCap", "Test", "BPU5")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkCap", "PickupTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

		dates := make([]string, 0, 61)
		day := timezone.NewDate(2026, 1, 1)
		for i := 0; i < 61; i++ {
			dates = append(dates, day.String())
			day = day.AddDays(1)
		}

		body := map[string]any{"dates": dates, "pickup_time": "14:00"}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions/bulk", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	})
}

func TestBulkUpsertStudentArrivalExceptions(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("writes_every_listed_date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkArrival", "Test", "BAR1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Bulk", "ArrivalTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
				ModelTableExpr("schedule.student_arrival_exceptions").
				Where("student_id = ?", student.ID).
				Exec(context.Background())
		}()

		body := map[string]any{
			"dates":            []string{"2026-05-04", "2026-05-05"},
			"expected_arrival": "09:15",
			"reason":           "Schwimmunterricht",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions/bulk", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		var count int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_arrival_exceptions").
			ColumnExpr("COUNT(*)").
			Where("student_id = ?", student.ID).
			Scan(context.Background(), &count))
		assert.Equal(t, 2, count, "every listed date must get a row")
	})

	t.Run("empty_arrival_means_does_not_come", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "BulkNoArrival", "Test", "BAR2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "BulkNone", "ArrivalTeacher")
		defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)
		defer func() {
			_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentArrivalException)(nil)).
				ModelTableExpr("schedule.student_arrival_exceptions").
				Where("student_id = ?", student.ID).
				Exec(context.Background())
		}()

		body := map[string]any{
			"dates":  []string{"2026-05-11", "2026-05-12"},
			"reason": "Kur",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/arrival-exceptions/bulk", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})

		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		var withTime int
		require.NoError(t, tc.db.NewSelect().
			TableExpr("schedule.student_arrival_exceptions").
			ColumnExpr("COUNT(*)").
			Where("student_id = ?", student.ID).
			Where("expected_arrival IS NOT NULL").
			Scan(context.Background(), &withTime))
		assert.Equal(t, 0, withTime, "an omitted arrival must persist as 'does not come'")
	})
}
