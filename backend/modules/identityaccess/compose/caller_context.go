package compose

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// CallerMembership is the School Membership seam: the staff member of a
// person and the teacher of a staff member.
type CallerMembership interface {
	FindStaffByPerson(ctx context.Context, personID int64) (staffID int64, found bool, err error)
	FindTeacherByStaff(ctx context.Context, staffID int64) (teacherID int64, found bool, err error)
}

// CallerStructure is the School Structure seam: a teacher's groups, the
// groups a staff member substitutes for on the school's current day
// (flagging an unassigned regular staff slot) and a staff member's school
// classes.
type CallerStructure interface {
	TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error)
	SubstitutedGroups(ctx context.Context, staffID int64) (map[int64]bool, error)
	SchoolClasses(ctx context.Context, staffID int64) ([]string, error)
}

// CallerActivities is the Timetable seam: the activity groups a staff
// member supervises as planned.
type CallerActivities interface {
	SupervisedActivityGroupIDs(ctx context.Context, staffID int64) ([]int64, error)
}

// CallerLiveTopics is the Student Presence seam of live-update clients:
// every open room session of the school, or a staff member's own.
type CallerLiveTopics interface {
	OpenSessionIDs(ctx context.Context) ([]int64, error)
	StaffSessionIDs(ctx context.Context, staffID int64) ([]int64, error)
}

// CallerContextDependencies binds the caller context at the composition
// root. Caller resolves the verified principal of the request; the tenant
// of the request context is added here. Memo resolves the request's memo
// slot; it returns nil for a context without one. LiveTopics and Overview
// are optional: without LiveTopics no subscription resolves, without
// Overview the school-wide subscription scope is never granted.
type CallerContextDependencies struct {
	Caller     func(context.Context) identityaccess.Caller
	Memo       func(context.Context) identityPorts.CallerMemo
	Accounts   identityaccess.AccountProfiles
	People     identityPorts.CallerPeople
	Membership CallerMembership
	Structure  CallerStructure
	Activities CallerActivities
	Sessions   identityPorts.CallerSessions
	LiveTopics CallerLiveTopics
	Overview   identityPorts.CallerOverview
	Review     *ParentRequestReviewDependencies
	Logger     *slog.Logger
}

// ParentRequestReviewDependencies binds the parent request review policy.
// Permissions evaluates the route-level permission facts; AbsenceReadRequired
// is the error a caller holding users:absence without users:read is refused
// with, so the queues and the write gate refuse alike.
type ParentRequestReviewDependencies struct {
	Permissions func([]string) identityaccess.ReviewPermissions
	Settings    interface {
		ResolveBool(ctx context.Context, key string) (bool, error)
		ResolveString(ctx context.Context, key string) (string, error)
	}
	GroupLeaderSettingKey string
	RequestSettingKey     string
	AbsenceSettingKey     string
	AbsenceReadRequired   error
}

// NewCallerContext composes the caller-context capability.
func NewCallerContext(deps CallerContextDependencies) (identityaccess.CallerContext, error) {
	if deps.Caller == nil || deps.Accounts == nil || deps.People == nil || deps.Membership == nil ||
		deps.Structure == nil || deps.Activities == nil || deps.Sessions == nil {
		return identityaccess.CallerContext{}, errors.New("identity access compose: caller context dependencies are required")
	}
	appDeps := application.CallerContextDependencies{
		Caller:     application.NewCallerReader(deps.Caller),
		Memo:       application.RequestMemo(deps.Memo),
		Accounts:   application.CallerAccounts{AccountProfiles: deps.Accounts},
		People:     application.CallerPeople{CallerPeople: deps.People},
		Membership: deps.Membership,
		Structure:  deps.Structure,
		Activities: deps.Activities,
		Sessions:   application.CallerSessions{CallerSessions: deps.Sessions},
		Write:      newCommandTransactions(nil).RunWrite,
		RemoveFile: os.Remove,
		Logger:     deps.Logger,
	}
	if deps.LiveTopics != nil {
		appDeps.LiveTopics = deps.LiveTopics
	}
	if deps.Overview != nil {
		appDeps.Overview = deps.Overview
	}
	app, err := application.NewCallerContext(appDeps)
	if err != nil {
		return identityaccess.CallerContext{}, err
	}
	engine := application.CallerEngine{App: app}
	result := identityaccess.CallerContext{CallerIdentities: engine, CallerReach: engine, CallerProfiles: engine}
	if deps.Review != nil {
		review, err := newParentRequestReview(app, *deps.Review)
		if err != nil {
			return identityaccess.CallerContext{}, err
		}
		result.ParentRequestReviews = review
	}
	return result, nil
}

func newParentRequestReview(caller *application.CallerContext, deps ParentRequestReviewDependencies) (application.ReviewEngine, error) {
	if deps.Permissions == nil {
		return application.ReviewEngine{}, errors.New("identity access compose: review permissions are required")
	}
	appDeps := application.ParentRequestReviewDependencies{
		Permissions:           application.NewReviewPermissions(deps.Permissions),
		GroupLeaderSettingKey: deps.GroupLeaderSettingKey,
		RequestSettingKey:     deps.RequestSettingKey,
		AbsenceSettingKey:     deps.AbsenceSettingKey,
		AbsenceReadRequired:   deps.AbsenceReadRequired,
	}
	if deps.Settings != nil {
		appDeps.Settings = deps.Settings
	}
	review, err := application.NewParentRequestReview(caller, appDeps)
	return application.ReviewEngine{Review: review}, err
}
