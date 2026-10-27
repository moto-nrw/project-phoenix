package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Delivery's staff recipient resolver takes the staff on duty from the
// root's binding, which applies Workforce's rule (#4072).

type fixedWorkPresence struct {
	presence map[int64]string
	err      error
}

func (p fixedWorkPresence) WorkPresenceMap(context.Context) (map[int64]string, error) {
	return p.presence, p.err
}

func TestDeliveryOnDutyStaffAppliesTheWorkforceRule(t *testing.T) {
	t.Parallel()

	onDuty, err := deliveryOnDutyStaff{presence: fixedWorkPresence{presence: map[int64]string{
		7: workforce.WorkSessionStatusPresent,
		8: workforce.WorkSessionStatusHomeOffice,
		9: "checked_out",
	}}}.OnDutyStaffIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 8}, onDuty)

	failure := errors.New("presence unavailable")
	_, err = deliveryOnDutyStaff{presence: fixedWorkPresence{err: failure}}.OnDutyStaffIDs(context.Background())
	assert.ErrorIs(t, err, failure)
}

// The moto schule notice of an absence (#4792) reaches the Lehrkräfte of the
// child's own class only. The binding reads every class's assignments at
// once and groups them back the way School Membership compares classes.

type fixedStudentClasses map[int64]string

func (f fixedStudentClasses) FindReadScopeByIDs(_ context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error) {
	out := make(map[int64]*peopleValues.StudentState, len(ids))
	for _, id := range ids {
		if schoolClass, ok := f[id]; ok {
			out[id] = &peopleValues.StudentState{ID: id, SchoolClass: schoolClass}
		}
	}
	return out, nil
}

type recordedClassAssignments struct {
	rows    []schoolmembership.ClassAssignment
	filters []schoolmembership.ClassAssignmentFilter
}

func (r *recordedClassAssignments) ListClassAssignments(_ context.Context, filter schoolmembership.ClassAssignmentFilter) ([]schoolmembership.ClassAssignment, error) {
	r.filters = append(r.filters, filter)
	return r.rows, nil
}

type fixedStaffAccounts map[int64]int64

func (f fixedStaffAccounts) ListAccountIDsByStaffIDs(_ context.Context, staffIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(staffIDs))
	for _, staffID := range staffIDs {
		if accountID, ok := f[staffID]; ok {
			out[staffID] = accountID
		}
	}
	return out, nil
}

func TestDeliverySchoolClassReachNamesTheTeachersOfTheChildsClass(t *testing.T) {
	t.Parallel()

	assignments := &recordedClassAssignments{rows: []schoolmembership.ClassAssignment{
		{StaffID: 1, SchoolClass: "klasse 1a"},
		{StaffID: 2, SchoolClass: "Klasse 2a"},
		{StaffID: 3, SchoolClass: "Klasse 1a"}, // no account: not addressable
	}}
	reach := deliverySchoolClassReach{
		students:    fixedStudentClasses{10: " Klasse 1a ", 11: "Klasse 2a", 12: ""},
		assignments: assignments,
		staff:       fixedStaffAccounts{1: 101, 2: 102},
	}

	classes, err := reach.StudentClassTeachers(context.Background(), []int64{10, 11, 12})
	require.NoError(t, err)

	assert.Equal(t, map[int64]deliveryContract.StudentClassTeachers{
		10: {SchoolClass: "Klasse 1a", AccountIDs: []int64{101}},
		11: {SchoolClass: "Klasse 2a", AccountIDs: []int64{102}},
	}, classes, "each child reaches only its own class's Lehrkräfte; a child without a class is left out")
	require.Len(t, assignments.filters, 1, "one assignment read for all classes")
	assert.Equal(t, []string{"klasse 1a", "klasse 2a"}, assignments.filters[0].ClassKeys)
}
