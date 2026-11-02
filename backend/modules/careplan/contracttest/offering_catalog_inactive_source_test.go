package contracttest_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Catalog edits of a Regeltermin's source offerings (#5313). The Regeltermin
// is sourced from A (Monday and Wednesday) and B (Monday). An inactive source
// contributes nothing while another stays valid, so an edit of B next to an
// inactive A and the deactivation of A next to an active B are saved: A's
// Wednesday leaves from today, B keeps planning the Monday, and the history,
// hand-decided and recorded attendance and the other child stay, as with the
// resyncs of #5305. An edit that leaves the Regeltermin without a valid
// source is refused with a code naming the offering and the Regeltermin.

// catalogRows is the catalog's update use case with the resync behind it.
func catalogRows(t *testing.T, env *decisionTestEnv) *scopedCareOfferingRows {
	t.Helper()
	resyncer, ok := env.bookings.(enrollmentTest.SourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the offering-update resync")
	return newScopedOfferingRows(testCareOfferingCatalog(t, env.db,
		api.WithCareOfferingToday(func() calendar.Date { return inactiveSourceToday }),
		api.WithCareOfferingRosterResync(resyncer),
		api.WithCareOfferingPickupResync(&pickupResyncer{}),
	))
}

// editOffering loads the offering, applies edit and saves it through the
// catalog. It returns the save's error and whether it asked for rollback.
func editOffering(t *testing.T, env *decisionTestEnv, offeringID int64, edit func(*offeringContract.CareOfferingRecord)) (error, bool) {
	t.Helper()
	rows := catalogRows(t, env)
	offering, err := rows.GetByID(testpkg.Ctx(t), offeringID)
	require.NoError(t, err)
	// The fixtures insert their offerings without the catalog's admin-save
	// checks; an admin save needs the pickup times of an active offering.
	offering.PickupTimes = carePickupTimes(offering.AvailableDays...)
	edit(offering)
	ctx := tenant.WithRollbackMarker(testpkg.Ctx(t))
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), rows.Update, offering)
	return err, tenant.RollbackRequested(ctx)
}

func rename(name string) func(*offeringContract.CareOfferingRecord) {
	return func(offering *offeringContract.CareOfferingRecord) { offering.Name = name }
}

func switchOff(offering *offeringContract.CareOfferingRecord) { offering.IsActive = false }

// runCatalogEditWithInactiveSource turns A inactive with drift, then
// saves the catalog edit; it must succeed and plan only B from today.
func runCatalogEditWithInactiveSource(t *testing.T, email string, periods divergingPeriodCase, drift bool, editedID func(inactiveSourcePlan) int64, edit func(*offeringContract.CareOfferingRecord)) {
	t.Helper()
	p := planInactiveSource(t, setupInactiveSourceCorrection(t, email, periods))
	if drift {
		p.deactivate(t, p.first.ID)
	}
	err, rollback := editOffering(t, p.env, editedID(p), edit)
	require.NoError(t, err, "the catalog edit is saved")
	assert.False(t, rollback, "the saved edit keeps its transaction")

	p.requireSplitAtToday(t, longPhaseEnd.AddDays(1))
	p.requireProtected(t)
	assert.Equal(t, p.childSwitchMonday, p.participants(t, p.switchMonday)[p.studentID],
		"a later Monday keeps the child's participant row untouched")
	later := p.participants(t, p.materializeOn(t, p.switchDate.AddDays(14)))
	assert.Contains(t, later, p.studentID, "a Monday materialized after the edit plans the child")
	assert.Contains(t, later, p.neighbourID)
	assert.NotContains(t, p.participants(t, p.materializeOn(t, p.switchDate.AddDays(16))), p.studentID,
		"a Wednesday materialized after the edit does not plan the child")

	snapshot := map[int64][]timetable.StudentEnrollment{p.studentID: p.rowsOf(t, p.studentID), p.neighbourID: p.rowsOf(t, p.neighbourID)}
	planned := map[int64]map[int64]testpkg.InstanceStudent{}
	for _, instanceID := range []int64{p.windowMonday, p.windowWednesday, p.decidedWednesday, p.switchMonday, p.recordedWednesday} {
		planned[instanceID] = p.participants(t, instanceID)
	}
	for _, step := range []struct {
		name string
		run  func()
	}{
		{"student resync after the edit", func() {
			require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), p.env.db, p.env.bookings.ResyncStudentSourcedRosters,
				[]int64{p.studentID, p.neighbourID}, inactiveSourceToday))
		}},
		{"tenant-wide resync after the edit", func() { p.resyncTenant(t) }},
		{"repeated catalog edit", func() {
			err, _ := editOffering(t, p.env, p.second.ID, rename("Spaetbetreuung wieder"))
			require.NoError(t, err)
		}},
	} {
		step.run()
		p.requireUnchangedSince(t, step.name, snapshot, planned)
	}
}

