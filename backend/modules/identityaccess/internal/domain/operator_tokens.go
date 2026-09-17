package domain

import (
	"errors"
	"time"
	"unicode/utf8"
)

var (
	// ErrOperatorInvitationNotFound reports a lookup that matched no
	// invitation, or a redemption, revocation or extension that found no
	// invitation in a state that allows it.
	ErrOperatorInvitationNotFound = errors.New("operator invitation not found")
	// ErrOperatorEmailChangeNotFound reports a redemption that found no
	// unused, unexpired e-mail change link.
	ErrOperatorEmailChangeNotFound = errors.New("operator email change not found")
	// ErrOperatorEmailChangeActive reports a create that met the
	// one-active-link-per-operator index. The initiation answers its caller
	// with the rate limit, which is what "there is already a live link"
	// means to them.
	ErrOperatorEmailChangeActive = errors.New("an active email change link already exists")
)

// maxDeliveryErrorRunes bounds the stored delivery error so a verbose SMTP
// reply cannot grow the row without limit.
const maxDeliveryErrorRunes = 1024

// OperatorInvitation is the one-time link that lets an invitee become a
// platform operator. It is redeemable while unused and unexpired.
type OperatorInvitation struct {
	ID          int64
	Email       string
	Token       string
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedBy   int64
	DisplayName *string
	Delivery    TokenDelivery
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate rejects an invitation that cannot be stored. An invitation is
// never stored already expired or used, so no delivery can carry a dead link.
func (i *OperatorInvitation) Validate(now time.Time) error {
	switch {
	case i.Email == "":
		return errors.New("email is required")
	case i.Token == "":
		return errors.New("token value is required")
	case i.CreatedBy <= 0:
		return errors.New("created_by operator ID is required")
	case i.UsedAt != nil:
		return errors.New("token has already been used")
	case !i.ExpiresAt.After(now):
		return errors.New("token has already expired")
	}
	return nil
}

// OperatorEmailChange is the one-time link that confirms an operator's new
// e-mail address. It is redeemable while unused and unexpired.
type OperatorEmailChange struct {
	ID         int64
	OperatorID int64
	NewEmail   string
	Token      string
	Expiry     time.Time
	Used       bool
	Delivery   TokenDelivery
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate rejects an e-mail change link that cannot be stored.
func (c *OperatorEmailChange) Validate(now time.Time) error {
	switch {
	case c.OperatorID <= 0:
		return errors.New("operator ID is required")
	case c.Token == "":
		return errors.New("token value is required")
	case c.NewEmail == "":
		return errors.New("new email is required")
	case !c.Expiry.After(now):
		return errors.New("token has already expired")
	case c.Used:
		return errors.New("token has already been used")
	}
	return nil
}

// TokenDelivery is the recorded outcome of mailing a one-time link.
type TokenDelivery struct {
	SentAt     *time.Time
	Error      *string
	RetryCount int
}

// Bounded returns the delivery with its error truncated to the stored limit.
func (d TokenDelivery) Bounded() TokenDelivery {
	if d.Error == nil || utf8.RuneCountInString(*d.Error) <= maxDeliveryErrorRunes {
		return d
	}
	truncated := string([]rune(*d.Error)[:maxDeliveryErrorRunes])
	d.Error = &truncated
	return d
}
