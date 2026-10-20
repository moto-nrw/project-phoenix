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

// EmploymentReset is the school's stored instruction, executed only when
// acceptance continues the same retired membership. The actor accepts the
// invitation; InvitationID preserves the school's decision in the audit note.
type EmploymentReset struct {
	PreviousPersonnelNumber *string
	StaffID                 int64
	AccountID               int64
	InvitationID            int64
	MasterData              bool
	Qualifications          bool
	WorkTime                bool
}

// EmploymentResetCommands joins the school's persisted instruction to the
// personnel owner. Prepare takes its locks before Membership is revived.
type EmploymentResetCommands struct {
	Prepare func(context.Context, EmploymentReset) (*string, error)
	Execute func(context.Context, EmploymentReset) error
}
