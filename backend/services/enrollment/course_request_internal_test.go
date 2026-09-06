package enrollment

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

type courseSettingsStub struct {
	values map[string]bool
	errKey string
	err    error
}

type courseOfferingRepoStub struct {
	enrollmentModels.CareOfferingRepository
	offerings []*enrollmentModels.CareOffering
}

type courseChangeRepoStub struct {
	enrollmentModels.OfferingChangeRequestRepository
	snapshot *enrollmentModels.OfferingChangeDecisionSnapshot
}

func (r *courseChangeRepoStub) UpdateDecisionSnapshot(
	_ context.Context,
	_ int64,
	snapshot *enrollmentModels.OfferingChangeDecisionSnapshot,
) error {
	r.snapshot = snapshot
	return nil
}

func (r courseOfferingRepoStub) ListByIDs(context.Context, []int64) ([]*enrollmentModels.CareOffering, error) {
	return r.offerings, nil
}

type courseProjectionStub struct {
	groups map[int64][]enrollmentModels.CourseGroup
}

func (r courseProjectionStub) ListManualPlanningOccurrences(context.Context, int64, string, string) ([]ManualPlanningOccurrence, error) {
	return nil, nil
}

func (r courseProjectionStub) CourseGroupsForOfferings(context.Context, []enrollmentModels.CourseOfferingReference, timezone.Date) (map[int64][]enrollmentModels.CourseGroup, error) {
	return r.groups, nil
}

func (courseProjectionStub) LockCourseGroups(context.Context, []int64) ([]enrollmentModels.CourseGroup, error) {
	return nil, nil
}

func (courseProjectionStub) CountActiveCourseEnrollments(context.Context, []int64, timezone.Date, timezone.Date, int64) (map[int64]int, error) {
	return nil, nil
}

func TestCourseRequestsEnabledReturnsSettingsFailure(t *testing.T) {
	t.Parallel()

	want := errors.New("settings unavailable")
	svc := &offeringChangeRequestService{OfferingChangeRequestServiceConfig: OfferingChangeRequestServiceConfig{
		Settings: courseSettingsStub{
			values: map[string]bool{
				configModel.KeyEnrollmentOfferingChangesEnabled: true,
				configModel.KeyEnrollmentCareOfferingsEnabled:   true,
			},
			errKey: configModel.KeyEnrollmentParentCourseRequestsEnabled,
			err:    want,
		},
		Logger: slog.Default(),
	}}

	_, err := svc.courseRequestsEnabled(context.Background())
	require.ErrorIs(t, err, want)
}

func TestCourseRequestsDisabledWhenCareOfferingsDisabled(t *testing.T) {
	t.Parallel()

	svc := &offeringChangeRequestService{OfferingChangeRequestServiceConfig: OfferingChangeRequestServiceConfig{
		Settings: courseSettingsStub{values: map[string]bool{
			configModel.KeyEnrollmentOfferingChangesEnabled: true,
			configModel.KeyEnrollmentCareOfferingsEnabled:   false,
		}},
		Logger: slog.Default(),
	}}

	enabled, err := svc.courseRequestsEnabled(context.Background())
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestWithdrawCourseRequestRejectsDisabledFeature(t *testing.T) {
	t.Parallel()

	svc := &offeringChangeRequestService{OfferingChangeRequestServiceConfig: OfferingChangeRequestServiceConfig{
		Settings: courseSettingsStub{values: map[string]bool{
			configModel.KeyEnrollmentOfferingChangesEnabled:      true,
			configModel.KeyEnrollmentCareOfferingsEnabled:        true,
			configModel.KeyEnrollmentParentCourseRequestsEnabled: false,
		}},
		Logger: slog.Default(),
	}}

	err := svc.WithdrawCourseRequest(context.Background(), 1, 2, 3)
	require.ErrorIs(t, err, ErrCourseRequestsDisabled)
}

func TestDecisionSnapshotKeepsCourseMarker(t *testing.T) {
	t.Parallel()

	repo := &courseChangeRepoStub{}
	svc := &offeringChangeRequestService{OfferingChangeRequestServiceConfig{ChangeRepo: repo}}
	err := svc.storeDecisionSnapshot(context.Background(), 1, &offeringDecisionDiff{entries: []OfferingChangeDiffEntry{{
		OfferingID: 2, Label: "Fußball", OldState: "not_booked", NewState: "booked", IsCourse: true,
	}}})
	require.NoError(t, err)
	require.Len(t, repo.snapshot.Diff, 1)
	assert.True(t, repo.snapshot.Diff[0].IsCourse)

	entries := diffEntriesFromSnapshot(repo.snapshot.Diff)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].IsCourse)
}

