package timetracking_test

// Mock-based tests for the auto-check-in Student Presence calls (#3996): the
// composition maps the work session service's row to the public session and
// hands the time clock's error over unchanged. int64 literals are fake
// in-memory IDs, not DB rows.

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ensureCheckedInSessions answers EnsureCheckedIn and nothing else.
type ensureCheckedInSessions struct {
	timetracking.WorkSessionService
	session *timetracking.WorkSession
	err     error
}

func (s ensureCheckedInSessions) EnsureCheckedIn(context.Context, int64, string) (*timetracking.WorkSession, error) {
	return s.session, s.err
}

// Student Presence recognises the planned-start refusal by its shape, so the
// auto-check-in must hand the time clock's error over unchanged.
func TestAutoCheckInPassesThePlannedStartRefusalUnchanged(t *testing.T) {
	t.Parallel()
	refusal := &timetracking.PlannedStartNotReachedError{PlannedStartTime: "08:00", CurrentTime: "07:30"}
	autoCheckIn := timetracking.AutoCheckInCapability(ensureCheckedInSessions{err: refusal})

	session, err := autoCheckIn.EnsureCheckedIn(context.Background(), 7, workforce.WorkSessionSourceNFC)

	assert.Nil(t, session)
	var shaped interface {
		PlannedStartNotReached() (string, string)
	}
	require.True(t, errors.As(err, &shaped))
	planned, current := shaped.PlannedStartNotReached()
	assert.Equal(t, "08:00", planned)
	assert.Equal(t, "07:30", current)
}

func TestAutoCheckInKeepsTheCheckedOutDayNil(t *testing.T) {
	t.Parallel()
	autoCheckIn := timetracking.AutoCheckInCapability(ensureCheckedInSessions{})

	session, err := autoCheckIn.EnsureCheckedIn(context.Background(), 7, workforce.WorkSessionSourceNFC)

	require.NoError(t, err)
	assert.Nil(t, session, "a staff member who already checked out today gets no block")
}

func TestAutoCheckInReturnsTheOpenedBlock(t *testing.T) {
	t.Parallel()
	opened := &timetracking.WorkSession{
		StaffID: 7, Date: timetracking.NewDate(2026, 10, 14), Status: workforce.WorkSessionStatusPresent,
		Source: workforce.WorkSessionSourceNFC, CreatedBy: 7,
	}
	opened.ID = 11
	autoCheckIn := timetracking.AutoCheckInCapability(ensureCheckedInSessions{session: opened})

	session, err := autoCheckIn.EnsureCheckedIn(context.Background(), 7, workforce.WorkSessionSourceNFC)

	require.NoError(t, err)
	require.NotNil(t, session)
	assert.EqualValues(t, 11, session.ID)
	assert.EqualValues(t, 7, session.StaffID)
	assert.Equal(t, "2026-10-14", session.Date)
	assert.Equal(t, workforce.WorkSessionSourceNFC, session.Source)
	assert.True(t, session.IsOpen())
}
