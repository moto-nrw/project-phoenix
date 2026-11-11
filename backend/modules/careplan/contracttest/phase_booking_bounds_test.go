package contracttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Phase edits carry the bounds the phase set on its offerings' bookings
// (#6597, ADR 0205). The env's phase runs from 2026-09-01 to 2027-07-31 and
// today is 2026-08-24, so the copied bookings are [2026-09-01, 2027-08-01).

// boundsPhases is the phase administration as the root binds it for a
// window edit: Care Plan's booking-bounds command and the real sourced
// roster resync. lock is the recurrence gate.
func boundsPhases(t *testing.T, env *decisionTestEnv, lock func(context.Context) error) *enrollmentTest.Phases {
	t.Helper()
	resyncer, ok := env.bookings.(enrollmentTest.SourcedTemplateResyncer)
	require.True(t, ok, "the decision env must compose the sourced-template resync")
	if lock == nil {
		lock = func(context.Context) error { return nil }
	}
	pendingWindows, err := owners.ComposePendingWindows(testpkg.NewDatabase(t, env.db))
	require.NoError(t, err)
	return enrollmentTest.NewPhases(enrollmentTest.PhaseDependencies{
		Records:                api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, env.db)).Enrollment(),
		Offerings:              phaseOfferings{carePlan: api.NewRepositoryTestFactory(testpkg.NewDatabase(t, env.db)).CarePlan()},
		LockTemplateRecurrence: lock,
		Resyncer:               resyncer,
		Bookings:               owners.NewEnrollmentPhaseBookings(carePlanCompose.NewOfferingBookings()),
		Students:               owners.NewEnrollmentPhaseStudents(pendingWindows),
		CareStudents:           owners.NewEnrollmentPhaseCareStudents(owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, env.db))),
		Today:                  func() calendar.Date { return offeringResyncToday },
	})
}

// movedSourcePhase is the env's phase with its start and end moved.
func movedSourcePhase(env *decisionTestEnv, startDays, endDays int) *enrollmentTest.Phase {
	phase := *env.sourcePhase
	phase.ServiceStartDate = enrollmentTest.Date(calendar.Date(env.sourcePhase.ServiceStartDate).AddDays(startDays))
	phase.ServiceEndDate = enrollmentTest.Date(calendar.Date(env.sourcePhase.ServiceEndDate).AddDays(endDays))
	return &phase
}

type bookingWindow struct{ from, until calendar.Date }

// childBookings reads the child's bookings ordered by id.
func childBookings(t *testing.T, env *decisionTestEnv, childID int64) []bookingWindow {
	t.Helper()
	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, carePlanCompose.NewOfferingBookings().CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	windows := make([]bookingWindow, 0, len(rows))
	for _, row := range rows {
		window := bookingWindow{}
		if row.ValidFrom != nil {
			window.from = calendar.Date(*row.ValidFrom)
		}
		if row.ValidUntil != nil {
			window.until = calendar.Date(*row.ValidUntil)
		}
		windows = append(windows, window)
	}
	return windows
}

// sourcedChild is an approved child with the offering and a template whose
// roster the offering feeds.
type sourcedChild struct {
	env       *decisionTestEnv
	offering  *offeringContract.CareOfferingRecord
	studentID int64
	childID   int64
	template  *timetable.Group
	start     calendar.Date
	end       calendar.Date
}

func approveSourcedChild(t *testing.T, env *decisionTestEnv, name string) *sourcedChild {
	t.Helper()
	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName(t.Name()+name), start.AddDays(-31), end.AddDays(31))
	offering := createSourceOffering(t, env, name, nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, name+"@example.com", name, 2)
	template := createSourcedTemplate(t, env, name+"Termin", offering.ID, nil, period.ID)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(testpkg.Ctx(t), timetable.OfferingRosterResyncInput{
		TemplateID: template.ID, OfferingIDs: []int64{offering.ID}, CalendarPeriodID: &period.ID, EffectiveFrom: offeringResyncToday,
	}))
	c := &sourcedChild{env: env, offering: offering, studentID: studentID, childID: childID, template: template, start: start, end: end}
	require.Equal(t, []bookingWindow{{start, end.AddDays(1)}}, childBookings(t, env, childID), "the approval copies the phase window")
	c.assertRoster(t, start, end.AddDays(1), "after the approval")
	return c
}