func (s courseSettingsStub) ResolveBool(_ context.Context, key string) (bool, error) {
	if key == s.errKey && s.err != nil {
		return false, s.err
	}
	return s.values[key], nil
}

func (courseSettingsStub) ResolveString(context.Context, string) (string, error) {
	return "", nil
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
		{
			name: "no limit anywhere stays unlimited",
		},
		{
			name:         "only the AG limits",
			groupLimit:   intPtr(20),
			groupTaken:   18,
			wantCapacity: intPtr(20),
			wantFree:     intPtr(2),
		},
		{
			name:             "only the offering limits",
			offeringCapacity: intPtr(10),
			offeringFree:     intPtr(3),
			wantCapacity:     intPtr(10),
			wantFree:         intPtr(3),
		},
		{
			name:             "the stricter limit wins",
			groupLimit:       intPtr(20),
			groupTaken:       5,
			offeringCapacity: intPtr(8),
			offeringFree:     intPtr(1),
			wantCapacity:     intPtr(8),
			wantFree:         intPtr(1),
		},
		{
			name:             "the stricter free count wins even with a larger cap",
			groupLimit:       intPtr(6),
			groupTaken:       6,
			offeringCapacity: intPtr(30),
			offeringFree:     intPtr(20),
			wantCapacity:     intPtr(6),
			wantFree:         intPtr(0),
		},
		{
			name:         "an overbooked AG never reports negative free slots",
			groupLimit:   intPtr(4),
			groupTaken:   9,
			wantCapacity: intPtr(4),
			wantFree:     intPtr(0),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capacity, free := effectiveCourseCapacity(
				tc.groupLimit, tc.groupTaken, tc.offeringCapacity, tc.offeringFree,
			)
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

	catalog := &OfferingChangeCatalog{Items: []OfferingChangeCatalogItem{
		{OfferingID: 1, Name: "Mittagessen", IsActive: true},
		{OfferingID: 2, Name: "Fußball", IsActive: true, Selected: true},
		{OfferingID: 3, Name: "Chor", IsActive: false},
		{OfferingID: 4, Name: "Elternwahl", IsActive: true, DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice},
		{OfferingID: 5, Name: "Ballett", IsActive: true},
	}}
	groups := map[int64][]enrollmentModels.CourseGroup{
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
	catalog := &OfferingChangeCatalog{TargetGradeLevel: &grade, TargetSchoolClass: " 3B "}
	assert.True(t, courseGroupMatchesTarget(enrollmentModels.CourseGroup{
		SourceGradeLevels: []int{3}, SourceSchoolClasses: []string{"3b"},
	}, catalog))
	assert.False(t, courseGroupMatchesTarget(enrollmentModels.CourseGroup{
		SourceGradeLevels: []int{2},
	}, catalog))
	assert.False(t, courseGroupMatchesTarget(enrollmentModels.CourseGroup{
		SourceSchoolClasses: []string{"3a"},
	}, catalog))
}

// TestAddedCourseIDs separates a course request from a care-offering change:
// only a course the child does not already hold makes it one.
func TestAddedCourseIDs(t *testing.T) {
	t.Parallel()

	courses := []CourseCatalogItem{
		{OfferingID: 2, Name: "Fußball"},
		{OfferingID: 5, Name: "Ballett", Booked: true},
	}

	t.Run("adds a course", func(t *testing.T) {
		t.Parallel()
		request := &enrollmentModels.OfferingChangeRequest{
			Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 2}, {OfferingID: 5}}),
		}
		added, err := addedCourseIDs(request, courses)
		require.NoError(t, err)
		assert.Equal(t, []int64{2}, added)
	})

	t.Run("keeps a held course out", func(t *testing.T) {
		t.Parallel()
		request := &enrollmentModels.OfferingChangeRequest{
			Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 5}}),
		}
		added, err := addedCourseIDs(request, courses)
		require.NoError(t, err)
		assert.Empty(t, added, "a request that only keeps what is booked adds no course")
	})

	t.Run("a care-only change is not a course request", func(t *testing.T) {
		t.Parallel()
		request := &enrollmentModels.OfferingChangeRequest{
			Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 1}}),
		}
		added, err := addedCourseIDs(request, courses)
		require.NoError(t, err)
		assert.Empty(t, added)
	})
}

