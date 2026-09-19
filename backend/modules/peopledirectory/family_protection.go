package peopledirectory

import (
	"context"
	"errors"
	"strings"
)

var (
	// ErrFamilyProtectionInvalid reports a malformed change request.
	ErrFamilyProtectionInvalid = errors.New("invalid family protection change")
	// ErrFamilyProtectionUnchanged says the child is already in the requested
	// state, so the append-only ledger stays untouched. It travels with the
	// current state, not instead of it: the caller renders the state it asked
	// for and tells the user nothing changed.
	ErrFamilyProtectionUnchanged = errors.New("family protection is already in the requested state")
)

// MaxFamilyProtectionReasonRunes bounds the free-text justification the
// append-only ledger stores with every change.
const MaxFamilyProtectionReasonRunes = 500

// SetFamilyProtection is one requested flip of a child's privacy rule. Reason
// is mandatory: the ledger is an audit trail, not a toggle.
type SetFamilyProtection struct {
	StudentID      int64
	Enabled        bool
	Reason         string
	ActorAccountID int64
}

// FamilyProtectionQuery reads the current privacy flag without exposing the
// audit event, its reason, or the account that changed it.
type FamilyProtectionQuery interface {
	CurrentFamilyProtection(context.Context, []int64) (map[int64]bool, error)
}

// FamilyProtectionCommand appends to the privacy ledger. Deciding WHO may ask
// stays with the caller; this owner decides what a valid change is and keeps
// the ledger consistent with the child's lifecycle.
type FamilyProtectionCommand interface {
	// SetFamilyProtection records the change and returns the state that is now
	// current. ErrFamilyProtectionUnchanged accompanies an unchanged state,
	// ErrStudentNotFound a missing or graduated child.
	SetFamilyProtection(context.Context, SetFamilyProtection) (bool, error)
}

// CurrentFamilyProtection returns the latest event's flag for each requested
// child in the tenant. Children without events are absent, meaning unprotected.
func (m *Module) CurrentFamilyProtection(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	for _, id := range studentIDs {
		if id <= 0 {
			return nil, &InvalidStudentError{Reason: "family protection requires positive student IDs"}
		}
	}
	if len(studentIDs) == 0 {
		return map[int64]bool{}, nil
	}
	return m.engine.CurrentFamilyProtection(ctx, uniquePositive(studentIDs))
}

func (m *Module) SetFamilyProtection(ctx context.Context, input SetFamilyProtection) (bool, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.StudentID <= 0 || input.ActorAccountID <= 0 || input.Reason == "" ||
		len([]rune(input.Reason)) > MaxFamilyProtectionReasonRunes {
		return false, ErrFamilyProtectionInvalid
	}
	return m.engine.SetFamilyProtection(ctx, input)
}
