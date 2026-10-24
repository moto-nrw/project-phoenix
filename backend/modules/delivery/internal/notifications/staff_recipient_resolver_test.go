package notifications_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffRecipientResolverReachesGroupTeamAndAdmins(t *testing.T) {
	t.Parallel()

	resolver := notifications.NewStaffRecipientResolver(
		&stubConsent{allowed: map[int64]struct{}{
			absenceAccountA: {},
			absenceAdmin:    {},
		}},
		&fakeStudentReader{groups: map[int64]int64{absenceStudentA: absenceGroupA}},
		&fakeGroupReader{staffByGroup: map[int64][]int64{
			absenceGroupA: {absenceStaffA},
		}},
		&fakeStaffAccountReader{accounts: map[int64]int64{
			absenceStaffA:     absenceAccountA,
			absenceAdminStaff: absenceAdmin,
		}},
		&fakeAdminReader{ids: []int64{absenceAdmin}},
		&fakeOnDutySetting{enabled: true},
		&fakeDutyReader{onDuty: map[int64]struct{}{absenceStaffA: {}, absenceAdminStaff: {}}},
	)

	scopes, err := resolver.Resolve(context.Background(), notifications.StaffRecipientRequest{
		StudentIDs:       []int64{absenceStudentA},
		NotificationType: notifications.TypeStudentAbsenceReported,
	})

	require.NoError(t, err)
	require.Len(t, scopes, 2)
	byAccount := make(map[int64][]int64, len(scopes))
	for _, scope := range scopes {
		byAccount[scope.AccountID] = scope.StudentIDs
	}
	assert.Equal(t, []int64{absenceStudentA}, byAccount[absenceAccountA])
	assert.Equal(t, []int64{absenceStudentA}, byAccount[absenceAdmin])
}