// assertActivation checks the day the still pending child is activated.
func (c *sourcedChild) assertActivation(t *testing.T, on calendar.Date, when string) {
	t.Helper()
	child, err := readOwnerChildForTest(testpkg.Ctx(t), c.env.reposEnrollment.Enrollment(), c.childID)
	require.NoError(t, err)
	require.NotNil(t, child.ActivateOn, "%s: activation plan", when)
	assert.Equal(t, on, *child.ActivateOn, "%s: the activation day", when)
}

// assertRoster checks the child's row in the offering-sourced list.
func (c *sourcedChild) assertRoster(t *testing.T, from, until calendar.Date, when string) {
	t.Helper()
	rows := loadTemplateEnrollments(t, c.env, c.template.ID)
	require.Len(t, rows, 1, "%s: one list row", when)
	assert.Equal(t, c.studentID, rows[0].StudentID, "%s: the child", when)
	assert.Equal(t, string(from), rows[0].ValidFrom, "%s: the list start", when)
	require.NotNil(t, rows[0].ValidUntil, "%s: the list end", when)
	assert.Equal(t, string(until), *rows[0].ValidUntil, "%s: the list end", when)
}

// A confirmed child's bookings and the list derived from them follow the
// phase: an earlier start puts the child on the list from the new start, a
// later end keeps it there until the new end. A rolled-back edit leaves both
// alone.
func TestPhaseUpdate_CarriesPhaseBoundsOfBookingsAndLists(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	c := approveSourcedChild(t, env, "Mitgefuehrt")
	phases := boundsPhases(t, env, nil)
	moved := movedSourcePhase(env, -7, 14)

	failure := errors.New("after phase update")
	err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if err := phases.UpdatePhase(txCtx, moved); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	assert.Equal(t, []bookingWindow{{c.start, c.end.AddDays(1)}}, childBookings(t, env, c.childID), "a rolled-back edit keeps the bookings")
	c.assertRoster(t, c.start, c.end.AddDays(1), "after a rolled-back edit")
	c.assertActivation(t, c.start, "after a rolled-back edit")

	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, phases.UpdatePhase, moved))
	assert.Equal(t, []bookingWindow{{c.start.AddDays(-7), c.end.AddDays(15)}}, childBookings(t, env, c.childID))
	c.assertRoster(t, c.start.AddDays(-7), c.end.AddDays(15), "after the edit")
	c.assertActivation(t, c.start.AddDays(-7), "after the edit: activation and list start on the same day")

	// Back again: the bounds still are the phase's and follow once more.
	back := movedSourcePhase(env, 7, -14)
	back.ServiceStartDate, back.ServiceEndDate = env.sourcePhase.ServiceStartDate, env.sourcePhase.ServiceEndDate
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, phases.UpdatePhase, back))
	assert.Equal(t, []bookingWindow{{c.start, c.end.AddDays(1)}}, childBookings(t, env, c.childID))
	c.assertRoster(t, c.start, c.end.AddDays(1), "after the edit back")
}

// A dated switch keeps its switch day and a care end keeps its end; only
// the outer bounds that are the phase's follow. Several applications move
// independently.
func TestPhaseUpdate_KeepsSwitchesAndCareEndsOfBookings(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	switched := approveSourcedChild(t, env, "Gewechselt")
	ended := approveSourcedChild(t, env, "Beendet")
	start, end := switched.start, switched.end
	switchDay, careEnd := start.AddDays(30), start.AddDays(60)
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(
		ctx, switched.childID, enrollmentTest.Date(switchDay), []*enrollmentTest.RequestChildOffering{{CareOfferingID: switched.offering.ID}}))
	_, err := testpkg.TenantQuery2(t, ctx, env.db, carePlanCompose.NewOfferingBookings().EndCareOfferingBookings, []int64{ended.childID}, careplan.Date(careEnd))
	require.NoError(t, err)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, boundsPhases(t, env, nil).UpdatePhase, movedSourcePhase(env, -7, 7)))

	assert.Equal(t, []bookingWindow{{start.AddDays(-7), switchDay}, {switchDay, end.AddDays(8)}}, childBookings(t, env, switched.childID),
		"the switch day stays, the outer bounds follow")
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), careEnd}}, childBookings(t, env, ended.childID),
		"the care end stays, the start follows")
	ended.assertRoster(t, start.AddDays(-7), careEnd, "after the edit")
}

