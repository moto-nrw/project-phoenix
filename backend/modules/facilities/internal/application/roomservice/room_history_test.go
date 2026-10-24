package facilities

import (
	"context"
	"errors"
	"testing"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/stretchr/testify/require"
)

type historyFactsStub struct {
	rows []facilitiesModule.RoomSessionFact
	err  error
}

func (s historyFactsStub) ListRoomSessionHistory(context.Context, int64, time.Time, time.Time, *int64) ([]facilitiesModule.RoomSessionFact, error) {
	return s.rows, s.err
}
func TestRoomHistoryResolvesNamesAndPropagatesOwnerErrors(t *testing.T) {
	t.Parallel()
	activityID := int64(10)
	facts := historyFactsStub{rows: []facilitiesModule.RoomSessionFact{{SessionID: 1, ActivityGroupID: &activityID, SupervisorStaffIDs: []int64{7, 6, 7}, StudentCount: 2}}}
	activities := activityProjectionStub{groups: map[int64]facilitiesModule.ActivityGroupName{10: activityGroup("Atelier", "Kreativ")}}
	membership := membershipQueryStub{staff: []schoolmembership.Staff{{ID: 7, PersonID: 70}, {ID: 6, PersonID: 60}}}
	people := personQueryStub{people: []peopledirectory.Person{{ID: 70, FirstName: " Bert", LastName: "Supervisor "}, {ID: 60, FirstName: "Ada", LastName: "Supervisor"}}}
	project := NewHistoryProjection(facts, activities, membership, people)
	rows, err := project(context.Background(), 2, time.Time{}, time.Time{}, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Atelier", rows[0].ActivityName)
	require.Equal(t, "Ada Supervisor, Bert Supervisor", rows[0].SupervisorName)
	require.Equal(t, 2, rows[0].StudentCount)
	failure := errors.New("owner unavailable")
	for _, owner := range []string{"presence", "activities", "membership", "people"} {
		t.Run(owner, func(t *testing.T) {
			f, a, m, p := facts, activities, membership, people
			switch owner {
			case "presence":
				f.err = failure
			case "activities":
				a.err = failure
			case "membership":
				m.err = failure
			case "people":
				p.err = failure
			}
			rows, err := NewHistoryProjection(f, a, m, p)(context.Background(), 2, time.Time{}, time.Time{}, nil)
			require.ErrorIs(t, err, failure)
			require.Nil(t, rows)
		})
	}
}
