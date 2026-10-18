package schoolmembership_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

const (
	caregiverSchool  int64 = 31
	otherSchool      int64 = 32
	caregiverAccount int64 = 801
)

type caregiverMembership struct {
	teachers []schoolmembership.Teacher
	staff    []schoolmembership.Staff
}

func (m caregiverMembership) ListTeachers(context.Context, schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error) {
	return m.teachers, nil
}
func (m caregiverMembership) ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	return m.staff, nil
}

type caregiverDirectory struct {
	persons   map[int64]schoolmembership.CaregiverPerson
	active    map[int64][]int64
	withRole  map[int64][]int64
	roleNames []string
}

func (d *caregiverDirectory) FindCaregiverPersons(context.Context, []int64) (map[int64]schoolmembership.CaregiverPerson, error) {
	return d.persons, nil
}
func (d *caregiverDirectory) ListActiveAccountIDsForTenant(_ context.Context, tenantID int64, _ []int64) ([]int64, error) {
	return d.active[tenantID], nil
}
func (d *caregiverDirectory) ListAccountIDsWithSystemRoleNames(_ context.Context, _ []int64, roleNames []string, tenantID int64) ([]int64, error) {
	d.roleNames = roleNames
	return d.withRole[tenantID], nil
}
func (d *caregiverDirectory) ListAccountEmails(_ context.Context, ids []int64) (map[int64]string, error) {
	emails := map[int64]string{}
	for _, id := range ids {
		emails[id] = "caregiver@example.org"
	}
	return emails, nil
}

func caregiverFixture() (caregiverMembership, *caregiverDirectory) {
	account, other, noRole := caregiverAccount, caregiverAccount+1, caregiverAccount+2
	membership := caregiverMembership{
		teachers: []schoolmembership.Teacher{
			{ID: 41, TenantID: caregiverSchool, StaffID: 51},
			{ID: 42, TenantID: otherSchool, StaffID: 52},
			{ID: 43, TenantID: caregiverSchool, StaffID: 53},
			{ID: 44, TenantID: caregiverSchool, StaffID: 54},
		},
		staff: []schoolmembership.Staff{
			{ID: 51, TenantID: caregiverSchool, PersonID: 61},
			{ID: 52, TenantID: otherSchool, PersonID: 62},
			{ID: 53, TenantID: caregiverSchool, PersonID: 63},
			{ID: 54, TenantID: caregiverSchool, PersonID: 64},
		},
	}
	directory := &caregiverDirectory{
		persons: map[int64]schoolmembership.CaregiverPerson{
			61: {ID: 61, TenantID: caregiverSchool, AccountID: &account, FirstName: "Mia", LastName: "Muster"},
			62: {ID: 62, TenantID: otherSchool, AccountID: &other, FirstName: "Ole", LastName: "Ost"},
			63: {ID: 63, TenantID: caregiverSchool, AccountID: &noRole, FirstName: "Nia", LastName: "Nord"},
			64: {ID: 64, TenantID: caregiverSchool, FirstName: "Kai", LastName: "Konto"},
		},
		active:   map[int64][]int64{caregiverSchool: {account, noRole}, otherSchool: {other}},
		withRole: map[int64][]int64{caregiverSchool: {account}, otherSchool: {other}},
	}
	return membership, directory
}