// An edit that would leave a booking empty is refused as a conflict and
// rolls back as a whole: the phase, the bookings and the lists stay.
func TestPhaseUpdate_RefusesEmptyBookingAtomically(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	c := approveSourcedChild(t, env, "Leer")
	other := approveSourcedChild(t, env, "Nachbar")
	switchDay := c.start.AddDays(30)
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(
		ctx, c.childID, enrollmentTest.Date(switchDay), []*enrollmentTest.RequestChildOffering{{CareOfferingID: c.offering.ID}}))
	before := childBookings(t, env, c.childID)

	err := testpkg.TenantCommand1(t, ctx, env.db, boundsPhases(t, env, nil).UpdatePhase, movedSourcePhase(env, 40, 0))
	require.ErrorIs(t, err, enrollmentTest.ErrPhaseBookingBoundsConflict)
	var conflict *enrollmentTest.PhaseBookingBoundsConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, c.childID, conflict.RequestChildID, "the refusal names the application")
	assert.Equal(t, "Leer Quelle", conflict.ChildName, "the refusal names the child")

	phase, err := testpkg.TenantQuery1(t, ctx, env.db, env.reposEnrollment.Enrollment().Phase, env.sourcePhase.ID)
	require.NoError(t, err)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, phase.ServiceStartDate, "the phase keeps its start")
	assert.Equal(t, before, childBookings(t, env, c.childID), "the refused child's bookings stay")
	assert.Equal(t, []bookingWindow{{other.start, other.end.AddDays(1)}}, childBookings(t, env, other.childID), "no other booking moved")
	other.assertRoster(t, other.start, other.end.AddDays(1), "after the refused edit")
}

// The command reads and writes only the bookings of its own tenant.
func TestFollowPhaseBookingBounds_StaysInItsTenant(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	c := approveSourcedChild(t, env, "Mandant")
	otherTenant, _ := testpkg.CreateTestTenant(t, env.db)
	move := careplan.PhaseBoundsMove{
		PhaseID: env.sourcePhase.ID, StartBefore: c.start, StartAfter: c.start.AddDays(-7),
		EndBefore: c.end, EndAfter: c.end.AddDays(7), Today: offeringResyncToday,
	}

	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), env.db, otherTenant, func(txCtx context.Context) error {
		return carePlanCompose.NewOfferingBookings().FollowPhaseBookingBounds(txCtx, move)
	}))
	assert.Equal(t, []bookingWindow{{c.start, c.end.AddDays(1)}}, childBookings(t, env, c.childID))
}

// A booking change that commits while the phase edit waits is the state
// the edit works on: the edit locks the application's bookings before it
// reads them, so it moves the replaced bookings instead of failing on, or
// overwriting, the rows it read before the change.
func TestPhaseUpdate_WaitsForConcurrentBookingChange(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	c := approveSourcedChild(t, env, "Gleichzeitig")
	switchDay := c.start.AddDays(30)
	phases := boundsPhases(t, env, nil)

	done := make(chan error, 1)
	holdErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		days := []string{"mon"}
		if err := carePlanCompose.NewOfferingBookings().ReplaceCareOfferingBookings(holdCtx, c.childID, []careplan.CareOfferingBooking{
			{CareOfferingID: c.offering.ID, ManualSelectedDays: days, ValidFrom: new(careplan.Date(c.start)), ValidUntil: new(careplan.Date(switchDay))},
			{CareOfferingID: c.offering.ID, ManualSelectedDays: days, ValidFrom: new(careplan.Date(switchDay)), ValidUntil: new(careplan.Date(c.end.AddDays(1)))},
		}); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(holdCtx, &pid))
		go func() {
			done <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(editCtx context.Context) error {
				return phases.UpdatePhase(editCtx, movedSourcePhase(env, -7, 7))
			})
		}()
		for {
			var waiting bool
			require.NoError(t, tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, pid).Scan(holdCtx, &waiting))
			if waiting {
				return nil
			}
			select {
			case editErr := <-done:
				return errors.Join(errors.New("the phase edit did not wait for the booking change"), editErr)
			case <-holdCtx.Done():
				return holdCtx.Err()
			case <-time.After(10 * time.Millisecond):
			}
		}
	})
	require.NoError(t, holdErr)
	select {
	case editErr := <-done:
		require.NoError(t, editErr, "the edit works on the committed booking change")
	case <-ctx.Done():
		t.Fatal("the phase edit did not finish")
	}
	assert.Equal(t, []bookingWindow{{c.start.AddDays(-7), switchDay}, {switchDay, c.end.AddDays(8)}}, childBookings(t, env, c.childID))
}

