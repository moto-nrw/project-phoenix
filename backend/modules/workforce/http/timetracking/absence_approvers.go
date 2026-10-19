package timetracking

import "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

// AbsenceApproverPermission is the permission of the staff members who decide
// absence requests: they open the approval routes and receive the mail of a
// new request (vacation:approve).
const AbsenceApproverPermission = permissions.VacationApprove
