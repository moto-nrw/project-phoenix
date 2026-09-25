package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A recurring weekday note must not need a pickup time (#3369): the child has
// no pickup row that weekday, stays not expected, and the note still reaches
// the day's tile through the bulk pickup-times payload.
func TestWeekdayPickupNoteWithoutPickupTime(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "WeekdayNote", "Test", "WDN1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Weekday", "NoteTeacher")
	claims := testutil.AdminTestClaims(int(account.ID))
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupNote)(nil)).
			ModelTableExpr("schedule.student_pickup_notes").
			Where("student_id = ?", student.ID).
			Exec(context.Background())
	})

	body := map[string]any{"weekday": 1, "content": "Montags bei den Großeltern"}
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
	rr := authExec(t, tc, req, claims, []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rr.Code, "Body: %s", rr.Body.String())

	var created struct {
		Data struct {
			ID       int64  `json:"id"`
			Weekday  int    `json:"weekday"`
			NoteDate string `json:"note_date"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	assert.Equal(t, 1, created.Data.Weekday)
	assert.Empty(t, created.Data.NoteDate, "a recurring note carries no date")

	t.Run("reaches_the_tile_on_every_such_weekday", func(t *testing.T) {
		for _, monday := range []string{"2026-01-26", "2026-02-02"} {
			req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", map[string]any{
				"student_ids": []int64{student.ID}, "date": monday,
			})
			rr := authExec(t, tc, req, claims, []string{"admin:*"})
			require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
			assert.Contains(t, rr.Body.String(), "Montags bei den Großeltern", "date %s", monday)
			assert.NotContains(t, rr.Body.String(), `"pickup_time":"`, "the note must not invent a pickup time")
		}
	})

	t.Run("stays_off_other_weekdays", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", map[string]any{
			"student_ids": []int64{student.ID}, "date": "2026-01-27", // Tuesday
		})
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code)
		assert.NotContains(t, rr.Body.String(), "Montags bei den Großeltern")
	})

	t.Run("update_keeps_it_recurring", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes/%d", student.ID, created.Data.ID),
			map[string]any{"weekday": 1, "content": "Montags bei Oma"})
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"weekday":1`)
		assert.Contains(t, rr.Body.String(), "Montags bei Oma")
	})

	t.Run("replaces_weekday_notes_without_touching_dated_notes", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID),
			map[string]any{"note_date": "2026-02-03", "content": "Nur an diesem Tag"})
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusCreated, rr.Code, "Body: %s", rr.Body.String())

		req = testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes", student.ID), map[string]any{
			"notes": []map[string]any{
				{"weekday": 1, "content": "Montags bei Tante"},
				{"weekday": 2, "content": "Dienstags zu Hause"},
			},
		})
		rr = authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		notes, err := tc.resource.PickupScheduleService.GetStudentPickupNotes(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		assert.Len(t, notes, 3)
		byWeekday := make(map[int]string, len(notes))
		var dated string
		for _, note := range notes {
			if note.Weekday == 0 {
				dated = note.Content
				continue
			}
			byWeekday[note.Weekday] = note.Content
		}
		assert.Equal(t, "Montags bei Tante", byWeekday[1])
		assert.Equal(t, "Dienstags zu Hause", byWeekday[2])
		assert.Equal(t, "Nur an diesem Tag", dated)
	})

	t.Run("rejects_date_and_weekday_together", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID),
			map[string]any{"weekday": 2, "note_date": "2026-01-27", "content": "x"})
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "not both")
	})

	t.Run("rejects_weekend_weekday", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID),
			map[string]any{"weekday": 6, "content": "x"})
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "weekday must be between")
	})

	t.Run("rejects_whitespace_only_weekday_note", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes", student.ID), map[string]any{
			"notes": []map[string]any{{"weekday": 2, "content": " \t "}},
		})
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "content is required")
	})
}

func TestReplaceWeekdayPickupNotesWaitsForStudentLock(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "WeekdayLock", "Test", "WDN3")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Weekday", "LockTeacher")
	claims := testutil.AdminTestClaims(int(account.ID))

	locked := make(chan struct{})
	release := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- testpkg.WithTenantTx(t, context.Background(), tc.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			if _, err := newStudentTestRepositories(tc.db).Student.FindByIDForUpdate(txCtx, student.ID); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()

	select {
	case <-locked:
	case err := <-lockDone:
		require.NoError(t, err)
		t.Fatal("student transaction ended before holding the lock")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out while acquiring the student lock")
	}

	released := false
	releaseLock := func() {
		if !released {
			close(release)
			released = true
		}
	}
	defer releaseLock()

	writeDone := make(chan int, 1)
	go func() {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-notes", student.ID), map[string]any{
			"notes": []map[string]any{{"weekday": 1, "content": "Montags zu Hause"}},
		})
		writeDone <- authExec(t, tc, req, claims, []string{"admin:*"}).Code
	}()

	select {
	case code := <-writeDone:
		require.Equal(t, http.StatusOK, code, "weekday note replacement bypassed the held student lock")
		t.Fatal("weekday note replacement bypassed the held student lock")
	case <-time.After(150 * time.Millisecond):
	}

	releaseLock()
	require.NoError(t, <-lockDone)
	assert.Equal(t, http.StatusOK, <-writeDone)
}

// Where bookings decide the care days, a child without a booking that day gets
// no pickup entry at all. The day's note must survive that boundary, and it
// must do so without turning the child into an expected one (#3369).
func TestWeekdayPickupNoteSurvivesBookingBoundary(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	require.NoError(t, tc.settings().SetValue(
		testpkg.Ctx(t), settings.KeyEnrollmentBookingsAuthoritative, true, nil, nil,
	))
	t.Cleanup(func() {
		_ = tc.settings().ResetValue(testpkg.Ctx(t), settings.KeyEnrollmentBookingsAuthoritative, nil, nil)
	})
	noted := testpkg.CreateTestStudent(t, tc.db, "Unbooked", "Noted", "WDN2")
	silent := testpkg.CreateTestStudent(t, tc.db, "Unbooked", "Silent", "WDN2")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Boundary", "NoteTeacher")
	claims := testutil.AdminTestClaims(int(account.ID))
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupNote)(nil)).
			ModelTableExpr("schedule.student_pickup_notes").
			Where("student_id = ?", noted.ID).
			Exec(context.Background())
	})

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", noted.ID),
		map[string]any{"weekday": 1, "content": "Montags beim Vater"})
	rr := authExec(t, tc, req, claims, []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rr.Code, "Body: %s", rr.Body.String())

	// The device-scan path resolves one child at a time rather than through
	// the tile's bulk endpoint. A note-only day must survive that path too.
	single, err := tc.resource.PickupScheduleService.GetEffectivePickupTimeForDate(
		testpkg.Ctx(t), noted.ID, timezone.NewDate(2026, time.January, 26),
	)
	require.NoError(t, err)
	require.NotNil(t, single)
	assert.Nil(t, single.PickupTime)
	require.Len(t, single.DayNotes, 1)
	assert.Equal(t, "Montags beim Vater", single.DayNotes[0].Content)

	req = testutil.NewAuthenticatedRequest(t, "POST", "/pickup-times/bulk", map[string]any{
		"student_ids": []int64{noted.ID, silent.ID}, "date": "2026-01-26", // Monday
	})
	rr = authExec(t, tc, req, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var response struct {
		Data []struct {
			StudentID  int64   `json:"student_id"`
			PickupTime *string `json:"pickup_time"`
			DayNotes   []struct {
				Content string `json:"content"`
			} `json:"day_notes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	require.Len(t, response.Data, 1, "a child without booking and without note stays out of the payload")
	assert.Equal(t, noted.ID, response.Data[0].StudentID)
	assert.Nil(t, response.Data[0].PickupTime)
	require.Len(t, response.Data[0].DayNotes, 1)
	assert.Equal(t, "Montags beim Vater", response.Data[0].DayNotes[0].Content)
}
