package usercontext

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// CurrentUserProvider implementation
// These methods retrieve the currently authenticated user and related entities

// getUserIDFromContext extracts the user ID from the JWT context
func (s *userContextService) getUserIDFromContext(ctx context.Context) (int, error) {
	// Try to get claims from context
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0, &UserContextError{Op: "get user ID from context", Err: ErrUserNotAuthenticated}
	}
	return claims.ID, nil
}

// GetCurrentUser retrieves the currently authenticated user account
func (s *userContextService) GetCurrentUser(ctx context.Context) (*auth.Account, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	account, err := s.accountRepo.FindByID(ctx, int64(userID))
	if err != nil {
		return nil, &UserContextError{Op: "get current user", Err: err}
	}
	if account == nil {
		return nil, &UserContextError{Op: "get current user", Err: ErrUserNotFound}
	}

	return account, nil
}

// GetCurrentPerson retrieves the person linked to the currently authenticated user
func (s *userContextService) GetCurrentPerson(ctx context.Context) (*users.Person, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	person, err := s.personRepo.FindByAccountID(ctx, int64(userID))
	if err != nil {
		return nil, &UserContextError{Op: "get current person", Err: err}
	}
	if person == nil {
		return nil, &UserContextError{Op: "get current person", Err: ErrUserNotLinkedToPerson}
	}

	return person, nil
}

// GetCurrentStaff retrieves the staff member linked to the currently authenticated user
func (s *userContextService) GetCurrentStaff(ctx context.Context) (*users.Staff, error) {
	person, err := s.GetCurrentPerson(ctx)
	if err != nil {
		return nil, err
	}

	staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		// Check if it's a "no rows" error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &UserContextError{Op: opGetCurrentStaff, Err: ErrUserNotLinkedToStaff}
		}
		return nil, &UserContextError{Op: opGetCurrentStaff, Err: err}
	}
	if staff == nil {
		return nil, &UserContextError{Op: opGetCurrentStaff, Err: ErrUserNotLinkedToStaff}
	}

	return staff, nil
}

// GetCurrentTeacher retrieves the teacher linked to the currently authenticated user
func (s *userContextService) GetCurrentTeacher(ctx context.Context) (*users.Teacher, error) {
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		return nil, err
	}

	teacher, err := s.teacherRepo.FindByStaffID(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get current teacher", Err: err}
	}
	if teacher == nil {
		return nil, &UserContextError{Op: "get current teacher", Err: ErrUserNotLinkedToTeacher}
	}

	return teacher, nil
}
