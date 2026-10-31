package enrollmenthttp_test

import (
	"fmt"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A student may hold approved enrollments of several phases at once, for
// example a custom phase for existing students beside the school year
// (#5366). A booking change of one enrollment leaves the others as they are;
// their bookings carry a hand-maintained Regeltermin by the same rule as the
// changed enrollment's (#5342), but only with a known pickup time.

// otherPhase creates a further custom phase of the school with the service
// window.
func (f *carryFixture) otherPhase(start, end calendar.Date) *enrollmentNative.Phase {
	f.t.Helper()
	phase := &enrollmentNative.Phase{
		Name:             fmt.Sprintf("carry-other-%s-%d", f.t.Name(), f.seq),
		Kind:             enrollmentNative.PhaseKindCustom,
		ServiceStartDate: enrollmentNative.Date(start),
		ServiceEndDate:   enrollmentNative.Date(end),
		IsActive:         true,
		CareOverflowMode: enrollmentNative.PhaseCareOverflowWaitlist,
		FormSchemaID:     f.env.sourcePhase.FormSchemaID,
	}
	phase.TenantID = testpkg.Tenant(f.t)
	require.NoError(f.t, enrollmentAPI.InsertOwnerPhaseForTest(testpkg.Ctx(f.t), f.env.repos.Enrollment(), phase))
	return phase
}

// phaseOffering creates a care offering of the phase picked up at the time
// on every weekday; an empty pickup time leaves it without pickup times.
func (f *carryFixture) phaseOffering(phase *enrollmentNative.Phase, name, pickup string) *offeringContract.CareOfferingRecord {
	f.t.Helper()
	return createAdjustmentCareOfferingWith(f.t, f.env, name, func(o *offeringContract.CareOfferingRecord) {
		o.PhaseID = phase.ID
		o.CountsAsCare, o.CountsAsCareSet = true, true
		if pickup != "" {
			o.PickupTimes = map[string]string{"mon": pickup, "tue": pickup, "wed": pickup, "thu": pickup, "fri": pickup}
		}
	})
}

// enrollAgain submits a request of the phase for the existing student with
// the bookings and approves it.
func (f *carryFixture) enrollAgain(phase *enrollmentNative.Phase, studentID int64, schoolClass string, bookings map[*offeringContract.CareOfferingRecord][]string) {
	f.t.Helper()
	ctx := testpkg.Ctx(f.t)
	f.seq++
	ids := make([]int64, 0, len(bookings))
	days := make([]enrollmentAPI.SubmitOfferingDays, 0, len(bookings))
	for offering, selected := range bookings {
		ids = append(ids, offering.ID)
		days = append(days, enrollmentAPI.SubmitOfferingDays{OfferingID: offering.ID, SelectedDays: selected})
	}
	submitted, err := f.env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(f.t), PhaseID: phase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Muster",
		GuardianEmail: fmt.Sprintf("carry-5366-%d@example.com", f.seq),
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Kind", LastName: "Muster",
			DateOfBirth:       calendar.NewDate(2019, 3, 4),
			TargetGradeLevel:  testpkg.Int16Ptr(1),
			TargetSchoolClass: testpkg.StrPtr(schoolClass),
			OfferingIDs:       ids,
			OfferingDays:      days,
		}},
	})
	require.NoError(f.t, err)
	childID := submitted.Children[0].ID
	matchChildToExistingStudent(f.t, f.env, childID, studentID)
	outcome, err := testpkg.TenantQuery1(f.t, ctx, f.env.db, f.env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		Status: enrollmentNative.DecisionApproved, ReviewedBy: f.env.creatorID,
	})
	require.NoError(f.t, err)
	require.NotNil(f.t, outcome.Child.CreatedStudentID)
	require.Equal(f.t, studentID, *outcome.Child.CreatedStudentID)
}

// The custom phase serves the Stichtag.
var (
	otherPhaseStart = carryStichtag.AddDays(-30)
	otherPhaseEnd   = carryStichtag.AddDays(45)
)

