package messaging

import (
	"context"
	"errors"
	"fmt"
)

// Count scopes: which parent conversations a staff member's own counter at
// "Nachrichten" counts (#3673). The choice is personal. It changes neither the
// inbox, the per-conversation unread marks, nor anything a colleague or a
// parent sees.
const (
	// CountScopeAll counts every conversation the account may read. It is the
	// default for an account that never chose.
	CountScopeAll = "all"
	// CountScopeOwnGroups counts only children of the account's own OGS
	// groups, including groups it covers today as a substitute.
	CountScopeOwnGroups = "own_groups"
	// CountScopeNone counts nothing.
	CountScopeNone = "none"
)

// ErrInvalidCountScope means the requested count scope is not one of the
// three above.
var ErrInvalidCountScope = errors.New("messaging: invalid count scope")

// CountPreferenceStore persists the account's count scope in the current
// school. CountScope answers "" for an account that never chose.
type CountPreferenceStore interface {
	CountScope(ctx context.Context, accountID int64) (string, error)
	SetCountScope(ctx context.Context, accountID int64, scope string) error
}

// CallerGroups resolves the OGS groups of the calling staff member.
type CallerGroups interface {
	MyGroupIDs(ctx context.Context) ([]int64, error)
}

func validCountScope(scope string) bool {
	switch scope {
	case CountScopeAll, CountScopeOwnGroups, CountScopeNone:
		return true
	}
	return false
}

// CountScope returns the caller's count scope, CountScopeAll when they never
// chose. A service built without a store (unit tests) always counts all.
func (s *Service) CountScope(ctx context.Context) (string, error) {
	if s.CountPreferences == nil {
		return CountScopeAll, nil
	}
	scope, err := s.CountPreferences.CountScope(ctx, accountIDFromCtx(ctx))
	if err != nil {
		return "", fmt.Errorf("messaging: load count scope: %w", err)
	}
	if !validCountScope(scope) {
		return CountScopeAll, nil
	}
	return scope, nil
}

// SetCountScope stores the caller's count scope for the current school.
func (s *Service) SetCountScope(ctx context.Context, scope string) error {
	if !validCountScope(scope) {
		return ErrInvalidCountScope
	}
	if s.CountPreferences == nil {
		return errors.New("messaging: count preference store is not configured")
	}
	if err := s.CountPreferences.SetCountScope(ctx, accountIDFromCtx(ctx), scope); err != nil {
		return fmt.Errorf("messaging: save count scope: %w", err)
	}
	return nil
}

// HasOwnGroups reports whether the caller belongs to at least one OGS group
// today, so the setting can say when "own groups" would count nothing.
func (s *Service) HasOwnGroups(ctx context.Context) (bool, error) {
	if s.Groups == nil {
		return false, nil
	}
	groupIDs, err := s.Groups.MyGroupIDs(ctx)
	if err != nil {
		return false, fmt.Errorf("messaging: resolve own groups: %w", err)
	}
	return len(groupIDs) > 0, nil
}

// scopedUnreadCount counts the caller's unread parent messages within their
// count scope.
func (s *Service) scopedUnreadCount(ctx context.Context, accountID int64, allStudents bool) (int, error) {
	scope, err := s.CountScope(ctx)
	if err != nil {
		return 0, err
	}
	switch scope {
	case CountScopeNone:
		return 0, nil
	case CountScopeOwnGroups:
		if s.Groups == nil {
			return 0, errors.New("messaging: caller groups are not configured")
		}
		groupIDs, err := s.Groups.MyGroupIDs(ctx)
		if err != nil {
			return 0, fmt.Errorf("messaging: resolve own groups: %w", err)
		}
		return s.ReadRepo.UnreadMessageCountForStaffInGroups(ctx, accountID, allStudents, groupIDs)
	default:
		return s.ReadRepo.UnreadMessageCountForStaff(ctx, accountID, allStudents)
	}
}
