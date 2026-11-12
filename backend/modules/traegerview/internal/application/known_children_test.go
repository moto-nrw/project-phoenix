package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

func TestKnownChildrenNeedExactlyOneManagedNamesake(t *testing.T) {
	t.Parallel()
	const north, south int64 = 1, 2
	children := []traegerview.RegisteredChild{
		{RegistrationID: 7, ChildID: 70}, // one managed namesake at the south school
		{RegistrationID: 7, ChildID: 71}, // managed namesakes at both schools
		{RegistrationID: 8, ChildID: 80}, // a namesake the school no longer manages
		{RegistrationID: 8, ChildID: 81}, // two managed namesakes at one school
		{RegistrationID: 9, ChildID: 90}, // one managed and one former namesake
	}
	namesakes := []traegerview.Namesake{
		{Child: 0, SchoolID: south, StudentProfileID: 100},
		{Child: 1, SchoolID: north, StudentProfileID: 110},
		{Child: 1, SchoolID: south, StudentProfileID: 111},
		{Child: 2, SchoolID: north, StudentProfileID: 120},
		{Child: 3, SchoolID: north, StudentProfileID: 130},
		{Child: 3, SchoolID: north, StudentProfileID: 131},
		{Child: 4, SchoolID: north, StudentProfileID: 140},
		{Child: 4, SchoolID: south, StudentProfileID: 141},
		{Child: 9, SchoolID: north, StudentProfileID: 150}, // outside the request
	}
	roster := map[int64]map[int64]int64{
		north: {110: 1, 130: 1, 131: 1, 140: 1, 150: 1},
		south: {100: 1, 111: 1},
	}

	assert.Equal(t, []traegerview.KnownChild{
		{RegistrationID: 7, ChildID: 70, SchoolID: south},
		{RegistrationID: 9, ChildID: 90, SchoolID: north},
	}, knownChildren(children, namesakes, roster))
	assert.Equal(t, []int64{south, north}, namesakeSchools(namesakes))
}
