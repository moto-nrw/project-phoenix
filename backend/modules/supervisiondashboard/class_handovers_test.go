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
	handovers map[int64]ClassHandover
	err       error
	asked     []int64
	calls     int
}

func (h *handoverReaderFake) ClassHandovers(_ context.Context, ids []int64, _ Date) (map[int64]ClassHandover, error) {
	h.calls++
	h.asked = ids
	return h.handovers, h.err
}

// TestDashboardCarriesTheClassHandover pins that the open visits, the
// released rooms and the planned rosters carry today's handover of the
// child, "da" or "nicht da", read once over their union and only with
// users:read.
func TestDashboardCarriesTheClassHandover(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.August, 19, 11, 45, 0, 0, time.UTC)
	absent := ClassHandover{SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft", Absent: true}
	present := ClassHandover{SchoolClass: "2b", HandedOverAt: at, HandedOverBy: "Petra Lehrkraft"}
	projection := func() *Projection {
		p := emptyProjection()
		p.Visits = []Visit{{StudentID: 1, SchoolClass: "1a"}, {StudentID: 2, SchoolClass: "2b"}}
		p.OpenRooms = []OpenRoom{{Students: []OpenRoomStudent{{Visit: Visit{StudentID: 2, SchoolClass: "2b"}}}}}
		p.PlannedNow = []PlannedInstance{{RosterPreview: []RosterRow{{StudentID: 3, SchoolClass: "2b"}, {StudentID: 1, SchoolClass: "1a"}}}}
		return p
	}

	reader := &handoverReaderFake{handovers: map[int64]ClassHandover{2: absent, 3: present}}
	got := projection()
	require.NoError(t, (&service{deps: Dependencies{Handovers: reader}}).applyClassHandovers(context.Background(), got, Caller{CanReadStudents: true}, "2026-08-19"))
	assert.Equal(t, []int64{1, 2, 3}, reader.asked, "one read over the union")
	assert.Nil(t, got.Visits[0].ClassHandover)
	assert.Equal(t, &absent, got.Visits[1].ClassHandover)
	assert.Equal(t, &absent, got.OpenRooms[0].Students[0].ClassHandover)
	assert.Equal(t, &present, got.PlannedNow[0].RosterPreview[0].ClassHandover)
	assert.Nil(t, got.PlannedNow[0].RosterPreview[1].ClassHandover)

	guest := &handoverReaderFake{handovers: reader.handovers}
	got = projection()
	require.NoError(t, (&service{deps: Dependencies{Handovers: guest}}).applyClassHandovers(context.Background(), got, Caller{}, "2026-08-19"))
	assert.Zero(t, guest.calls, "like sick and excused, only with users:read")
	assert.Nil(t, got.Visits[1].ClassHandover)

	failing := &handoverReaderFake{err: errors.New("presence unavailable")}
	err := (&service{deps: Dependencies{Handovers: failing}}).applyClassHandovers(context.Background(), projection(), Caller{CanReadStudents: true}, "2026-08-19")
	require.ErrorContains(t, err, "load class handovers")
}
