package notifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type moduleSwitchStub map[string]bool

func (s moduleSwitchStub) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	return s[key], nil
}

func (moduleSwitchStub) ResolveStringForTenant(context.Context, int64, string) (string, error) {
	return "", nil
}

// A colleague's message reaches the Lehrkraft's device in moto schule only
// while the school booked Unterricht am Vormittag (#4895); the OGS staff
// keep theirs.
func TestSchoolPortalDevicesNeedUnterrichtVormittag(t *testing.T) {
	t.Parallel()

	event := Event{Type: TypeStaffMessage, Audience: Audience{TenantID: 41, Scope: ScopeStaff, StaffAccountIDs: []int64{7}}}
	closed := &router{settings: moduleSwitchStub{"module.unterricht_vormittag": false}}
	deliver, err := closed.applyModuleBooking(context.Background(), &event)
	require.NoError(t, err)
	assert.True(t, deliver, "the staff portal still gets the message")
	assert.True(t, event.schoolPortalClosed)

	booked := Event{Type: TypeStaffMessage, Audience: Audience{TenantID: 41, Scope: ScopeStaff, StaffAccountIDs: []int64{7}}}
	open := &router{settings: moduleSwitchStub{"module.unterricht_vormittag": true}}
	_, err = open.applyModuleBooking(context.Background(), &booked)
	require.NoError(t, err)
	assert.False(t, booked.schoolPortalClosed)
}