// An active caregiver is a teacher of the school whose person holds an
// account that is active there and has a caregiver role there.
func TestActiveCaregiversOfASchool(t *testing.T) {
	t.Parallel()
	membership, directory := caregiverFixture()
	caregivers, err := schoolmembership.ActiveCaregivers(context.Background(), membership, directory, caregiverSchool, nil)
	require.NoError(t, err)
	require.Len(t, caregivers, 1)
	assert.Equal(t, schoolmembership.ActiveCaregiver{AccountID: caregiverAccount, PersonID: 61, StaffID: 51, TeacherID: 41,
		FirstName: "Mia", LastName: "Muster", Email: "caregiver@example.org"}, caregivers[0])
	assert.Equal(t, []string{"user", "teacher"}, directory.roleNames)

	everywhere, err := schoolmembership.ActiveCaregivers(context.Background(), membership, directory, 0, nil)
	require.NoError(t, err)
	ids := []int64{}
	for _, caregiver := range everywhere {
		ids = append(ids, caregiver.TeacherID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	assert.Equal(t, []int64{41, 42}, ids, "without a school every school's caregivers count")

	other := caregiverAccount + 1
	none, err := schoolmembership.ActiveCaregivers(context.Background(), membership, directory, caregiverSchool, &other)
	require.NoError(t, err)
	assert.Empty(t, none, "an account of another school is no caregiver here")
}

func TestCaregiverStatusNeedsTheUserRoleToBeActive(t *testing.T) {
	t.Parallel()
	profile, active := schoolmembership.CaregiverStatus(true, false)
	assert.True(t, profile)
	assert.False(t, active)
	profile, active = schoolmembership.CaregiverStatus(false, true)
	assert.False(t, profile)
	assert.False(t, active)
	_, active = schoolmembership.CaregiverStatus(true, true)
	assert.True(t, active)
}

func TestExternalCaregiverIsAGuestWithoutAccount(t *testing.T) {
	t.Parallel()
	assert.True(t, schoolmembership.ExternalCaregiver(true, true, false))
	assert.False(t, schoolmembership.ExternalCaregiver(true, true, true), "a guest with an account is staff")
	assert.False(t, schoolmembership.ExternalCaregiver(true, false, false), "without the person nothing is known")
	assert.False(t, schoolmembership.ExternalCaregiver(false, true, false))
}

func TestTeachingAssignmentValidation(t *testing.T) {
	t.Parallel()
	require.EqualError(t, schoolmembership.ValidateClassAssignment(0, "1a"), "staff ID is required")
	require.EqualError(t, schoolmembership.ValidateClassAssignment(51, "  "), "school class is required")
	require.NoError(t, schoolmembership.ValidateClassAssignment(51, "1a"))
	require.EqualError(t, schoolmembership.ValidateGroupAssignment(0, 41), "group ID is required")
	require.EqualError(t, schoolmembership.ValidateGroupAssignment(71, 0), "teacher ID is required")
	require.NoError(t, schoolmembership.ValidateGroupAssignment(71, 41))
}

type groupAssignments []schoolmembership.GroupAssignment

func (a groupAssignments) ListGroupAssignments(_ context.Context, filter schoolmembership.GroupAssignmentFilter) ([]schoolmembership.GroupAssignment, error) {
	var result []schoolmembership.GroupAssignment
	for _, assignment := range a {
		for _, teacherID := range filter.TeacherIDs {
			if assignment.TeacherID == teacherID {
				result = append(result, assignment)
			}
		}
		for _, groupID := range filter.GroupIDs {
			if assignment.GroupID == groupID {
				result = append(result, assignment)
			}
		}
	}
	return result, nil
}

// The blockers of a teacher are their group assignments at the school, each
// with every teacher of the group, a group the caller cannot see labelled
// "Unbekannte Gruppe", sorted by name.
func TestGroupTeacherBlockers(t *testing.T) {
	t.Parallel()
	assignments := groupAssignments{
		{ID: 91, TenantID: caregiverSchool, GroupID: 71, TeacherID: 41},
		{ID: 92, TenantID: caregiverSchool, GroupID: 72, TeacherID: 41},
		{ID: 93, TenantID: caregiverSchool, GroupID: 71, TeacherID: 43},
		{ID: 94, TenantID: otherSchool, GroupID: 73, TeacherID: 41},
	}
	names := func(context.Context, []int64) (map[int64]string, error) { return map[int64]string{71: "Wölfe"}, nil }
	blockers, err := schoolmembership.GroupTeacherBlockers(context.Background(), assignments, names, 41, caregiverSchool)
	require.NoError(t, err)
	assert.Equal(t, []schoolmembership.GroupTeacherBlocker{
		{AssignmentID: 92, GroupID: 72, GroupName: "Unbekannte Gruppe", TeacherID: 41, TeacherIDs: []int64{41}},
		{AssignmentID: 91, GroupID: 71, GroupName: "Wölfe", TeacherID: 41, TeacherIDs: []int64{41, 43}},
	}, blockers)
}

func TestSortActiveCaregiversUsesTheGermanPickerOrder(t *testing.T) {
	t.Parallel()
	caregivers := []schoolmembership.ActiveCaregiver{
		{StaffID: 15, FirstName: "Zoe", LastName: "Adam"},
		{StaffID: 14, FirstName: "Özlem", LastName: "Yilmaz"},
		{StaffID: 13, FirstName: "otto", LastName: "Berg"},
		{StaffID: 12, FirstName: "Otto", LastName: "Berg"},
		{StaffID: 11, FirstName: "Otto", LastName: "Adam"},
	}
	schoolmembership.SortActiveCaregivers(caregivers)
	ids := make([]int64, 0, len(caregivers))
	for _, caregiver := range caregivers {
		ids = append(ids, caregiver.StaffID)
	}
	assert.Equal(t, []int64{11, 12, 13, 14, 15}, ids)
}
