package usercontext

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// CurrentUserProvider retrieves the currently authenticated user and related entities
type CurrentUserProvider interface {
	// GetCurrentUser retrieves the currently authenticated user account
	GetCurrentUser(ctx context.Context) (*auth.Account, error)

	// GetCurrentPerson retrieves the person linked to the currently authenticated user
	GetCurrentPerson(ctx context.Context) (*users.Person, error)

	// GetCurrentStaff retrieves the staff member linked to the currently authenticated user
	GetCurrentStaff(ctx context.Context) (*users.Staff, error)

	// GetCurrentTeacher retrieves the teacher linked to the currently authenticated user
	GetCurrentTeacher(ctx context.Context) (*users.Teacher, error)
}

// UserGroupProvider retrieves groups associated with the current user
type UserGroupProvider interface {
	// GetMyGroups retrieves educational groups associated with the current user
	GetMyGroups(ctx context.Context) ([]*education.Group, error)

	// GetMyActivityGroups retrieves activity groups associated with the current user
	GetMyActivityGroups(ctx context.Context) ([]*activities.Group, error)

	// GetMyActiveGroups retrieves active groups associated with the current user
	GetMyActiveGroups(ctx context.Context) ([]*active.Group, error)

	// GetMySupervisedGroups retrieves active groups supervised by the current user
	GetMySupervisedGroups(ctx context.Context) ([]*active.Group, error)

	// GetActiveSubstitutionGroupIDs returns group IDs where the staff member is an active substitute
	GetActiveSubstitutionGroupIDs(ctx context.Context, staffID int64) (map[int64]bool, error)
}

// GroupAccessProvider retrieves students and visits for accessible groups
type GroupAccessProvider interface {
	// GetGroupStudents retrieves students in a specific group where the current user has access
	GetGroupStudents(ctx context.Context, groupID int64) ([]*users.Student, error)

	// GetGroupVisits retrieves active visits for a specific group where the current user has access
	GetGroupVisits(ctx context.Context, groupID int64) ([]*active.Visit, error)
}

// ProfileManager handles profile CRUD operations for the current user
type ProfileManager interface {
	// GetCurrentProfile retrieves the full profile for the current user
	GetCurrentProfile(ctx context.Context) (map[string]interface{}, error)

	// UpdateCurrentProfile updates the current user's profile with the provided data
	UpdateCurrentProfile(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error)
}

// AvatarManager handles avatar operations for the current user
type AvatarManager interface {
	// UpdateAvatar updates the current user's avatar
	UpdateAvatar(ctx context.Context, avatarURL string) (map[string]interface{}, error)

	// UploadAvatar handles the complete avatar upload flow including validation and file storage
	UploadAvatar(ctx context.Context, input AvatarUploadInput) (map[string]interface{}, error)

	// DeleteAvatar removes the current user's avatar
	DeleteAvatar(ctx context.Context) (map[string]interface{}, error)

	// ValidateAvatarAccess checks if the current user can access the requested avatar
	ValidateAvatarAccess(ctx context.Context, filename string) error
}

// UserContextService composes all user context operations.
// Existing callers can continue using this full interface.
// New code can depend on smaller sub-interfaces for better decoupling.
type UserContextService interface {
	base.TransactionalService
	CurrentUserProvider
	UserGroupProvider
	GroupAccessProvider
	ProfileManager
	AvatarManager
}
