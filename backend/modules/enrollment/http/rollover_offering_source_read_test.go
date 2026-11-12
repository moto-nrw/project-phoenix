package enrollmenthttp_test

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-ogs/project-phoenix/modules/enrollment/offeringcontract"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// These tests pin what each rolled child carries when the rollover copies
// the source bookings of many children (#5836): every new child gets exactly
// the booking its own source child had at the end of the source phase, in
// the source order, and an abort in a later request rolls back the earlier
// requests' copies.

// rolloverFamily is one source request with its approved children.
type rolloverFamily struct {
	children []*enrollmentNative.RequestChild
}

// seedApprovedFamily submits one request with the given children and
// approves each of them.
func seedApprovedFamily(t *testing.T, env *rolloverTestEnv, family string, childFirstNames []string) rolloverFamily {
	t.Helper()
	ctx := testpkg.Ctx(t)
	grade := int16(2)
	children := make([]enrollmentAPI.SubmitChild, 0, len(childFirstNames))
	for _, first := range childFirstNames {
		children = append(children, enrollmentAPI.SubmitChild{
			FirstName: first, LastName: family, DateOfBirth: calendar.NewDate(2018, 4, 15), TargetGradeLevel: &grade,
		})
	}
	res, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Gina",
		GuardianLastName:  family,
		GuardianEmail:     strings.ToLower(family) + "@example.com",
		ConsentFlags:      testFormValues(rolloverConsentFlags()),
		Children:          children,
	})
	require.NoError(t, err)
	require.Len(t, res.Children, len(childFirstNames))
	out := rolloverFamily{}
	for _, child := range res.Children {
		require.NoError(t, testpkg.TenantCommand4(t, ctx, env.db, env.repos.Enrollment().UpdateChildStatus, child.ID, enrollmentNative.ChildStatusApproved, nil, env.creatorID))
		updated, err := enrollmentAPI.ReadOwnerChildForTest(ctx, env.repos.Enrollment(), child.ID)
		require.NoError(t, err)
		out.children = append(out.children, updated)
	}
	return out
}

// carriedBooking is the comparable form of one copied booking.
type carriedBooking struct {
	offering  string
	selected  string
	manual    string
	automatic string
	notes     string
}

func (b carriedBooking) String() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", b.offering, b.selected, b.manual, b.automatic, b.notes)
}

func carriedBookingsOf(rows []*enrollmentNative.RequestChildOffering, names map[int64]string) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		notes := ""
		if row.Notes != nil {
			notes = *row.Notes
		}
		out = append(out, carriedBooking{
			offering: names[row.CareOfferingID], selected: strings.Join(row.SelectedDays, ","),
			manual: strings.Join(row.ManualSelectedDays, ","), automatic: strings.Join(row.AutomaticSelectedDays, ","),
			notes: notes,
		}.String())
	}
	sort.Strings(out)
	return out
}

// seedRolloverSourceBookings gives the i-th source child: nothing (i%3==0),
// one parent-choice booking with a note (i%3==1), or a replaced historical
// booking plus two effective bookings (i%3==2). It returns the expected
// carried bookings by source child ID.
func seedRolloverSourceBookings(t *testing.T, env *rolloverTestEnv, children []*enrollmentNative.RequestChild) map[int64][]string {
	t.Helper()
	choice := sourceOffering(t, env, &offeringContract.CareOfferingRecord{
		Name: "Wahlbetreuung", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true,
	})
	lunch := sourceOffering(t, env, &offeringContract.CareOfferingRecord{
		Name: "Mittag", DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IncludesLunch: true, IsActive: true,
	})
	historyEnd := calendar.NewDate(2027, 1, 1)
	expected := make(map[int64][]string, len(children))
	for i, child := range children {
		note := fmt.Sprintf("Notiz %d", i)
		switch i % 3 {
		case 0:
			expected[child.ID] = []string{}
		case 1:
			days := []string{"mon", "wed"}
			if i%2 == 0 {
				days = []string{"fri"}
			}
			linkChildOffering(t, env, &enrollmentNative.RequestChildOffering{
				RequestChildID: child.ID, CareOfferingID: choice.ID, SelectedDays: days, Notes: &note,
			})
			joined := strings.Join(days, ",")
			expected[child.ID] = []string{carriedBooking{offering: choice.Name, selected: joined, manual: joined, notes: note}.String()}
		default:
			linkChildOffering(t, env, &enrollmentNative.RequestChildOffering{
				RequestChildID: child.ID, CareOfferingID: choice.ID, SelectedDays: []string{"thu"}, Notes: &note,
				ValidUntil: (*enrollmentNative.Date)(&historyEnd),
			})
			linkChildOffering(t, env, &enrollmentNative.RequestChildOffering{
				RequestChildID: child.ID, CareOfferingID: choice.ID, SelectedDays: []string{"tue", "thu"},
				ManualSelectedDays: []string{"tue"}, AutomaticSelectedDays: []string{"thu"},
				ValidFrom: (*enrollmentNative.Date)(&historyEnd),
			})
			linkChildOffering(t, env, &enrollmentNative.RequestChildOffering{RequestChildID: child.ID, CareOfferingID: lunch.ID})
			expected[child.ID] = []string{
				carriedBooking{offering: choice.Name, selected: "tue,thu", manual: "tue", automatic: "thu", notes: note}.String(),
				carriedBooking{offering: lunch.Name}.String(),
			}
			sort.Strings(expected[child.ID])
		}
	}
	return expected
}

