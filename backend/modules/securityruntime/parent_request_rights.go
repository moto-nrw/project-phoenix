package securityruntime

// PermissionUsersAbsence lets a caller maintain a child's absence statuses and
// decide the families' sick and excused requests. It restates the permission
// registry's name, which this public package may not import; a test pins it.
const PermissionUsersAbsence = "users:absence"

// ParentRequestReviewRights reports which parent-request queues the
// permissions may decide: users:update decides every queue, users:absence
// the sick and excused queue only (#2232).
func ParentRequestReviewRights(permissions []string) (writeQueues, absences bool) {
	writeQueues = HasPermission(PermissionUsersUpdate, permissions)
	return writeQueues, writeQueues || HasPermission(PermissionUsersAbsence, permissions)
}
