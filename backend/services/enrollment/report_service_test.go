package enrollment

import (
	"context"
	"database/sql"
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

	row, err := classRosterRow(student, person, "Eulen", enrollment, offerings, schemas, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "Eulen", row.GroupName)
	assert.True(t, row.Registered)
	assert.Equal(t, "Angemeldet: Randstunde", row.EnrollmentSummary)
	assert.Equal(t, []string{"mon"}, row.CareDays)
	assert.Equal(t, []string{"Randstunde"}, row.OfferingsByDay["mon"])
	assert.Equal(t, "11:30", row.ArrivalByDay["mon"])
	assert.Equal(t, "14:30", row.PickupByDay["mon"])
	assert.Contains(t, row.Departure, "Mo: Abholung, Mit anderem Kind")
	assert.Contains(t, row.Departure, "(mit: Mia)")
}

func TestClassRosterRowBuildsDailyOfferingNames(t *testing.T) {
	req := &enrollmentModels.Request{
		Model:       baseModels.Model{ID: 13},
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}
	child := &enrollmentModels.RequestChild{
		Model:       baseModels.Model{ID: 23},
		RequestID:   13,
		DateOfBirth: timezone.NewDate(2018, 5, 4),
		Status:      enrollmentModels.ChildStatusApproved,
	}
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 103},
		PersonID:    203,
		SchoolClass: "1a",
	}
	person := &userModels.Person{FirstName: "Lina", LastName: "Muster"}
	enrollment := &classRosterApprovedEnrollment{
		request: req,
		child:   child,
		links: []*enrollmentModels.RequestChildOffering{
			{RequestChildID: 23, CareOfferingID: 1, SelectedDays: []string{"mon", "wed"}},
			{RequestChildID: 23, CareOfferingID: 2, SelectedDays: []string{"wed", "fri"}},
		},
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "Randstunde", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice},
		2: {Name: "Ganztag bis 16:00", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice},
	}

	row, err := classRosterRow(student, person, "", enrollment, offerings, nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"mon", "wed", "fri"}, row.CareDays)
	assert.Equal(t, []string{"Randstunde"}, row.OfferingsByDay["mon"])
	assert.Equal(t, []string{"Ganztag bis 16:00", "Randstunde"}, row.OfferingsByDay["wed"])
	assert.Equal(t, []string{"Ganztag bis 16:00"}, row.OfferingsByDay["fri"])
	assert.Empty(t, row.OfferingsByDay["tue"])
	assert.Empty(t, row.OfferingsByDay["thu"])
}

func TestCareUsageScheduleReadsGuardianLevelFields(t *testing.T) {
	schemaID := int64(89)
	req := &enrollmentModels.Request{
		Model: baseModels.Model{ID: 11},
		CustomData: map[string]any{
			"guardian_pickup":  map[string]any{"mon": "15:30"},
			"guardian_arrival": map[string]any{"mon": "11:15"},
		},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		SchemaID:          &schemaID,
	}
	child := &enrollmentModels.RequestChild{
		Model:      baseModels.Model{ID: 21},
		RequestID:  11,
		FirstName:  "Lina",
		LastName:   "Muster",
		Status:     enrollmentModels.ChildStatusApproved,
		CustomData: map[string]any{},
	}
	schemas := map[int64]*enrollmentModels.FormSchema{
		schemaID: {
			Fields: []enrollmentModels.FormField{
				{Key: "guardian_pickup", Target: enrollmentModels.TargetSchedulePickup, Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: false},
				{Key: "guardian_arrival", Target: enrollmentModels.TargetScheduleArrival, Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: false},
			},
		},
	}

	pickupByDay, err := careUsagePickupByDay(req, child, schemas)
	require.NoError(t, err)
	arrivalByDay, err := careUsageScheduleByTarget(req, child, schemas, enrollmentModels.TargetScheduleArrival)
	require.NoError(t, err)
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 21, CareOfferingID: 1, SelectedDays: []string{"mon"}},
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "Randstunde", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice},
	}
	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true}, pickupByDay)

	assert.Equal(t, "15:30", pickupByDay["mon"])
	assert.Equal(t, "11:15", arrivalByDay["mon"])
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", PickupTime: "15:30"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", PickupTime: "14:30"}))
}

