package grouplive

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holidayGroupsFake serves the Feriengruppen of the school (ADR 0120,
// Nachtrag 2026-11-11).
type holidayGroupsFake struct {
	weeks []HolidayGroupWeek
	err   error
	calls int
}

func (h *holidayGroupsFake) HolidayGroupWeeks(context.Context) ([]HolidayGroupWeek, error) {
	h.calls++
	return h.weeks, h.err
}

// holidayGroupFixture is a school with a regular group "Bären" (1), a
// Feriengruppe made for a week after today (2) and a regular group that is
// also the Feriengruppe of that week (3). Children 11 and 31 are in care
// today; child 21 waits in the Feriengruppe without care today.
func holidayGroupFixture(weeks ...HolidayGroupWeek) (*fakes, *holidayGroupsFake) {
	f := &fakes{
		caller: func(context.Context) (Caller, error) {
			return Caller{CanReadGroups: true, OperationalOverview: true}, nil
		},
		tenantGroups: func(context.Context) ([]GroupRecord, error) {
			return []GroupRecord{{ID: 1, Name: "Bären"}, {ID: 2, Name: "Ferienbetreuung Januar"}, {ID: 3, Name: "Füchse"}}, nil
		},
		members: func(_ context.Context, groupID int64) ([]RosterStudent, error) {
			switch groupID {
			case 1:
				return []RosterStudent{{ID: 11}}, nil
			case 3:
				return []RosterStudent{{ID: 31}}, nil
			}
			return nil, nil
		},
		participants: func(_ context.Context, ids []int64, _ Date) (map[int64]bool, error) {
			result := map[int64]bool{}
			for _, id := range ids {
				result[id] = id == 11 || id == 31
			}
			return result, nil
		},
	}
	return f, &holidayGroupsFake{weeks: weeks}
}

func holidayGroupService(f *fakes, holiday HolidayGroups) Query {
	return New(Dependencies{
		Access: f, Groups: f, Roster: f, Presence: f, Planning: f, Transfers: f,
		Settings: f, Calendar: f, ExcusedRequests: f, HolidayGroups: holiday,
	})
}

func groupIDs(groups []Group) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}

// futureWeek is a hosted week after today (2026-08-21).
func futureWeek(groupID int64) HolidayGroupWeek {
	return HolidayGroupWeek{GroupID: groupID, StartsOn: "2027-01-04", EndsOn: "2027-01-08"}
}

func TestAwaitingHolidayGroupsLeaveTheDailyGroupList(t *testing.T) {
	t.Parallel()
	f, holiday := holidayGroupFixture(futureWeek(2), futureWeek(3))

	got, err := holidayGroupService(f, holiday).ListGroups(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 3}, groupIDs(got),
		"the empty Feriengruppe of a later week waits outside the list; a regular group that is also Feriengruppe stays")
	assert.Equal(t, 1, holiday.calls, "one read of the Feriengruppen per list")
	assert.Equal(t, [][]int64{{31}}, f.participantsCalled, "only the waiting groups' children are asked for care, in one read")
}

func TestHolidayGroupWithAChildInCareTodayStays(t *testing.T) {
	t.Parallel()
	f, holiday := holidayGroupFixture(futureWeek(2))
	members := f.members
	f.members = func(ctx context.Context, groupID int64) ([]RosterStudent, error) {
		if groupID == 2 {
			return []RosterStudent{{ID: 21}, {ID: 22}}, nil
		}
		return members(ctx, groupID)
	}
	f.participants = func(_ context.Context, ids []int64, _ Date) (map[int64]bool, error) {
		return map[int64]bool{22: true}, nil
	}

	got, err := holidayGroupService(f, holiday).ListGroups(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, groupIDs(got), "a child in care today brings the Feriengruppe back")
}

func TestHolidayGroupOfAStartedWeekStays(t *testing.T) {
	t.Parallel()
	for name, week := range map[string]HolidayGroupWeek{
		"week starts today":  {GroupID: 2, StartsOn: today, EndsOn: "2026-08-25"},
		"week runs":          {GroupID: 2, StartsOn: "2026-08-17", EndsOn: "2026-08-21"},
		"second week later":  futureWeek(2),
		"first week running": {GroupID: 2, StartsOn: "2026-08-20", EndsOn: "2026-08-24"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			weeks := []HolidayGroupWeek{week}
			if name == "second week later" {
				weeks = append(weeks, HolidayGroupWeek{GroupID: 2, StartsOn: "2026-08-17", EndsOn: "2026-08-21"})
			}
			f, holiday := holidayGroupFixture(weeks...)
			got, err := holidayGroupService(f, holiday).ListGroups(context.Background())
			require.NoError(t, err)
			assert.Equal(t, []int64{1, 2, 3}, groupIDs(got), "a Feriengruppe whose week has begun is a daily group")
			assert.Empty(t, f.participantsCalled, "no group waits, so no care is read")
		})
	}
}

func TestHolidayGroupWithoutWeeksIsAnOrdinaryGroup(t *testing.T) {
	t.Parallel()
	f, holiday := holidayGroupFixture()
	got, err := holidayGroupService(f, holiday).ListGroups(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, groupIDs(got), "after its last week the group shows like any other")

	f, _ = holidayGroupFixture()
	got, err = holidayGroupService(f, nil).ListGroups(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, groupIDs(got), "without the Feriengruppen read every group stays")
}

func TestLiveGroupKeepsAnAwaitingHolidayGroupThatIsAskedFor(t *testing.T) {
	t.Parallel()
	f, holiday := holidayGroupFixture(futureWeek(2))

	projection, err := holidayGroupService(f, holiday).LiveGroup(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 3}, groupIDs(projection.Groups), "the live view lists the daily groups only")

	f, holiday = holidayGroupFixture(futureWeek(2))
	projection, err = holidayGroupService(f, holiday).LiveGroup(context.Background(), 2)
	require.NoError(t, err, "a link to the Feriengruppe still opens it")
	assert.Equal(t, []int64{1, 2, 3}, groupIDs(projection.Groups))
	require.NotNil(t, projection.GroupID)
	assert.Equal(t, int64(2), *projection.GroupID)
}

func TestHolidayGroupsReadFailureFailsTheList(t *testing.T) {
	t.Parallel()
	f, holiday := holidayGroupFixture()
	holiday.err = errors.New("enrollment unavailable")
	_, err := holidayGroupService(f, holiday).ListGroups(context.Background())
	require.ErrorContains(t, err, "load holiday groups")
}

func TestAwaitingHolidayGroupIDs(t *testing.T) {
	t.Parallel()
	assert.Equal(t, map[int64]bool{2: true}, awaitingHolidayGroupIDs([]HolidayGroupWeek{
		futureWeek(2),
		futureWeek(3), {GroupID: 3, StartsOn: "2026-08-17", EndsOn: "2026-08-21"},
		{GroupID: 4, StartsOn: today, EndsOn: today},
	}, today))
}
