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
	absences map[int64]HandoverAbsence
	err      error
	calls    int
	asked    []int64
	day      Date
}

func (h *handoverFake) HandoverAbsences(_ context.Context, ids []int64, day Date) (map[int64]HandoverAbsence, error) {
	h.calls++
	h.asked, h.day = ids, day
	return h.absences, h.err
}

func handoverDeps(f *fakes, handovers HandoverReader) Dependencies {
	return Dependencies{
		Access: f, Groups: f, Roster: f, Presence: f, Planning: f, Transfers: f,
		Settings: f, Calendar: f, Handovers: handovers,
	}
}

// TestLiveGroupMarksChildrenAbsentAtTheHandover pins that the group view
// carries the moment, class and Lehrkraft of today's handover on each child
// marked "nicht da", for callers who read plans, and fails with the port.
func TestLiveGroupMarksChildrenAbsentAtTheHandover(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.August, 21, 11, 45, 0, 0, time.UTC)
	roster := func(context.Context, int64) ([]RosterStudent, error) {
		return []RosterStudent{{ID: 11}, {ID: 12}}, nil
	}
	newFakes := func(caller Caller) *fakes {
		return &fakes{
			caller:           func(context.Context) (Caller, error) { return caller, nil },
			supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
			members:          roster,
		}
	}

	handovers := &handoverFake{absences: map[int64]HandoverAbsence{
		12: {SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft"},
	}}
	got, err := New(handoverDeps(newFakes(Caller{CanReadStudentPlans: true, CanReadGroups: true}), handovers)).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12}, handovers.asked)
	assert.Equal(t, today, handovers.day, "today's handover")
	require.Len(t, got.Students, 2)
	assert.Nil(t, got.Students[0].HandoverAbsent)
	assert.Equal(t, &HandoverAbsence{SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft"}, got.Students[1].HandoverAbsent)

	restricted := &handoverFake{absences: handovers.absences}
	got, err = New(handoverDeps(newFakes(Caller{CanReadGroups: true}), restricted)).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	assert.Zero(t, restricted.calls, "like sick and excused, only for callers who read plans")
	assert.Nil(t, got.Students[1].HandoverAbsent)

	failing := &handoverFake{err: errors.New("presence unavailable")}
	_, err = New(handoverDeps(newFakes(Caller{CanReadStudentPlans: true, CanReadGroups: true}), failing)).LiveGroup(context.Background(), 1)
	require.ErrorContains(t, err, "apply class handover absences")
}