// blockedBy reports whether a statement waits for a lock the backend pid
// holds.
func blockedBy(t *testing.T, ctx context.Context, db *bun.DB, pid int) bool {
	t.Helper()
	var waiting bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, pid).Scan(ctx, &waiting))
	return waiting
}

// waitersOf counts the backends that wait for a lock pid holds.
func waitersOf(t *testing.T, ctx context.Context, db *bun.DB, pid int) int {
	t.Helper()
	var waiters int
	require.NoError(t, db.NewRaw(`SELECT count(DISTINCT l.pid) FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid))`, pid).Scan(ctx, &waiters))
	return waiters
}

// waitUntilBlockedBy waits until a statement waits for pid, or until done
// delivers; it returns the delivered error then.
func waitUntilBlockedBy(t *testing.T, ctx context.Context, db *bun.DB, pid int, done <-chan error) (bool, error) {
	t.Helper()
	for {
		if blockedBy(t, ctx, db, pid) {
			return true, nil
		}
		select {
		case err := <-done:
			return false, err
		case <-ctx.Done():
			t.Fatal("nothing waited for the holder")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// An application whose bookings commit while the edit waits for another
// application's lock is locked before the edit decides on it. Otherwise a
// writer holding only that application's lock replaced its bookings under
// the edit, which then failed on rows that were gone.
func TestPhaseUpdate_LocksApplicationsThatAppearWhileItWaits(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	first := approveSourcedChild(t, env, "Zuerst")
	late := approveSourcedChild(t, env, "Spaeter")
	bookings := carePlanCompose.NewOfferingBookings()
	require.NoError(t, testpkg.TenantCommand2(t, ctx, env.db, bookings.ReplaceCareOfferingBookings, late.childID, []careplan.CareOfferingBooking{}))
	start, end, switchDay := first.start, first.end, first.start.AddDays(30)
	days := []string{"mon"}
	window := func(offeringID int64, from, until calendar.Date) careplan.CareOfferingBooking {
		return careplan.CareOfferingBooking{CareOfferingID: offeringID, ManualSelectedDays: days, ValidFrom: new(careplan.Date(from)), ValidUntil: new(careplan.Date(until))}
	}
	phases := boundsPhases(t, env, nil)

	edit, replaced, release := make(chan error, 1), make(chan int, 1), make(chan struct{})
	second := make(chan error, 1)
	holdErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		// The first holder changes the first application and records the
		// late one's bookings, which the edit cannot see yet.
		if err := bookings.ReplaceCareOfferingBookings(holdCtx, first.childID, []careplan.CareOfferingBooking{
			window(first.offering.ID, start, switchDay), window(first.offering.ID, switchDay, end.AddDays(1)),
		}); err != nil {
			return err
		}
		if err := bookings.RecordCareOfferingBookings(holdCtx, late.childID, []careplan.CareOfferingBooking{window(late.offering.ID, start, end.AddDays(1))}); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(holdCtx, &pid))
		go func() {
			edit <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(editCtx context.Context) error {
				return phases.UpdatePhase(editCtx, movedSourcePhase(env, -7, 7))
			})
		}()
		if waiting, err := waitUntilBlockedBy(t, ctx, env.db, pid, edit); !waiting {
			return errors.Join(errors.New("the edit did not wait for the first holder"), err)
		}
		// The second holder queues for the late application's lock and
		// replaces its bookings as soon as the first holder commits.
		go func() {
			second <- testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(secondCtx context.Context, secondTx bun.Tx) error {
				if err := bookings.LockCareOfferingBookings(secondCtx, []int64{late.childID}); err != nil {
					return err
				}
				if err := bookings.ReplaceCareOfferingBookings(secondCtx, late.childID, []careplan.CareOfferingBooking{
					window(late.offering.ID, start, switchDay), window(late.offering.ID, switchDay, end.AddDays(1)),
				}); err != nil {
					return err
				}
				var secondPID int
				if err := secondTx.NewRaw("SELECT pg_backend_pid()").Scan(secondCtx, &secondPID); err != nil {
					return err
				}
				replaced <- secondPID
				<-release
				return nil
			})
		}()
		for waitersOf(t, holdCtx, env.db, pid) < 2 {
			select {
			case err := <-second:
				return errors.Join(errors.New("the second holder did not wait for the first"), err)
			case <-holdCtx.Done():
				return holdCtx.Err()
			case <-time.After(10 * time.Millisecond):
			}
		}
		return nil
	})
	require.NoError(t, holdErr)

	var secondPID int
	select {
	case secondPID = <-replaced:
	case err := <-second:
		t.Fatalf("the second holder failed: %v", err)
	case <-ctx.Done():
		t.Fatal("the second holder did not replace the bookings")
	}
	editFinished, editErr := false, error(nil)
	if waiting, err := waitUntilBlockedBy(t, ctx, env.db, secondPID, edit); !waiting {
		editFinished, editErr = true, err
	}
	close(release)
	require.NoError(t, <-second)
	if !editFinished {
		editErr = <-edit
	}
	require.NoError(t, editErr, "the edit works on the committed bookings of the late application")
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), switchDay}, {switchDay, end.AddDays(8)}}, childBookings(t, env, first.childID))
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), switchDay}, {switchDay, end.AddDays(8)}}, childBookings(t, env, late.childID))
}

