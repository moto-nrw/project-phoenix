package contracttest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Bind the real ledger and root bridge: corrections must consume the events
// that decisions and earlier corrections actually persisted.
func pickupCorrectionFixture(t *testing.T) (*careFixture, calendar.Date, int64) {
	t.Helper()
	f := newPickupChangeFixture(t)
	ctx := correctionContext(t, f)
	date := calendar.Date("2026-08-26")
	require.NoError(t, f.schedules.Pickups.UpsertStudentPickupSchedule(ctx, &careplan.PickupSchedule{
		StudentID: f.chain.StudentID, Weekday: int(date.Weekday()), PickupTime: *wallClockAt(16, 0), CreatedBy: f.staffID,
	}))
	room := testpkg.CreateTestRoom(t, f.db, "Correction block")
	block := testpkg.CreateTestActivityInstance(t, f.db, date, room.ID, testpkg.ActivityInstanceOpts{StartHHMM: "15:00", EndHHMM: "16:00"})
	slot := testpkg.CreateTestInstanceStudent(t, f.db, block.ID, f.chain.StudentID, studentpresence.SessionAttendanceExpected)
	slotID := slot.ID
	permissionChecks := owners.NewPickupPermissionApprovals(
		owners.NewParentGuardianProfileReads(f.reposPeople.GuardianProfile),
		owners.NewParentStudentGuardianReads(f.reposPeople.StudentGuardian),
		owners.NewParentGuardianPolicy(), nil)
	f.svc = owners.NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		f.reposCare.CarePlan(), f.persons, f.schedules.Arrivals, f.schedules.Pickups,
		newPickupChangePresence(t, f.db), f.autoExcusal, f.reposCare.CarePlan(), f.caller,
		f.emitter(t, messagingOnSettings{}, nil), nil, testpkg.RequestReviewPolicy[*peopledirectory.StudentState]{UserContext: f.caller},
		testpkg.PermissionsFromContext, f.reposPeople.ParentRequestEvent, slog.Default(), f.services.StudentAudit,
		owners.WithCareRequestToday(func() calendar.Date { return "2026-08-24" }),
		owners.WithPickupPermissionApprovals(permissionChecks))
	return f, date, slotID
}

func correctionContext(t *testing.T, f *careFixture) context.Context {
	t.Helper()
	return testpkg.WithTenantRuntime(t, f.staffCtx(f.staffAccount), f.db)
}

func correctionRequest(t *testing.T, f *careFixture, date calendar.Date, approve bool) carerequests.Request {
	t.Helper()
	contact := testpkg.CreateTestGuardianProfile(t, f.db, "collector@example.test")
	link := &testpkg.StudentGuardian{TenantID: f.chain.TenantID, StudentID: f.chain.StudentID,
		GuardianProfileID: contact.ID, RelationshipType: "other"}
	testpkg.ApplyStudentGuardianRole(link, "contact")
	require.NoError(t, testpkg.InsertTestStudentGuardian(correctionContext(t, f), f.db, link))
	raw, err := json.Marshal(map[string]any{
		"date": date.String(), "pickup_time": "14:00", "departure_mode": "pickup", "reason": "Termin",
		"pickup_person": map[string]any{"guardian_profile_id": contact.ID, "name": "Sabine Schneider"},
	})
	require.NoError(t, err)
	owner := f.reposCare.CarePlan()
	row, err := owner.CreateCareScheduleRequest(correctionContext(t, f), carerequests.Request{
		StudentID: f.chain.StudentID, SubmittedBy: f.chain.AccountID, RequestKind: carerequests.KindPickupChange,
		Status: carerequests.StatusPending, Payload: raw,
	})
	require.NoError(t, err)
	result, err := f.svc.Decide(correctionContext(t, f), carerequests.DecideInput{
		RequestID: row.ID, Approve: approve, Reason: "Geprüft", ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)
	return *result.Request
}

func correctPickup(ctx context.Context, t *testing.T, f *careFixture, row carerequests.Request, approve bool) error {
	t.Helper()
	return testpkg.WithTenantTx(t, ctx, f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return f.svc.(pickupCorrecter).Correct(txCtx, row.ID, approve, careplan.ParentRequestVersion(row.UpdatedAt), "Korrektur", f.staffAccount)
	})
}