func TestClassRosterRowReadsGuardianLevelSchedules(t *testing.T) {
	schemaID := int64(90)
	req := &enrollmentModels.Request{
		Model:       baseModels.Model{ID: 12},
		SchemaID:    &schemaID,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
		CustomData: map[string]any{
			"guardian_arrival": map[string]any{"mon": "11:15"},
			"guardian_pickup":  map[string]any{"mon": "15:30"},
		},
	}
	child := &enrollmentModels.RequestChild{
		Model:      baseModels.Model{ID: 22},
		RequestID:  12,
		FirstName:  "Lina",
		LastName:   "Muster",
		Status:     enrollmentModels.ChildStatusApproved,
		CustomData: map[string]any{},
	}
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 102},
		PersonID:    202,
		SchoolClass: "1a",
	}
	person := &userModels.Person{FirstName: "Lina", LastName: "Muster"}
	enrollment := &classRosterApprovedEnrollment{
		request: req,
		child:   child,
	}
	schemas := map[int64]*enrollmentModels.FormSchema{
		schemaID: {
			Fields: []enrollmentModels.FormField{
				{Key: "guardian_arrival", Target: enrollmentModels.TargetScheduleArrival, Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: false},
				{Key: "guardian_pickup", Target: enrollmentModels.TargetSchedulePickup, Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: false},
			},
		},
	}

	row, err := classRosterRow(student, person, "Eulen", enrollment, nil, schemas, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "11:15", row.ArrivalByDay["mon"])
	assert.Equal(t, "15:30", row.PickupByDay["mon"])
}

func TestClassRosterRowMarksMissingEnrollmentAsNoRegistration(t *testing.T) {
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 101},
		PersonID:    201,
		SchoolClass: "1a",
	}
	person := &userModels.Person{FirstName: "Tom", LastName: "Ohne"}

	row, err := classRosterRow(student, person, "", nil, nil, nil, []ClassRosterGuardian{{Name: "Eva Ohne", Email: "eva@example.test", Phone: "02551 123"}}, nil)

	require.NoError(t, err)
	assert.False(t, row.Registered)
	assert.Equal(t, "Keine Anmeldung", row.EnrollmentSummary)
	assert.Equal(t, []string{}, row.CareDays)
	assert.Equal(t, map[string][]string{}, row.OfferingsByDay)
	assert.Equal(t, "Geht alleine", row.Departure)
	assert.Equal(t, []ClassRosterGuardian{{Name: "Eva Ohne", Email: "eva@example.test", Phone: "02551 123"}}, row.Guardians)
}

func TestClassRosterRowFallsBackToLegacyStudentGuardianFields(t *testing.T) {
	guardianName := "Stamm Kontakt"
	guardianEmail := "stamm@example.test"
	guardianPhone := "02551 456"
	guardianContact := "0170 123456"
	student := &userModels.Student{
		Model:           baseModels.Model{ID: 101},
		PersonID:        201,
		SchoolClass:     "1a",
		GuardianName:    &guardianName,
		GuardianEmail:   &guardianEmail,
		GuardianPhone:   &guardianPhone,
		GuardianContact: &guardianContact,
	}
	person := &userModels.Person{FirstName: "Tom", LastName: "Ohne"}
	want := []ClassRosterGuardian{{
		Name:  "Stamm Kontakt",
		Email: "stamm@example.test",
		Phone: "02551 456; 0170 123456",
	}}

	t.Run("without enrollment", func(t *testing.T) {
		row, err := classRosterRow(student, person, "", nil, nil, nil, nil, nil)

		require.NoError(t, err)
		assert.False(t, row.Registered)
		assert.Equal(t, want, row.Guardians)
	})

	t.Run("with enrollment but without request guardians", func(t *testing.T) {
		req := &enrollmentModels.Request{Model: baseModels.Model{ID: 10}}
		child := &enrollmentModels.RequestChild{
			Model:       baseModels.Model{ID: 20},
			RequestID:   10,
			DateOfBirth: timezone.NewDate(2018, 5, 4),
			Status:      enrollmentModels.ChildStatusApproved,
		}
		row, err := classRosterRow(student, person, "", &classRosterApprovedEnrollment{
			request: req,
			child:   child,
		}, nil, nil, nil, nil)

		require.NoError(t, err)
		assert.True(t, row.Registered)
		assert.Equal(t, want, row.Guardians)
	})
}

