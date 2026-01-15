package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// PIN validation operation names
const (
	opValidateStaffPIN         = "validate staff PIN"
	opValidateStaffPINSpecific = "validate staff PIN for specific staff"
)

// ValidateStaffPIN validates a staff member's PIN and returns the staff record
func (s *personService) ValidateStaffPIN(ctx context.Context, pin string) (*userModels.Staff, error) {
	if pin == "" {
		return nil, &UsersError{Op: opValidateStaffPIN, Err: errors.New("PIN cannot be empty")}
	}

	accounts, err := s.accountRepo.List(ctx, nil)
	if err != nil {
		return nil, &UsersError{Op: opValidateStaffPIN, Err: err}
	}

	for _, account := range accounts {
		staff, err := s.tryValidatePINForAccount(ctx, account, pin)
		if err != nil {
			// Propagate repository errors immediately
			return nil, &UsersError{Op: opValidateStaffPIN, Err: err}
		}
		if staff != nil {
			return staff, nil
		}
	}

	return nil, &UsersError{Op: opValidateStaffPIN, Err: ErrInvalidPIN}
}

// tryValidatePINForAccount attempts to validate PIN for a single account and returns staff if successful
// Returns (staff, nil) if PIN is valid and staff found
// Returns (nil, nil) if PIN is invalid or account has no staff record
// Returns (nil, error) if repository operations fail
func (s *personService) tryValidatePINForAccount(ctx context.Context, account *auth.Account, pin string) (*userModels.Staff, error) {
	if !account.HasPIN() || account.IsPINLocked() {
		return nil, nil
	}

	if !account.VerifyPIN(pin) {
		s.handleFailedPINAttempt(ctx, account)
		return nil, nil
	}

	staff, err := s.findStaffByAccount(ctx, account)
	if err != nil {
		return nil, err // Propagate repository errors
	}

	if staff != nil {
		s.handleSuccessfulPINAuth(ctx, account)
		// Load person details (ignore error as this is supplementary data)
		staff.Person, _ = s.personRepo.FindByAccountID(ctx, account.ID)
		return staff, nil
	}

	return nil, nil
}

// findStaffByAccount finds staff record from account, returning error if repository operations fail
func (s *personService) findStaffByAccount(ctx context.Context, account *auth.Account) (*userModels.Staff, error) {
	person, err := s.personRepo.FindByAccountID(ctx, account.ID)
	if err != nil {
		// Distinguish between "not found" and actual errors
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found is OK - account might not be linked to person
		}
		return nil, err // Propagate repository errors
	}

	if person == nil {
		return nil, nil // No person linked to account
	}

	staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Person exists but is not staff
		}
		return nil, err // Propagate repository errors
	}

	return staff, nil
}

// handleSuccessfulPINAuth resets PIN attempts after successful authentication
func (s *personService) handleSuccessfulPINAuth(ctx context.Context, account *auth.Account) {
	account.ResetPINAttempts()
	_ = s.accountRepo.Update(ctx, account)
}

// handleFailedPINAttempt increments PIN attempts after failed authentication
func (s *personService) handleFailedPINAttempt(ctx context.Context, account *auth.Account) {
	account.IncrementPINAttempts()
	_ = s.accountRepo.Update(ctx, account)
}

// ValidateStaffPINForSpecificStaff validates a PIN for a specific staff member
func (s *personService) ValidateStaffPINForSpecificStaff(ctx context.Context, staffID int64, pin string) (*userModels.Staff, error) {
	if pin == "" {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("PIN cannot be empty")}
	}

	// Get the specific staff member
	staff, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, &UsersError{Op: "validate staff PIN for specific staff - find staff", Err: err}
	}
	if staff == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("staff member not found")}
	}

	// Get the person associated with this staff member
	person, err := s.personRepo.FindByID(ctx, staff.PersonID)
	if err != nil {
		return nil, &UsersError{Op: "validate staff PIN for specific staff - find person", Err: err}
	}
	if person == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("person not found for staff member")}
	}

	// Check if person has an account
	if person.AccountID == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("staff member has no account")}
	}

	// Get the account
	account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
	if err != nil {
		return nil, &UsersError{Op: "validate staff PIN for specific staff - find account", Err: err}
	}
	if account == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("account not found")}
	}

	// Check if account has PIN and is not locked
	if !account.HasPIN() {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("staff member has no PIN set")}
	}
	if account.IsPINLocked() {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("account is locked")}
	}

	// Verify the PIN
	if !account.VerifyPIN(pin) {
		// Increment failed attempts
		account.IncrementPINAttempts()
		if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
			// Log error but don't fail the authentication check
			_ = updateErr
		}
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: ErrInvalidPIN}
	}

	// PIN is valid - reset attempts
	account.ResetPINAttempts()
	if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
		// Log error but don't fail authentication
		_ = updateErr
	}

	// Load the person relation for the authenticated staff
	staff.Person = person

	return staff, nil
}