func editSecond(p inactiveSourcePlan) int64 { return p.second.ID }
func editFirst(p inactiveSourcePlan) int64  { return p.first.ID }

// Renaming B while A is already inactive.
func TestCatalogEdit_ValidSourceNextToInactiveSourceWithoutPeriod(t *testing.T) {
	t.Parallel()
	runCatalogEditWithInactiveSource(t, "katalog-inaktiv-ohne@example.com", divergingPeriodCase{}, true, editSecond, rename("Spaetbetreuung neu"))
}

func TestCatalogEdit_ValidSourceNextToInactiveSourceWithSharedPeriod(t *testing.T) {
	t.Parallel()
	runCatalogEditWithInactiveSource(t, "katalog-inaktiv-zeitraum@example.com",
		divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q", want: "Q"}, true, editSecond, rename("Spaetbetreuung neu"))
}

// Renaming the inactive A itself is an edit like any other.
func TestCatalogEdit_InactiveSourceItselfNextToValidSource(t *testing.T) {
	t.Parallel()
	runCatalogEditWithInactiveSource(t, "katalog-inaktiv-selbst@example.com", divergingPeriodCase{}, true, editFirst, rename("Mittagessen alt"))
}

// Switching A off in the catalog while B stays active.
func TestCatalogEdit_SwitchingOffOneOfSeveralSources(t *testing.T) {
	t.Parallel()
	runCatalogEditWithInactiveSource(t, "katalog-ausschalten@example.com", divergingPeriodCase{}, false, editFirst, switchOff)
}

func TestCatalogEdit_SwitchingOffOneOfSeveralSourcesWithMixedPeriods(t *testing.T) {
	t.Parallel()
	runCatalogEditWithInactiveSource(t, "katalog-ausschalten-gemischt@example.com",
		divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q2", want: ""}, false, editFirst, switchOff)
}

// requireSourceConflict asserts the refusal's code and named values, that
// it asks for rollback, and that it changed no row or participant.
func requireSourceConflict(
	t *testing.T, f multiSourceCorrection, err error, rollback bool, code, offering string,
	snapshot map[int64][]timetable.StudentEnrollment,
) {
	t.Helper()
	require.ErrorIs(t, err, enrollmentTest.ErrCareOfferingSourceConflict)
	require.ErrorIs(t, err, enrollmentTest.ErrCareOfferingInvalid, "the refusal stays a client-correctable catalog refusal")
	var rejection interface {
		ErrorCode() string
		ErrorDetails() any
	}
	require.True(t, errors.As(err, &rejection), "the refusal carries code and details")
	assert.Equal(t, code, rejection.ErrorCode())
	assert.Equal(t, careplan.CareOfferingSourceConflictValues{Offering: offering, Template: f.template.Name}, rejection.ErrorDetails())
	assert.True(t, rollback, "the refused edit discards the written offering row")
	f.requireUnchangedSince(t, "refused catalog edit", snapshot, nil)
}

// Switching off the last active source leaves the Regeltermin without one:
// refused as before, now with its own code.
func TestCatalogEdit_SwitchingOffLastActiveSourceIsRefused(t *testing.T) {
	t.Parallel()
	f := setupMultiSourceCorrection(t, "katalog-letzte-quelle@example.com", divergingPeriodCase{phaseStart: inactiveSourceToday.AddDays(-10)})
	snapshot := map[int64][]timetable.StudentEnrollment{f.studentID: f.rowsOf(t, f.studentID), f.neighbourID: f.rowsOf(t, f.neighbourID)}
	f.deactivate(t, f.first.ID)

	second, err := catalogRows(t, f.env).GetByID(testpkg.Ctx(t), f.second.ID)
	require.NoError(t, err)
	err, rollback := editOffering(t, f.env, f.second.ID, switchOff)
	requireSourceConflict(t, f, err, rollback, careplan.CodeCareOfferingTemplateNoActiveSource, second.Name, snapshot)
}

// Moving B into another phase splits the Regeltermin's sources across two
// phases.
func TestCatalogEdit_MixedPhasesAreRefusedWithCode(t *testing.T) {
	t.Parallel()
	f := setupMultiSourceCorrection(t, "katalog-phasen@example.com", divergingPeriodCase{phaseStart: inactiveSourceToday.AddDays(-10)})
	snapshot := map[int64][]timetable.StudentEnrollment{f.studentID: f.rowsOf(t, f.studentID), f.neighbourID: f.rowsOf(t, f.neighbourID)}
	otherPhase := &enrollmentTest.Phase{
		Name:             uniqueSchemaName("andere-phase-" + t.Name()),
		Kind:             enrollmentTest.PhaseKindSchoolYear,
		ServiceStartDate: f.env.sourcePhase.ServiceStartDate,
		ServiceEndDate:   f.env.sourcePhase.ServiceEndDate,
		IsActive:         true,
		CareOverflowMode: enrollmentTest.PhaseCareOverflowWaitlist,
	}
	otherPhase.TenantID = testpkg.Tenant(t)
	require.NoError(t, insertOwnerPhaseForTest(testpkg.Ctx(t), f.env.reposEnrollment.Enrollment(), otherPhase))

	second, err := catalogRows(t, f.env).GetByID(testpkg.Ctx(t), f.second.ID)
	require.NoError(t, err)
	err, rollback := editOffering(t, f.env, f.second.ID, func(offering *offeringContract.CareOfferingRecord) { offering.PhaseID = otherPhase.ID })
	requireSourceConflict(t, f, err, rollback, careplan.CodeCareOfferingTemplateMixedPhases, second.Name, snapshot)
}

// The catalog resync behind phase edits keeps its refusal as well: Care
// Plan's public refusal names the problem, so the phase edit can still be
// refused (#2147 review round 7).
func TestCatalogEdit_OfferingScopedResyncNamesTheProblem(t *testing.T) {
	t.Parallel()
	f := setupMultiSourceCorrection(t, "katalog-problem@example.com", divergingPeriodCase{phaseStart: inactiveSourceToday.AddDays(-10)})
	f.deactivate(t, f.first.ID, f.second.ID)
	scoped, ok := f.env.bookings.(enrollmentTest.SourcedTemplateResyncer)
	require.True(t, ok)
	err := scoped.ResyncTemplatesSourcedFromOffering(testpkg.Ctx(t), f.second.ID, inactiveSourceToday)
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid)
	var refusal careplan.SourcedTemplateRefusal
	require.True(t, errors.As(err, &refusal))
	assert.Equal(t, careplan.SourcedTemplateNoActiveSource, refusal.Problem)
	assert.Equal(t, f.template.Name, refusal.Template)
}

// The Regeltermin editor counts the children a save would plan: a stored
// inactive source counts nobody instead of failing the count (#5313), and a
// selection without any valid source keeps its refusal.
func TestCombinedOfferingSourceCounts_SkipsInactiveSource(t *testing.T) {
	t.Parallel()
	f := setupMultiSourceCorrection(t, "editor-inaktiv@example.com", divergingPeriodCase{phaseStart: inactiveSourceToday.AddDays(-10)})
	_, _, onlyFirst := submitApprovedAdjustmentChild(t, f.env, "editor-nur-a@example.com", "NurA",
		[]*offeringContract.CareOfferingRecord{f.first})
	f.deactivate(t, f.first.ID)
	ids := []int64{f.first.ID, f.second.ID}

	counts, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), f.env.db, f.env.bookings.CombinedOfferingSourceCounts, ids, f.template.CalendarPeriodID)
	require.NoError(t, err, "a stored inactive source must not fail the editor's count")
	assert.Equal(t, 2, counts.TotalCount, "only the valid source's children count")
	counted := make([]int64, 0, len(counts.Students))
	for _, student := range counts.Students {
		counted = append(counted, student.StudentID)
	}
	assert.ElementsMatch(t, []int64{f.studentID, f.neighbourID}, counted)
	assert.NotContains(t, counted, onlyFirst, "the inactive source's own child is not counted")

	f.deactivate(t, f.second.ID)
	_, err = testpkg.TenantQuery2(t, testpkg.Ctx(t), f.env.db, f.env.bookings.CombinedOfferingSourceCounts, ids, f.template.CalendarPeriodID)
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid, "without a valid source the count is refused as before")
}