func TestClassRosterStudentGuardiansPreferLinkedContactsOverLegacyFields(t *testing.T) {
	guardianName := "Stamm Kontakt"
	guardianEmail := "stamm@example.test"
	stalePhone := "02551 456"
	staleContact := "0170 123456"
	student := &userModels.Student{
		GuardianName:    &guardianName,
		GuardianEmail:   &guardianEmail,
		GuardianPhone:   &stalePhone,
		GuardianContact: &staleContact,
	}
	linked := []ClassRosterGuardian{{
		Name:  "Stamm Kontakt",
		Email: "stamm@example.test",
		Phone: "02551 333",
	}}

	got := classRosterStudentGuardians(student, linked)

	assert.Equal(t, linked, got)
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
	newestOfferingID := int64(20)
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
		{RequestChildID: 20, CareOfferingID: newestOfferingID},
	})

	require.Len(t, got, 1)
	assert.Equal(t, int64(20), got[studentID].child.ID)
	assert.Equal(t, []int64{20}, childIDs)
	require.Len(t, got[studentID].links, 1)
	assert.Equal(t, newestOfferingID, got[studentID].links[0].CareOfferingID)
}

func TestClassRosterScopesRequestLimitToSelectedClass(t *testing.T) {
	studentID := int64(100)
	personID := int64(200)
	requestID := int64(300)
	childID := int64(400)
	repo := &fakeClassRosterRequestRepo{
		requests: []*enrollmentModels.Request{
			{Model: baseModels.Model{ID: requestID}, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
		},
	}
	svc := classRosterTestService(
		[]*userModels.Student{{Model: baseModels.Model{ID: studentID}, PersonID: personID, SchoolClass: "1a"}},
		map[int64]*userModels.Person{personID: {FirstName: "Lina", LastName: "Muster"}},
		repo,
		&fakeClassRosterChildRepo{children: []*enrollmentModels.RequestChild{
			{Model: baseModels.Model{ID: childID}, RequestID: requestID, Status: enrollmentModels.ChildStatusApproved, CreatedStudentID: &studentID},
		}},
	)

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.Equal(t, []int64{studentID}, repo.seenFilters.CreatedStudentIDs)
	assert.Equal(t, 1, report.Totals.Registered)
}

func TestClassRosterAppliesChildLimitAfterClassFiltering(t *testing.T) {
	studentID := int64(100)
	otherStudentID := int64(200)
	personID := int64(300)
	requestID := int64(400)
	childID := int64(500)
	children := make([]*enrollmentModels.RequestChild, 0, maxReportRows+2)
	for i := 0; i <= maxReportRows; i++ {
		children = append(children, &enrollmentModels.RequestChild{
			Model:            baseModels.Model{ID: int64(1000 + i)},
			RequestID:        requestID,
			Status:           enrollmentModels.ChildStatusApproved,
			CreatedStudentID: &otherStudentID,
		})
	}
	children = append(children, &enrollmentModels.RequestChild{
		Model:            baseModels.Model{ID: childID},
		RequestID:        requestID,
		Status:           enrollmentModels.ChildStatusApproved,
		CreatedStudentID: &studentID,
	})
	svc := classRosterTestService(
		[]*userModels.Student{{Model: baseModels.Model{ID: studentID}, PersonID: personID, SchoolClass: "1a"}},
		map[int64]*userModels.Person{personID: {FirstName: "Lina", LastName: "Muster"}},
		&fakeClassRosterRequestRepo{requests: []*enrollmentModels.Request{
			{Model: baseModels.Model{ID: requestID}, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
		}},
		&fakeClassRosterChildRepo{children: children},
	)

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	assert.True(t, report.Rows[0].Registered)
}

func TestClassRosterLoadsGuardianContactsFromEnrollmentAndStudentFallback(t *testing.T) {
	registeredStudentID := int64(100)
	manualStudentID := int64(101)
	registeredPersonID := int64(200)
	manualPersonID := int64(201)
	requestID := int64(300)
	childID := int64(400)
	primaryPhone := "02551 111"
	additionalEmail := "zweiter@example.test"
	additionalPhone := "02551 222"
	students := []*userModels.Student{
		{Model: baseModels.Model{ID: registeredStudentID}, PersonID: registeredPersonID, SchoolClass: "1a"},
		{Model: baseModels.Model{ID: manualStudentID}, PersonID: manualPersonID, SchoolClass: "1a"},
	}
	svc := classRosterTestService(
		students,
		map[int64]*userModels.Person{
			registeredPersonID: {FirstName: "Lina", LastName: "Muster"},
			manualPersonID:     {FirstName: "Tom", LastName: "Ohne"},
		},
		&fakeClassRosterRequestRepo{requests: []*enrollmentModels.Request{{
			Model:             baseModels.Model{ID: requestID},
			GuardianFirstName: "Eva",
			GuardianLastName:  "Muster",
			GuardianEmail:     "eva@example.test",
			GuardianPhone:     &primaryPhone,
			SubmittedAt:       time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		}}},
		&fakeClassRosterChildRepo{children: []*enrollmentModels.RequestChild{{
			Model:            baseModels.Model{ID: childID},
			RequestID:        requestID,
			Status:           enrollmentModels.ChildStatusApproved,
			CreatedStudentID: &registeredStudentID,
		}}},
	)
	svc.RequestGuardianRepo = &fakeClassRosterRequestGuardianRepo{guardians: []*enrollmentModels.RequestGuardian{{
		Model:     baseModels.Model{ID: 500},
		RequestID: requestID,
		FirstName: "Zweiter",
		LastName:  "Kontakt",
		Email:     &additionalEmail,
		Phone:     &additionalPhone,
	}}}
	svc.StudentGuardianRepo = &fakeClassRosterStudentGuardianRepo{rows: []userModels.GuardianEmergencyContactRow{
		{
			StudentID:         manualStudentID,
			GuardianProfileID: 700,
			FirstName:         sql.NullString{String: "Stamm", Valid: true},
			LastName:          sql.NullString{String: "Kontakt", Valid: true},
			Email:             sql.NullString{String: "stamm@example.test", Valid: true},
			PhoneNumber:       sql.NullString{String: "02551 333", Valid: true},
		},
		{
			StudentID:         registeredStudentID,
			GuardianProfileID: 701,
			FirstName:         sql.NullString{String: "Nicht", Valid: true},
			LastName:          sql.NullString{String: "Verwenden", Valid: true},
			Email:             sql.NullString{String: "fallback@example.test", Valid: true},
			PhoneNumber:       sql.NullString{String: "02551 999", Valid: true},
		},
	}}

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})

	require.NoError(t, err)
	rowByFirstName := map[string]ClassRosterRow{}
	for _, row := range report.Rows {
		rowByFirstName[row.FirstName] = row
	}
	assert.Equal(t, []ClassRosterGuardian{
		{Name: "Eva Muster", Email: "eva@example.test", Phone: "02551 111"},
		{Name: "Zweiter Kontakt", Email: "zweiter@example.test", Phone: "02551 222"},
	}, rowByFirstName["Lina"].Guardians)
	assert.Equal(t, []ClassRosterGuardian{
		{Name: "Stamm Kontakt", Email: "stamm@example.test", Phone: "02551 333"},
	}, rowByFirstName["Tom"].Guardians)
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
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{EducationGroupRepo: repo}}

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

