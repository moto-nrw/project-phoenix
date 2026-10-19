package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
)

// The caterer is an identity across schools (ADR 0074): the business, its
// login accounts, and the invitation a school sends it.

var (
	// ErrAccountNotCaterer refuses the caterer portal to an account that
	// belongs to no caterer.
	ErrAccountNotCaterer = errors.New("account is not a caterer account")
	// ErrCatererWithoutSchool refuses a caterer that supplies no school any
	// more: there is nothing to sign in to.
	ErrCatererWithoutSchool = errors.New("caterer supplies no school")
	// ErrCatererAddressTaken refuses an invitation for an address that holds
	// an account of another kind; one address is one account.
	ErrCatererAddressTaken = errors.New("address belongs to an account that is not a caterer")
	// ErrInvitationRevoked: the school took the invitation back.
	ErrInvitationRevoked = errors.New("invitation has been revoked")
	// ErrCatererAlreadySupplies refuses a new invitation while the school's
	// current caterer still supplies it: the school ends that supply first.
	ErrCatererAlreadySupplies = errors.New("the school's caterer still supplies it")
	// ErrInvalidCatererInvitation: the school's input is incomplete.
	ErrInvalidCatererInvitation = errors.New("caterer invitation needs an e-mail address and a name")
)

// CatererRoleName is the role a caterer token carries. It is no role of
// auth.roles: a caterer holds no permission at any school.
const CatererRoleName = "caterer"

// CatererInvitationLifetime is how long a school's invitation stays open.
const CatererInvitationLifetime = 14 * 24 * time.Hour

// Caterer is a caterer business.
type Caterer struct {
	ID   int64
	Name string
}

// CatererInvitation is a school's invitation to its caterer. Only the hash
// of the link token is kept.
type CatererInvitation struct {
	ID                int64
	TenantID          int64
	TokenHash         string
	Email             string
	CatererName       string
	DeliveryAddress   string
	CreatedBy         *int64
	ExpiresAt         time.Time
	AcceptedAt        *time.Time
	AcceptedAccountID *int64
	RevokedAt         *time.Time
	CreatedAt         time.Time
}

// CatererInvitationRequest is what a school enters to invite its caterer.
type CatererInvitationRequest struct {
	Email           string
	CatererName     string
	DeliveryAddress string
	CreatedBy       int64
}

// Normalized trims the request and checks the address and the name.
func (r CatererInvitationRequest) Normalized() (CatererInvitationRequest, error) {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.CatererName = strings.TrimSpace(r.CatererName)
	r.DeliveryAddress = strings.TrimSpace(r.DeliveryAddress)
	parsed, err := mail.ParseAddress(r.Email)
	if err != nil || parsed.Address != r.Email || r.CatererName == "" || len(r.CatererName) > 200 || len(r.DeliveryAddress) > 500 || r.CreatedBy <= 0 {
		return r, ErrInvalidCatererInvitation
	}
	return r, nil
}

// Redeemable reports why an invitation can no longer be accepted.
func (i CatererInvitation) Redeemable(now time.Time) error {
	switch {
	case i.RevokedAt != nil:
		return ErrInvitationRevoked
	case i.AcceptedAt != nil:
		return ErrInvitationUsed
	case now.After(i.ExpiresAt):
		return ErrInvitationExpired
	default:
		return nil
	}
}

// Open reports whether the invitation still waits for the caterer.
func (i CatererInvitation) Open(now time.Time) bool {
	return i.Redeemable(now) == nil
}

// CatererRegistration is what the caterer enters when it accepts: a new
// password with its confirmation, or the password of its existing caterer
// account.
type CatererRegistration struct {
	Password        string
	ConfirmPassword string
}

// CatererAcceptance is the outcome of an accepted invitation: the account
// and caterer the school is now supplied by, and the school's delivery
// address for the supply.
type CatererAcceptance struct {
	AccountID       int64
	CatererID       int64
	TenantID        int64
	DeliveryAddress string
}

// CatererInvitationPreview is what the public acceptance page shows. With
// HasCatererAccount the caterer confirms with its existing password; with
// AddressTaken the address holds an account of another kind and the
// invitation cannot be accepted.
type CatererInvitationPreview struct {
	Email             string
	CatererName       string
	SchoolName        string
	ExpiresAt         time.Time
	HasCatererAccount bool
	AddressTaken      bool
}

// HashInvitationToken is the stored form of a link token.
func HashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
