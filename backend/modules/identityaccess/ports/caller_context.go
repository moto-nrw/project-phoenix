package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// CallerPeople is the People Directory seam of the caller context: the
// person linked to an account and the caller's own name edits. found is
// false, without an error, when the account has no person in the tenant.
type CallerPeople interface {
	FindPersonByAccount(ctx context.Context, accountID int64) (person identityaccess.CallerPerson, found bool, err error)
	CreatePerson(ctx context.Context, accountID int64, firstName, lastName string) error
	RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error
}

// CallerSessions is the Student Presence seam: the room sessions a caller
// reaches and their visits.
type CallerSessions interface {
	OpenSessionIDsForActivities(ctx context.Context, activityGroupIDs []int64) ([]int64, error)
	SupervisedSessionIDs(ctx context.Context, staffID int64) ([]int64, error)
	SessionExists(ctx context.Context, sessionID int64) (bool, error)
	SessionVisits(ctx context.Context, sessionID int64) ([]identityaccess.CallerVisit, error)
}

// CallerOverview decides the school-wide operational overview scope
// (#2380) for a caller whose staff identity staff verifies.
type CallerOverview interface {
	HasOperationalOverview(ctx context.Context, staff interface {
		HasCurrentStaff(context.Context) (bool, error)
	}, assignmentBound, admin bool) (bool, error)
}