// seedRolloverFamilies spreads n children over families of at most three.
func seedRolloverFamilies(t *testing.T, env *rolloverTestEnv, n int, prefix string) ([]rolloverFamily, []*enrollmentNative.RequestChild) {
	t.Helper()
	var families []rolloverFamily
	var all []*enrollmentNative.RequestChild
	for start := 0; start < n; start += 3 {
		var names []string
		for i := start; i < n && i < start+3; i++ {
			names = append(names, fmt.Sprintf("%sKind%d", prefix, i))
		}
		family := seedApprovedFamily(t, env, fmt.Sprintf("%sFamilie%d", prefix, start/3), names)
		families = append(families, family)
		all = append(all, family.children...)
	}
	return families, all
}

func assertRolledChildrenCarrySourceBookings(t *testing.T, n int) {
	t.Helper()
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	families, children := seedRolloverFamilies(t, env, n, "Rolle")
	expected := seedRolloverSourceBookings(t, env, children)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx,
		validRolloverRequest(env, enrollmentNative.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	assert.Equal(t, len(families), result.RequestCount)
	assert.Equal(t, n, result.RolledCount)
	assert.Len(t, env.outbox.ByKind(enrollmentNative.MailKindRolloverOptOut), len(families))

	clones, err := newCareOfferingFixtures(env.reposCare.CarePlan).ListByPhase(ctx, result.Phase.ID)
	require.NoError(t, err)
	names := make(map[int64]string, len(clones))
	for _, clone := range clones {
		names[clone.ID] = clone.Name
	}
	rolled, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, result.Phase.ID, []string{enrollmentNative.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolled, n)
	seen := make(map[int64]bool, n)
	for _, child := range rolled {
		require.NotNil(t, child.RolloverSourceChildID)
		sourceID := *child.RolloverSourceChildID
		want, ok := expected[sourceID]
		require.True(t, ok, "rolled child %d names an unknown source child %d", child.ID, sourceID)
		seen[sourceID] = true
		copied, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().RequestChildOfferingHistory, child.ID)
		require.NoError(t, err)
		assert.Equal(t, want, carriedBookingsOf(copied, names), "source child %d", sourceID)
		for _, row := range copied {
			require.NotNil(t, row.ValidFrom)
			assert.Equal(t, enrollmentNative.Date(result.Phase.ServiceStartDate), *row.ValidFrom)
		}
	}
	assert.Len(t, seen, n)
}

func TestRolloverCarriesEachChildsOwnSourceBookingN3(t *testing.T) {
	t.Parallel()
	assertRolledChildrenCarrySourceBookings(t, 3)
}

func TestRolloverCarriesEachChildsOwnSourceBookingN8(t *testing.T) {
	t.Parallel()
	assertRolledChildrenCarrySourceBookings(t, 8)
}

