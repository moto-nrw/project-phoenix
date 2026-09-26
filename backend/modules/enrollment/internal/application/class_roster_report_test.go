package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestClassRosterIgnoresLeftoverCatalogWhenOfferingsDisabled(t *testing.T) {
	t.Parallel()

	studentID := int64(100)
	personID := int64(200)
	requestID := int64(300)
	childID := int64(400)
	svc := classRosterTestService(
		[]*RosterStudent{{ID: studentID, PersonID: personID, SchoolClass: "1a"}},
		map[int64]*RosterPerson{personID: {FirstName: "Lina", LastName: "Muster"}},
		&fakeClassRosterRequestRepo{requests: []*enrollmentModels.Request{
			{ID: requestID, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
		}},
		&fakeClassRosterChildRepo{children: []*reportChild{
			{ID: childID, RequestID: requestID, Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &studentID},
		}},
	)
	svc.deps.Offerings = &fakeClassRosterCareOfferingRepo{offerings: []*enrollmentModels.CareOffering{
		{ID: 1, Name: "Ganztag", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice, IsActive: true},
	}}
	svc.deps.Settings = classRosterSettingsStub{careOfferingsEnabled: false}

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, report.Rows[0].CareDays)
}

func TestClassRosterParticipationUsesOneTodaySnapshot(t *testing.T) {
	t.Parallel()
	student := &RosterStudent{ID: 43, SchoolClass: "2b"}
	recorder := &recordingClassCareParticipation{}
	calls := 0
	svc := NewReports(ReportDependencies{
		Students:          &fakeClassRosterStudentRepo{students: []*RosterStudent{student}},
		CareParticipation: recorder,
		Now: func() time.Time {
			calls++
			return time.Date(2026, 8, 25, 23, 59, 59, 0, calendar.Berlin)
		},
	})

	students, err := svc.classRosterStudents(context.Background(), enrollment.ClassRosterFilters{SchoolClass: "2b"})

	require.NoError(t, err)
	require.Len(t, students, 1)
	assert.Equal(t, 1, calls)
	assert.Equal(t, calendar.NewDate(2026, 8, 25), recorder.on)
	assert.Equal(t, recorder.on, recorder.today)
}

func TestClassRosterAppliesBookingParticipationBoundary(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 8, 24)
	// Student 41's care ended yesterday, student 42 is active: the care
	// participation resolver alone decides who belongs to the roster.
	students := []*RosterStudent{
		{ID: 41, PersonID: 141, SchoolClass: "2a"},
		{ID: 42, PersonID: 142, SchoolClass: "2a"},
	}
	svc := classRosterTestService(students, map[int64]*RosterPerson{
		141: {FirstName: "Vor", LastName: "Lücke"},
		142: {FirstName: "Ab", LastName: "Lücke"},
	}, &fakeClassRosterRequestRepo{}, &fakeClassRosterChildRepo{})
	svc.deps.CareParticipation = fakeClassCareParticipation{41: true}
	svc.deps.Now = func() time.Time { return today.BerlinMidnight().Add(12 * time.Hour) }

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "2a"})
	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.Equal(t, int64(41), report.Rows[0].StudentID)
}

func TestClassRosterFailsWhenCareOfferingsSettingCannotBeResolved(t *testing.T) {
	t.Parallel()

	svc := classRosterTestService(
		[]*RosterStudent{{ID: 100, PersonID: 200, SchoolClass: "1a"}},
		map[int64]*RosterPerson{200: {FirstName: "Lina", LastName: "Muster"}},
		&fakeClassRosterRequestRepo{},
		&fakeClassRosterChildRepo{},
	)
	svc.deps.Settings = classRosterSettingsStub{err: errors.New("settings unavailable")}

	_, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment.care_offerings_enabled")
}

