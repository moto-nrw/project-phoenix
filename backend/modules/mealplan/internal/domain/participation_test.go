package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortDailyParticipantsOrdersByClassNameAndID(t *testing.T) {
	t.Parallel()
	participants := []DailyParticipant{
		{StudentID: 14, FirstName: "Ana", LastName: "Zorn", SchoolClass: "2a"},
		{StudentID: 13, FirstName: "Ben", LastName: "Adam", SchoolClass: "3b"},
		{StudentID: 12, FirstName: "Mia", LastName: "Adam", SchoolClass: "2a"},
		{StudentID: 11, FirstName: "Mia", LastName: "Adam", SchoolClass: "2a"},
		{StudentID: 15, FirstName: "Lea", LastName: "Adam", SchoolClass: "2a"},
	}
	SortDailyParticipants(participants)
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.StudentID)
	}
	assert.Equal(t, []int64{15, 11, 12, 14, 13}, ids)
}
