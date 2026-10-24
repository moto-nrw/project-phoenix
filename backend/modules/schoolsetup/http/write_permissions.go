package schoolsetuphttp

import "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

// WritePermissions are the permissions of which the wizard's caller must
// hold at least one: config:update, or config:manage, which includes it. The
// wizard is for school admins; the staff first steps need none.
func WritePermissions() []string {
	return []string{permissions.ConfigUpdate, permissions.ConfigManage}
}