func TestClassRosterScopesRequestLimitToSelectedClass(t *testing.T) {
	t.Parallel()

	studentID := int64(100)
	personID := int64(200)
	requestID := int64(300)
	childID := int64(400)
	repo := &fakeClassRosterRequestRepo{
		requests: []*enrollmentModels.Request{
			{ID: requestID, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
		},
	}
	svc := classRosterTestService(
		[]*RosterStudent{{ID: studentID, PersonID: personID, SchoolClass: "1a"}},
		map[int64]*RosterPerson{personID: {FirstName: "Lina", LastName: "Muster"}},
		repo,
		&fakeClassRosterChildRepo{children: []*reportChild{
			{ID: childID, RequestID: requestID, Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &studentID},
		}},
	)

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.Equal(t, []int64{studentID}, repo.seenFilters.CreatedStudentIDs)
	assert.Equal(t, 1, report.Totals.Registered)
}

func TestClassRosterAppliesChildLimitAfterClassFiltering(t *testing.T) {
	t.Parallel()

	studentID := int64(100)
	otherStudentID := int64(200)
	personID := int64(300)
	requestID := int64(400)
	childID := int64(500)
	children := make([]*reportChild, 0, maxReportRows+2)
	for i := 0; i <= maxReportRows; i++ {
		children = append(children, &reportChild{
			ID:               int64(1000 + i),
			RequestID:        requestID,
			Status:           enrollmentModels.ChildStatusApproved,
			CreatedStudentID: &otherStudentID,
		})
	}
	children = append(children, &reportChild{
		ID:               childID,
		RequestID:        requestID,
		Status:           enrollmentModels.ChildStatusApproved,
		CreatedStudentID: &studentID,
	})
	svc := classRosterTestService(
		[]*RosterStudent{{ID: studentID, PersonID: personID, SchoolClass: "1a"}},
		map[int64]*RosterPerson{personID: {FirstName: "Lina", LastName: "Muster"}},
		&fakeClassRosterRequestRepo{requests: []*enrollmentModels.Request{
			{ID: requestID, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
		}},
		&fakeClassRosterChildRepo{children: children},
	)

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.True(t, report.Rows[0].Registered)
}

func TestClassRosterAppliesStudentLimitAfterParticipationFiltering(t *testing.T) {
	t.Parallel()

	students := make([]*RosterStudent, 0, maxReportRows+1)
	participating := fakeClassCareParticipation{1: true}
	for id := 1; id <= maxReportRows+1; id++ {
		students = append(students, &RosterStudent{
			ID: int64(id), SchoolClass: "1a",
		})
	}
	svc := NewReports(ReportDependencies{
		Students:          &fakeClassRosterStudentRepo{students: students},
		CareParticipation: participating,
	})

	result, err := svc.classRosterStudents(context.Background(), enrollment.ClassRosterFilters{AllClasses: true})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, students[0].ID, result[0].ID)
}

func TestClassRosterLoadsGuardianContactsFromEnrollmentAndStudentFallback(t *testing.T) {
	t.Parallel()

	registeredStudentID := int64(100)
	manualStudentID := int64(101)
	registeredPersonID := int64(200)
	manualPersonID := int64(201)
	requestID := int64(300)
	childID := int64(400)
	primaryPhone := "02551 111"
	additionalEmail := "zweiter@example.test"
	additionalPhone := "02551 222"
	students := []*RosterStudent{
		{ID: registeredStudentID, PersonID: registeredPersonID, SchoolClass: "1a"},
		{ID: manualStudentID, PersonID: manualPersonID, SchoolClass: "1a"},
	}
	svc := classRosterTestService(
		students,
		map[int64]*RosterPerson{
			registeredPersonID: {FirstName: "Lina", LastName: "Muster"},
			manualPersonID:     {FirstName: "Tom", LastName: "Ohne"},
		},
		&fakeClassRosterRequestRepo{requests: []*enrollmentModels.Request{{
			ID:                requestID,
			GuardianFirstName: "Eva",
			GuardianLastName:  "Muster",
			GuardianEmail:     "eva@example.test",
			GuardianPhone:     &primaryPhone,
			SubmittedAt:       time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		}}},
		&fakeClassRosterChildRepo{children: []*reportChild{{
			ID:               childID,
			RequestID:        requestID,
			Status:           enrollmentModels.ChildStatusApproved,
			CreatedStudentID: &registeredStudentID,
		}}},
	)
	svc.deps.Guardians = &fakeClassRosterRequestGuardianRepo{guardians: []*enrollment.RequestGuardian{{
		ID:        500,
		RequestID: requestID,
		FirstName: "Zweiter",
		LastName:  "Kontakt",
		Email:     &additionalEmail,
		Phone:     &additionalPhone,
	}}}
	svc.deps.GuardianContacts = &fakeClassRosterStudentGuardianRepo{rows: []GuardianContactRow{
		{
			StudentID:         manualStudentID,
			GuardianProfileID: 700,
			FirstName:         "Stamm",
			LastName:          "Kontakt",
			Email:             "stamm@example.test",
			PhoneNumber:       "02551 333",
		},
		{
			StudentID:         registeredStudentID,
			GuardianProfileID: 701,
			FirstName:         "Nicht",
			LastName:          "Verwenden",
			Email:             "fallback@example.test",
			PhoneNumber:       "02551 999",
		},
	}}

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.NoError(t, err)
	rowByFirstName := map[string]enrollment.ClassRosterRow{}
	for _, row := range report.Rows {
		rowByFirstName[row.FirstName] = row
	}
	assert.Equal(t, []enrollment.ClassRosterGuardian{
		{Name: "Eva Muster", Email: "eva@example.test", Phone: "02551 111"},
		{Name: "Zweiter Kontakt", Email: "zweiter@example.test", Phone: "02551 222"},
	}, rowByFirstName["Lina"].Guardians)
	assert.Equal(t, []enrollment.ClassRosterGuardian{
		{Name: "Stamm Kontakt", Email: "stamm@example.test", Phone: "02551 333"},
	}, rowByFirstName["Tom"].Guardians)
}

func TestReportServiceClassRosterGroupNamesLoadsUniqueGroups(t *testing.T) {
	t.Parallel()

	groupID := int64(12)
	otherGroupID := int64(13)
	repo := &fakeEducationGroupRepo{groups: map[int64]string{
		groupID:      "Klasse 2a",
		otherGroupID: "Klasse 3b",
	}}
	svc := NewReports(ReportDependencies{Groups: repo, Persons: &fakeClassRosterPersonRepo{}})
	in := &classRosterInputs{students: []*RosterStudent{
		{GroupID: &groupID},
		{GroupID: &groupID},
		{GroupID: &otherGroupID},
		{},
	}}

	err := svc.loadClassRosterStudentReads(context.Background(), enrollment.ClassRosterFilters{SkipGuardianData: true}, in, nil)

	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{groupID, otherGroupID}, repo.seenIDs)
	assert.Equal(t, "Klasse 2a", in.groups[groupID])
}

