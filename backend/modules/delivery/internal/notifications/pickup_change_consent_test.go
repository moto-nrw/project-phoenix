// The changed pickup time (#6855) reaches the responsible staff without a
// prior opt-in: it is on until a person switches it off, and only for whoever
// may read the child's pickup times. FilterDeliverable is the shared rule.
package notifications_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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

func TestFilterDeliverableDefaultOnNeedsNoOptInButTheRight(t *testing.T) {
	t.Parallel()
	const reader, declined, withoutRight, otherSchool = int64(501), int64(502), int64(503), int64(504)
	repo := &fakePreferenceRepo{optedOut: map[string][]int64{notifications.TypeStudentPickupChanged: {declined}}}
	accounts := fakeAccountPermissions{
		reader:       {permissions.UsersRead},
		declined:     {permissions.UsersRead},
		withoutRight: {permissions.StudentsRead},
	}
	svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, accounts, securityruntime.HasPermission)
	ctx := tenant.WithTenantID(context.Background(), 9)

	got, err := svc.FilterDeliverable(ctx, notifications.TypeStudentPickupChanged, []int64{reader, declined, withoutRight, otherSchool})
	require.NoError(t, err)
	assert.Equal(t, []int64{reader}, got, "on by default, minus who switched it off and who may not read pickup times")

	got, err = svc.FilterDeliverable(context.Background(), notifications.TypeStudentPickupChanged, []int64{reader})
	require.NoError(t, err)
	assert.Empty(t, got, "without a school nobody holds the right")
}

func TestFilterDeliverableKeepsOptInTypesConsentFirst(t *testing.T) {
	t.Parallel()
	repo := &fakePreferenceRepo{optedIn: map[string][]int64{notifications.TypeStudentAbsenceReported: {601}}}
	svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, fakeAccountPermissions{}, securityruntime.HasPermission)

	got, err := svc.FilterDeliverable(tenant.WithTenantID(context.Background(), 9), notifications.TypeStudentAbsenceReported, []int64{601, 602})
	require.NoError(t, err)
	assert.Equal(t, []int64{601}, got)

	_, err = svc.FilterDeliverable(context.Background(), "not_a_type", []int64{601})
	require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)
}

// TestPickupChangeReachesOnlyTheChildsTeam resolves recipients through the
// shared resolver and the real consent rule: the child's group team and the
// office hear about it without opting in; the team of another group, a
// person who switched it off and the family itself do not.
func TestPickupChangeReachesOnlyTheChildsTeam(t *testing.T) {
	t.Parallel()
	const declinedStaff, declinedAccount = int64(94), int64(105)
	repo := &fakePreferenceRepo{optedOut: map[string][]int64{notifications.TypeStudentPickupChanged: {declinedAccount}}}
	accounts := fakeAccountPermissions{
		absenceAccountA: {permissions.UsersRead}, absenceAccountB: {permissions.UsersRead},
		absenceAdmin: {"*:*"}, declinedAccount: {permissions.UsersRead},
	}
	preferences := notifications.NewPreferenceService(repo, allSettingsOn(), nil, accounts, securityruntime.HasPermission)
	resolver := notifications.NewStaffRecipientResolver(preferences,
		&fakeStudentReader{groups: map[int64]int64{absenceStudentA: absenceGroupA, absenceStudentB: absenceGroupB}},
		&fakeGroupReader{staffByGroup: map[int64][]int64{absenceGroupA: {absenceStaffA, declinedStaff}, absenceGroupB: {absenceStaffB}}},
		&fakeStaffAccountReader{accounts: map[int64]int64{
			absenceStaffA: absenceAccountA, absenceStaffB: absenceAccountB, declinedStaff: declinedAccount, absenceAdminStaff: absenceAdmin,
		}},
		&fakeAdminReader{ids: []int64{absenceAdmin}},
		&fakeOnDutySetting{},
		&fakeDutyReader{},
	)
	capture := &captureNotifier{}
	producer := notifications.NewAbsenceNotifier(capture, resolver, nil, nil).(notifications.PickupChangeNotifier)

	require.NoError(t, producer.NotifyPickupChanged(tenant.WithTenantID(context.Background(), absenceTenant), notifications.PickupChangeReport{
		TenantID: absenceTenant, StudentID: absenceStudentA, Date: calendar.NewDate(2099, 3, 2), PickupTime: "14:30", ActorAccountID: absenceActorAcct,
	}))

	require.Len(t, capture.events, 1)
	assert.Equal(t, []int64{absenceAccountA, absenceAdmin}, capture.events[0].Audience.StaffAccountIDs)
	assert.Equal(t, absenceTenant, capture.events[0].Audience.TenantID)
}
