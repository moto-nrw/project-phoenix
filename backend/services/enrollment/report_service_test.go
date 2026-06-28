package enrollment

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	baseModels "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestCareUsageRowCountsEffectiveDaysAsUnion(t *testing.T) {
	req := &enrollmentModels.Request{
		Model:             baseModels.Model{ID: 10},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}
	child := &enrollmentModels.RequestChild{
		Model:     baseModels.Model{ID: 20},
		RequestID: 10,
		FirstName: "Lina",
		LastName:  "Muster",
		Status:    enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Regelbetreuung",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
		2: {
			Name:           "AG",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"tue", "wed"},
		},
	}
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue"}},
		{RequestChildID: 20, CareOfferingID: 2},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true, 2: true})

	require.Len(t, row.Offerings, 2)
	assert.Equal(t, []string{"mon", "tue", "wed"}, row.EffectiveDays)
	assert.Equal(t, 3, row.DayCount)
	assert.Equal(t, []string{"tue", "wed"}, row.Offerings[0].Days)
	assert.Equal(t, "available", row.Offerings[0].DaysSource)
	assert.Equal(t, []string{"mon", "tue"}, row.Offerings[1].Days)
	assert.Equal(t, "selected", row.Offerings[1].DaysSource)
}

func TestClassRosterRowUsesPhaseEnrollmentData(t *testing.T) {
	schemaID := int64(88)
	req := &enrollmentModels.Request{
		Model:       baseModels.Model{ID: 10},
		SchemaID:    &schemaID,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}
	child := &enrollmentModels.RequestChild{
		Model:       baseModels.Model{ID: 20},
		RequestID:   10,
		FirstName:   "Lina",
		LastName:    "Muster",
		DateOfBirth: timezone.NewDate(2018, 5, 4),
		Status:      enrollmentModels.ChildStatusApproved,
		CustomData: map[string]any{
			"arrival": map[string]any{"mon": "11:30"},
			"pickup":  map[string]any{"mon": "14:30"},
			"departure": map[string]any{
				"mon": []any{"pickup", "accompanied"},
			},
			enrollmentModels.TargetStudentDepartureCompanionNote: "Mia",
		},
	}
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 100},
		PersonID:    200,
		SchoolClass: "1a",
	}
	person := &userModels.Person{
		FirstName: "Lina",
		LastName:  "Muster",
	}
	enrollment := &classRosterApprovedEnrollment{
		request: req,
		child:   child,
		links: []*enrollmentModels.RequestChildOffering{
			{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon"}},
		},
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "Randstunde", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice},
	}
	schemas := map[int64]*enrollmentModels.FormSchema{
		schemaID: {
			Fields: []enrollmentModels.FormField{
				{Key: "arrival", Target: enrollmentModels.TargetScheduleArrival, Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: true},
				{Key: "pickup", Target: enrollmentModels.TargetSchedulePickup, Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: true},
				{Key: "departure", Target: enrollmentModels.TargetStudentAllowedDepartureModes, Type: enrollmentModels.FormFieldWeekdayMultiMode, AppliesToCh: true},
			},
		},
	}

	row, err := classRosterRow(student, person, "Eulen", enrollment, offerings, schemas)

	require.NoError(t, err)
	assert.Equal(t, "Eulen", row.GroupName)
	assert.True(t, row.Registered)
	assert.Equal(t, "Angemeldet: Randstunde", row.EnrollmentSummary)
	assert.Equal(t, []string{"mon"}, row.CareDays)
	assert.Equal(t, "11:30", row.ArrivalByDay["mon"])
	assert.Equal(t, "14:30", row.PickupByDay["mon"])
	assert.Contains(t, row.Departure, "Mo: Abholung, Mit anderem Kind")
	assert.Contains(t, row.Departure, "(mit: Mia)")
}

func TestClassRosterRowMarksMissingEnrollmentAsNoRegistration(t *testing.T) {
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 101},
		PersonID:    201,
		SchoolClass: "1a",
	}
	person := &userModels.Person{FirstName: "Tom", LastName: "Ohne"}

	row, err := classRosterRow(student, person, "", nil, nil, nil)

	require.NoError(t, err)
	assert.False(t, row.Registered)
	assert.Equal(t, "Keine Anmeldung", row.EnrollmentSummary)
	assert.Equal(t, []string{}, row.CareDays)
	assert.Equal(t, "Geht alleine", row.Departure)
}

