package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

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
		{OfferingID: 5, Name: "Ballett", IsActive: true},
	}}
	groups := map[int64][]int64{
		2: {70},     // legacy link on the offering
		3: {71},     // linked, but the offering is inactive
		5: {72, 73}, // one offering split across two Regeltermine (#2137)
	}

	items := courseItemsFromGroups(catalog, groups)

	require.Len(t, items, 2)
	assert.Equal(t, "Ballett", items[0].Name, "sorted by name")
	assert.Equal(t, int64(72), items[0].ActivityGroupID, "the first AG identifies the course")
	assert.Equal(t, "Fußball", items[1].Name)
	assert.True(t, items[1].Booked, "a held course is marked as attended")
	assert.Equal(t, int64(70), items[1].ActivityGroupID)
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
		course := &OfferingChangeCatalogItem{
			OfferingID:     7,
			Name:           "Fußball",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"wed"},
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

	t.Run("a course the family may pick days for keeps them", func(t *testing.T) {
		t.Parallel()
		course := &OfferingChangeCatalogItem{
			OfferingID:     7,
			Name:           "Fußball",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"wed", "thu"},
		}

		selections := courseSelectionsWith(catalog, course)

		require.Len(t, selections, 3)
		assert.Equal(t, []string{"wed", "thu"}, selections[2].SelectedDays)
	})
}

// TestCourseCatalogEntry pins the three answers the create path can give for
// one id: unknown, already booked, or requestable.
func TestCourseCatalogEntry(t *testing.T) {
	t.Parallel()

	catalog := &OfferingChangeCatalog{Items: []OfferingChangeCatalogItem{
		{OfferingID: 1, Name: "Mittagessen", IsActive: true},
		{OfferingID: 2, Name: "Fußball", IsActive: true},
		{OfferingID: 3, Name: "Chor", IsActive: true, Selected: true},
	}}
	groups := map[int64][]int64{2: {70}, 3: {71}}

	entry, err := courseCatalogEntry(catalog, groups, 2)
	require.NoError(t, err)
	assert.Equal(t, "Fußball", entry.Name)

	_, err = courseCatalogEntry(catalog, groups, 1)
	assert.ErrorIs(t, err, ErrCourseNotFound, "a care offering is not a course")

	_, err = courseCatalogEntry(catalog, groups, 3)
	assert.ErrorIs(t, err, ErrCourseAlreadyBooked)

	_, err = courseCatalogEntry(catalog, groups, 404)
	assert.ErrorIs(t, err, ErrCourseNotFound)
}
