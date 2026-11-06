package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

func TestGroupOccupancyCountsOnlyTheRosterByMembershipGroup(t *testing.T) {
	t.Parallel()
	const (
		bears, foxes, empty, foreign int64 = 1, 2, 3, 99
		anna, ben, cem, dora         int64 = 10, 11, 12, 13
		annaM, benM, cemM, doraM     int64 = 20, 21, 22, 23
		leaverM                      int64 = 24
	)
	roster := map[int64]int64{anna: annaM, ben: benM, cem: cemM, dora: doraM}
	memberships := map[int64]int64{annaM: bears, benM: bears, cemM: foreign, leaverM: foxes}
	names := []traegerview.NamedGroup{{ID: bears, Name: "Bären"}, {ID: empty, Name: "Eulen"}, {ID: foxes, Name: "Füchse"}}

	groups, without := groupOccupancy(roster, memberships, names)

	assert.Equal(t, []traegerview.OccupancyGroup{
		{ID: bears, Name: "Bären", Children: 2},
		{ID: empty, Name: "Eulen", Children: 0},
		{ID: foxes, Name: "Füchse", Children: 0},
	}, groups, "a leaver's membership does not count, every named group is listed")
	assert.Equal(t, 2, without, "a child without a group and a child in a group of another school count as without a group")
}

func TestCareFormOccupancyCountsEachRosterChildOncePerOffer(t *testing.T) {
	t.Parallel()
	const (
		anna, ben, cem, leaver                    int64 = 10, 11, 12, 14
		annaR, annaR2, benR, leaverR, unapprovedR int64 = 30, 31, 32, 33, 34
	)
	roster := map[int64]int64{anna: 20, ben: 21, cem: 22}
	children := map[int64]int64{annaR: anna, annaR2: anna, benR: ben, leaverR: leaver}
	capacity := 25
	offerings := []traegerview.CareOfferingBookings{
		{ID: 1, Name: "Frühbetreuung", RequestChildIDs: []int64{annaR}},
		{ID: 2, Name: "OGS bis 16 Uhr", Capacity: &capacity, RequestChildIDs: []int64{annaR, annaR2, benR, leaverR, unapprovedR}},
	}

	forms, without := careFormOccupancy(roster, offerings, children)

	assert.Equal(t, []traegerview.OccupancyCareForm{
		{ID: 1, Name: "Frühbetreuung", Children: 1},
		{ID: 2, Name: "OGS bis 16 Uhr", Children: 2, Capacity: &capacity},
	}, forms, "two bookings of one child count once; a leaver and an unapproved child do not count")
	assert.Equal(t, 1, without, "only the child without any booking counts as without a care form")
}

func TestOccupancySchoolWithoutReadsIsEmpty(t *testing.T) {
	t.Parallel()
	school := traegerview.School{ID: 7, Name: "OGS Am Park"}

	row := occupancyReads{}.school(school)

	assert.Equal(t, traegerview.SchoolOccupancy{School: school, Groups: []traegerview.OccupancyGroup{}, CareForms: []traegerview.OccupancyCareForm{}}, row)
}
