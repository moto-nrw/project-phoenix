package staffdirectoryview

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type membersStub struct {
	staff   []schoolmembership.Staff
	err     error
	filters []schoolmembership.StaffFilter
}

func (m *membersStub) ListStaff(_ context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	m.filters = append(m.filters, filter)
	if len(filter.IDs) == 0 {
		return m.staff, m.err
	}
	var result []schoolmembership.Staff
	for _, member := range m.staff {
		if slices.Contains(filter.IDs, member.ID) {
			result = append(result, member)
		}
	}
	return result, m.err
}

func (m *membersStub) FindStaff(_ context.Context, id int64) (schoolmembership.Staff, error) {
	for _, member := range m.staff {
		if member.ID == id {
			return member, nil
		}
	}
	return schoolmembership.Staff{}, schoolmembership.ErrStaffNotFound
}

type personsStub map[int64]Person

func (p personsStub) Person(_ context.Context, id int64) (Person, error) {
	person, found := p[id]
	if !found {
		return Person{}, ErrPersonNotFound
	}
	return person, nil
}

func (p personsStub) PersonsByID(_ context.Context, ids []int64) (map[int64]Person, error) {
	result := map[int64]Person{}
	for _, id := range ids {
		if person, found := p[id]; found {
			result[id] = person
		}
	}
	return result, nil
}

type accountsStub struct {
	active      map[int64][]int64
	permissions map[int64][]string
	roles       map[int64]int
	emails      map[int64]string
}

func (a accountsStub) ListActiveAccountIDsForTenant(_ context.Context, tenantID int64, accountIDs []int64) ([]int64, error) {
	var result []int64
	for _, id := range accountIDs {
		if slices.Contains(a.active[tenantID], id) {
			result = append(result, id)
		}
	}
	return result, nil
}

func (a accountsStub) FindEffectivePermissionNamesByAccountIDsForTenant(context.Context, []int64, int64) (map[int64][]string, error) {
	return a.permissions, nil
}

func (a accountsStub) CountRoleNameMatchesByAccountIDs(context.Context, []int64, []string) (map[int64]int, error) {
	return a.roles, nil
}

func (a accountsStub) ListAccountEmails(_ context.Context, ids []int64) (map[int64]string, error) {
	result := map[int64]string{}
	for _, id := range ids {
		if email, found := a.emails[id]; found {
			result[id] = email
		}
	}
	return result, nil
}

// The fixture: staff 11 (person 21, account 31, active at school 41, holds
// "calendar:own"), staff 12 (person 22 without an account), staff 13
// (person 23, account 33, inactive at the school) and staff 14 (person 24,
// account 34, active, birthday opted out).
const school int64 = 41

func mustDate(t *testing.T, value string) calendar.Date {
	t.Helper()
	date, err := calendar.ParseDate(value)
	require.NoError(t, err)
	return date
}

func fixture(t *testing.T) (*Directory, *membersStub) {
	t.Helper()
	const first, third, fourth int64 = 31, 33, 34
	account31, account33, account34 := first, third, fourth
	birthday := mustDate(t, "1990-05-17")
	members := &membersStub{staff: []schoolmembership.Staff{
		{ID: 11, TenantID: school, PersonID: 21},
		{ID: 12, TenantID: school, PersonID: 22},
		{ID: 13, TenantID: school, PersonID: 23},
		{ID: 14, TenantID: school, PersonID: 24, BirthdayDisplayOptOut: true},
	}}
	directory, err := New(Dependencies{
		Members: members,
		Persons: personsStub{
			21: {ID: 21, FirstName: "Anna", LastName: "Arndt", AccountID: &account31, Birthday: &birthday},
			22: {ID: 22, FirstName: "Bert", LastName: "Brand", Birthday: &birthday},
			23: {ID: 23, FirstName: "Cora", LastName: "Claus", AccountID: &account33},
			24: {ID: 24, FirstName: "Dora", LastName: "Dahl", AccountID: &account34, Birthday: &birthday},
		},
		Accounts: accountsStub{
			active:      map[int64][]int64{school: {31, 34}},
			permissions: map[int64][]string{31: {"calendar:own"}},
			roles:       map[int64]int{31: 2},
			emails:      map[int64]string{31: "anna@example.test", 34: "dora@example.test"},
		},
		CallerTenant:  func(context.Context) int64 { return school },
		HasPermission: func(required string, granted []string) bool { return slices.Contains(granted, required) },
	})
	require.NoError(t, err)
	return directory, members
}

