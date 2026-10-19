package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// InvitationGrantPolicy answers whether the inviting account may hand out
// the role, given its own permissions and the role's.
type InvitationGrantPolicy interface {
	CanGrantRole(role identityaccess.RoleFacts, actorPermissions, rolePermissions []string) bool
}

// SchoolInvitationDelivery mails an invitation. The module decides the
// portal; the root resolves its host and the reply-to identity and records
// the outcome back through the module.
type SchoolInvitationDelivery interface {
	DispatchSchoolInvitation(ctx context.Context, invitation identityaccess.SchoolInvitation, schoolName string, portal identityaccess.InvitationPortal, expiry time.Duration) error
	// QueueSchoolWelcome persists the welcome before the invitation commits.
	// A resend calls DispatchSchoolInvitation only, so it never repeats the
	// welcome.
	QueueSchoolWelcome(ctx context.Context, invitation identityaccess.SchoolInvitation, schoolName string, portal identityaccess.InvitationPortal, rolePermissions []string) error
}
