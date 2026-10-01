package application

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// offeringChangeSettingsStub answers the offering-change settings; errOn
// names the one setting that fails with err.
type offeringChangeSettingsStub struct {
	changes, careOfferings, courses, authoritative bool
	leadDays                                       string
	errOn                                          string
	err                                            error
}

func (s offeringChangeSettingsStub) failing(name string) error {
	if s.errOn == name {
		return s.err
	}
	return nil
}

func (s offeringChangeSettingsStub) OfferingChangesEnabled(context.Context) (bool, error) {
	return s.changes, s.failing("changes")
}

func (s offeringChangeSettingsStub) CareOfferingsEnabled(context.Context) (bool, error) {
	return s.careOfferings, s.failing("care_offerings")
}

func (s offeringChangeSettingsStub) CourseRequestsEnabled(context.Context) (bool, error) {
	return s.courses, s.failing("courses")
}

func (s offeringChangeSettingsStub) BookingsAuthoritative(context.Context) (bool, error) {
	return s.authoritative, s.failing("authoritative")
}

func (s offeringChangeSettingsStub) OfferingChangeLeadDays(context.Context) (string, error) {
	return s.leadDays, s.failing("lead_days")
}

// offeringChangeRowsStub captures the decision snapshot a decision stores.
type offeringChangeRowsStub struct {
	ports.OfferingChangeRows
	snapshot json.RawMessage
}

func (r *offeringChangeRowsStub) UpdateDecisionSnapshot(_ context.Context, _ int64, snapshot json.RawMessage) error {
	r.snapshot = snapshot
	return nil
}

type offeringCatalogStub struct {
	offerings []careplan.CareOffering
}

func (c offeringCatalogStub) ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	return c.offerings, nil
}

type coursePlanningStub struct {
	ports.OfferingChangePlanning
	groups map[int64][]ports.CourseGroup
}

func (p coursePlanningStub) CourseGroupsForOfferings(context.Context, []ports.CourseOfferingReference, calendar.Date) (map[int64][]ports.CourseGroup, error) {
	return p.groups, nil
}

func offeringChangesForTest(deps OfferingChangeDependencies) *OfferingChanges {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &OfferingChanges{deps: deps}
}

func requestWithSelections(t *testing.T, selections ...careplan.OfferingChangeSelection) careplan.OfferingChangeRequest {
	t.Helper()
	payload, err := payloadFromSelections(selections)
	require.NoError(t, err)
	return careplan.OfferingChangeRequest{Payload: payload}
}

func TestCourseRequestsEnabledReturnsSettingsFailure(t *testing.T) {
	t.Parallel()

	want := errors.New("settings unavailable")
	svc := offeringChangesForTest(OfferingChangeDependencies{Settings: offeringChangeSettingsStub{
		changes: true, careOfferings: true, errOn: "courses", err: want,
	}})

	_, err := svc.courseRequestsEnabled(context.Background())
	require.ErrorIs(t, err, want)
}

func TestCourseRequestsDisabledWhenCareOfferingsDisabled(t *testing.T) {
	t.Parallel()

	svc := offeringChangesForTest(OfferingChangeDependencies{Settings: offeringChangeSettingsStub{
		changes: true, careOfferings: false,
	}})

	enabled, err := svc.courseRequestsEnabled(context.Background())
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestWithdrawCourseRequestRejectsDisabledFeature(t *testing.T) {
	t.Parallel()

	svc := offeringChangesForTest(OfferingChangeDependencies{Settings: offeringChangeSettingsStub{
		changes: true, careOfferings: true, courses: false,
	}})

	err := svc.WithdrawCourseRequest(context.Background(), 1, 2, 3)
	require.ErrorIs(t, err, careplan.ErrCourseRequestsDisabled)
}

func TestDecisionSnapshotKeepsCourseMarker(t *testing.T) {
	t.Parallel()

	rows := &offeringChangeRowsStub{}
	svc := offeringChangesForTest(OfferingChangeDependencies{Rows: rows})
	err := svc.storeDecisionSnapshot(context.Background(), 1, &offeringDecisionDiff{entries: []careplan.OfferingChangeDiffEntry{{
		OfferingID: 2, Label: "Fußball", OldState: "not_booked", NewState: "booked", IsCourse: true,
	}}})
	require.NoError(t, err)
	snapshot, err := decisionSnapshot(careplan.OfferingChangeRequest{DecisionSnapshot: rows.snapshot})
	require.NoError(t, err)
	require.Len(t, snapshot.Diff, 1)
	assert.True(t, snapshot.Diff[0].IsCourse)

	entries := diffEntriesFromSnapshot(snapshot.Diff)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].IsCourse)
}

