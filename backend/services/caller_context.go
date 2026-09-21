package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// callerSettings resolves the tenant settings the caller context reads: the
// school-wide overview scope and the parent request review scopes.
type callerSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// callerContextWiring names the owners the caller context reads the
// caller's chain from. SupervisedActivities reads the planned supervisions
// of a staff member, which no owner contract serves yet. Settings may be
// nil: the school-wide live-update scope is then never granted and the
// parent request review policy reports itself unconfigured.
type callerContextWiring struct {
	Accounts             identityaccess.AccountProfiles
	Persons              userModels.PersonRepository
	Membership           schoolmembership.Capability
	StaffGroups          schoolstructure.StaffGroupQuery
	SupervisedActivities func(context.Context, int64) ([]int64, error)
	Presence             *studentpresence.Module
	Settings             callerSettings
	Logger               *slog.Logger
}

// newCallerContext composes the Identity & Access caller context (#3501)
// over People Directory, School Membership, School Structure, Timetable and
// Student Presence.
func newCallerContext(wiring callerContextWiring) (identityaccess.CallerContext, error) {
	review := &identityCompose.ParentRequestReviewDependencies{
		Permissions:           ReviewPermissions,
		GroupLeaderSettingKey: configModels.KeyParentRequestGroupLeaderReviewEnabled,
		AbsenceSettingKey:     configModels.KeyParentAbsenceReviewScope,
		AbsenceReadRequired:   securityruntime.ErrAbsenceReadRequired,
	}
	deps := identityCompose.CallerContextDependencies{
		Caller:     CallerFromClaims,
		Memo:       requestIdentityMemo,
		Accounts:   wiring.Accounts,
		People:     callerPeople{persons: wiring.Persons},
		Membership: callerMembership{membership: wiring.Membership},
		Structure:  callerStructure{groups: wiring.StaffGroups},
		Activities: callerActivities(wiring.SupervisedActivities),
		Sessions:   callerSessions{presence: wiring.Presence},
		LiveTopics: callerLiveTopics{presence: wiring.Presence},
		Review:     review,
		Logger:     wiring.Logger,
	}
	if wiring.Settings != nil {
		deps.Overview = CallerOverview{Settings: wiring.Settings}
		review.Settings = wiring.Settings
	}
	return identityCompose.NewCallerContext(deps)
}

// newCallerRows composes the caller context and the rows its retained
// consumers read through it.
func newCallerRows(wiring callerContextWiring, sources repositories.CallerRowSources) (*repositories.CallerRows, error) {
	caller, err := newCallerContext(wiring)
	if err != nil {
		return nil, fmt.Errorf("compose caller context: %w", err)
	}
	return repositories.NewCallerRows(caller, sources), nil
}

// requestIdentityMemo resolves the request identity memo the request
// middleware attached (#2099).
func requestIdentityMemo(ctx context.Context) identityCompose.CallerMemo {
	if cache := authjwt.RequestIdentityCacheFrom(ctx); cache != nil {
		return cache
	}
	return nil
}

// CallerFromClaims resolves the principal the authenticator verified.
func CallerFromClaims(ctx context.Context) identityaccess.Caller {
	claims, ok := ctx.Value(authjwt.CtxClaims).(authjwt.AppClaims)
	if !ok {
		return identityaccess.Caller{}
	}
	return identityaccess.Caller{
		Authenticated:  true,
		AccountID:      int64(claims.ID),
		ClaimsTenantID: claims.TenantID,
		Scope:          claims.Scope,
		SchoolScope:    claims.IsSchoolScope(),
		AdminRole:      claims.IsAdmin,
		AdminWildcard:  securityruntime.HasAdminWildcard(authjwt.PermissionsFromCtx(ctx)),
	}
}

type callerPeople struct{ persons userModels.PersonRepository }

func (p callerPeople) FindPersonByAccount(ctx context.Context, accountID int64) (identityaccess.CallerPerson, bool, error) {
	person, err := p.persons.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return identityaccess.CallerPerson{}, false, err
	}
	return identityaccess.CallerPerson{
		ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID,
		CreatedAt: person.CreatedAt, UpdatedAt: person.UpdatedAt,
	}, true, nil
}

func (p callerPeople) CreatePerson(ctx context.Context, accountID int64, firstName, lastName string) error {
	person := &userModels.Person{AccountID: &accountID, FirstName: firstName, LastName: lastName}
	person.SetTenantID(tenant.FromContext(ctx))
	return p.persons.Create(ctx, person)
}

func (p callerPeople) RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error {
	person, err := p.persons.FindByID(ctx, personID)
	if err != nil {
		return err
	}
	if person == nil {
		return errors.New("person not found")
	}
	if firstName != nil {
		person.FirstName = *firstName
	}
	if lastName != nil {
		person.LastName = *lastName
	}
	return p.persons.Update(ctx, person)
}

type callerMembership struct{ membership schoolmembership.Capability }

func (m callerMembership) FindStaffByPerson(ctx context.Context, personID int64) (int64, bool, error) {
	staff, err := m.membership.FindStaffByPerson(ctx, personID)
	if errors.Is(err, schoolmembership.ErrStaffNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return staff.ID, true, nil
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
	groups, err := s.groups.ListSubstitutedGroups(ctx, staffID, timezone.TodayDate().String())
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

// supervisedActivityGroupIDs reads the activity groups a staff member
// supervises as planned.
func supervisedActivityGroupIDs(activities repositories.ActivityGroupRepository) func(context.Context, int64) ([]int64, error) {
	return func(ctx context.Context, staffID int64) ([]int64, error) {
		rows, err := activities.FindByStaffSupervisor(ctx, staffID)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return ids, nil
	}
}

type callerActivities func(context.Context, int64) ([]int64, error)

func (read callerActivities) SupervisedActivityGroupIDs(ctx context.Context, staffID int64) ([]int64, error) {
	return read(ctx, staffID)
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
