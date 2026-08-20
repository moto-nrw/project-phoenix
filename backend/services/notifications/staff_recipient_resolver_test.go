package notifications_test

import (
	"context"
	"testing"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffRecipientResolverReachesGroupTeamAndAdmins(t *testing.T) {
	t.Parallel()

	groupID := absenceGroupA
	resolver := notifications.NewStaffRecipientResolver(
		&stubConsent{allowed: map[int64]struct{}{
			absenceAccountA: {},
			absenceAdmin:    {},
		}},
		&fakeStudentReader{byID: map[int64]*userModel.Student{
			absenceStudentA: {GroupID: &groupID},
		}},
		&fakeGroupReader{staffByGroup: map[int64][]int64{
			absenceGroupA: {absenceStaffA},
		}},
		&fakeStaffAccountReader{accounts: map[int64]int64{
			absenceStaffA:     absenceAccountA,
			absenceAdminStaff: absenceAdmin,
		}},
		&fakeAdminReader{ids: []int64{absenceAdmin}},
		&fakeOnDutySetting{enabled: true},
		&fakeDutyReader{presence: map[int64]string{
			absenceStaffA:     activeModel.WorkSessionStatusPresent,
			absenceAdminStaff: activeModel.WorkSessionStatusPresent,
		}},
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
