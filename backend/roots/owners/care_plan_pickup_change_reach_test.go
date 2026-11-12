package owners

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	deliveryCompose "github.com/moto-ogs/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/tenant"
)

type responsibleStaffFunc func(context.Context, int64, string, []int64) ([]int64, error)

func (f responsibleStaffFunc) ResponsibleStaffAccountIDs(ctx context.Context, studentID int64, kind string, excluded []int64) ([]int64, error) {
	return f(ctx, studentID, kind, excluded)
}

// The pickup change notice (#6855) asks Delivery's responsibility rule under
// its own type and sends the device notification through the shared staff
// push, which drops the people who switched the type off.
func TestPickupChangeReachUsesTheSharedStaffPush(t *testing.T) {
	t.Parallel()
	var sent deliveryCompose.NotificationEvent
	reach := pickupChangeTeamReach{
		staff: responsibleStaffFunc(func(_ context.Context, studentID int64, kind string, excluded []int64) ([]int64, error) {
			assert.Equal(t, int64(77), studentID)
			assert.Equal(t, deliveryContract.NotificationStudentPickupChanged, kind)
			assert.Equal(t, []int64{9}, excluded)
			return []int64{11, 12}, nil
		}),
		push: StaffPushNotifier{
			preferences: staffPushPreferencesFunc(func(_ context.Context, kind string, ids []int64) ([]int64, error) {
				assert.Equal(t, deliveryContract.NotificationStudentPickupChanged, kind)
				return ids[1:], nil // 11 switched it off
			}),
			router: staffPushRouterFunc(func(_ context.Context, event deliveryCompose.NotificationEvent) error {
				sent = event
				return nil
			}),
		},
		now: func() time.Time { return time.Unix(100, 0) },
	}

	responsible, err := reach.ResponsibleAccountIDs(t.Context(), 77, 9)
	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12}, responsible)

	ctx := tenant.WithTenantID(t.Context(), 4)
	require.NoError(t, reach.NotifyPickupChanged(ctx, careplan.PickupChangePush{
		StudentID: 77, RequestID: 501, RecipientAccountIDs: responsible, Title: "Abholzeit geändert", Body: "B", DeepLink: "/students/77",
	}))
	assert.Equal(t, deliveryContract.NotificationStudentPickupChanged, sent.Type)
	assert.Equal(t, []int64{12}, sent.Audience.StaffAccountIDs)
	assert.Equal(t, int64(4), sent.Audience.TenantID)
	assert.Equal(t, "/students/77", sent.DeepLink)
	assert.Equal(t, "care_schedule_request", sent.RelatedType)
	assert.Equal(t, int64(501), sent.RelatedID)
	assert.Equal(t, "student_pickup_changed:501:false:100000000000", sent.IdempotencyKey)
}

func TestPickupChangeReachNeedsDeliveryNotifications(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewPickupChangeTeamReach(DeliveryNotifications{}), "without Delivery nobody is told")
}