// An approval holds the gates while it derives the child's list rows from
// its bookings. A phase edit in the meantime waits for it at the recurrence
// gate and then moves the approved child's bookings, list and activation
// together, so the approval cannot leave rows from the old bounds behind.
func TestPhaseUpdate_WaitsForConcurrentApproval(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites
	decisions := newDecisionServiceForTestWithCareWithdrawal(env, nil, lock, nil)
	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), ctx, testpkg.Tenant(t), uniqueSchemaName(t.Name()), start.AddDays(-31), end.AddDays(31))
	offering := createSourceOffering(t, env, "Bestaetigt", nil)
	template := createSourcedTemplate(t, env, "BestaetigtTermin", offering.ID, nil, period.ID)
	grade := int16(2)
	submitted, err := testpkg.TenantQuery1(t, ctx, env.db, env.requestSvc.Submit, enrollmentTest.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Quelle", GuardianEmail: "bestaetigt@example.com",
		ConsentFlags: consentFlags(t),
		Children: []enrollmentTest.SubmitChild{{
			FirstName: "Bea", LastName: "Quelle", DateOfBirth: calendar.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade, OfferingIDs: []int64{offering.ID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	childID := submitted.Children[0].ID
	phases := boundsPhases(t, env, lock)

	edit := make(chan error, 1)
	var studentID int64
	holdErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		outcome, err := decisions.Decide(holdCtx, enrollmentTest.DecideInput{
			RequestID: submitted.Request.ID, ChildID: childID, Status: enrollmentTest.DecisionApproved, ReviewedBy: env.creatorID,
		})
		if err != nil {
			return err
		}
		require.NotNil(t, outcome.Child.CreatedStudentID)
		studentID = *outcome.Child.CreatedStudentID
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(holdCtx, &pid))
		go func() {
			edit <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(editCtx context.Context) error {
				return phases.UpdatePhase(editCtx, movedSourcePhase(env, -7, 7))
			})
		}()
		if waiting, err := waitUntilBlockedBy(t, holdCtx, env.db, pid, edit); !waiting {
			return errors.Join(errors.New("the edit did not wait for the approval"), err)
		}
		return nil
	})
	require.NoError(t, holdErr)
	select {
	case err := <-edit:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("the phase edit did not finish")
	}

	c := &sourcedChild{env: env, offering: offering, studentID: studentID, childID: childID, template: template, start: start, end: end}
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), end.AddDays(8)}}, childBookings(t, env, childID))
	c.assertRoster(t, start.AddDays(-7), end.AddDays(8), "after the approval and the edit")
	c.assertActivation(t, start.AddDays(-7), "after the approval and the edit")
}
