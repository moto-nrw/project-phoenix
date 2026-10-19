package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// CallerSettings resolves the tenant settings the caller context reads: the
// school-wide overview scope and the parent request review scopes.
type CallerSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// newCallerContext composes the Identity & Access caller context (#3501)
// over People Directory, School Membership, School Structure, Timetable and
// Student Presence. tokens reads the verified principal and the request
// identity memo (#2099) the request middleware attached; activities reads
// the planned supervisions of a staff member, which no owner contract serves
// yet. settings may be nil: the school-wide live-update scope is then never
// granted and the parent request review policy reports itself unconfigured.
func NewCallerContext(
	tokens TokenAdapter, accounts identityaccess.AccountProfiles, persons CallerPersons, membership schoolmembership.Capability,
	staffGroups schoolstructure.StaffGroupQuery, activities identityCompose.CallerActivities, presence *studentpresence.Module,
	settings CallerSettings, logger *slog.Logger,
) (identityaccess.CallerContext, error) {
	if accounts == nil || persons.Rows == nil || persons.People == nil || membership == nil || staffGroups == nil ||
		activities == nil || presence == nil || logger == nil {
		return identityaccess.CallerContext{}, errors.New("compose caller context: the accounts, persons, membership, staff groups, supervised activities, presence and a logger are required")
	}
	review := &identityCompose.ParentRequestReviewDependencies{
		Permissions:           ReviewPermissions,
		GroupLeaderSettingKey: configModels.KeyParentRequestGroupLeaderReviewEnabled,
		RequestSettingKey:     configModels.KeyParentRequestReviewScope,
		AbsenceSettingKey:     configModels.KeyParentAbsenceReviewScope,
		AbsenceReadRequired:   securityruntime.ErrAbsenceReadRequired,
	}
	deps := identityCompose.CallerContextDependencies{
		Caller:     tokens.Caller,
		Memo:       tokens.memo,
		Accounts:   accounts,
		People:     callerPeople{persons: persons},
		Membership: callerMembership{membership: membership},
		Structure:  callerStructure{groups: staffGroups},
		Activities: activities,
		Sessions:   callerSessions{presence: presence},
		LiveTopics: callerLiveTopics{presence: presence},
		Review:     review,
		Logger:     logger,
	}
	if settings != nil {
		deps.Overview = CallerOverview{Settings: settings}
		review.Settings = settings
	}
	caller, err := identityCompose.NewCallerContext(deps)
	if err != nil {
		return identityaccess.CallerContext{}, fmt.Errorf("compose caller context: %w", err)
	}
	return caller, nil
}

// callerPersons reads the caller's person from the retained rows and writes
// the names of the caller's own profile through the People Directory.
type CallerPersons struct {
	Rows   personRows
	People peopledirectory.AccountLifecycleCommand
}

type callerPeople struct{ persons CallerPersons }

func (p callerPeople) FindPersonByAccount(ctx context.Context, accountID int64) (identityaccess.CallerPerson, bool, error) {
	person, err := p.persons.Rows.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return identityaccess.CallerPerson{}, false, err
	}
	return identityaccess.CallerPerson{
		ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID,
		CreatedAt: person.CreatedAt, UpdatedAt: person.UpdatedAt,
	}, true, nil
}

func (p callerPeople) CreatePerson(ctx context.Context, accountID int64, firstName, lastName string) error {
	return p.persons.People.CreateAccountPerson(ctx, accountID, firstName, lastName)
}

func (p callerPeople) RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error {
	return p.persons.People.RenamePerson(ctx, personID, firstName, lastName)
}

type callerMembership struct{ membership schoolmembership.Capability }

func (m callerMembership) FindStaffByPerson(ctx context.Context, personID int64) (int64, bool, error) {
	// The caller context needs the staff ID only: the membership read alone
	// skips the Workforce employment half every request would otherwise pay
	// for (#2753).
	return m.membership.FindStaffIDByPerson(ctx, personID)
}

func (m callerMembership) FindTeacherByStaff(ctx context.Context, staffID int64) (int64, bool, error) {
	teacher, err := m.membership.FindTeacherByStaff(ctx, staffID)
	if errors.Is(err, schoolmembership.ErrTeacherNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return teacher.ID, true, nil
}

type callerStructure struct {
	groups schoolstructure.StaffGroupQuery
}

func (s callerStructure) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	groups, err := s.groups.ListGroupsByTeacher(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids, nil
}

func (s callerStructure) SubstitutedGroups(ctx context.Context, staffID int64) (map[int64]bool, error) {
	groups, err := schoolstructure.ListSubstitutedGroupsToday(ctx, s.groups, staffID)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(groups))
	for _, group := range groups {
		result[group.Group.ID] = group.ViaSubstitution
	}
	return result, nil
}

func (s callerStructure) SchoolClasses(ctx context.Context, staffID int64) ([]string, error) {
	return s.groups.ListSchoolClassesByStaff(ctx, staffID)
}

type callerSessions struct{ presence *studentpresence.Module }

func (s callerSessions) OpenSessionIDsForActivities(ctx context.Context, activityGroupIDs []int64) ([]int64, error) {
	if len(activityGroupIDs) == 0 {
		return []int64{}, nil
	}
	sessions, err := s.presence.ListOpenLiveGroupsForActivities(ctx, activityGroupIDs)
	return liveGroupIDs(sessions), err
}

func (s callerSessions) SupervisedSessionIDs(ctx context.Context, staffID int64) ([]int64, error) {
	sessions, err := s.presence.ListSupervisedLiveGroups(ctx, staffID)
	return liveGroupIDs(sessions), err
}

func (s callerSessions) SessionExists(ctx context.Context, sessionID int64) (bool, error) {
	sessions, err := s.presence.ListLiveGroups(ctx, []int64{sessionID})
	return len(sessions) > 0, err
}

func (s callerSessions) SessionVisits(ctx context.Context, sessionID int64) ([]identityaccess.CallerVisit, error) {
	visits, err := s.presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{sessionID}})
	if err != nil {
		return nil, err
	}
	result := make([]identityaccess.CallerVisit, 0, len(visits))
	for _, visit := range visits {
		result = append(result, identityaccess.CallerVisit{
			ID: visit.ID, CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt, TenantID: visit.TenantID,
			StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID, EntryTime: visit.EntryTime, ExitTime: visit.ExitTime,
		})
	}
	return result, nil
}

func liveGroupIDs(sessions []studentpresence.LiveGroup) []int64 {
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	return ids
}

type callerLiveTopics struct{ presence *studentpresence.Module }

func (t callerLiveTopics) OpenSessionIDs(ctx context.Context) ([]int64, error) {
	groups, err := ssePresence{groups: t.presence}.ListSSEGroups(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group.IsOpen() {
			ids = append(ids, group.ID)
		}
	}
	return ids, nil
}

func (t callerLiveTopics) StaffSessionIDs(ctx context.Context, staffID int64) ([]int64, error) {
	return ssePresence{groups: t.presence}.GetStaffActiveGroupIDs(ctx, staffID)
}

type CallerOverview struct {
	Settings securityruntime.OverviewSettings
}

func (o CallerOverview) HasOperationalOverview(ctx context.Context, staff interface {
	HasCurrentStaff(context.Context) (bool, error)
}, assignmentBound, admin bool) (bool, error) {
	return securityruntime.CanViewOperationalOverview(ctx, o.Settings, staff, assignmentBound, admin)
}