func TestCourseWasAddedForGroups(t *testing.T) {
	t.Parallel()

	row := &enrollmentModels.OfferingChangeRequest{
		Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 2}, {OfferingID: 5}}),
	}
	targetGroupIDs := map[int64]bool{70: true}
	groupsByOffering := map[int64][]enrollmentModels.CourseGroup{
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

// TestCourseSelectionsWith proves the payload stays a COMPLETE selection: the
// child's current manual bookings plus the requested course. A delta would
// unbook everything else on approval. Days travel only for an offering that
// lets parents pick them — sending days for a "fixed" one is refused.
func TestCourseSelectionsWith(t *testing.T) {
	t.Parallel()

	catalog := &OfferingChangeCatalog{Items: []OfferingChangeCatalogItem{
		{
			OfferingID: 1, Name: "Regelbetreuung", Selected: true,
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			SelectedDays:   []string{"mon", "tue"},
		},
		{
			OfferingID: 2, Name: "Ferienbetreuung", Selected: true,
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			SelectedDays:   []string{"mon"},
		},
		{
			OfferingID: 3, Name: "Mittagessen", Selected: true, Automatic: true,
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			SelectedDays:   []string{"mon"},
		},
		{OfferingID: 9, Name: "Nicht gebucht"},
	}}

	t.Run("a course with fixed days travels without days", func(t *testing.T) {
		t.Parallel()
		course := &CourseCatalogItem{
			OfferingID:    7,
			Name:          "Fußball",
			AvailableDays: []string{"wed"},
		}

		selections := courseSelectionsWith(catalog, course)

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
		course := &CourseCatalogItem{
			OfferingID:    7,
			Name:          "Fußball",
			AvailableDays: []string{"wed", "thu"},
		}

		selections := courseSelectionsWith(catalog, course)

		require.Len(t, selections, 3)
		assert.Empty(t, selections[2].SelectedDays)
	})
}

// TestCourseCatalogEntry pins the three answers the create path can give for
// one id: unknown, already booked, or requestable.
func TestCourseCatalogEntry(t *testing.T) {
	t.Parallel()

	courses := []CourseCatalogItem{
		{OfferingID: 2, Name: "Fußball"},
		{OfferingID: 3, Name: "Chor", Booked: true},
	}

	entry, err := courseCatalogEntry(courses, 2)
	require.NoError(t, err)
	assert.Equal(t, "Fußball", entry.Name)

	_, err = courseCatalogEntry(courses, 1)
	assert.ErrorIs(t, err, ErrCourseNotFound, "a care offering is not a course")

	_, err = courseCatalogEntry(courses, 3)
	assert.ErrorIs(t, err, ErrCourseAlreadyBooked)

	_, err = courseCatalogEntry(courses, 404)
	assert.ErrorIs(t, err, ErrCourseNotFound)
}

func TestIsCourseOnlyRequestRejectsMixedCareChanges(t *testing.T) {
	t.Parallel()

	catalog := &OfferingChangeCatalog{Items: []OfferingChangeCatalogItem{
		{
			OfferingID: 1, Selected: true,
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			SelectedDays:   []string{"mon", "tue"},
		},
		{OfferingID: 2},
	}}

	pure := &enrollmentModels.OfferingChangeRequest{
		Payload: payloadFromSelections([]OfferingChangeSelection{
			{OfferingID: 1, SelectedDays: []string{"mon", "tue"}},
			{OfferingID: 2},
		}),
	}
	isPure, err := isCourseOnlyRequest(pure, catalog, []int64{2})
	require.NoError(t, err)
	assert.True(t, isPure)

	mixed := &enrollmentModels.OfferingChangeRequest{
		Payload: payloadFromSelections([]OfferingChangeSelection{
			{OfferingID: 1, SelectedDays: []string{"mon"}},
			{OfferingID: 2},
		}),
	}
	isPure, err = isCourseOnlyRequest(mixed, catalog, []int64{2})
	require.NoError(t, err)
	assert.False(t, isPure, "withdrawing it as a course request would discard the care change")
}

func TestMarkCourseDiffEntriesMarksOnlyEligibleDirectCourseAdditions(t *testing.T) {
	t.Parallel()

	grade := int16(3)
	catalog := &OfferingChangeCatalog{TargetGradeLevel: &grade, TargetSchoolClass: "3a"}
	entries := []OfferingChangeDiffEntry{
		{OfferingID: 1, OldState: "not_booked", NewState: "booked"},
		{OfferingID: 2, OldState: "booked", NewState: "removed"},
		{OfferingID: 3, OldState: "not_booked", NewState: "booked"},
		{OfferingID: 4, OldState: "not_booked", NewState: "booked"},
	}
	groups := map[int64][]enrollmentModels.CourseGroup{
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
	groups, hadCourseTarget := activeCourseGroupsForTarget([]enrollmentModels.CourseGroup{
		{ID: 1, Active: false, SourceGradeLevels: []int{3}},
		{ID: 2, Active: true, SourceGradeLevels: []int{4}},
	}, &OfferingChangeCatalog{TargetGradeLevel: &grade})

	assert.True(t, hadCourseTarget)
	assert.Empty(t, groups, "approval must not treat an archived course as ordinary care")
}

func TestCourseWaitlistPositionUsesGroupTargetAndRequestIDOrder(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	rows := []*enrollmentModels.OfferingChangeRequest{
		{ID: 11, RequestChildID: 101, CreatedAt: createdAt, Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 8}})},
		{ID: 12, RequestChildID: 102, CreatedAt: createdAt, Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 7}})},
		{ID: 13, RequestChildID: 103, CreatedAt: createdAt, Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 7}})},
		{ID: 14, RequestChildID: 104, CreatedAt: createdAt.Add(-time.Minute), Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 7}})},
	}
	gradeThree, gradeFour := int16(3), int16(4)
	children := map[int64]*RequestChild{
		101: {TargetGradeLevel: &gradeThree},
		102: {TargetGradeLevel: &gradeThree},
		103: {TargetGradeLevel: &gradeThree},
		104: {TargetGradeLevel: &gradeFour},
	}

	position := courseWaitlistPositionFromRows(
		rows,
		[]enrollmentModels.CourseGroup{{ID: 70, Active: true, SourceGradeLevels: []int{3}}},
		map[int64][]enrollmentModels.CourseGroup{
			7: {{ID: 70, Active: true, SourceGradeLevels: []int{3}}},
			8: {{ID: 70, Active: true, SourceGradeLevels: []int{3}}},
		},
		rows[1], children, map[int64]map[int64]bool{},
	)

	assert.Equal(t, 2, position, "the earlier request through another offering reaches the same course group")
}