func currentCorrectionRequest(t *testing.T, f *careFixture, id int64) carerequests.Request {
	t.Helper()
	row, err := f.reposCare.CarePlan().FindCareScheduleRequest(correctionContext(t, f), id, false)
	require.NoError(t, err)
	return row
}

func assertCorrectionEffects(t *testing.T, f *careFixture, date calendar.Date, slotID int64, approved bool) {
	t.Helper()
	ctx := correctionContext(t, f)
	owner := f.reposCare.CarePlan()
	filter := careplan.StudentScheduleFilter{StudentIDs: []int64{f.chain.StudentID}, Date: careplan.Date(date)}
	pickups, err := owner.ListPickupExceptions(ctx, filter)
	require.NoError(t, err)
	departures, err := owner.ListDepartureExceptions(ctx, filter)
	require.NoError(t, err)
	notes, err := owner.ListPickupNotes(ctx, filter)
	require.NoError(t, err)
	slot := testpkg.InstanceStudentByIDContext(t, ctx, f.db, slotID)
	if approved {
		require.Len(t, pickups, 1)
		require.Len(t, departures, 1)
		require.Len(t, notes, 1)
		assert.Equal(t, studentpresence.SessionAttendanceAbsent, slot.Status)
		require.NotNil(t, slot.PickupExceptionID)
		assert.Equal(t, pickups[0].ID, *slot.PickupExceptionID)
	} else {
		assert.Empty(t, pickups)
		assert.Empty(t, departures)
		assert.Empty(t, notes)
		assert.Equal(t, studentpresence.SessionAttendanceExpected, slot.Status)
		assert.Nil(t, slot.Substatus)
		assert.Nil(t, slot.PickupExceptionID)
	}
}

func TestPickupCorrectionUsesCurrentGeneration(t *testing.T) {
	t.Parallel()
	for _, initialApproval := range []bool{false, true} {
		name := "initially rejected"
		if initialApproval {
			name = "initially approved"
		}
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f, date, slotID := pickupCorrectionFixture(t)
			ctx := correctionContext(t, f)
			row := correctionRequest(t, f, date, initialApproval)
			if initialApproval {
				require.NoError(t, correctPickup(ctx, t, f, row, false))
				row = currentCorrectionRequest(t, f, row.ID)
			}
			for range 2 {
				require.NoError(t, correctPickup(ctx, t, f, row, true))
				row = currentCorrectionRequest(t, f, row.ID)
				assertCorrectionEffects(t, f, date, slotID, true)
				require.NoError(t, correctPickup(ctx, t, f, row, false), "the corrected approval's generation must be reversible")
				row = currentCorrectionRequest(t, f, row.ID)
				assertCorrectionEffects(t, f, date, slotID, false)
			}
		})
	}
}

func TestPickupCorrectionReleasesAutoExcusal(t *testing.T) {
	t.Parallel()
	f, date, slotID := pickupCorrectionFixture(t)
	row := correctionRequest(t, f, date, true)
	assertCorrectionEffects(t, f, date, slotID, true)
	require.NoError(t, correctPickup(correctionContext(t, f), t, f, row, false))
	assertCorrectionEffects(t, f, date, slotID, false)
}

// Two corrections overlap on real connections. The winner consumes the latest
// approved generation; after waiting, the loser must reject its stale version.
func TestPickupCorrectionConcurrentGeneration(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f, date, slotID := pickupCorrectionFixture(t)
	row := correctionRequest(t, f, date, false)
	require.NoError(t, correctPickup(correctionContext(t, f), t, f, row, true))
	row = currentCorrectionRequest(t, f, row.ID)
	ctx, cancel := context.WithTimeout(correctionContext(t, f), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	err := testpkg.WithTenantTx(t, ctx, f.db, f.chain.TenantID, func(txCtx context.Context, tx bun.Tx) error {
		require.NoError(t, f.reposCare.CarePlan().LockStudentAndExceptionDay(txCtx, f.chain.StudentID, date.String()))
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid))
		go func() { done <- correctPickup(ctx, t, f, row, false) }()
		waitForRequestWriter(t, txCtx, tx, pid)
		return f.svc.(pickupCorrecter).Correct(txCtx, row.ID, false, careplan.ParentRequestVersion(row.UpdatedAt), "Korrektur", f.staffAccount)
	})
	loser := awaitRequestWriter(t, ctx, done)
	require.NoError(t, err)
	require.ErrorContains(t, loser, "stale")
	assertCorrectionEffects(t, f, date, slotID, false)
}

