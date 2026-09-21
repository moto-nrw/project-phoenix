package repositories

import (
	"context"
	"errors"
	"log/slog"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// CallerRows serves the Identity & Access caller context (#3501) to the
// consumers that still speak the retained rows: it resolves the caller's
// reach as IDs through the public capability and loads the rows those IDs
// name. It owns no identity decision; every "who is the caller" and "what do
// they reach" answer comes from the caller context.
type CallerRows struct {
	caller     identityaccess.CallerContext
	groups     educationModels.GroupRepository
	staff      userModels.StaffRepository
	teachers   userModels.TeacherRepository
	students   userModels.StudentRepository
	activities activitiesModels.GroupRepository
	sessions   activeModels.GroupRepository
	logger     *slog.Logger
}

// CallerRowSources names the retained repositories the rows load from. A
// nil Logger logs through slog.Default.
type CallerRowSources struct {
	Groups     educationModels.GroupRepository
	Staff      userModels.StaffRepository
	Teachers   userModels.TeacherRepository
	Students   userModels.StudentRepository
	Activities activitiesModels.GroupRepository
	Sessions   activeModels.GroupRepository
	Logger     *slog.Logger
}

func NewCallerRows(caller identityaccess.CallerContext, sources CallerRowSources) *CallerRows {
	return &CallerRows{
		caller: caller, groups: sources.Groups, staff: sources.Staff, teachers: sources.Teachers,
		students: sources.Students, activities: sources.Activities, sessions: sources.Sessions,
		logger: sources.Logger,
	}
}

func (r *CallerRows) getLogger() *slog.Logger {
	if r.logger == nil {
		return slog.Default()
	}
	return r.logger
}

// ActivityGroupRepository is the retained activity group repository the
// caller context reads planned supervisions from; no Timetable contract
// serves that read yet.
type ActivityGroupRepository = activitiesModels.GroupRepository

// Caller returns the caller context the rows are resolved through.
func (r *CallerRows) Caller() identityaccess.CallerContext { return r.caller }

// CurrentStaffID returns the caller's staff member. found is false, without
// an error, for a caller whose account has no person or whose person is no
// staff member.
func (r *CallerRows) CurrentStaffID(ctx context.Context) (int64, bool, error) {
	staffID, err := r.caller.StaffID(ctx)
	if errors.Is(err, identityaccess.ErrCallerNotLinkedToStaff) || errors.Is(err, identityaccess.ErrCallerNotLinkedToPerson) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return staffID, true, nil
}

// CurrentStaffIDOfPerson returns the caller's staff member for callers whose
// account has a person. found is false, without an error, only for a person
// who is no staff member; an account without a person is an error.
func (r *CallerRows) CurrentStaffIDOfPerson(ctx context.Context) (int64, bool, error) {
	staffID, err := r.caller.StaffID(ctx)
	if errors.Is(err, identityaccess.ErrCallerNotLinkedToStaff) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return staffID, true, nil
}

// CurrentPersonName returns the names of the person linked to the caller's
// account.
func (r *CallerRows) CurrentPersonName(ctx context.Context) (firstName, lastName string, err error) {
	person, err := r.caller.Person(ctx)
	if err != nil {
		return "", "", err
	}
	return person.FirstName, person.LastName, nil
}

// ResolveSSETopics resolves the caller's live-update topics. A rejected
// caller fails with an *identityaccess.SSESetupError.
func (r *CallerRows) ResolveSSETopics(ctx context.Context) (staffID int64, activeGroupIDs, eduTopics, allTopics []string, err error) {
	subscription, err := r.caller.SSESubscription(ctx)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	return subscription.StaffID, subscription.ActiveGroupIDs, subscription.EduTopics, subscription.AllTopics, nil
}

// HasCurrentStaff reports whether the caller is a verified staff member.
func (r *CallerRows) HasCurrentStaff(ctx context.Context) (bool, error) {
	return r.caller.HasCurrentStaff(ctx)
}

// HasFullStudentAccess reports whether the caller sees unredacted student
// data (#2329).
func (r *CallerRows) HasFullStudentAccess(ctx context.Context) bool {
	return r.caller.StudentAccess(ctx).HasFullAccess()
}

// StudentAccessFacts reports the two facts the student access decision
// rests on: the admin permission and the verified staff record.
func (r *CallerRows) StudentAccessFacts(ctx context.Context) (admin, staff bool) {
	access := r.caller.StudentAccess(ctx)
	return access.Admin, access.Staff
}

// GetMySchoolClasses returns the caller's school classes.
func (r *CallerRows) GetMySchoolClasses(ctx context.Context) ([]string, error) {
	return r.caller.MySchoolClasses(ctx)
}

// GetSubstitutedGroupIDs returns the groups the caller reaches only through
// a substitution.
func (r *CallerRows) GetSubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	return r.caller.SubstitutedGroupIDs(ctx)
}

// GetMyGroups returns the caller's educational groups. A partial group set
// is returned together with its CallerGroupsPartialError.
func (r *CallerRows) GetMyGroups(ctx context.Context) ([]*educationModels.Group, error) {
	ids, err := r.caller.MyGroupIDs(ctx)
	if ids == nil {
		return nil, err
	}
	groups, loadErr := r.groupsByID(ctx, ids)
	if loadErr != nil {
		return nil, loadErr
	}
	return groups, err
}

