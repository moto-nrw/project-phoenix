package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebAttendanceStaffAttributionKeepsContextAndPresence(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bound := WithAttendanceStaff(ctx, 7, 9)
	principal := AttendancePrincipal(bound)
	require.True(t, principal.HasStaff)
	require.EqualValues(t, 7, principal.StaffID)
	require.False(t, principal.IsIoT)
	require.False(t, AttendancePrincipal(ctx).HasStaff)
	zero := AttendancePrincipal(WithAttendanceStaff(bound, 0, 9))
	require.True(t, zero.HasStaff)
	require.Zero(t, zero.StaffID)
	cancel()
	require.ErrorIs(t, bound.Err(), context.Canceled)
}
