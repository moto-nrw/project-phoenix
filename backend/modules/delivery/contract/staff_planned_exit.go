package contract

// NotificationStaffPlannedExit is the notification type of the planned
// staff exit notices (#4809): the reminders a week before and on the last
// working day, and the notice that the exit is still blocked. The root sends
// them; Delivery registers the type for the holders of users:delete.
const NotificationStaffPlannedExit = "staff_planned_exit"