func TestClassRosterUsesOfferingDateForPickupProjection(t *testing.T) {
	t.Parallel()

	studentID := int64(100)
	pickupSvc := &fakeCareUsagePickupScheduleSvc{}
	svc := classRosterTestService(
		[]*RosterStudent{{ID: studentID, PersonID: 200, SchoolClass: "1a"}},
		map[int64]*RosterPerson{200: {FirstName: "Lina", LastName: "Muster"}},
		&fakeClassRosterRequestRepo{},
		&fakeClassRosterChildRepo{},
	)
	svc.deps.PickupSchedules = pickupSvc
	reportDate := calendar.NewDate(2026, 8, 24).AddDays(30)

	_, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{
		PhaseID: 55, SchoolClass: "1a", OfferingDate: &reportDate,
	})

	require.NoError(t, err)
	assert.Equal(t, reportDate, pickupSvc.date)
	assert.Equal(t, []int64{studentID}, pickupSvc.studentIDs)
}

func TestSortClassRosterRowsGermanNameOrder(t *testing.T) {
	t.Parallel()

	rows := []enrollment.ClassRosterRow{
		{StudentID: 1, FirstName: "Jan", LastName: "Zimmermann"},
		{StudentID: 2, FirstName: "Emre", LastName: "Özdemir"},
		{StudentID: 3, FirstName: "Lena", LastName: "Ärmel"},
		{StudentID: 5, FirstName: "Anna", LastName: "Müller"},
		{StudentID: 4, FirstName: "Anna", LastName: "Müller"},
		{StudentID: 6, FirstName: "Tim", LastName: "Mueller"},
	}

	sortClassRosterRows(rows)

	wantIDs := []int64{3, 6, 4, 5, 2, 1}
	gotIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		gotIDs = append(gotIDs, row.StudentID)
	}
	assert.Equal(t, wantIDs, gotIDs, "expected Ärmel, Mueller, Müller (ID tiebreak), Özdemir, Zimmermann")
}