// A failing child insert in the third request aborts the whole rollover:
// the copies, requests and phase of the first two requests roll back, and
// the error is the insert's, not a business refusal.
func TestRolloverAbortInLaterRequestRollsBackEarlierCopies(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	marker := fmt.Sprintf("Abbruch%d", testpkg.Tenant(t))
	_, children := seedRolloverFamilies(t, env, 8, marker)
	seedRolloverSourceBookings(t, env, children)

	function := "p8_rollover_abort_" + fmt.Sprint(testpkg.Tenant(t))
	_, err := env.db.ExecContext(ctx, fmt.Sprintf(`CREATE FUNCTION enrollment.%s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.rollover_source_child_id IS NOT NULL AND NEW.first_name = '%sKind6' THEN
		RAISE EXCEPTION 'injected rollover child insert failure';
	END IF;
	RETURN NEW;
END $$`, function, marker))
	require.NoError(t, err)
	_, err = env.db.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON enrollment.request_children
FOR EACH ROW EXECUTE FUNCTION enrollment.%s()`, function, function))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := env.db.ExecContext(context.Background(), fmt.Sprintf(`DROP TRIGGER %s ON enrollment.request_children`, function))
		require.NoError(t, dropErr)
		_, dropErr = env.db.ExecContext(context.Background(), fmt.Sprintf(`DROP FUNCTION enrollment.%s()`, function))
		require.NoError(t, dropErr)
	})

	_, err = env.rolloverSvc.CreatePhaseFromSource(ctx,
		validRolloverRequest(env, enrollmentNative.PhaseRolloverModeOptOut, true))
	require.Error(t, err)
	assert.ErrorContains(t, err, "rollover: create request_child")
	assert.ErrorContains(t, err, "injected rollover child insert failure")

	exists, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().HasRolloverSuccessor, env.sourcePhase.ID)
	require.NoError(t, err)
	assert.False(t, exists, "the target phase rolls back")
	var carried int
	require.NoError(t, env.db.NewSelect().TableExpr("enrollment.request_children").
		ColumnExpr("count(*)").
		Where("tenant_id = ? AND rollover_source_child_id IS NOT NULL", testpkg.Tenant(t)).
		Scan(ctx, &carried))
	assert.Zero(t, carried, "no rolled child survives")
	var bookings int
	require.NoError(t, env.db.NewSelect().TableExpr("enrollment.care_offering_bookings AS b").
		ColumnExpr("count(*)").
		Where("b.tenant_id = ?", testpkg.Tenant(t)).
		Where("b.request_child_id NOT IN (?)", bun.List(testpkgChildIDs(children))).
		Scan(ctx, &bookings))
	assert.Zero(t, bookings, "no copied booking survives")
	// The renewal mails of the first two requests were handed to the
	// in-memory outbox before the abort; the durable outbox rolls back with
	// the transaction.
	assert.Len(t, env.outbox.ByKind(enrollmentNative.MailKindRolloverOptOut), 2)
}

func testpkgChildIDs(children []*enrollmentNative.RequestChild) []int64 {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
	}
	return ids
}

// Every read that decides a copy runs after the catalog clone's recurrence
// gate. A booking change committed while the rollover waits at the gate is
// what the rolled child carries.
func TestRolloverCopiesSourceBookingCommittedWhileWaitingAtGate(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	_, children := seedRolloverFamilies(t, env, 3, "Gate")
	seedRolloverSourceBookings(t, env, children)
	changed := children[1]

	lock := func(lockCtx context.Context) error {
		return owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites(lockCtx)
	}
	svc := newTestRolloverService(owners.EnrollmentRolloverSources{
		Bookings:   requestTestBookingCommands(),
		Phases:     env.repos.Enrollment(),
		Requests:   env.repos.Enrollment(),
		Children:   env.repos.Enrollment(),
		Catalog:    testCareOfferingCatalog(t, env.db, api.WithCareOfferingSettings(env.settings), api.WithCareOfferingRecurrenceLock(lock)),
		Outbox:     env.outbox,
		Settings:   env.settings,
		ParentsURL: "http://parents.localhost:3000",
		Logger:     slog.Default(),
	})

	holderAcquired := make(chan struct{})
	releaseHolder := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			if err := lock(txCtx); err != nil {
				return err
			}
			if _, err := tx.NewRaw(`UPDATE enrollment.care_offering_bookings
				SET manual_selected_days = '["thu"]'::jsonb, automatic_selected_days = NULL
				WHERE tenant_id = ? AND request_child_id = ?`, testpkg.Tenant(t), changed.ID).Exec(txCtx); err != nil {
				return err
			}
			close(holderAcquired)
			<-releaseHolder
			return nil
		})
	}()
	select {
	case <-holderAcquired:
	case err := <-holderDone:
		require.NoError(t, err)
		require.FailNow(t, "holder ended before taking the recurrence gate")
	case <-time.After(10 * time.Second):
		require.FailNow(t, "timed out taking the recurrence gate in the holder")
	}

	type rolloverOutcome struct {
		result *enrollmentNative.RolloverResult
		err    error
	}
	rolloverDone := make(chan rolloverOutcome, 1)
	go func() {
		result, err := svc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentNative.PhaseRolloverModeOptOut, true))
		rolloverDone <- rolloverOutcome{result: result, err: err}
	}()
	require.Eventually(t, func() bool {
		var waiting int
		err := env.db.NewSelect().TableExpr("pg_locks").ColumnExpr("count(*)").
			Where("locktype = 'advisory' AND NOT granted").
			Where("database = (SELECT oid FROM pg_database WHERE datname = current_database())").
			Scan(context.Background(), &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 20*time.Millisecond, "the rollover must wait at the recurrence gate")

	close(releaseHolder)
	require.NoError(t, <-holderDone)
	var outcome rolloverOutcome
	select {
	case outcome = <-rolloverDone:
	case <-time.After(30 * time.Second):
		require.FailNow(t, "rollover did not resume after the gate was released")
	}
	require.NoError(t, outcome.err)

	rolled, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, outcome.result.Phase.ID, []string{enrollmentNative.ChildStatusAutoRenewed})
	require.NoError(t, err)
	var copied []*enrollmentNative.RequestChildOffering
	for _, child := range rolled {
		if child.RolloverSourceChildID != nil && *child.RolloverSourceChildID == changed.ID {
			copied, err = testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().RequestChildOfferingHistory, child.ID)
			require.NoError(t, err)
		}
	}
	require.Len(t, copied, 1)
	assert.Equal(t, []string{"thu"}, copied[0].ManualSelectedDays, "the copy carries the booking committed under the gate")
	assert.Equal(t, []string{"thu"}, copied[0].SelectedDays)
	assert.Empty(t, copied[0].AutomaticSelectedDays)
}
