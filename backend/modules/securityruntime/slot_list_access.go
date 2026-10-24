package securityruntime

// permissionAdminAll is the system-wide admin scope.
const permissionAdminAll = "admin:*"

// SlotListStudentAccess decides whether the caller may read the children of
// the class-day slot lists by name (#1565): an administrator always may and
// is not asked for a staff record; any other account only with a staff
// record in the school, which hasStaffRecord resolves. Its failures
// propagate so a request fails loudly instead of returning an empty list.
func SlotListStudentAccess(permissions []string, hasStaffRecord func() (bool, error)) (bool, error) {
	if HasPermission(permissionAdminAll, permissions) {
		return true, nil
	}
	return hasStaffRecord()
}
