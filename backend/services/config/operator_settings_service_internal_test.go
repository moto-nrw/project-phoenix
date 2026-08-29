package config

import (
	"context"
	"errors"
	"fmt"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAttendanceChecker records whether HasOpenAttendanceOn was called and
// returns a canned result. A nil-valued *stubAttendanceChecker is never used
// by the early-return guard paths, mirroring the pre-extraction tests that
// passed a nil active service.
type stubAttendanceChecker struct {
	called bool
	exists bool
	err    error
}

func (s *stubAttendanceChecker) HasOpenAttendanceOn(_ context.Context, _ configModel.CalendarDate) (bool, error) {
	s.called = true
	return s.exists, s.err
}

func TestCheckPresenceModeSwitch_IgnoresOtherKeys(t *testing.T) {
	t.Parallel()

	err := CheckPresenceModeSwitch(
		context.Background(),
		nil,
		configModel.CalendarDate{},
		configModel.KeyCheckoutSchulhofEnabled,
		false,
	)
	assert.NoError(t, err)
}

func TestCheckPresenceModeSwitch_ForceBypass(t *testing.T) {
	t.Parallel()

	err := CheckPresenceModeSwitch(
		context.Background(),
		nil,
		configModel.CalendarDate{},
		configModel.KeyPresenceMode,
		true,
	)
	assert.NoError(t, err)
}

func TestCheckPresenceModeSwitch_NonPresenceForceIsNoop(t *testing.T) {
	t.Parallel()

	err := CheckPresenceModeSwitch(
		context.Background(),
		nil,
		configModel.CalendarDate{},
		configModel.KeyCheckoutSchulhofEnabled,
		true,
	)
	assert.NoError(t, err)
}

func TestCheckPresenceModeSwitch_NilCheckerIsNoop(t *testing.T) {
	t.Parallel()

	err := CheckPresenceModeSwitch(
		context.Background(),
		nil,
		configModel.CalendarDate{},
		configModel.KeyPresenceMode,
		false,
	)
	assert.NoError(t, err)
}

func TestCheckPresenceModeSwitch_BlocksWithOpenAttendance(t *testing.T) {
	t.Parallel()

	checker := &stubAttendanceChecker{exists: true}
	err := CheckPresenceModeSwitch(
		context.Background(),
		checker,
		configModel.CalendarDate{},
		configModel.KeyPresenceMode,
		false,
	)
	require.True(t, checker.called)
	assert.ErrorIs(t, err, ErrPresenceModeSwitchBlocked)
}

func TestCheckPresenceModeSwitch_PassesWithNoOpenAttendance(t *testing.T) {
	t.Parallel()

	checker := &stubAttendanceChecker{exists: false}
	err := CheckPresenceModeSwitch(
		context.Background(),
		checker,
		configModel.CalendarDate{},
		configModel.KeyPresenceMode,
		false,
	)
	require.True(t, checker.called)
	assert.NoError(t, err)
}

func TestCheckPresenceModeSwitch_WrapsCheckerError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	checker := &stubAttendanceChecker{err: sentinel}
	err := CheckPresenceModeSwitch(
		context.Background(),
		checker,
		configModel.CalendarDate{},
		configModel.KeyPresenceMode,
		false,
	)
	assert.ErrorIs(t, err, sentinel)
}

func TestErrPresenceModeSwitchBlocked_IsSentinel(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("set value: %w", ErrPresenceModeSwitchBlocked)
	assert.True(t, errors.Is(wrapped, ErrPresenceModeSwitchBlocked))

	other := errors.New(errPresenceModeSwitchBlockedMsg)
	assert.False(t, errors.Is(other, ErrPresenceModeSwitchBlocked))
}
