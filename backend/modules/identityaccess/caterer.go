package identityaccess

import (
	"context"
	"errors"
	"time"
)

// The caterer is an identity across schools (ADR 0074): a school invites its
// caterer, the caterer accepts with a password and signs in to the caterer
// portal. The school-side flows and the acceptance run in the school's
// transaction the caller opens; the caterer's schools come from the global
// identity read (#4088).

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

// CatererInvitation is a school's invitation as the school sees it; the
// link token is shown once at creation and never stored.
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
	AccountID       int64
	CatererID       int64
	TenantID        int64
	DeliveryAddress string
}

// CatererAcceptanceHook starts the supply inside the acceptance's
// transaction, in the invitation's school.
type CatererAcceptanceHook func(ctx context.Context, acceptance CatererAcceptance) error

// CatererAccessEngine is what the composition binds behind CatererAccess.
type CatererAccessEngine interface {
	CreateCatererInvitation(ctx context.Context, request CatererInvitationRequest) (CatererInvitation, string, error)
	ListCatererInvitations(ctx context.Context) ([]CatererInvitation, error)
	RevokeCatererInvitation(ctx context.Context, invitationID int64) error
	PreviewCatererInvitation(ctx context.Context, token string) (CatererInvitationPreview, error)
	AcceptCatererInvitation(ctx context.Context, token string, registration CatererRegistration, start CatererAcceptanceHook) (CatererAcceptance, error)
	// EndCatererSchool ends the school's link to its caterer; the caller
	// ends the supply in the same transaction.
	EndCatererSchool(ctx context.Context) (bool, error)
	// CatererSchools names the caterer of an account and the schools it
	// supplies now.
	CatererSchools(ctx context.Context, accountID int64) (Caterer, []int64, error)
	LoginCatererWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
}

// CatererAccess is the capability of the caterer flows, separate from the
// wide module capability: the school's invitation, the public acceptance and
// the caterer's login.
type CatererAccess struct{ CatererAccessEngine }

func NewCatererAccess(engine CatererAccessEngine) *CatererAccess {
	if engine == nil {
		panic("caterer access requires an engine")
	}
	return &CatererAccess{CatererAccessEngine: engine}
}