// The day-status writer commits while the correction is waiting for the child.
// Releasing the pickup must restore the newly valid illness, never expected or
// a provenance-free excused absence from the revoked pickup.
func TestPickupCorrectionConcurrentStatusDay(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f, date, slotID := pickupCorrectionFixture(t)
	row := correctionRequest(t, f, date, true)
	ctx, cancel := context.WithTimeout(correctionContext(t, f), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	var statusID int64
	err := testpkg.WithTenantTx(t, ctx, f.db, f.chain.TenantID, func(txCtx context.Context, tx bun.Tx) error {
		owner := f.reposCare.CarePlan()
		require.NoError(t, owner.LockStudentAndExceptionDay(txCtx, f.chain.StudentID, date.String()))
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid))
		go func() { done <- correctPickup(ctx, t, f, row, false) }()
		waitForRequestWriter(t, txCtx, tx, pid)
		status, err := owner.UpsertStudentStatusDay(txCtx, careplan.StudentStatusDay{StudentID: f.chain.StudentID, Status: "sick", Date: careplan.Date(date), Source: "manual", ReportedAt: time.Now()})
		statusID = status.ID
		return err
	})
	correctionErr := awaitRequestWriter(t, ctx, done)
	require.NoError(t, err)
	require.NoError(t, correctionErr)
	slot := testpkg.InstanceStudentByIDContext(t, ctx, f.db, slotID)
	assert.Equal(t, studentpresence.SessionAttendanceAbsent, slot.Status)
	require.NotNil(t, slot.Substatus)
	assert.Equal(t, "sick", *slot.Substatus)
	require.NotNil(t, slot.StudentStatusDayID)
	assert.Equal(t, statusID, *slot.StudentStatusDayID)
	assert.Nil(t, slot.PickupExceptionID)
}

func TestPickupCorrectionPreservesProtectedAttendance(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"sick", "excused", "manual", "completed"} {
		t.Run(kind, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f, date, slotID := pickupCorrectionFixture(t)
			row := correctionRequest(t, f, date, true)
			ctx := correctionContext(t, f)
			presence := api.NewStudentPresenceForTests(testpkg.NewDatabase(t, f.db))
			switch kind {
			case "sick", "excused":
				_, err := f.reposCare.CarePlan().UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
					StudentID: f.chain.StudentID, Date: careplan.Date(date), Status: kind, Source: "manual", ReportedAt: time.Now(),
				})
				require.NoError(t, err)
			case "manual":
				status := studentpresence.SessionAttendancePresent
				require.NoError(t, presence.PatchSessionAttendance(ctx, slotID, studentpresence.SessionAttendancePatch{Status: &status}))
			case "completed":
				slot := testpkg.InstanceStudentByIDContext(t, ctx, f.db, slotID)
				block := testpkg.ActivityInstanceByID(t, ctx, f.db, slot.InstanceID)
				block.Status = "completed"
				testpkg.UpsertActivitySession(t, ctx, f.db, block)
			}
			before := testpkg.InstanceStudentByIDContext(t, ctx, f.db, slotID)
			require.NoError(t, correctPickup(ctx, t, f, row, false))
			after := testpkg.InstanceStudentByIDContext(t, ctx, f.db, slotID)
			if kind == "manual" || kind == "completed" {
				assert.Equal(t, before.Status, after.Status)
				assert.Equal(t, before.Substatus, after.Substatus)
				assert.Equal(t, before.ManualStatusAt, after.ManualStatusAt)
			} else {
				assert.Equal(t, studentpresence.SessionAttendanceAbsent, after.Status)
				require.NotNil(t, after.Substatus)
				assert.Equal(t, kind, *after.Substatus)
				assert.NotNil(t, after.StudentStatusDayID)
			}
			assert.Nil(t, after.PickupExceptionID)
		})
	}
}

