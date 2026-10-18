package staffclock

import (
	"context"
	"testing"
	"time"

	clockcontract "github.com/moto-nrw/project-phoenix/workflows/staffclock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingTimeClock keeps the stamps the workflow files.
type recordingTimeClock struct {
	*stubTimeClock
	stamps []clockcontract.Stamp
}

func (c *recordingTimeClock) CheckIn(ctx context.Context, stamp clockcontract.Stamp) (clockcontract.Session, error) {
	c.stamps = append(c.stamps, stamp)
	return c.stubTimeClock.CheckIn(ctx, stamp)
}

func TestKioskStampsCarryTheCardScanSource(t *testing.T) {
	t.Parallel()
	staffID := testStaffID
	clock := &fakeClock{now: func() time.Time { return time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC) }}
	stamps := &recordingTimeClock{stubTimeClock: &stubTimeClock{clock: clock}}
	service := NewService(clockcontract.Dependencies{
		Cards:     &stubCards{card: &clockcontract.Card{ID: testTag, Active: true}},
		Staff:     &stubStaff{identity: clockcontract.StaffIdentity{StaffID: staffID, FullName: "Nora Kiosk"}},
		TimeClock: stamps,
		Clock:     clock,
	})

	_, err := service.ExecuteStaffClock(context.Background(), checkInCommand())

	require.NoError(t, err)
	require.Len(t, stamps.stamps, 1)
	assert.Equal(t, clockcontract.SourceNFC, stamps.stamps[0].Source)
	assert.Equal(t, "nfc", stamps.stamps[0].Source)
}
