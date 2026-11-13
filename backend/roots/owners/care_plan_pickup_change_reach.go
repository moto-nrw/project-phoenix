package owners

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// pickupChangeTeamReach binds Care Plan's notice about a changed pickup time
// (#6855) to Delivery: who is responsible for the child is Delivery's
// responsibility rule (group supervisors and admins who may read pickup
// times), and the device notification is the shared staff push, which drops
// the people who switched the type off.
type pickupChangeTeamReach struct {
	staff deliveryContract.ResponsibleStaff
	push  StaffPushNotifier
	now   func() time.Time
}

var _ careplan.PickupChangeTeamReach = pickupChangeTeamReach{}

// NewPickupChangeTeamReach binds the reach over the root's notifications.
// Without Delivery's notifications it is nil, and Care Plan tells nobody.
func NewPickupChangeTeamReach(notifications DeliveryNotifications) careplan.PickupChangeTeamReach {
	if notifications.ResponsibleStaff == nil || notifications.Notifier == nil || notifications.Preferences == nil {
		return nil
	}
	return pickupChangeTeamReach{staff: notifications.ResponsibleStaff, push: NewStaffPushNotifier(notifications), now: time.Now}
}

func (r pickupChangeTeamReach) ResponsibleAccountIDs(ctx context.Context, studentID, excludedAccountID int64) ([]int64, error) {
	if r.staff == nil {
		return nil, errors.New("pickup change team: responsible staff are not configured")
	}
	accounts, err := r.staff.ResponsibleStaffAccountIDs(ctx, studentID, deliveryContract.NotificationStudentPickupChanged, []int64{excludedAccountID})
	if err != nil {
		return nil, fmt.Errorf("pickup change team: responsible staff: %w", err)
	}
	return accounts, nil
}

// NotifyPickupChanged sends one push per decision. Approving again after a
// correction is a new decision, so the key carries the moment.
func (r pickupChangeTeamReach) NotifyPickupChanged(ctx context.Context, push careplan.PickupChangePush) error {
	return r.push.Notify(ctx, StaffPush{
		TenantID: tenant.FromContext(ctx), Type: deliveryContract.NotificationStudentPickupChanged,
		IdempotencyKey: fmt.Sprintf("student_pickup_changed:%d:%t:%d", push.RequestID, push.Withdrawn, r.now().UnixNano()),
		RelatedType:    "care_schedule_request", RelatedID: push.RequestID,
		Title: push.Title, Body: push.Body, DeepLink: push.DeepLink, AccountIDs: push.RecipientAccountIDs,
	})
}