func TestPickupCorrectionRollsBackEveryEffect(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"release", "ledger", "notification"} {
		t.Run(stage, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			f, date, slotID := pickupCorrectionFixture(t)
			ctx := correctionContext(t, f)
			row := correctionRequest(t, f, date, true)
			eventsBefore, err := testpkg.TenantQuery2(t, ctx, f.db, f.reposPeople.ParentRequestEvent.ParentRequestEventsForRequest, "pickup_change", row.ID)
			require.NoError(t, err)
			require.NotEmpty(t, eventsBefore)
			var messagesBefore int
			require.NoError(t, db.NewRaw("SELECT count(*) FROM users.parent_messages WHERE tenant_id = ?", f.chain.TenantID).Scan(ctx, &messagesBefore))
			table, condition := "active.activity_session_attendance", "NEW.status = 'expected'"
			if stage == "ledger" {
				table, condition = "users.parent_request_events", "NEW.event_type = 'corrected'"
			}
			if stage == "notification" {
				table, condition = "users.parent_messages", "true"
			}
			_, err = db.ExecContext(ctx, fmt.Sprintf(`
    CREATE FUNCTION public.reject_pickup_correction() RETURNS trigger AS $$
    BEGIN
     IF %s THEN RAISE EXCEPTION 'injected correction failure'; END IF;
     RETURN NEW;
    END $$ LANGUAGE plpgsql;
    CREATE TRIGGER reject_pickup_correction BEFORE INSERT OR UPDATE ON %s
     FOR EACH ROW EXECUTE FUNCTION public.reject_pickup_correction();`, condition, table))
			require.NoError(t, err)
			require.ErrorContains(t, correctPickup(ctx, t, f, row, false), "injected correction failure")
			assert.Equal(t, row, currentCorrectionRequest(t, f, row.ID))
			assertCorrectionEffects(t, f, date, slotID, true)
			eventsAfter, err := testpkg.TenantQuery2(t, ctx, f.db, f.reposPeople.ParentRequestEvent.ParentRequestEventsForRequest, "pickup_change", row.ID)
			require.NoError(t, err)
			assert.Equal(t, eventsBefore, eventsAfter)
			var messagesAfter int
			require.NoError(t, db.NewRaw("SELECT count(*) FROM users.parent_messages WHERE tenant_id = ?", f.chain.TenantID).Scan(ctx, &messagesAfter))
			assert.Equal(t, messagesBefore, messagesAfter)
		})
	}
}

func TestPickupCorrectionDoesNotFallBackToAnOlderGeneration(t *testing.T) {
	t.Parallel()
	f, date, slotID := pickupCorrectionFixture(t)
	ctx := correctionContext(t, f)
	row := correctionRequest(t, f, date, true)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, f.db, f.reposPeople.ParentRequestEvent.AppendParentRequestEvent, &peopledirectory.ParentRequestEventSnapshot{
		StudentID: f.chain.StudentID, RequestType: "pickup_change", RequestID: row.ID, EventType: "corrected", Payload: json.RawMessage(`{"approve":true}`),
	}))
	require.ErrorContains(t, correctPickup(ctx, t, f, row, false), "cannot be corrected")
	assertCorrectionEffects(t, f, date, slotID, true)
}

func TestPickupCorrectionRequiresExcusalForAutomaticAbsence(t *testing.T) {
	t.Parallel()
	f, date, slotID := pickupCorrectionFixture(t)
	ctx := correctionContext(t, f)
	row := correctionRequest(t, f, date, true)
	b := &correctionBoundary{allowed: true, decisionBoundary: decisionBoundary{student: compose.ReviewStudent{ID: f.chain.StudentID}}}
	service, err := compose.NewRequestCorrections(compose.RequestCorrectionDependencies{
		Records: f.reposCare.CarePlan(), People: b, Plans: b, Effects: b,
	})
	require.NoError(t, err)
	err = testpkg.WithTenantTx(t, ctx, f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		b.history, err = owners.NewParentRequestLedger(f.reposPeople.ParentRequestEvent).CorrectionHistory(txCtx, &row)
		if err != nil {
			return err
		}
		return service.Correct(txCtx, row.ID, false, careplan.ParentRequestVersion(row.UpdatedAt), "Korrektur", f.staffAccount)
	})
	require.ErrorContains(t, err, "pickup auto excusal not configured")
	assertCorrectionEffects(t, f, date, slotID, true)
	assert.Equal(t, row, currentCorrectionRequest(t, f, row.ID))
}
