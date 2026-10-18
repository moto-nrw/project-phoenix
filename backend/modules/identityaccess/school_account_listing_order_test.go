package identityaccess_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

const (
	listingSchoolA int64 = 10
	listingSchoolB int64 = 20
	listingPersonA int64 = 100
	listingPersonB int64 = 200
)

// One school's accounts order by last and first name, accounts without a
// person last.
func TestCompareSchoolAccountsPutsPersonlessRowsLast(t *testing.T) {
	t.Parallel()
	rows := []identityaccess.ListedAccountKey{
		{LastName: "Zorn", FirstName: "Ada"},
		{HasPerson: true, LastName: "Berg", FirstName: "Uwe"},
		{HasPerson: true, LastName: "Berg", FirstName: "Anna"},
		{HasPerson: true, LastName: "Abel", FirstName: "Tom"},
	}
	slices.SortStableFunc(rows, identityaccess.CompareSchoolAccounts)
	assert.Equal(t, []identityaccess.ListedAccountKey{
		{HasPerson: true, LastName: "Abel", FirstName: "Tom"},
		{HasPerson: true, LastName: "Berg", FirstName: "Anna"},
		{HasPerson: true, LastName: "Berg", FirstName: "Uwe"},
		{LastName: "Zorn", FirstName: "Ada"},
	}, rows)
}

// Several schools group by school ID first.
func TestCompareAccountsBySchoolIDGroupsSchools(t *testing.T) {
	t.Parallel()
	rows := []identityaccess.ListedAccountKey{
		{SchoolID: listingSchoolB, HasPerson: true, LastName: "Abel"},
		{SchoolID: listingSchoolA, LastName: "Abel"},
		{SchoolID: listingSchoolA, HasPerson: true, LastName: "Zorn"},
	}
	slices.SortStableFunc(rows, identityaccess.CompareAccountsBySchoolID)
	assert.Equal(t, []identityaccess.ListedAccountKey{
		{SchoolID: listingSchoolA, HasPerson: true, LastName: "Zorn"},
		{SchoolID: listingSchoolA, LastName: "Abel"},
		{SchoolID: listingSchoolB, HasPerson: true, LastName: "Abel"},
	}, rows)
}

// The listing across schools orders by school name, then last and first
// name, and keeps the earlier order among equal rows.
func TestCompareAccountsBySchoolNameOrdersAcrossSchools(t *testing.T) {
	t.Parallel()
	rows := []identityaccess.ListedAccountKey{
		{SchoolName: "Waldschule", LastName: "Abel", FirstName: "Tom"},
		{SchoolName: "Burgschule", LastName: "Berg", FirstName: "Uwe", HasPerson: true},
		{SchoolName: "Burgschule", LastName: "Berg", FirstName: "Uwe"},
		{SchoolName: "Burgschule", LastName: "Berg", FirstName: "Anna"},
	}
	slices.SortStableFunc(rows, identityaccess.CompareAccountsBySchoolName)
	assert.Equal(t, []identityaccess.ListedAccountKey{
		{SchoolName: "Burgschule", LastName: "Berg", FirstName: "Anna"},
		{SchoolName: "Burgschule", LastName: "Berg", FirstName: "Uwe", HasPerson: true},
		{SchoolName: "Burgschule", LastName: "Berg", FirstName: "Uwe"},
		{SchoolName: "Waldschule", LastName: "Abel", FirstName: "Tom"},
	}, rows)
}

// The first staff record listed for a person counts, and only at its own
// school.
func TestStaffRecordCountsOnlyAtItsSchool(t *testing.T) {
	t.Parallel()
	schools := identityaccess.StaffSchoolByPerson([]identityaccess.PersonStaffRecord{
		{PersonID: listingPersonA, SchoolID: listingSchoolA},
		{PersonID: listingPersonA, SchoolID: listingSchoolB},
		{PersonID: listingPersonB, SchoolID: listingSchoolB},
	})
	assert.Equal(t, map[int64]int64{listingPersonA: listingSchoolA, listingPersonB: listingSchoolB}, schools)

	school, found := schools[listingPersonA]
	assert.True(t, identityaccess.StaffRecordCountsAtSchool(found, school, listingSchoolA))
	assert.False(t, identityaccess.StaffRecordCountsAtSchool(found, school, listingSchoolB))
	assert.False(t, identityaccess.StaffRecordCountsAtSchool(false, 0, 0), "no record backs no school")
}