func TestAccountIDsMapLiveStaffToTheirActiveAccount(t *testing.T) {
	t.Parallel()
	directory, members := fixture(t)

	all, err := directory.AllStaffAccountIDs(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{11: 31, 14: 34}, all)
	assert.True(t, members.filters[0].MembershipOnly)

	none, err := directory.AccountIDsByStaffIDs(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{}, none)
	assert.Len(t, members.filters, 1, "no staff member reads nothing")
}

func TestPermittedStaffKeepTheHoldersOfThePermission(t *testing.T) {
	t.Parallel()
	directory, _ := fixture(t)

	contacts, err := directory.StaffWithPermission(t.Context(), "calendar:own")
	require.NoError(t, err)
	assert.Equal(t, []StaffContact{{StaffID: 11, PersonID: 21, FirstName: "Anna", LastName: "Arndt", AccountID: 31, Email: "anna@example.test"}}, contacts)

	reachable, err := directory.ReachableStaffIDs(t.Context(), []int64{11, 13, 14}, "calendar:own")
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{11: true}, reachable)
}

func TestStaffContactNeedsPersonAccountAndAddress(t *testing.T) {
	t.Parallel()
	directory, _ := fixture(t)

	contact, err := directory.StaffContact(t.Context(), 11)
	require.NoError(t, err)
	assert.Equal(t, "anna@example.test", contact.Email)
	_, err = directory.StaffContact(t.Context(), 12)
	assert.ErrorIs(t, err, ErrStaffContactNotFound, "no account")
	_, err = directory.StaffContact(t.Context(), 13)
	assert.ErrorIs(t, err, ErrStaffContactNotFound, "no address")
	_, err = directory.StaffContact(t.Context(), 99)
	var membership *MembershipError
	require.ErrorAs(t, err, &membership)
	assert.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
}

func TestStaffByRolesEmitsOneEntryPerMatchingRole(t *testing.T) {
	t.Parallel()
	directory, _ := fixture(t)

	contacts, err := directory.StaffByRoles(t.Context(), []string{"admin"})
	require.NoError(t, err)
	require.Len(t, contacts, 2)
	const anna int64 = 11
	assert.Equal(t, anna, contacts[0].StaffID)
	assert.Equal(t, contacts[0], contacts[1])
}

func TestBirthdaysHonourTheOptOutExceptInTheExport(t *testing.T) {
	t.Parallel()
	directory, _ := fixture(t)

	on, err := directory.BirthdaysOn(t.Context(), []MonthDay{{Month: time.May, Day: 17}})
	require.NoError(t, err)
	assert.Equal(t, []StaffBirthday{
		{StaffID: 11, FirstName: "Anna", LastName: "Arndt", Birthday: mustDate(t, "1990-05-17")},
		{StaffID: 12, FirstName: "Bert", LastName: "Brand", Birthday: mustDate(t, "1990-05-17")},
	}, on, "the opted-out staff member stays off the list")
	other, err := directory.BirthdaysOn(t.Context(), []MonthDay{{Month: time.June, Day: 1}})
	require.NoError(t, err)
	assert.Empty(t, other)
	none, err := directory.BirthdaysOn(t.Context(), nil)
	require.NoError(t, err)
	assert.Nil(t, none)

	export, err := directory.AllBirthdays(t.Context())
	require.NoError(t, err)
	assert.Len(t, export, 3, "the export ignores the opt-out and skips staff without a birthday")
}

func TestMembershipFailuresAreMarked(t *testing.T) {
	t.Parallel()
	directory, members := fixture(t)
	members.err = errors.New("membership down")

	_, err := directory.AllBirthdays(t.Context())
	var membership *MembershipError
	assert.ErrorAs(t, err, &membership)
	_, err = New(Dependencies{})
	assert.Error(t, err)
}
