package compose

import "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"

// lifecycleStore is the module persistence used by lifecycle and role administration.
type lifecycleStore interface {
	ports.PermissionStore
	ports.AccountLifecycleStore
	ports.AccountLoginStore
	ports.GuardianInvitationStore
	ports.Store
}