// TestEffectiveCourseCapacity pins the rule the school actually maintains:
// both limits count, and the stricter one decides (#3075).
func TestEffectiveCourseCapacity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		groupLimit       *int
		groupTaken       int
		offeringCapacity *int
		offeringFree     *int
		wantCapacity     *int
		wantFree         *int
	}{
		{name: "no limit anywhere stays unlimited"},
		{name: "only the AG limits", groupLimit: intPtr(20), groupTaken: 18, wantCapacity: intPtr(20), wantFree: intPtr(2)},
		{name: "only the offering limits", offeringCapacity: intPtr(10), offeringFree: intPtr(3), wantCapacity: intPtr(10), wantFree: intPtr(3)},
		{
			name: "the stricter limit wins", groupLimit: intPtr(20), groupTaken: 5,
			offeringCapacity: intPtr(8), offeringFree: intPtr(1), wantCapacity: intPtr(8), wantFree: intPtr(1),
		},
		{
			name: "the stricter free count wins even with a larger cap", groupLimit: intPtr(6), groupTaken: 6,
			offeringCapacity: intPtr(30), offeringFree: intPtr(20), wantCapacity: intPtr(6), wantFree: intPtr(0),
		},
		{name: "an overbooked AG never reports negative free slots", groupLimit: intPtr(4), groupTaken: 9, wantCapacity: intPtr(4), wantFree: intPtr(0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capacity, free := effectiveCourseCapacity(tc.groupLimit, tc.groupTaken, tc.offeringCapacity, tc.offeringFree)
			assert.Equal(t, tc.wantCapacity, capacity)
			assert.Equal(t, tc.wantFree, free)
		})
	}
}

// TestCourseItemsFromGroupsKeepsOnlyCourses guards the definition of a Kurs:
// an active care offering that feeds at least one AG, no matter which of the
// two link shapes carries it. Everything else on the enrollment form is care
// and must not appear in the Kurse section.
func TestCourseItemsFromGroupsKeepsOnlyCourses(t *testing.T) {
	t.Parallel()

	catalog := &careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{
		{OfferingID: 1, Name: "Mittagessen", IsActive: true},
		{OfferingID: 2, Name: "Fußball", IsActive: true, Selected: true},
		{OfferingID: 3, Name: "Chor", IsActive: false},
		{OfferingID: 4, Name: "Elternwahl", IsActive: true, DaysOfWeekMode: daysOfWeekModeParentChoice},
		{OfferingID: 5, Name: "Ballett", IsActive: true},
	}}
	groups := map[int64][]ports.CourseGroup{
		2: {{ID: 70, ScheduledWeekdays: []int{3}}},                                           // legacy link on the offering
		3: {{ID: 71, ScheduledWeekdays: []int{1}}},                                           // linked, but the offering is inactive
		4: {{ID: 74, ScheduledWeekdays: []int{2}}},                                           // course days would require a picker the course view has not
		5: {{ID: 72, ScheduledWeekdays: []int{1, 3}}, {ID: 73, ScheduledWeekdays: []int{5}}}, // one offering split across two Regeltermine (#2137)
	}

	items := courseItemsFromGroups(catalog, groups)

	require.Len(t, items, 2)
	assert.Equal(t, "Ballett", items[0].Name, "sorted by name")
	assert.Equal(t, int64(72), items[0].ActivityGroupID, "the first AG identifies the course")
	assert.Equal(t, []string{"mon", "wed", "fri"}, items[0].AvailableDays)
	assert.Equal(t, "Fußball", items[1].Name)
	assert.True(t, items[1].Booked, "a held course is marked as attended")
	assert.Equal(t, int64(70), items[1].ActivityGroupID)
	assert.Equal(t, []string{"wed"}, items[1].AvailableDays, "the course schedule, not the offering, defines displayed days")
}