func TestCourseRequestQueuePositionRequiresTheOldestRequest(t *testing.T) {
	t.Parallel()

	assert.NoError(t, assertCourseRequestQueuePosition(1))
	assert.ErrorIs(t, assertCourseRequestQueuePosition(2), ErrOfferingChangeCapacityFull)
}

func TestCourseGroupsForCompetingRequestsLoadsUnavailableOffering(t *testing.T) {
	t.Parallel()

	groupID := int64(70)
	rows := []*enrollmentModels.OfferingChangeRequest{
		{ID: 11, Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 8}})},
		{ID: 12, Payload: payloadFromSelections([]OfferingChangeSelection{{OfferingID: 7}})},
	}
	service := &offeringChangeRequestService{OfferingChangeRequestServiceConfig: OfferingChangeRequestServiceConfig{
		CareOfferingRepo: courseOfferingRepoStub{offerings: []*enrollmentModels.CareOffering{{
			ID: 8, ActivityGroupID: &groupID,
		}}},
		ImpactRepo: courseProjectionStub{groups: map[int64][]enrollmentModels.CourseGroup{
			8: {{ID: groupID, Active: true}},
		}},
	}}

	groups, err := service.courseGroupsForCompetingRequests(
		context.Background(), rows, rows[1], &OfferingChangeCatalog{Items: []OfferingChangeCatalogItem{{OfferingID: 7}}},
		map[int64][]enrollmentModels.CourseGroup{7: {{ID: groupID, Active: true}}},
	)

	require.NoError(t, err)
	assert.Equal(t, []enrollmentModels.CourseGroup{{ID: groupID, Active: true}}, groups[8])
}