func TestClassRosterApprovedEnrollmentsOnlyUsesApprovedChildrenInClass(t *testing.T) {
	studentID := int64(100)
	otherStudentID := int64(200)
	requestByID := map[int64]*enrollmentModels.Request{
		1: {Model: baseModels.Model{ID: 1}, SubmittedAt: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)},
		2: {Model: baseModels.Model{ID: 2}, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
	}
	studentByID := map[int64]*userModels.Student{
		studentID: {Model: baseModels.Model{ID: studentID}},
	}
	children := []*enrollmentModels.RequestChild{
		{Model: baseModels.Model{ID: 10}, RequestID: 1, Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &studentID},
		{Model: baseModels.Model{ID: 11}, RequestID: 2, Status: enrollmentModels.ChildStatusRejected, CreatedStudentID: &studentID},
		{Model: baseModels.Model{ID: 12}, RequestID: 2, Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &otherStudentID},
	}

	got, childIDs := classRosterApprovedEnrollments(children, requestByID, studentByID)

	require.Len(t, got, 1)
	assert.Equal(t, int64(10), got[studentID].child.ID)
	assert.Equal(t, []int64{10}, childIDs)
}

func TestClassRosterApprovedEnrollmentsUsesOnlyNewestChildLinks(t *testing.T) {
	studentID := int64(100)
	requestByID := map[int64]*enrollmentModels.Request{
		1: {Model: baseModels.Model{ID: 1}, SubmittedAt: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)},
		2: {Model: baseModels.Model{ID: 2}, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
	}
	studentByID := map[int64]*userModels.Student{
		studentID: {Model: baseModels.Model{ID: studentID}},
	}
	children := []*enrollmentModels.RequestChild{
		{Model: baseModels.Model{ID: 10}, RequestID: 1, Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &studentID},
		{Model: baseModels.Model{ID: 20}, RequestID: 2, Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &studentID},
	}

	got, childIDs := classRosterApprovedEnrollments(children, requestByID, studentByID)
	classRosterAttachOfferingLinks(got, []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 10, CareOfferingID: 1},
		{RequestChildID: 20, CareOfferingID: 2},
	})

	require.Len(t, got, 1)
	assert.Equal(t, int64(20), got[studentID].child.ID)
	assert.Equal(t, []int64{20}, childIDs)
	require.Len(t, got[studentID].links, 1)
	assert.Equal(t, int64(2), got[studentID].links[0].CareOfferingID)
}

func TestClassRosterGroupNameResolvesAssignedGroup(t *testing.T) {
	groupID := int64(12)
	student := &userModels.Student{GroupID: &groupID}
	groups := map[int64]*educationModels.Group{
		groupID: {Name: "Klasse 2a"},
	}

	assert.Equal(t, "Klasse 2a", classRosterGroupName(student, groups))
}

func TestReportServiceClassRosterGroupNamesLoadsUniqueGroups(t *testing.T) {
	groupID := int64(12)
	otherGroupID := int64(13)
	repo := &fakeEducationGroupRepo{groups: map[int64]*educationModels.Group{
		groupID:      {Name: "Klasse 2a"},
		otherGroupID: {Name: "Klasse 3b"},
	}}
	svc := &reportService{educationGroupRepo: repo}

	groups, err := svc.classRosterGroupNames(context.Background(), []*userModels.Student{
		{GroupID: &groupID},
		{GroupID: &groupID},
		{GroupID: &otherGroupID},
		{},
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{groupID, otherGroupID}, repo.seenIDs)
	assert.Equal(t, "Klasse 2a", groups[groupID].Name)
}

func TestCareUsageRowDoesNotInflateMissingParentChoiceDays(t *testing.T) {
	req := &enrollmentModels.Request{
		Model:             baseModels.Model{ID: 10},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}
	child := &enrollmentModels.RequestChild{
		Model:     baseModels.Model{ID: 20},
		RequestID: 10,
		FirstName: "Lina",
		LastName:  "Muster",
		Status:    enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Regelbetreuung",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
	}
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true})

	require.Len(t, row.Offerings, 1)
	assert.Empty(t, row.Offerings[0].Days)
	assert.NotNil(t, row.Offerings[0].Days)
	assert.Equal(t, "selected", row.Offerings[0].DaysSource)
	assert.Empty(t, row.EffectiveDays)
	assert.NotNil(t, row.EffectiveDays)
	assert.Equal(t, 0, row.DayCount)
}

func TestSortedDayCodesDedupesAndOrdersWeekdays(t *testing.T) {
	got := sortedDayCodes([]string{"fri", "mon", "mon", "wed", "tue"})
	assert.Equal(t, []string{"mon", "tue", "wed", "fri"}, got)
}

func TestSortedDayCodesReturnsEmptySliceForEmptyInput(t *testing.T) {
	got := sortedDayCodes(nil)
	assert.Empty(t, got)
	assert.NotNil(t, got)
}

