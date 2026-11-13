// The changed pickup time (#6855) reaches the staff responsible for the child
// who may read its pickup times. Responsibility and the right decide who gets
// the unread marker in moto; the personal "off" only stops the device
// notification, which the root's staff push filters (FilterNotOptedOut).
package notifications_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentPickupChangedNoticeFollowsTheRight(t *testing.T) {
	t.Parallel()
	def, ok := notifications.GetType(notifications.TypeStudentPickupChanged)
	require.True(t, ok)
	assert.Equal(t, permissions.UsersRead, def.Permission)
	assert.True(t, def.DefaultOn)
	assert.Equal(t, notifications.PortalStaff, def.Portal)
	assert.False(t, def.SchoolPortal, "a pickup time is OGS work, not moto schule")
}

func TestFilterPermittedKeepsWhoHoldsTheRight(t *testing.T) {
	t.Parallel()
	const reader, withoutRight, otherSchool = int64(501), int64(503), int64(504)
	accounts := fakeAccountPermissions{reader: {permissions.UsersRead}, withoutRight: {permissions.StudentsRead}}
	svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, allSettingsOn(), nil, accounts, securityruntime.HasPermission)
	ctx := tenant.WithTenantID(context.Background(), 9)

	got, err := svc.FilterPermitted(ctx, notifications.TypeStudentPickupChanged, []int64{reader, withoutRight, otherSchool})
	require.NoError(t, err)
	assert.Equal(t, []int64{reader}, got)

	got, err = svc.FilterPermitted(context.Background(), notifications.TypeStudentPickupChanged, []int64{reader})
	require.NoError(t, err)
	assert.Empty(t, got, "without a school nobody holds the right")

	got, err = svc.FilterPermitted(ctx, notifications.TypeStudentAbsenceReported, []int64{reader, withoutRight})
	require.NoError(t, err)
	assert.Equal(t, []int64{reader, withoutRight}, got, "a type without a right keeps everyone")

	_, err = svc.FilterPermitted(ctx, "not_a_type", []int64{reader})
	require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)
}

// TestPickupChangeReachesOnlyTheChildsTeam: the child's group team and the
// office are responsible; the team of another group, a person without the
// right and the person who decided are not. Someone who switched the type
// off still gets the marker, and on-duty rules do not apply to it.
func TestPickupChangeReachesOnlyTheChildsTeam(t *testing.T) {
	t.Parallel()
	const declinedStaff, declinedAccount, guestStaff, guestAccount = int64(94), int64(105), int64(95), int64(106)
	repo := &fakePreferenceRepo{optedOut: map[string][]int64{notifications.TypeStudentPickupChanged: {declinedAccount}}}
	accounts := fakeAccountPermissions{
		absenceAccountA: {permissions.UsersRead}, absenceAccountB: {permissions.UsersRead},
		absenceAdmin: {"*:*"}, declinedAccount: {permissions.UsersRead}, guestAccount: {permissions.StudentsRead},
		absenceActorAcct: {permissions.UsersRead},
	}
	preferences := notifications.NewPreferenceService(repo, allSettingsOn(), nil, accounts, securityruntime.HasPermission)
	resolver := notifications.NewStaffRecipientResolver(preferences,
		&fakeStudentReader{groups: map[int64]int64{absenceStudentA: absenceGroupA, absenceStudentB: absenceGroupB}},
		&fakeGroupReader{staffByGroup: map[int64][]int64{absenceGroupA: {absenceStaffA, declinedStaff, guestStaff}, absenceGroupB: {absenceStaffB}}},
		&fakeStaffAccountReader{accounts: map[int64]int64{
			absenceStaffA: absenceAccountA, absenceStaffB: absenceAccountB, declinedStaff: declinedAccount,
			guestStaff: guestAccount, absenceAdminStaff: absenceAdmin,
		}},
		&fakeAdminReader{ids: []int64{absenceAdmin, absenceActorAcct}},
		&fakeOnDutySetting{enabled: true},
		&fakeDutyReader{},
	)
	responsible, ok := resolver.(interface {
		ResponsibleStaffAccountIDs(context.Context, int64, string, []int64) ([]int64, error)
	})
	require.True(t, ok)

	got, err := responsible.ResponsibleStaffAccountIDs(tenant.WithTenantID(context.Background(), absenceTenant),
		absenceStudentA, notifications.TypeStudentPickupChanged, []int64{absenceActorAcct})
	require.NoError(t, err)
	assert.Equal(t, []int64{absenceAccountA, absenceAdmin, declinedAccount}, got)
}
