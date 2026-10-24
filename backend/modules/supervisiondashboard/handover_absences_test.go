package supervisiondashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handoverReaderFake answers the class handover port (#4794).
type handoverReaderFake struct {
	absences map[int64]HandoverAbsence
	err      error
	asked    []int64
	calls    int
}

func (h *handoverReaderFake) HandoverAbsences(_ context.Context, ids []int64, _ Date) (map[int64]HandoverAbsence, error) {
	h.calls++
	h.asked = ids
	return h.absences, h.err
}

// TestDashboardMarksChildrenAbsentAtTheHandover pins that the open visits,
// the released rooms and the planned rosters carry today's handover absence,
// read once over their union and only with users:read.
func TestDashboardMarksChildrenAbsentAtTheHandover(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.August, 19, 11, 45, 0, 0, time.UTC)
	absence := HandoverAbsence{SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft"}
	projection := func() *Projection {
		p := emptyProjection()
		p.Visits = []Visit{{StudentID: 1}, {StudentID: 2}}
		p.OpenRooms = []OpenRoom{{Students: []OpenRoomStudent{{Visit: Visit{StudentID: 2}}}}}
		p.PlannedNow = []PlannedInstance{{RosterPreview: []RosterRow{{StudentID: 3}, {StudentID: 1}}}}
		return p
	}

	reader := &handoverReaderFake{absences: map[int64]HandoverAbsence{2: absence, 3: absence}}
	got := projection()
	require.NoError(t, (&service{deps: Dependencies{Handovers: reader}}).applyHandoverAbsences(context.Background(), got, Caller{CanReadStudents: true}, "2026-08-19"))
	assert.Equal(t, []int64{1, 2, 3}, reader.asked, "one read over the union")
	assert.Nil(t, got.Visits[0].HandoverAbsent)
	assert.Equal(t, &absence, got.Visits[1].HandoverAbsent)
	assert.Equal(t, &absence, got.OpenRooms[0].Students[0].HandoverAbsent)
	assert.Equal(t, &absence, got.PlannedNow[0].RosterPreview[0].HandoverAbsent)
	assert.Nil(t, got.PlannedNow[0].RosterPreview[1].HandoverAbsent)

	guest := &handoverReaderFake{absences: reader.absences}
	got = projection()
	require.NoError(t, (&service{deps: Dependencies{Handovers: guest}}).applyHandoverAbsences(context.Background(), got, Caller{}, "2026-08-19"))
	assert.Zero(t, guest.calls, "like sick and excused, only with users:read")
	assert.Nil(t, got.Visits[1].HandoverAbsent)

	failing := &handoverReaderFake{err: errors.New("presence unavailable")}
	err := (&service{deps: Dependencies{Handovers: failing}}).applyHandoverAbsences(context.Background(), projection(), Caller{CanReadStudents: true}, "2026-08-19")
	require.ErrorContains(t, err, "load class handover absences")
}