func TestCareUsageRowMatchesBookedPickupDayFilters(t *testing.T) {
	row := CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed"},
		PickupByDay:   map[string]string{"mon": "14:30", "wed": "16:00", "fri": "15:30"},
	}

	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "fri"}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", PickupTime: "15:30"}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "fri", PickupTime: "15:30"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "fri", PickupTime: "14:30"}))
}

func TestCareUsageRowMatchesWeekdayWithoutPickupTimes(t *testing.T) {
	row := CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon"},
		PickupByDay:   map[string]string{},
	}

	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "mon"}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "fri"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Weekday: "mon", PickupTime: "14:30"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", PickupTime: "14:30"}))
	assert.Empty(t, careUsageBookedPickupDays(row, CareUsageFilters{Status: "all"}))
}

func TestCareUsageRowMatchesScopedBookedPickupDayFilters(t *testing.T) {
	row := CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed"},
		PickupByDay:   map[string]string{"mon": "14:30", "wed": "16:00", "fri": "15:30"},
	}
	ogsOnly := CareUsageFilters{Status: "all", CareOfferingIDsSet: true, CareOfferingIDs: []int64{10}}
	randstundeOnly := CareUsageFilters{Status: "all", CareOfferingIDsSet: true, CareOfferingIDs: []int64{11}}

	assert.Equal(t, []string{"mon", "wed"}, careUsageBookedPickupDays(row, ogsOnly))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{10},
		Weekday:            "fri",
	}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{10},
		PickupTime:         "15:30",
	}))
	assert.Equal(t, []string{"fri"}, careUsageBookedPickupDays(row, randstundeOnly))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{11},
		Weekday:            "fri",
		PickupTime:         "15:30",
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

