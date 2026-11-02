package identityaccess

import (
	"context"
	"errors"
	"time"
)

// The caterer is an identity across schools (ADR 0077): a school invites its
// caterer, the caterer accepts with a password and signs in to the caterer
// portal. The school-side flows and the acceptance run in the school's
// transaction the caller opens; the caller also holds the school's caterer
// lock across them. The caterer's schools come from the global identity
// read, filtered on the caterer.

var (
	// ErrAccountNotCaterer refuses the caterer portal to any other account.
	ErrAccountNotCaterer = errors.New("account is not a caterer account")
	// ErrCatererWithoutSchool refuses a caterer that supplies no school.
	ErrCatererWithoutSchool = errors.New("caterer supplies no school")
	// ErrCatererAddressTaken refuses an invitation for an address that holds
	// an account of another kind.
	ErrCatererAddressTaken = errors.New("address belongs to an account that is not a caterer")
	// ErrInvitationRevoked: the school took the invitation back.
	ErrInvitationRevoked = errors.New("invitation has been revoked")
	// ErrCatererAlreadySupplies refuses a new invitation while the school's
	// current caterer still supplies it.
	ErrCatererAlreadySupplies = errors.New("the school's caterer still supplies it")
	// ErrInvalidCatererInvitation: the school's input is incomplete.
	ErrInvalidCatererInvitation = errors.New("caterer invitation needs an e-mail address and a name")
)

// Caterer is a caterer business.
type Caterer struct {
	ID   int64
	Name string
}

// CatererInvitationRequest is what a school enters to invite its caterer.
type CatererInvitationRequest struct {
	Email           string
	CatererName     string
	DeliveryAddress string
	CreatedBy       int64
}

// CatererInvitation is a school's invitation as the school sees it. The link
// goes to the caterer by mail only; identity stores just its hash.
type CatererInvitation struct {
	ID              int64
	Email           string
	CatererName     string
	DeliveryAddress string
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	RevokedAt       *time.Time
	CreatedAt       time.Time
}

// CatererInvitationPreview is what the public acceptance page shows.
type CatererInvitationPreview struct {
	Email             string
	CatererName       string
	SchoolName        string
	ExpiresAt         time.Time
	HasCatererAccount bool
	AddressTaken      bool
}

// CatererRegistration is a new password with its confirmation, or the
// password of the caterer's existing account.
type CatererRegistration struct {
	Password        string
	ConfirmPassword string
}

// CatererAcceptance is the outcome of an accepted invitation.
type CatererAcceptance struct {
	AccountID int64
	CatererID int64
	TenantID  int64
}

// CatererSupply is a school's running supply by its caterer: who supplies
// it, where to and since when (ADR 0077).
type CatererSupply struct {
	Caterer         Caterer
	DeliveryAddress string
	StartedAt       time.Time
}

// CatererAccessEngine is what the composition binds behind CatererAccess.
type CatererAccessEngine interface {
	// CreateCatererInvitation mails the link to the caterer; the school
	// never sees it.
	CreateCatererInvitation(ctx context.Context, request CatererInvitationRequest) (CatererInvitation, error)
	ListCatererInvitations(ctx context.Context) ([]CatererInvitation, error)
	RevokeCatererInvitation(ctx context.Context, invitationID int64) error
	PreviewCatererInvitation(ctx context.Context, token string) (CatererInvitationPreview, error)
	// AcceptCatererInvitation spends the invitation and starts the school's
	// supply by the caterer.
	AcceptCatererInvitation(ctx context.Context, token string, registration CatererRegistration) (CatererAcceptance, error)
	// EndCatererSchool ends the school's supply by its caterer.
	EndCatererSchool(ctx context.Context, accountID int64) (bool, error)
	// SchoolCaterer reads the school's running supply.
	SchoolCaterer(ctx context.Context) (CatererSupply, bool, error)
	// CatererSchools names the caterer of an account and the schools it
	// supplies now.
	CatererSchools(ctx context.Context, accountID int64) (Caterer, []int64, error)
	// ChangeCatererPassword replaces a signed-in caterer's password after
	// checking the current one like a login, and ends every session of the
	// account (#4496).
	ChangeCatererPassword(ctx context.Context, accountID int64, currentPassword, newPassword, ipAddress, userAgent string) error
}

// CatererAccess is the capability of the caterer flows, separate from the
// wide module capability: the school's invitation, the public acceptance,
// the caterer's sign-in with its second factor (#4497), the caterer's own
// management of it, and the operator's reset.
type CatererAccess struct {
	CatererAccessEngine
	CatererSignIn
	CatererSecondFactor
	CatererMFAAdministration
}

func NewCatererAccess(engine CatererAccessEngine, signIn CatererSignIn, secondFactor CatererSecondFactor, administration CatererMFAAdministration) *CatererAccess {
	if engine == nil || signIn == nil || secondFactor == nil || administration == nil {
		panic("caterer access requires every part")
	}
	return &CatererAccess{CatererAccessEngine: engine, CatererSignIn: signIn, CatererSecondFactor: secondFactor, CatererMFAAdministration: administration}
}