// otherEnrollmentFriday books Ganztag on Monday and Friday in the school
// year and keeps a hand list on Friday until 12:45. The second enrollment
// books an offering of the phase on Friday. The change drops Friday from the
// school year and keeps Monday, so it is no complete withdrawal.
func otherEnrollmentFriday(t *testing.T, phaseStart, phaseEnd calendar.Date, otherPickup string) (map[int64][]int, int64) {
	t.Helper()
	f := newCarryFixture(t)
	ganztag := f.offering("Ganztag bis 16 Uhr", "16:00", true)
	handList := f.template("Hausaufgaben Fr", carryFriday, "11:45", "12:45", "")
	requestID, childID, studentID := f.child("1a", map[*offeringContract.CareOfferingRecord][]string{
		ganztag: {"mon", "fri"},
	})
	f.plan(studentID, handList)
	phase := f.otherPhase(phaseStart, phaseEnd)
	f.enrollAgain(phase, studentID, "1a", map[*offeringContract.CareOfferingRecord][]string{
		f.phaseOffering(phase, "Ferien-Ganztag", otherPickup): {"fri"},
	})

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{
		ganztag: {"mon"},
	}, false)
	return reported, handList
}

// #5366: the other enrollment's Friday until 16:00 still carries the hand
// list until 12:45, so it is not reported.
func TestRosterImpact_OtherEnrollmentCarriesTheHandList_NoNotice(t *testing.T) {
	t.Parallel()

	reported, _ := otherEnrollmentFriday(t, otherPhaseStart, otherPhaseEnd, "16:00")

	assert.Empty(t, reported, "the unchanged Friday of the other enrollment carries the hand list")
}

// Counter-check: the other enrollment's Friday ends before the hand list, so
// nothing carries it and the notice stays.
func TestRosterImpact_OtherEnrollmentPickedUpBeforeTheEnd_Reports(t *testing.T) {
	t.Parallel()

	reported, handList := otherEnrollmentFriday(t, otherPhaseStart, otherPhaseEnd, "12:00")

	assert.Equal(t, map[int64][]int{handList: {carryFriday}}, reported)
}

// Counter-check: an enrollment whose phase does not serve the Stichtag,
// because it ended before or starts later, carries nothing.
func TestRosterImpact_OtherEnrollmentOutsideItsPhase_Reports(t *testing.T) {
	t.Parallel()

	for name, window := range map[string][2]calendar.Date{
		"ended":   {carryStichtag.AddDays(-60), carryStichtag.AddDays(-1)},
		"not yet": {carryStichtag.AddDays(14), carryStichtag.AddDays(45)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)

			reported, handList := otherEnrollmentFriday(t, window[0], window[1], "16:00")

			assert.Equal(t, map[int64][]int{handList: {carryFriday}}, reported)
		})
	}
}

// Without a pickup time the other enrollment cannot show that it carries
// the hand list. The day is decided as with one enrollment: the change drops
// the child's only Friday, so the notice stays, as before.
func TestRosterImpact_OtherEnrollmentWithoutPickupTime_KeepsTheNotice(t *testing.T) {
	t.Parallel()

	reported, handList := otherEnrollmentFriday(t, otherPhaseStart, otherPhaseEnd, "")

	assert.Equal(t, map[int64][]int{handList: {carryFriday}}, reported)
}

// A school without pickup times keeps the earlier rule exactly, with a
// second enrollment as well: Ganztag dropped beside the Randstunde on Friday
// is a net loss, so the hand list stays reported although the other
// enrollment books Friday too.
func TestRosterImpact_OtherEnrollmentWithoutPickupTimes_KeepsTheNetLossRule(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstunde", "", true)
	ganztag := f.offering("Ganztag", "", true)
	handList := f.template("Hausaufgaben Fr", carryFriday, "13:00", "14:00", "")
	requestID, childID, studentID := f.child("1a", map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"fri"}, ganztag: {"fri"},
	})
	f.plan(studentID, handList)
	phase := f.otherPhase(otherPhaseStart, otherPhaseEnd)
	f.enrollAgain(phase, studentID, "1a", map[*offeringContract.CareOfferingRecord][]string{
		f.phaseOffering(phase, "Ferien-Ganztag", ""): {"fri"},
	})

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"fri"},
	}, false)

	assert.Equal(t, map[int64][]int{handList: {carryFriday}}, reported)
}
