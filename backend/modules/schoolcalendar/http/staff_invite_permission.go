package staffcalendarhttp

import "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

// StaffInvitePermission is the permission a staff member's account must hold
// to be invited to appointments and to receive the calendar feed: personal
// calendar access (calendar:own). A colleague without it cannot use the
// calendar, so the picker never offers them.
const StaffInvitePermission = permissions.CalendarOwn
