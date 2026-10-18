package api

import (
	"context"
	"errors"
	"log/slog"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// CallerRows serves the Identity & Access caller context (#3501) to the
// consumers that still speak the retained rows: it resolves the caller's
// reach as IDs through the public capability and loads the rows those IDs
// name. It owns no identity decision; every "who is the caller" and "what do
// they reach" answer comes from the caller context. The rows it loads come
// through ports the root declares: the groups, staff, teachers and students
// of the identity chain, the planned activity groups Timetable serves and
// the room sessions of Student Presence.
type CallerRows struct {
	caller     identityaccess.CallerContext
	groups     groupRows
	staff      staffRows
	teachers   teacherRows
	students   studentRows
	activities callerActivityRows
	sessions   callerSessionRows
	logger     *slog.Logger
}

// callerActivityRows are the planned activity groups the /api/me routes
// render.
type callerActivityRows interface {
	ActivityGroupRows(ctx context.Context, ids []int64) (any, error)
}

// callerSessionRows are the room sessions the caller supervises: the
// details the supervision consumers read and the rows the /api/me routes
// render.
type callerSessionRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*studentpresence.SessionDetail, error)
	SessionRows(ctx context.Context, ids []int64) (any, error)
}

// NewCallerRows serves caller over the rows the retained consumers read.
func NewCallerRows(caller identityaccess.CallerContext, groups groupRows, staff staffRows, teachers teacherRows, students studentRows,
	activities callerActivityRows, sessions callerSessionRows, logger *slog.Logger,
) (*CallerRows, error) {
	if caller.CallerIdentities == nil || groups == nil || staff == nil || teachers == nil || students == nil ||
		activities == nil || sessions == nil || logger == nil {
		return nil, errors.New("caller rows: the caller context, the group, staff, teacher, student, activity and session rows and a logger are required")
	}
	return &CallerRows{
		caller: caller, groups: groups, staff: staff, teachers: teachers, students: students,
		activities: activities, sessions: sessions, logger: logger,
	}, nil
}

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

// MyGroupIDs returns the IDs of the caller's educational groups. The note
// audience (#3632) needs the IDs only, so it does not pay for the group rows
// GetMyGroups loads.
func (r *CallerRows) MyGroupIDs(ctx context.Context) ([]int64, error) {
	return r.caller.MyGroupIDs(ctx)
}

// MyTeacherGroupIDs returns only the caller's direct teacher assignments.
// Temporary substitutions provide care-team reach but never group leadership.
func (r *CallerRows) MyTeacherGroupIDs(ctx context.Context) ([]int64, error) {
	return r.caller.MyTeacherGroupIDs(ctx)
}

// MySchoolClasses returns the caller's school classes under the caller-context
// name, next to the legacy Get-prefixed one above.
func (r *CallerRows) MySchoolClasses(ctx context.Context) ([]string, error) {
	return r.caller.MySchoolClasses(ctx)
}

// MyActivityGroupIDs returns the activities the caller supervises.
func (r *CallerRows) MyActivityGroupIDs(ctx context.Context) ([]int64, error) {
	return r.caller.MyActivityGroupIDs(ctx)
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
func (r *CallerRows) GetMyGroups(ctx context.Context) ([]*educationRepo.Group, error) {
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
func (r *CallerRows) GetMySupervisedGroups(ctx context.Context) ([]*studentpresence.SessionDetail, error) {
	ids, err := r.caller.MySupervisedSessionIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*studentpresence.SessionDetail, 0, len(ids))
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

func (r *CallerRows) groupsByID(ctx context.Context, ids []int64) ([]*educationRepo.Group, error) {
	result := make([]*educationRepo.Group, 0, len(ids))
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
	*educationRepo.Group
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
	return r.activities.ActivityGroupRows(ctx, ids)
}

// SessionRows returns the room sessions the /api/me/groups/active and
// /api/me/groups/supervised routes render.
func (r *CallerRows) SessionRows(ctx context.Context, ids []int64) (any, error) {
	return r.sessions.SessionRows(ctx, ids)
}

// StudentRows returns the students the /api/me/groups/{id}/students route
// renders.
func (r *CallerRows) StudentRows(ctx context.Context, ids []int64) (any, error) {
	result := make([]*peopleValues.StudentState, 0, len(ids))
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

// callerNavigationRow is the /api/me/navigation wire shape. The supervised
// groups and the current staff member carry the rows SessionRows and
// StaffRow load, like the routes that render them alone.
type callerNavigationRow struct {
	EducationalGroups   []*callerGroupRow `json:"educational_groups"`
	SupervisedGroups    any               `json:"supervised_groups"`
	CurrentStaff        any               `json:"current_staff"`
	Incomplete          bool              `json:"incomplete"`
	UnavailableSections []string          `json:"unavailable_sections"`
}

// NavigationRow loads the rows of the caller's navigation context. A group
// section whose rows fail to load is reported unavailable, like a section
// whose IDs failed to resolve.
func (r *CallerRows) NavigationRow(ctx context.Context, navigation identityaccess.CallerNavigation) (any, error) {
	row := &callerNavigationRow{UnavailableSections: navigation.UnavailableSections}
	groups, err := r.educationalGroupRows(ctx, navigation.Groups)
	if err != nil {
		r.logger.Warn("navigation context groups unavailable", slog.String("error", err.Error()))
		groups = []*callerGroupRow{}
		row.UnavailableSections = append(row.UnavailableSections, "educational_groups")
	}
	row.EducationalGroups = groups
	sessions, err := r.sessions.SessionRows(ctx, navigation.SupervisedSessionIDs)
	if err != nil {
		r.logger.Warn("navigation context supervision unavailable", slog.String("error", err.Error()))
		sessions = []any{}
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
