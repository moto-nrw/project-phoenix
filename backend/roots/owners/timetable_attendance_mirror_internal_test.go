package owners

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingAttendanceMirror records the batch checkout it receives. Every
// other mirror call panics on the nil embedded mirror, so a nil visit that
// reached the Timetable owner would fail the test.
type recordingAttendanceMirror struct {
	timetable.AttendanceMirror
	checkOutBatches [][]timetable.AttendanceVisit
}

func (m *recordingAttendanceMirror) MirrorCheckOutForVisits(_ context.Context, visits []timetable.AttendanceVisit, _ time.Time) error {
	m.checkOutBatches = append(m.checkOutBatches, visits)
	return nil
}

// The nil-visit cases of Student Presence's syncer port end in the root
// adapter; the mirror's own branches are tested with the Timetable owner
// (#4076).
func TestPresenceAttendanceMirrorNilVisitIsNoop(t *testing.T) {
	t.Parallel()
	mirror := &recordingAttendanceMirror{}
	syncer := PresenceAttendanceMirror{Mirror: mirror}

	checkInSnapshot, checkInErr := syncer.MirrorCheckInForVisit(context.Background(), nil)
	require.NoError(t, checkInErr)
	assert.Nil(t, checkInSnapshot)

	checkOutSnapshot, checkOutErr := syncer.MirrorCheckOutForVisit(context.Background(), nil)
	require.NoError(t, checkOutErr)
	assert.Nil(t, checkOutSnapshot)

	require.NoError(t, syncer.MirrorCheckOutForVisits(context.Background(), []*studentpresence.Visit{nil}, time.Now()))
	require.Len(t, mirror.checkOutBatches, 1)
	assert.Empty(t, mirror.checkOutBatches[0], "nil visit entries are skipped")
}
