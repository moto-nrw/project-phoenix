package grouplive

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handoverFake answers the class handover port (#4794).
type handoverFake struct {
	handovers map[int64]ClassHandover
	err       error
	calls     int
	asked     []int64
	day       Date
}

func (h *handoverFake) ClassHandovers(_ context.Context, ids []int64, day Date) (map[int64]ClassHandover, error) {
	h.calls++
	h.asked, h.day = ids, day
	return h.handovers, h.err
}

func handoverDeps(f *fakes, handovers HandoverReader) Dependencies {
	return Dependencies{
		Access: f, Groups: f, Roster: f, Presence: f, Planning: f, Transfers: f,
		Settings: f, Calendar: f, Handovers: handovers,
	}
}

// TestLiveGroupCarriesTheClassHandover pins that the group view carries
// today's handover of each child, with the class, the moment, the Lehrkraft
// and whether she marked the child "nicht da", for callers who read plans,
// and fails with the port.
func TestLiveGroupCarriesTheClassHandover(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.August, 21, 11, 45, 0, 0, time.UTC)
	roster := func(context.Context, int64) ([]RosterStudent, error) {
		return []RosterStudent{{ID: 11, SchoolClass: "2a"}, {ID: 12, SchoolClass: "2b"}, {ID: 13, SchoolClass: "3c"}}, nil
	}
	newFakes := func(caller Caller) *fakes {
		return &fakes{
			caller:           func(context.Context) (Caller, error) { return caller, nil },
			supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
			members:          roster,
		}
	}

	handovers := &handoverFake{handovers: map[int64]ClassHandover{
		11: {SchoolClass: "2a", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft"},
		12: {SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft", Absent: true},
	}}
	got, err := New(handoverDeps(newFakes(Caller{CanReadStudentPlans: true, CanReadGroups: true}), handovers)).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12, 13}, handovers.asked)
	assert.Equal(t, today, handovers.day, "today's handover")
	require.Len(t, got.Students, 3)
	assert.Equal(t, &ClassHandover{SchoolClass: "2a", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft"}, got.Students[0].ClassHandover, "handed over and there")
	assert.Equal(t, &ClassHandover{SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft", Absent: true}, got.Students[1].ClassHandover)
	assert.Nil(t, got.Students[2].ClassHandover, "not handed over")

	restricted := &handoverFake{handovers: handovers.handovers}
	got, err = New(handoverDeps(newFakes(Caller{CanReadGroups: true}), restricted)).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	assert.Zero(t, restricted.calls, "like sick and excused, only for callers who read plans")
	assert.Nil(t, got.Students[1].ClassHandover)

	failing := &handoverFake{err: errors.New("presence unavailable")}
	_, err = New(handoverDeps(newFakes(Caller{CanReadStudentPlans: true, CanReadGroups: true}), failing)).LiveGroup(context.Background(), 1)
	require.ErrorContains(t, err, "apply class handovers")
}