func TestCourseGroupMatchesTarget(t *testing.T) {
	t.Parallel()

	grade := int16(3)
	catalog := &careplan.OfferingChangeCatalog{TargetGradeLevel: &grade, TargetSchoolClass: " 3B "}
	assert.True(t, courseGroupMatchesTarget(ports.CourseGroup{
		SourceGradeLevels: []int{3}, SourceSchoolClasses: []string{"3b"},
	}, catalog))
	assert.False(t, courseGroupMatchesTarget(ports.CourseGroup{SourceGradeLevels: []int{2}}, catalog))
	assert.False(t, courseGroupMatchesTarget(ports.CourseGroup{SourceSchoolClasses: []string{"3a"}}, catalog))
}

// TestAddedCourseIDs separates a course request from a care-offering change:
// only a course the child does not already hold makes it one.
func TestAddedCourseIDs(t *testing.T) {
	t.Parallel()

	courses := []careplan.CourseCatalogItem{
		{OfferingID: 2, Name: "Fußball"},
		{OfferingID: 5, Name: "Ballett", Booked: true},
	}

	t.Run("adds a course", func(t *testing.T) {
		t.Parallel()
		added, err := addedCourseIDs(requestWithSelections(t, careplan.OfferingChangeSelection{OfferingID: 2}, careplan.OfferingChangeSelection{OfferingID: 5}), courses)
		require.NoError(t, err)
		assert.Equal(t, []int64{2}, added)
	})

	t.Run("keeps a held course out", func(t *testing.T) {
		t.Parallel()
		added, err := addedCourseIDs(requestWithSelections(t, careplan.OfferingChangeSelection{OfferingID: 5}), courses)
		require.NoError(t, err)
		assert.Empty(t, added, "a request that only keeps what is booked adds no course")
	})

	t.Run("a care-only change is not a course request", func(t *testing.T) {
		t.Parallel()
		added, err := addedCourseIDs(requestWithSelections(t, careplan.OfferingChangeSelection{OfferingID: 1}), courses)
		require.NoError(t, err)
		assert.Empty(t, added)
	})
}

func TestCourseWasAddedForGroups(t *testing.T) {
	t.Parallel()

	row := requestWithSelections(t, careplan.OfferingChangeSelection{OfferingID: 2}, careplan.OfferingChangeSelection{OfferingID: 5})
	targetGroupIDs := map[int64]bool{70: true}
	groupsByOffering := map[int64][]ports.CourseGroup{
		2: {{ID: 70, Active: true}},
		5: {{ID: 70, Active: true}},
	}

	added, err := courseWasAddedForGroups(row, targetGroupIDs, groupsByOffering, map[int64]bool{5: true})
	require.NoError(t, err)
	assert.True(t, added)

	retained, err := courseWasAddedForGroups(row, targetGroupIDs, groupsByOffering, map[int64]bool{2: true, 5: true})
	require.NoError(t, err)
	assert.False(t, retained, "a care change that keeps a booked course is not queued")
}

// TestCourseSelectionsWith proves the payload stays a COMPLETE selection:
// the child's current manual bookings plus the requested course. A delta
// would unbook everything else on approval. Days travel only for an offering
// that lets parents pick them.
func TestCourseSelectionsWith(t *testing.T) {
	t.Parallel()

	catalog := &careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{
		{OfferingID: 1, Name: "Regelbetreuung", Selected: true, DaysOfWeekMode: daysOfWeekModeParentChoice, SelectedDays: []string{"mon", "tue"}},
		{OfferingID: 2, Name: "Ferienbetreuung", Selected: true, DaysOfWeekMode: daysOfWeekModeFixed, SelectedDays: []string{"mon"}},
		{OfferingID: 3, Name: "Mittagessen", Selected: true, Automatic: true, DaysOfWeekMode: daysOfWeekModeParentChoice, SelectedDays: []string{"mon"}},
		{OfferingID: 9, Name: "Nicht gebucht"},
	}}

	t.Run("a course with fixed days travels without days", func(t *testing.T) {
		t.Parallel()
		selections := courseSelectionsWith(catalog, &careplan.CourseCatalogItem{OfferingID: 7, Name: "Fußball", AvailableDays: []string{"wed"}})

		require.Len(t, selections, 3)
		assert.Equal(t, int64(1), selections[0].OfferingID)
		assert.Equal(t, []string{"mon", "tue"}, selections[0].SelectedDays)
		assert.Equal(t, int64(2), selections[1].OfferingID)
		assert.Empty(t, selections[1].SelectedDays, "a fixed offering carries no day selection")
		assert.Equal(t, int64(7), selections[2].OfferingID, "the requested course is appended")
		assert.Empty(t, selections[2].SelectedDays)
	})

	t.Run("course days are never inferred from availability", func(t *testing.T) {
		t.Parallel()
		selections := courseSelectionsWith(catalog, &careplan.CourseCatalogItem{OfferingID: 7, Name: "Fußball", AvailableDays: []string{"wed", "thu"}})

		require.Len(t, selections, 3)
		assert.Empty(t, selections[2].SelectedDays)
	})
}