func TestCareUsageBookedPickupDaysIncludesNonCareOfferingDays(t *testing.T) {
	row := CareUsageRow{
		Offerings: []CareUsageRowOffering{
			{Name: "OGS Ganztag", Days: []string{"mon", "wed"}},
			{Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed"},
		PickupByDay:   map[string]string{"mon": "14:30", "wed": "16:00", "fri": "14:30"},
	}

	assert.Equal(t, []string{"mon", "wed", "fri"}, careUsageBookedPickupDays(row, CareUsageFilters{Status: "all"}))
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

func classRosterTestService(students []*userModels.Student, persons map[int64]*userModels.Person, requestRepo *fakeClassRosterRequestRepo, childRepo *fakeClassRosterChildRepo) *reportService {
	return &reportService{ReportServiceConfig: ReportServiceConfig{RequestRepo: requestRepo, RequestChildRepo: childRepo, RequestGuardianRepo: &fakeClassRosterRequestGuardianRepo{}, RequestChildOfferingRepo: &fakeClassRosterChildOfferingRepo{}, CareOfferingRepo: &fakeClassRosterCareOfferingRepo{}, PhaseRepo: &fakeClassRosterPhaseRepo{}, StudentRepo: &fakeClassRosterStudentRepo{students: students}, StudentGuardianRepo: &fakeClassRosterStudentGuardianRepo{}, PersonRepo: &fakeClassRosterPersonRepo{persons: persons}, EducationGroupRepo: &fakeEducationGroupRepo{}}}
}

type fakeClassRosterStudentRepo struct {
	userModels.StudentRepository
	students []*userModels.Student
}

func (r *fakeClassRosterStudentRepo) FindBySchoolClass(_ context.Context, _ string) ([]*userModels.Student, error) {
	return r.students, nil
}

type fakeClassRosterPersonRepo struct {
	userModels.PersonRepository
	persons map[int64]*userModels.Person
}

func (r *fakeClassRosterPersonRepo) FindByIDs(_ context.Context, ids []int64) (map[int64]*userModels.Person, error) {
	out := make(map[int64]*userModels.Person, len(ids))
	for _, id := range ids {
		if person := r.persons[id]; person != nil {
			out[id] = person
		}
	}
	return out, nil
}

type fakeClassRosterRequestRepo struct {
	enrollmentModels.RequestRepository
	requests    []*enrollmentModels.Request
	seenFilters enrollmentModels.RequestListFilters
}

func (r *fakeClassRosterRequestRepo) ListAdmin(_ context.Context, filters enrollmentModels.RequestListFilters) ([]*enrollmentModels.Request, error) {
	r.seenFilters = filters
	if len(filters.CreatedStudentIDs) == 0 {
		return make([]*enrollmentModels.Request, maxExportRequests+1), nil
	}
	return r.requests, nil
}

type fakeClassRosterChildRepo struct {
	enrollmentModels.RequestChildRepository
	children []*enrollmentModels.RequestChild
}

func (r *fakeClassRosterChildRepo) ListByRequestIDs(_ context.Context, _ []int64) ([]*enrollmentModels.RequestChild, error) {
	return r.children, nil
}

type fakeClassRosterRequestGuardianRepo struct {
	enrollmentModels.RequestGuardianRepository
	guardians []*enrollmentModels.RequestGuardian
}

func (r *fakeClassRosterRequestGuardianRepo) ListByRequestIDs(_ context.Context, requestIDs []int64) ([]*enrollmentModels.RequestGuardian, error) {
	seen := map[int64]bool{}
	for _, id := range requestIDs {
		seen[id] = true
	}
	out := make([]*enrollmentModels.RequestGuardian, 0, len(r.guardians))
	for _, guardian := range r.guardians {
		if guardian != nil && seen[guardian.RequestID] {
			out = append(out, guardian)
		}
	}
	return out, nil
}

type fakeClassRosterChildOfferingRepo struct {
	enrollmentModels.RequestChildOfferingRepository
}

func (r *fakeClassRosterChildOfferingRepo) ListByRequestChildIDs(_ context.Context, _ []int64) ([]*enrollmentModels.RequestChildOffering, error) {
	return nil, nil
}

func (r *fakeClassRosterChildOfferingRepo) ListByRequestChildIDsAtDate(_ context.Context, _ []int64, _ timezone.Date) ([]*enrollmentModels.RequestChildOffering, error) {
	return nil, nil
}

type fakeClassRosterCareOfferingRepo struct {
	enrollmentModels.CareOfferingRepository
}

func (r *fakeClassRosterCareOfferingRepo) ListByPhase(_ context.Context, _ int64) ([]*enrollmentModels.CareOffering, error) {
	return nil, nil
}

type fakeClassRosterStudentGuardianRepo struct {
	userModels.StudentGuardianRepository
	rows []userModels.GuardianEmergencyContactRow
}

func (r *fakeClassRosterStudentGuardianRepo) ListEmergencyContactRows(_ context.Context, studentIDs []int64) ([]userModels.GuardianEmergencyContactRow, error) {
	seen := map[int64]bool{}
	for _, id := range studentIDs {
		seen[id] = true
	}
	out := make([]userModels.GuardianEmergencyContactRow, 0, len(r.rows))
	for _, row := range r.rows {
		if seen[row.StudentID] {
			out = append(out, row)
		}
	}
	return out, nil
}

type fakeClassRosterPhaseRepo struct {
	enrollmentModels.PhaseRepository
}

func (r *fakeClassRosterPhaseRepo) FindByID(_ context.Context, id int64) (*enrollmentModels.Phase, error) {
	return &enrollmentModels.Phase{Model: baseModels.Model{ID: id}, Name: "Schuljahr 2026"}, nil
}

func TestSortClassRosterRowsGermanNameOrder(t *testing.T) {
	rows := []ClassRosterRow{
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

// TestClassRosterRowNamesStructuredCompanions pins that the class roster prints
// WHO an accompanied child walks home with. The structured links satisfy the
// "mit wem" requirement per weekday, so such a child legitimately has no
// free-text note — a roster built from the note alone would hand staff a sheet
// that says "Mit anderem Kind" and no name (#1694).
func TestClassRosterRowNamesStructuredCompanions(t *testing.T) {
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 101},
		PersonID:    201,
		SchoolClass: "1a",
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			userModels.PickupDayMonday: []userModels.DepartureMode{userModels.DepartureAccompanied},
		},
	}
	person := &userModels.Person{FirstName: "Lina", LastName: "Läuft"}
	companions := []userModels.CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{userModels.PickupDayMonday}},
	}

	row, err := classRosterRow(student, person, "", nil, nil, nil, nil, companions)

	require.NoError(t, err)
	assert.Equal(t, "Mo: Mit anderem Kind (mit: Mia Schulz (Mo))", row.Departure)
}

