package domain

import "errors"

var (
	// ErrFamilyProtectionInvalid reports a malformed change request.
	ErrFamilyProtectionInvalid = errors.New("invalid family protection change")
	// ErrFamilyProtectionUnchanged says the child is already in the requested
	// state, so the append-only ledger stays untouched. It travels with the
	// current state, not instead of it.
	ErrFamilyProtectionUnchanged = errors.New("family protection is already in the requested state")
)

// MaxFamilyProtectionReasonRunes bounds the free-text justification the
// append-only ledger stores with every change.
const MaxFamilyProtectionReasonRunes = 500

// FamilyProtectionChange is one requested flip of a child's privacy rule.
type FamilyProtectionChange struct {
	StudentID      int64
	Enabled        bool
	Reason         string
	ActorAccountID int64
}