// allClassesTestService serves per-class student sets in byte order, so
// "10a" comes before "1a" here — the report must re-sort.
func allClassesTestService() *Reports {
	students := []*RosterStudent{
		{ID: 3, PersonID: 13, SchoolClass: "10a"},
		{ID: 2, PersonID: 12, SchoolClass: "1a"},
		{ID: 1, PersonID: 11, SchoolClass: "1a"},
		{ID: 4, PersonID: 14, SchoolClass: "2b"},
	}
	persons := map[int64]*RosterPerson{
		11: {FirstName: "Mila", LastName: "Anders"},
		12: {FirstName: "Finn", LastName: "Becker"},
		13: {FirstName: "Ida", LastName: "Conrad"},
		14: {FirstName: "Emma", LastName: "Dreyer"},
	}
	return classRosterTestService(students, persons, &fakeClassRosterRequestRepo{}, &fakeClassRosterChildRepo{})
}

func TestClassRosterAllClassesSortsClassFirstThenName(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, AllClasses: true})

	require.NoError(t, err)
	require.Len(t, report.Rows, 4)
	got := make([][2]string, 0, len(report.Rows))
	for _, row := range report.Rows {
		got = append(got, [2]string{row.SchoolClass, row.LastName})
	}
	assert.Equal(t, [][2]string{
		{"1a", "Anders"},
		{"1a", "Becker"},
		{"2b", "Dreyer"},
		{"10a", "Conrad"},
	}, got)
	assert.True(t, report.Filters.AllClasses)
	assert.Equal(t, 4, report.Totals.Students)
}

// TestClassRosterAllClassesDeduplicatesCaseVariantClasses ensures one bulk
// read cannot duplicate a student merely because class labels differ in case.
func TestClassRosterAllClassesDeduplicatesCaseVariantClasses(t *testing.T) {
	t.Parallel()

	persons := map[int64]*RosterPerson{
		11: {FirstName: "Mila", LastName: "Anders"},
		12: {FirstName: "Finn", LastName: "Becker"},
		13: {FirstName: "Ida", LastName: "Conrad"},
	}
	svc := classRosterTestService(nil, persons, &fakeClassRosterRequestRepo{}, &fakeClassRosterChildRepo{})
	repo := &fakeClassRosterStudentRepo{
		students: []*RosterStudent{
			{ID: 1, PersonID: 11, SchoolClass: "1a"},
			{ID: 2, PersonID: 12, SchoolClass: "1A"},
			{ID: 3, PersonID: 13, SchoolClass: "2b"},
		},
	}
	svc.deps.Students = repo

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, AllClasses: true})

	require.NoError(t, err)
	require.Len(t, report.Rows, 3)
	assert.Equal(t, 3, report.Totals.Students)
	assert.Equal(t, 1, repo.listCalls, "all classes load in one query")
	seen := map[int64]bool{}
	for _, row := range report.Rows {
		assert.False(t, seen[row.StudentID], "student %d appears more than once", row.StudentID)
		seen[row.StudentID] = true
	}
}

func TestClassRosterFilterValidation(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()

	_, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a", AllClasses: true})
	require.ErrorIs(t, err, enrollment.ErrReportInvalidFilter)

	_, err = svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55})
	require.ErrorIs(t, err, enrollment.ErrReportInvalidFilter)
}