// TestClassRosterRowCombinesCompanionsAndNote pins that a child using both
// sources keeps both: the link answers Monday, the note answers the day no link
// covers.
func TestClassRosterRowCombinesCompanionsAndNote(t *testing.T) {
	note := "Dienstags mit Nachbarskind Tom"
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 101},
		PersonID:    201,
		SchoolClass: "1a",
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			userModels.PickupDayMonday:  []userModels.DepartureMode{userModels.DepartureAccompanied},
			userModels.PickupDayTuesday: []userModels.DepartureMode{userModels.DepartureAccompanied},
		},
		DepartureCompanionNote: &note,
	}
	person := &userModels.Person{FirstName: "Lina", LastName: "Läuft"}
	companions := []userModels.CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{userModels.PickupDayMonday}},
	}

	row, err := classRosterRow(student, person, "", nil, nil, nil, nil, companions)

	require.NoError(t, err)
	assert.Equal(t, "Mo: Mit anderem Kind, Di: Mit anderem Kind (mit: Mia Schulz (Mo); Dienstags mit Nachbarskind Tom)", row.Departure)
}

// TestClassRosterRowDropsCompanionDaysOutsideThePhasePlan pins that the roster
// never prints a "läuft mit" day the plan on the same line forbids. The plan
// comes from the approved enrollment phase while the links belong to the live
// child and follow every later Stammdaten edit, so the two sources can disagree
// — and a sheet that says "Di: Bus" and "dienstags mit Mia" at once is worse
// than one that says nothing about Tuesday (#1694).
func TestClassRosterRowDropsCompanionDaysOutsideThePhasePlan(t *testing.T) {
	schemaID := int64(88)
	req := &enrollmentModels.Request{
		Model:       baseModels.Model{ID: 10},
		SchemaID:    &schemaID,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}
	child := &enrollmentModels.RequestChild{
		Model:     baseModels.Model{ID: 20},
		RequestID: 10,
		FirstName: "Lina",
		LastName:  "Muster",
		Status:    enrollmentModels.ChildStatusApproved,
		CustomData: map[string]any{
			"departure": map[string]any{
				"mon": []any{"accompanied"},
				"tue": []any{"bus"},
			},
		},
	}
	student := &userModels.Student{
		Model:       baseModels.Model{ID: 100},
		PersonID:    200,
		SchoolClass: "1a",
	}
	person := &userModels.Person{FirstName: "Lina", LastName: "Muster"}
	enrollment := &classRosterApprovedEnrollment{request: req, child: child}
	schemas := map[int64]*enrollmentModels.FormSchema{
		schemaID: {
			Fields: []enrollmentModels.FormField{
				{Key: "departure", Target: enrollmentModels.TargetStudentAllowedDepartureModes, Type: enrollmentModels.FormFieldWeekdayMultiMode, AppliesToCh: true},
			},
		},
	}
	companions := []userModels.CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{userModels.PickupDayMonday, userModels.PickupDayTuesday}},
	}

	row, err := classRosterRow(student, person, "", enrollment, nil, schemas, nil, companions)

	require.NoError(t, err)
	assert.Equal(t, "Mo: Mit anderem Kind, Di: Bus (mit: Mia Schulz (Mo))", row.Departure)
}