// TestCourseCatalogEntry pins the three answers the create path can give for
// one id: unknown, already booked, or requestable.
func TestCourseCatalogEntry(t *testing.T) {
	t.Parallel()

	courses := []careplan.CourseCatalogItem{
		{OfferingID: 2, Name: "Fußball"},
		{OfferingID: 3, Name: "Chor", Booked: true},
	}

	entry, err := courseCatalogEntry(courses, 2)
	require.NoError(t, err)
	assert.Equal(t, "Fußball", entry.Name)

	_, err = courseCatalogEntry(courses, 1)
	assert.ErrorIs(t, err, careplan.ErrCourseNotFound, "a care offering is not a course")

	_, err = courseCatalogEntry(courses, 3)
	assert.ErrorIs(t, err, careplan.ErrCourseAlreadyBooked)

	_, err = courseCatalogEntry(courses, 404)
	assert.ErrorIs(t, err, careplan.ErrCourseNotFound)
}

func TestIsCourseOnlyRequestRejectsMixedCareChanges(t *testing.T) {
	t.Parallel()

	catalog := &careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{
		{OfferingID: 1, Selected: true, DaysOfWeekMode: daysOfWeekModeParentChoice, SelectedDays: []string{"mon", "tue"}},
		{OfferingID: 2},
	}}

	pure := requestWithSelections(t,
		careplan.OfferingChangeSelection{OfferingID: 1, SelectedDays: []string{"mon", "tue"}},
		careplan.OfferingChangeSelection{OfferingID: 2},
	)
	isPure, err := isCourseOnlyRequest(pure, catalog, []int64{2})
	require.NoError(t, err)
	assert.True(t, isPure)

	mixed := requestWithSelections(t,
		careplan.OfferingChangeSelection{OfferingID: 1, SelectedDays: []string{"mon"}},
		careplan.OfferingChangeSelection{OfferingID: 2},
	)
	isPure, err = isCourseOnlyRequest(mixed, catalog, []int64{2})
	require.NoError(t, err)
	assert.False(t, isPure, "withdrawing it as a course request would discard the care change")
}

func TestMarkCourseDiffEntriesMarksOnlyEligibleDirectCourseAdditions(t *testing.T) {
	t.Parallel()

	grade := int16(3)
	catalog := &careplan.OfferingChangeCatalog{TargetGradeLevel: &grade, TargetSchoolClass: "3a"}
	entries := []careplan.OfferingChangeDiffEntry{
		{OfferingID: 1, OldState: "not_booked", NewState: "booked"},
		{OfferingID: 2, OldState: "booked", NewState: "removed"},
		{OfferingID: 3, OldState: "not_booked", NewState: "booked"},
		{OfferingID: 4, OldState: "not_booked", NewState: "booked"},
	}
	groups := map[int64][]ports.CourseGroup{
		1: {{ID: 10, Active: true, SourceGradeLevels: []int{3}, SourceSchoolClasses: []string{"3A"}}},
		2: {{ID: 20, Active: true, SourceGradeLevels: []int{3}}},
		3: {{ID: 30, Active: true, SourceGradeLevels: []int{4}}},
		4: {{ID: 40, Active: true, SourceGradeLevels: []int{3}}},
	}

	markCourseDiffEntriesForGroups(entries, groups, catalog, map[int64]bool{1: true, 2: true, 3: true})

	assert.True(t, entries[0].IsCourse)
	assert.False(t, entries[1].IsCourse, "removing a course is not a course request")
	assert.False(t, entries[2].IsCourse, "the child does not match this course's source grade")
	assert.False(t, entries[3].IsCourse, "automatic or otherwise unrequested additions are not course requests")
}

