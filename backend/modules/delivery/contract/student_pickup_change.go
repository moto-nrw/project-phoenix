package contract

import "context"

// NotificationStudentPickupChanged is the notification type of a child's
// changed one-day pickup time (#6855). The root sends it; Delivery registers
// it for the holders of users:read, who may read the pickup times.
const NotificationStudentPickupChanged = "student_pickup_changed"

// ResponsibleStaff names the staff responsible for a child: the supervisors
// of the child's group and the school's admins, narrowed to the accounts
// that hold the notification type's permission. It applies no personal
// choice and no on-duty rule; whoever sends a device notification filters
// the switched-off recipients itself.
type ResponsibleStaff interface {
	ResponsibleStaffAccountIDs(ctx context.Context, studentID int64, notificationType string, excludedAccountIDs []int64) ([]int64, error)
}
