package application

import (
	"testing"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// classRosterRowGroupID is the group a fixture student is placed in when a
// test names its group.
const classRosterRowGroupID = int64(9000)

// classRosterRow builds one roster row from single reads: the person, the
// group name, the approved enrollment, the offering catalog, the schemas, the
// student's guardian contacts and companion links, and the care offerings
// setting.
func classRosterRow(
	student *RosterStudent,
	person *RosterPerson,
	groupName string,
	approved *classRosterApprovedEnrollment,
	offeringByID map[int64]*offeringContract.CareOfferingRecord,
	schemas map[int64]*enrollmentNative.FormSchema,
	studentGuardians []enrollmentNative.ClassRosterGuardian,
	companions []CompanionLink,
	careOfferingsEnabled bool,
) (enrollmentNative.ClassRosterRow, error) {
	in := &classRosterInputs{
		students:            []*RosterStudent{student},
		studentGuardians:    map[int64][]enrollmentNative.ClassRosterGuardian{student.ID: studentGuardians},
		companions:          map[int64][]CompanionLink{student.ID: companions},
		persons:             map[int64]*RosterPerson{},
		enrollments:         map[int64]*classRosterApprovedEnrollment{},
		offeringByID:        offeringByID,
		schemas:             schemas,
		careOfferingsActive: careOfferingsEnabled,
	}
	if person != nil {
		in.persons[student.PersonID] = person
	}
	if groupName != "" {
		placed := *student
		groupID := classRosterRowGroupID
		placed.GroupID = &groupID
		student = &placed
		in.groups = map[int64]string{groupID: groupName}
	}
	if approved != nil {
		in.enrollments[student.ID] = approved
	}
	return in.row(student)
}

func TestClassRosterRowUsesPhaseEnrollmentData(t *testing.T) {
	t.Parallel()

	schemaID := int64(88)
	req := &Request{
		ID:          10,
		SchemaID:    &schemaID,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}
	child := &reportChild{
		ID:          20,
		RequestID:   10,
		FirstName:   "Lina",
		LastName:    "Muster",
		DateOfBirth: "2018-05-04",
		Status:      enrollmentNative.ChildStatusApproved,
		CustomData: map[string]any{
			"arrival": map[string]any{"mon": "11:30"},
			"pickup":  map[string]any{"mon": "14:30"},
			"departure": map[string]any{
				"mon": []any{"pickup", "accompanied"},
			},
			enrollmentNative.TargetStudentDepartureCompanionNote: "Mia",
		},
	}
	student := &RosterStudent{
		ID:          100,
		PersonID:    200,
		SchoolClass: "1a",
	}
	person := &RosterPerson{
		FirstName: "Lina",
		LastName:  "Muster",
	}
	approved := &classRosterApprovedEnrollment{
		request: req,
		child:   child,
		links: []*enrollmentNative.RequestChildOfferingRecord{
			{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon"}},
		},
	}
	offerings := map[int64]*offeringContract.CareOfferingRecord{
		1: {Name: "Randstunde", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice},
	}
	schemas := map[int64]*enrollmentNative.FormSchema{
		schemaID: {
			Fields: []enrollmentNative.FormField{
				{Key: "arrival", Target: enrollmentNative.TargetScheduleArrival, Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: true},
				{Key: "pickup", Target: enrollmentNative.TargetSchedulePickup, Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: true},
				{Key: "departure", Target: enrollmentNative.TargetStudentAllowedDepartureModes, Type: enrollmentNative.FormFieldWeekdayMultiMode, AppliesToCh: true},
			},
		},
	}

	row, err := classRosterRow(student, person, "Eulen", approved, offerings, schemas, nil, nil, true)

	require.NoError(t, err)
	assert.Equal(t, "Eulen", row.GroupName)
	assert.True(t, row.Registered)
	assert.Equal(t, "Angemeldet: Randstunde", row.EnrollmentSummary)
	assert.Equal(t, []string{"mon"}, row.CareDays)
	assert.Equal(t, []string{"Randstunde"}, row.OfferingsByDay["mon"])
	assert.Equal(t, "11:30", row.ArrivalByDay["mon"])
	assert.Equal(t, "14:30", row.PickupByDay["mon"])
	assert.Equal(t, "wird abgeholt / mit anderem Kind (mit: Mia)", row.DepartureByDay["mon"])
	assert.Equal(t, "geht alleine", row.DepartureByDay["tue"])
}

func TestClassRosterRowBuildsDailyOfferingNames(t *testing.T) {
	t.Parallel()

	req := &Request{
		ID:          13,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}
	child := &reportChild{
		ID:          23,
		RequestID:   13,
		DateOfBirth: "2018-05-04",
		Status:      enrollmentNative.ChildStatusApproved,
	}
	student := &RosterStudent{
		ID:          103,
		PersonID:    203,
		SchoolClass: "1a",
	}
	person := &RosterPerson{FirstName: "Lina", LastName: "Muster"}
	approved := &classRosterApprovedEnrollment{
		request: req,
		child:   child,
		links: []*enrollmentNative.RequestChildOfferingRecord{
			{RequestChildID: 23, CareOfferingID: 1, SelectedDays: []string{"mon", "wed"}},
			{RequestChildID: 23, CareOfferingID: 2, SelectedDays: []string{"wed", "fri"}},
		},
	}
	offerings := map[int64]*offeringContract.CareOfferingRecord{
		1: {Name: "Randstunde", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice},
		2: {Name: "Ganztag bis 16:00", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice},
	}

	row, err := classRosterRow(student, person, "", approved, offerings, nil, nil, nil, true)

	require.NoError(t, err)
	assert.Equal(t, []string{"mon", "wed", "fri"}, row.CareDays)
	assert.Equal(t, []string{"Randstunde"}, row.OfferingsByDay["mon"])
	assert.Equal(t, []string{"Ganztag bis 16:00", "Randstunde"}, row.OfferingsByDay["wed"])
	assert.Equal(t, []string{"Ganztag bis 16:00"}, row.OfferingsByDay["fri"])
	assert.Empty(t, row.OfferingsByDay["tue"])
	assert.Empty(t, row.OfferingsByDay["thu"])
}

func TestClassRosterRowReadsGuardianLevelSchedules(t *testing.T) {
	t.Parallel()

	schemaID := int64(90)
	req := &Request{
		ID:          12,
		SchemaID:    &schemaID,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
		CustomData: map[string]any{
			"guardian_arrival": map[string]any{"mon": "11:15"},
			"guardian_pickup":  map[string]any{"mon": "15:30"},
		},
	}
	child := &reportChild{
		ID:         22,
		RequestID:  12,
		FirstName:  "Lina",
		LastName:   "Muster",
		Status:     enrollmentNative.ChildStatusApproved,
		CustomData: map[string]any{},
	}
	student := &RosterStudent{
		ID:          102,
		PersonID:    202,
		SchoolClass: "1a",
	}
	person := &RosterPerson{FirstName: "Lina", LastName: "Muster"}
	approved := &classRosterApprovedEnrollment{
		request: req,
		child:   child,
	}
	schemas := map[int64]*enrollmentNative.FormSchema{
		schemaID: {
			Fields: []enrollmentNative.FormField{
				{Key: "guardian_arrival", Target: enrollmentNative.TargetScheduleArrival, Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: false},
				{Key: "guardian_pickup", Target: enrollmentNative.TargetSchedulePickup, Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: false},
			},
		},
	}

	row, err := classRosterRow(student, person, "Eulen", approved, nil, schemas, nil, nil, true)

	require.NoError(t, err)
	assert.Equal(t, "11:15", row.ArrivalByDay["mon"])
	assert.Equal(t, "15:30", row.PickupByDay["mon"])
}

func TestClassRosterRowTreatsAllWeekdaysAsCareDaysWithoutOfferings(t *testing.T) {
	t.Parallel()

	schemaID := int64(91)
	req := &Request{
		ID:          14,
		SchemaID:    &schemaID,
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}
	child := &reportChild{
		ID:         24,
		RequestID:  14,
		FirstName:  "Lina",
		LastName:   "Muster",
		Status:     enrollmentNative.ChildStatusApproved,
		CustomData: map[string]any{"pickup": map[string]any{"mon": "14:30", "thu": "15:00"}},
	}
	student := &RosterStudent{
		ID:          104,
		PersonID:    204,
		SchoolClass: "1a",
	}
	person := &RosterPerson{FirstName: "Lina", LastName: "Muster"}
	approved := &classRosterApprovedEnrollment{
		request: req,
		child:   child,
	}
	schemas := map[int64]*enrollmentNative.FormSchema{
		schemaID: {
			Fields: []enrollmentNative.FormField{
				{Key: "pickup", Target: enrollmentNative.TargetSchedulePickup, Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: true},
			},
		},
	}

	row, err := classRosterRow(student, person, "", approved, nil, schemas, nil, nil, true)

	require.NoError(t, err)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, row.CareDays)
	assert.Equal(t, "14:30", row.PickupByDay["mon"])
	assert.Equal(t, "15:00", row.PickupByDay["thu"])
	assert.Empty(t, row.PickupByDay["tue"])

	offerings := map[int64]*offeringContract.CareOfferingRecord{
		1: {Name: "Ganztag", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice, IsActive: true},
	}
	constrained, err := classRosterRow(student, person, "", approved, offerings, schemas, nil, nil, true)
	require.NoError(t, err)
	assert.Empty(t, constrained.CareDays)
	assert.Equal(t, "14:30", constrained.PickupByDay["mon"])

	inactiveOnly := map[int64]*offeringContract.CareOfferingRecord{
		1: {Name: "Ganztag", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice, IsActive: false},
	}
	unconstrained, err := classRosterRow(student, person, "", approved, inactiveOnly, schemas, nil, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, unconstrained.CareDays)
	assert.Equal(t, "14:30", unconstrained.PickupByDay["mon"])

	// The form does not load leftover catalog rows when offerings are off,
	// so pickup times without a booking stay unrestricted.
	disabledCatalog, err := classRosterRow(student, person, "", approved, offerings, schemas, nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, disabledCatalog.CareDays)
	assert.Equal(t, "14:30", disabledCatalog.PickupByDay["mon"])
}

func TestClassRosterRowMarksMissingEnrollmentAsNoRegistration(t *testing.T) {
	t.Parallel()

	student := &RosterStudent{
		ID:          101,
		PersonID:    201,
		SchoolClass: "1a",
	}
	person := &RosterPerson{FirstName: "Tom", LastName: "Ohne"}

	row, err := classRosterRow(student, person, "", nil, nil, nil, []enrollmentNative.ClassRosterGuardian{{Name: "Eva Ohne", Email: "eva@example.test", Phone: "02551 123"}}, nil, true)

	require.NoError(t, err)
	assert.False(t, row.Registered)
	assert.Equal(t, "Keine Anmeldung", row.EnrollmentSummary)
	assert.Equal(t, []string{}, row.CareDays)
	assert.Equal(t, map[string][]string{}, row.OfferingsByDay)
	assert.Equal(t, map[string]string{
		"mon": "geht alleine", "tue": "geht alleine", "wed": "geht alleine",
		"thu": "geht alleine", "fri": "geht alleine",
	}, row.DepartureByDay)
	assert.Equal(t, []enrollmentNative.ClassRosterGuardian{{Name: "Eva Ohne", Email: "eva@example.test", Phone: "02551 123"}}, row.Guardians)
}

func TestClassRosterRowWithoutContactsHasNoInventedContact(t *testing.T) {
	t.Parallel()

	student := &RosterStudent{
		ID:          101,
		PersonID:    201,
		SchoolClass: "1a",
	}
	person := &RosterPerson{FirstName: "Tom", LastName: "Ohne"}
	want := []enrollmentNative.ClassRosterGuardian{}

	t.Run("without enrollment", func(t *testing.T) {
		row, err := classRosterRow(student, person, "", nil, nil, nil, nil, nil, true)

		require.NoError(t, err)
		assert.False(t, row.Registered)
		assert.Equal(t, want, row.Guardians)
	})

	t.Run("with enrollment but without request guardians", func(t *testing.T) {
		req := &Request{ID: 10}
		child := &reportChild{
			ID:          20,
			RequestID:   10,
			DateOfBirth: "2018-05-04",
			Status:      enrollmentNative.ChildStatusApproved,
		}
		row, err := classRosterRow(student, person, "", &classRosterApprovedEnrollment{
			request: req,
			child:   child,
		}, nil, nil, nil, nil, true)

		require.NoError(t, err)
		assert.True(t, row.Registered)
		assert.Equal(t, want, row.Guardians)
	})
}

func TestClassRosterStudentGuardiansUseLinkedContacts(t *testing.T) {
	t.Parallel()

	linked := []enrollmentNative.ClassRosterGuardian{{
		Name:  "Stamm Kontakt",
		Email: "stamm@example.test",
		Phone: "02551 333",
	}}

	got := normalizeClassRosterGuardians(linked)

	assert.Equal(t, linked, got)
}

func TestClassRosterApprovedEnrollmentsOnlyUsesApprovedChildrenInClass(t *testing.T) {
	t.Parallel()

	studentID := int64(100)
	otherStudentID := int64(200)
	requestByID := map[int64]*Request{
		1: {ID: 1, SubmittedAt: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)},
		2: {ID: 2, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
	}
	studentByID := map[int64]*RosterStudent{
		studentID: {ID: studentID},
	}
	children := []*reportChild{
		{ID: 10, RequestID: 1, Status: enrollmentNative.ChildStatusApproved, CreatedStudentID: &studentID},
		{ID: 11, RequestID: 2, Status: enrollmentNative.ChildStatusRejected, CreatedStudentID: &studentID},
		{ID: 12, RequestID: 2, Status: enrollmentNative.ChildStatusApproved, CreatedStudentID: &otherStudentID},
	}

	got, childIDs := classRosterApprovedEnrollments(children, requestByID, studentByID)

	require.Len(t, got, 1)
	assert.Equal(t, int64(10), got[studentID].child.ID)
	assert.Equal(t, []int64{10}, childIDs)
}

func TestClassRosterApprovedEnrollmentsUsesOnlyNewestChildLinks(t *testing.T) {
	t.Parallel()

	studentID := int64(100)
	newestOfferingID := int64(20)
	requestByID := map[int64]*Request{
		1: {ID: 1, SubmittedAt: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)},
		2: {ID: 2, SubmittedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)},
	}
	studentByID := map[int64]*RosterStudent{
		studentID: {ID: studentID},
	}
	children := []*reportChild{
		{ID: 10, RequestID: 1, Status: enrollmentNative.ChildStatusApproved, CreatedStudentID: &studentID},
		{ID: 20, RequestID: 2, Status: enrollmentNative.ChildStatusApproved, CreatedStudentID: &studentID},
	}

	got, childIDs := classRosterApprovedEnrollments(children, requestByID, studentByID)
	classRosterAttachOfferingLinks(got, []*enrollmentNative.RequestChildOfferingRecord{
		{RequestChildID: 10, CareOfferingID: 1},
		{RequestChildID: 20, CareOfferingID: newestOfferingID},
	})

	require.Len(t, got, 1)
	assert.Equal(t, int64(20), got[studentID].child.ID)
	assert.Equal(t, []int64{20}, childIDs)
	require.Len(t, got[studentID].links, 1)
	assert.Equal(t, newestOfferingID, got[studentID].links[0].CareOfferingID)
}

func TestClassRosterCareDaysKeepsLeftoverBookingsWhenOfferingsDisabled(t *testing.T) {
	t.Parallel()

	got := classRosterCareDays(
		[]string{"mon", "wed"},
		map[int64]*offeringContract.CareOfferingRecord{
			1: {Name: "Ganztag", IsActive: true},
		},
		false,
	)
	assert.Equal(t, []string{"mon", "wed"}, got)
}

func TestClassRosterGroupNameResolvesAssignedGroup(t *testing.T) {
	t.Parallel()

	groupID := int64(12)
	student := &RosterStudent{GroupID: &groupID}
	groups := map[int64]string{
		groupID: "Klasse 2a",
	}

	assert.Equal(t, "Klasse 2a", classRosterGroupName(student, groups))
}
