package usercontext

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// UserContextService defines operations available in the user context service layer
type UserContextService interface {
	// GetCurrentUser retrieves the currently authenticated user account
	GetCurrentUser(ctx context.Context) (*auth.Account, error)

	// GetCurrentPerson retrieves the person linked to the currently authenticated user
	GetCurrentPerson(ctx context.Context) (*users.Person, error)

	// GetCurrentStaff retrieves the staff member linked to the currently authenticated user
	GetCurrentStaff(ctx context.Context) (*users.Staff, error)

	// ResolveSSESubscription resolves the SSE topic subscription (staff
	// resolution + supervised/active groups + educational groups) for the
	// authenticated caller. Returns an *SSESetupError carrying an HTTP status
	// when a non-admin caller has no staff record.
	ResolveSSESubscription(ctx context.Context) (*SSESubscription, error)

	// GetCurrentTeacher retrieves the teacher linked to the currently authenticated user
	GetCurrentTeacher(ctx context.Context) (*users.Teacher, error)

	// GetMyGroups retrieves educational groups associated with the current user.
	//
	// The identity-free bulk counterpart is
	// education.GroupRepository.ListSupervisedGroupIDsByStaff: same two sources
	// (teacher assignments plus active substitutions), keyed by staff ID, for
	// callers with no authenticated user. The two are pinned against each other
	// by an equivalence test.
	GetMyGroups(ctx context.Context) ([]*education.Group, error)

	// GetSubstitutedGroupIDs returns the set of education group IDs the current
	// staff member can access via an active substitution where the regular
	// staff slot is unassigned. Returns an empty set when the user is not
	// staff or has no active substitutions.
	GetSubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error)

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