func TestActiveCourseGroupsForTargetRejectsArchivedCourse(t *testing.T) {
	t.Parallel()

	grade := int16(3)
	groups, hadCourseTarget := activeCourseGroupsForTarget([]ports.CourseGroup{
		{ID: 1, Active: false, SourceGradeLevels: []int{3}},
		{ID: 2, Active: true, SourceGradeLevels: []int{4}},
	}, &careplan.OfferingChangeCatalog{TargetGradeLevel: &grade})

	assert.True(t, hadCourseTarget)
	assert.Empty(t, groups, "approval must not treat an archived course as ordinary care")
}

func TestCourseWaitlistPositionUsesGroupTargetAndRequestIDOrder(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	withID := func(id, childID int64, created time.Time, offeringID int64) careplan.OfferingChangeRequest {
		row := requestWithSelections(t, careplan.OfferingChangeSelection{OfferingID: offeringID})
		row.ID, row.RequestChildID, row.CreatedAt = id, childID, created
		return row
	}
	rows := []careplan.OfferingChangeRequest{
		withID(11, 101, createdAt, 8),
		withID(12, 102, createdAt, 7),
		withID(13, 103, createdAt, 7),
		withID(14, 104, createdAt.Add(-time.Minute), 7),
	}
	gradeThree, gradeFour := int16(3), int16(4)
	queue := &courseWaitlist{
		rows: rows, pending: rows[1],
		childrenByID: map[int64]*ports.OfferingChangeChild{
			101: {TargetGradeLevel: &gradeThree},
			102: {TargetGradeLevel: &gradeThree},
			103: {TargetGradeLevel: &gradeThree},
			104: {TargetGradeLevel: &gradeFour},
		},
		bookedByChild: map[int64]map[int64]bool{},
	}

	position := queue.position(
		[]ports.CourseGroup{{ID: 70, Active: true, SourceGradeLevels: []int{3}}},
		map[int64][]ports.CourseGroup{
			7: {{ID: 70, Active: true, SourceGradeLevels: []int{3}}},
			8: {{ID: 70, Active: true, SourceGradeLevels: []int{3}}},
		},
	)

	assert.Equal(t, 2, position, "the earlier request through another offering reaches the same course group")
}

func TestCourseRequestQueuePositionRequiresTheOldestRequest(t *testing.T) {
	t.Parallel()

	assert.NoError(t, assertCourseRequestQueuePosition(1))
	assert.ErrorIs(t, assertCourseRequestQueuePosition(2), careplan.ErrOfferingChangeCapacityFull)
}

func TestCourseGroupsForCompetingRequestsLoadsUnavailableOffering(t *testing.T) {
	t.Parallel()

	groupID := int64(70)
	first := requestWithSelections(t, careplan.OfferingChangeSelection{OfferingID: 8})
	first.ID = 11
	second := requestWithSelections(t, careplan.OfferingChangeSelection{OfferingID: 7})
	second.ID = 12
	svc := offeringChangesForTest(OfferingChangeDependencies{
		Catalog:  offeringCatalogStub{offerings: []careplan.CareOffering{{ID: 8, ActivityGroupID: &groupID}}},
		Planning: coursePlanningStub{groups: map[int64][]ports.CourseGroup{8: {{ID: groupID, Active: true}}}},
	})

	groups, err := svc.courseGroupsForCompetingRequests(
		context.Background(), &courseWaitlist{rows: []careplan.OfferingChangeRequest{first, second}, pending: second},
		&careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{{OfferingID: 7}}},
		map[int64][]ports.CourseGroup{7: {{ID: groupID, Active: true}}},
	)

	require.NoError(t, err)
	assert.Equal(t, []ports.CourseGroup{{ID: groupID, Active: true}}, groups[8])
}