func TestCareUsageRowMatchesFilters(t *testing.T) {
	grade := int16(2)
	dayCount := 3
	row := CareUsageRow{
		ChildFirstName:   "Lina",
		ChildLastName:    "Muster",
		TargetGradeLevel: &grade,
		Status:           enrollmentModels.ChildStatusApproved,
		Offerings: []CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed", "fri"}},
		},
		EffectiveDays:     []string{"mon", "wed", "fri"},
		DayCount:          3,
		PickupByDay:       map[string]string{"mon": "14:30", "wed": "16:00", "fri": "14:30"},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}

	assert.True(t, careUsageRowMatches(row, CareUsageFilters{
		Status:     enrollmentModels.ChildStatusApproved,
		DayCount:   &dayCount,
		GradeLevel: &grade,
		Search:     "eva@example",
	}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: enrollmentModels.ChildStatusRejected}))
	otherDayCount := 4
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", DayCount: &otherDayCount}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "mon"}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "wed", PickupTime: "16:00"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "wed", PickupTime: "16:00", Search: "unbekannt"}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", PickupTime: "14:30"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "tue"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "wed", PickupTime: "14:30"}))

	otherGrade := int16(3)
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", GradeLevel: &otherGrade}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Search: "unbekannt"}))
}

func TestCareUsageRowMatchesExplicitOfferingFilter(t *testing.T) {
	row := CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed", "fri"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed", "fri"},
		DayCount:      3,
	}

	assert.True(t, careUsageRowMatches(row, CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{11},
	}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{12},
	}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{},
	}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{
		Status:          "all",
		CareOfferingIDs: []int64{12},
	}))
}

func TestCareUsageRowMatchesZeroDayFilter(t *testing.T) {
	zero := 0
	row := CareUsageRow{
		Status:        enrollmentModels.ChildStatusApproved,
		EffectiveDays: []string{},
		DayCount:      0,
	}

	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", DayCount: &zero}))
}

func TestCareUsageRowExcludesNonIncludedOfferingsFromDayCount(t *testing.T) {
	req := &enrollmentModels.Request{Model: baseModels.Model{ID: 10}}
	child := &enrollmentModels.RequestChild{
		Model:  baseModels.Model{ID: 20},
		Status: enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
		2: {
			Name:           "Randstunde",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
		},
	}
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue", "wed", "thu"}},
		{RequestChildID: 20, CareOfferingID: 2, SelectedDays: []string{"fri"}},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true})

	require.Len(t, row.Offerings, 2)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, row.EffectiveDays)
	assert.Equal(t, 4, row.DayCount)
}

func TestCareUsageRowKeepsOfferingsVisibleWhenNoOfferingsAreIncluded(t *testing.T) {
	req := &enrollmentModels.Request{Model: baseModels.Model{ID: 10}}
	child := &enrollmentModels.RequestChild{
		Model:  baseModels.Model{ID: 20},
		Status: enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
	}
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue"}},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{})

	require.Len(t, row.Offerings, 1)
	assert.Equal(t, []string{"mon", "tue"}, row.Offerings[0].Days)
	assert.Empty(t, row.EffectiveDays)
	assert.NotNil(t, row.EffectiveDays)
	assert.Equal(t, 0, row.DayCount)
}

func TestNormalizedCareUsageOfferingIDsDefaultsOnlyWhenSelectionIsMissing(t *testing.T) {
	offerings := []*enrollmentModels.CareOffering{
		{Model: baseModels.Model{ID: 1}, Name: "Ganztag", CountsAsCare: true},
		{Model: baseModels.Model{ID: 2}, Name: "Randstunde", CountsAsCare: false},
		{Model: baseModels.Model{ID: 3}, Name: "Kurzbetreuung", CountsAsCare: true},
	}

	assert.Equal(t, []int64{1, 3}, normalizedCareUsageOfferingIDs(nil, offerings, false))
	assert.Equal(t, []int64{}, normalizedCareUsageOfferingIDs(nil, offerings, true))
	assert.Equal(t, []int64{2}, normalizedCareUsageOfferingIDs([]int64{2, 2, -1}, offerings, true))
}

func TestCareUsageRowCarriesManualAndAutomaticDays(t *testing.T) {
	req := &enrollmentModels.Request{Model: baseModels.Model{ID: 10}}
	child := &enrollmentModels.RequestChild{
		Model:  baseModels.Model{ID: 20},
		Status: enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Randstunde",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
	}
	links := []*enrollmentModels.RequestChildOffering{
		{
			RequestChildID:        20,
			CareOfferingID:        1,
			SelectedDays:          []string{"mon", "tue", "wed", "thu", "fri"},
			ManualSelectedDays:    []string{"fri"},
			AutomaticSelectedDays: []string{"mon", "tue", "wed", "thu"},
		},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true})

	require.Len(t, row.Offerings, 1)
	assert.Equal(t, []string{"fri"}, row.Offerings[0].ManualSelectedDays)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, row.Offerings[0].AutomaticSelectedDays)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, row.Offerings[0].Days)
}

type fakeEducationGroupRepo struct {
	educationModels.GroupRepository
	groups  map[int64]*educationModels.Group
	seenIDs []int64
}

func (r *fakeEducationGroupRepo) FindByIDs(_ context.Context, ids []int64) (map[int64]*educationModels.Group, error) {
	r.seenIDs = append([]int64(nil), ids...)
	out := make(map[int64]*educationModels.Group, len(ids))
	for _, id := range ids {
		if group := r.groups[id]; group != nil {
			out[id] = group
		}
	}
	return out, nil
}