func TestClassRosterMergesClassListEntries(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()
	svc.deps.ClassListEntries = &fakeClassListEntries{entries: []ClassListEntry{
		classListEntry(101, "Zoe", "Aalders", "1a"),
		classListEntry(102, "Ben", "Zorn", "3c"),
	}}

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, AllClasses: true})
	require.NoError(t, err)

	// 4 students + 2 entries; the 3c entry creates its own class section even
	// though no student carries that class.
	require.Len(t, report.Rows, 6)
	assert.Equal(t, 6, report.Totals.Students)
	assert.Equal(t, 2, report.Totals.ListEntries)

	got := make([][2]string, 0, len(report.Rows))
	for _, row := range report.Rows {
		got = append(got, [2]string{row.SchoolClass, row.LastName})
	}
	assert.Equal(t, [][2]string{
		{"1a", "Aalders"}, // list entry, alphabetically first in 1a
		{"1a", "Anders"},
		{"1a", "Becker"},
		{"2b", "Dreyer"},
		{"3c", "Zorn"}, // entry-only class gets its own section
		{"10a", "Conrad"},
	}, got)

	for _, row := range report.Rows {
		if row.ListEntry {
			assert.Zero(t, row.StudentID)
			assert.False(t, row.Registered)
			assert.Equal(t, enrollment.ClassListEntryNoCareLabel, row.EnrollmentSummary)
			assert.Empty(t, row.CareDays)
		}
	}
}

func TestClassRosterSingleClassMergesOnlyThatClass(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()
	svc.deps.ClassListEntries = &fakeClassListEntries{entries: []ClassListEntry{
		classListEntry(101, "Zoe", "Aalders", "1a"),
		classListEntry(102, "Ben", "Zorn", "3c"),
	}}

	report, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})
	require.NoError(t, err)

	require.Len(t, report.Rows, 3)
	assert.Equal(t, 1, report.Totals.ListEntries)
	assert.Equal(t, "Aalders", report.Rows[0].LastName)
	assert.True(t, report.Rows[0].ListEntry)
}

// Klassen-Rosters nach dem Ende einer Betreuung (#2487): the roster date
// decides which children still belong to the class.
func TestClassRosterFiltersCareDate(t *testing.T) {
	t.Parallel()

	t.Run("defaults to today", func(t *testing.T) {
		today := calendar.NewDate(2026, 8, 24)
		assert.Equal(t, today, classRosterCareDate(enrollment.ClassRosterFilters{}, today))
	})

	t.Run("follows the day the class view is paged to", func(t *testing.T) {
		// The Lehrkraft class day view pages through the week. A sheet for
		// last Tuesday must show who was in care THEN, not who is today.
		paged := calendar.NewDate(2026, 5, 12)
		assert.Equal(t, paged, classRosterCareDate(enrollment.ClassRosterFilters{OfferingDate: &paged}, calendar.NewDate(2026, 8, 24)))
	})
}

// capturingChildOfferingReader records the date the offering links were
// requested for.
type capturingChildOfferingReader struct {
	ReportChildren
	seenDate calendar.Date
}

func (r *capturingChildOfferingReader) RequestChildOfferingsForChildrenAtDate(_ context.Context, _ []int64, date enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	r.seenDate = calendar.Date(date)
	return nil, nil
}

func TestClassRosterOfferingDatePinsSelection(t *testing.T) {
	t.Parallel()

	svc := classRosterTestService(
		[]*RosterStudent{{ID: 1, PersonID: 11, SchoolClass: "1a"}},
		map[int64]*RosterPerson{11: {FirstName: "Mila", LastName: "Anders"}},
		&fakeClassRosterRequestRepo{},
		&fakeClassRosterChildRepo{},
	)
	capture := &capturingChildOfferingReader{ReportChildren: svc.deps.Children}
	svc.deps.Children = capture
	pinned := calendar.NewDate(2026, 8, 10)

	_, err := svc.classRoster(context.Background(), enrollment.ClassRosterFilters{PhaseID: 55, SchoolClass: "1a", OfferingDate: &pinned})

	require.NoError(t, err)
	assert.Equal(t, pinned, capture.seenDate)
}