// GetMySupervisedGroups returns the room sessions the caller supervises.
func (r *CallerRows) GetMySupervisedGroups(ctx context.Context) ([]*activeModels.Group, error) {
	ids, err := r.caller.MySupervisedSessionIDs(ctx)
	if err != nil {
		return nil, err
	}
	return r.sessionsByID(ctx, ids)
}

func (r *CallerRows) groupsByID(ctx context.Context, ids []int64) ([]*educationModels.Group, error) {
	result := make([]*educationModels.Group, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	byID, err := r.groups.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if group := byID[id]; group != nil {
			result = append(result, group)
		}
	}
	return result, nil
}

func (r *CallerRows) sessionsByID(ctx context.Context, ids []int64) ([]*activeModels.Group, error) {
	result := make([]*activeModels.Group, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	byID, err := r.sessions.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if session := byID[id]; session != nil {
			result = append(result, session)
		}
	}
	return result, nil
}

// StaffRow returns the staff row the /api/me/staff route renders.
func (r *CallerRows) StaffRow(ctx context.Context, staffID int64) (any, error) {
	return r.staff.FindByID(ctx, staffID)
}

// TeacherRow returns the teacher row the /api/me/teacher route renders.
func (r *CallerRows) TeacherRow(ctx context.Context, teacherID int64) (any, error) {
	return r.teachers.FindByID(ctx, teacherID)
}

// callerGroupRow is an educational group with the caller's access path.
type callerGroupRow struct {
	*educationModels.Group
	ViaSubstitution bool `json:"via_substitution"`
}

// EducationalGroupRows returns the caller's groups the /api/me/groups route
// renders.
func (r *CallerRows) EducationalGroupRows(ctx context.Context, groups []identityaccess.CallerGroup) (any, error) {
	return r.educationalGroupRows(ctx, groups)
}

func (r *CallerRows) educationalGroupRows(ctx context.Context, groups []identityaccess.CallerGroup) ([]*callerGroupRow, error) {
	ids := make([]int64, 0, len(groups))
	via := make(map[int64]bool, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
		via[group.ID] = group.ViaSubstitution
	}
	rows, err := r.groupsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*callerGroupRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, &callerGroupRow{Group: row, ViaSubstitution: via[row.ID]})
	}
	return result, nil
}

// ActivityGroupRows returns the planned activity groups the
// /api/me/groups/activity route renders, in the order of ids.
func (r *CallerRows) ActivityGroupRows(ctx context.Context, ids []int64) (any, error) {
	result := make([]*activitiesModels.Group, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.activities.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*activitiesModels.Group, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, id := range ids {
		if row := byID[id]; row != nil {
			result = append(result, row)
		}
	}
	return result, nil
}

// SessionRows returns the room sessions the /api/me/groups/active and
// /api/me/groups/supervised routes render.
func (r *CallerRows) SessionRows(ctx context.Context, ids []int64) (any, error) {
	return r.sessionsByID(ctx, ids)
}

// StudentRows returns the students the /api/me/groups/{id}/students route
// renders.
func (r *CallerRows) StudentRows(ctx context.Context, ids []int64) (any, error) {
	result := make([]*userModels.Student, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	byID, err := r.students.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if student := byID[id]; student != nil {
			result = append(result, student)
		}
	}
	return result, nil
}

// callerNavigationRow is the /api/me/navigation wire shape.
type callerNavigationRow struct {
	EducationalGroups   []*callerGroupRow     `json:"educational_groups"`
	SupervisedGroups    []*activeModels.Group `json:"supervised_groups"`
	CurrentStaff        *userModels.Staff     `json:"current_staff"`
	Incomplete          bool                  `json:"incomplete"`
	UnavailableSections []string              `json:"unavailable_sections"`
}

// NavigationRow loads the rows of the caller's navigation context. A group
// section whose rows fail to load is reported unavailable, like a section
// whose IDs failed to resolve.
func (r *CallerRows) NavigationRow(ctx context.Context, navigation identityaccess.CallerNavigation) (any, error) {
	row := &callerNavigationRow{UnavailableSections: navigation.UnavailableSections}
	groups, err := r.educationalGroupRows(ctx, navigation.Groups)
	if err != nil {
		r.getLogger().Warn("navigation context groups unavailable", slog.String("error", err.Error()))
		groups = []*callerGroupRow{}
		row.UnavailableSections = append(row.UnavailableSections, "educational_groups")
	}
	row.EducationalGroups = groups
	sessions, err := r.sessionsByID(ctx, navigation.SupervisedSessionIDs)
	if err != nil {
		r.getLogger().Warn("navigation context supervision unavailable", slog.String("error", err.Error()))
		sessions = []*activeModels.Group{}
		row.UnavailableSections = append(row.UnavailableSections, "supervised_groups")
	}
	row.SupervisedGroups = sessions
	if navigation.StaffID > 0 {
		staff, err := r.staff.FindByID(ctx, navigation.StaffID)
		if err != nil {
			return nil, err
		}
		row.CurrentStaff = staff
	}
	row.Incomplete = len(row.UnavailableSections) > 0
	return row, nil
}
