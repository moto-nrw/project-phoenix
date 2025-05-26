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

// UserContextService defines operations available in the user context service layer
type UserContextService interface {
	base.TransactionalService

	// GetCurrentUser retrieves the currently authenticated user account
	GetCurrentUser(ctx context.Context) (*auth.Account, error)

	// GetCurrentPerson retrieves the person linked to the currently authenticated user
	GetCurrentPerson(ctx context.Context) (*users.Person, error)

	// GetCurrentStaff retrieves the staff member linked to the currently authenticated user
	GetCurrentStaff(ctx context.Context) (*users.Staff, error)

	// GetCurrentTeacher retrieves the teacher linked to the currently authenticated user
	GetCurrentTeacher(ctx context.Context) (*users.Teacher, error)

	// GetMyGroups retrieves educational groups associated with the current user
	GetMyGroups(ctx context.Context) ([]*education.Group, error)

	// GetMyActivityGroups retrieves activity groups associated with the current user
	GetMyActivityGroups(ctx context.Context) ([]*activities.Group, error)

	// GetMyActiveGroups retrieves active groups associated with the current user
	GetMyActiveGroups(ctx context.Context) ([]*active.Group, error)

	// GetMySupervisedGroups retrieves active groups supervised by the current user
	GetMySupervisedGroups(ctx context.Context) ([]*active.Group, error)

	// GetGroupStudents retrieves students in a specific group where the current user has access
	GetGroupStudents(ctx context.Context, groupID int64) ([]*users.Student, error)

	// GetGroupVisits retrieves active visits for a specific group where the current user has access
	GetGroupVisits(ctx context.Context, groupID int64) ([]*active.Visit, error)

	// GetCurrentProfile retrieves the full profile for the current user including person, account, and profile data
	GetCurrentProfile(ctx context.Context) (map[string]interface{}, error)

	// UpdateCurrentProfile updates the current user's profile with the provided data
	UpdateCurrentProfile(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error)

	// UpdateAvatar updates the current user's avatar
	UpdateAvatar(ctx context.Context, avatarURL string) (map[string]interface{}, error)
}
