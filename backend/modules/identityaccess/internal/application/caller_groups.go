package application

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// errSessionNotFound reports a room session the session read did not find.
var errSessionNotFound = errors.New("find by id: record not found")

const (
	opGetMyGroups         = "get my groups"
	opGetSubstitutedIDs   = "get substituted group IDs"
	opGetMySchoolClasses  = "get my school classes"
	opGetSupervisedGroups = "get supervised groups"
	opGetGroupStudents    = "get group students"
	opGetGroupVisits      = "get group visits"
)

// MyGroupIDs returns the educational groups of the caller: the groups of
// their teacher profile and the groups they substitute for today. A caller
// without a staff or teacher link has none. A failed substitution read
// keeps the teacher groups and reports a CallerGroupsPartialError; only a
// complete result is memoized.
func (c *CallerContext) MyGroupIDs(ctx context.Context) ([]int64, error) {
	if c.principal(ctx).AccountID <= 0 {
		return nil, &domain.CallerError{Op: opGetMyGroups, Err: domain.ErrCallerNotAuthenticated}
	}
	entry := c.entry(ctx)
	if groups, ok := entry.cachedGroups(); ok {
		return groups, nil
	}

	staffID, staffErr := c.StaffID(ctx)
	// The teacher stage reuses the resolved staff member instead of walking
	// the chain again, and shares its memo with TeacherID.
	var teacherID int64
	teacherErr := staffErr
	if staffErr == nil {
		teacherID, teacherErr = c.teacherForStaff(ctx, staffID)
	}
	valid, unexpected := linkage(staffErr, teacherErr)
	if unexpected != nil {
		return nil, &domain.CallerError{Op: opGetMyGroups, Err: unexpected}
	}
	if !valid {
		entry.storeGroups([]int64{})
		return []int64{}, nil
	}

	groups := make(map[int64]struct{})
	if teacherErr == nil {
		if err := c.addTeacherGroups(ctx, teacherID, groups); err != nil {
			return nil, err
		}
	}
	var partial *domain.CallerGroupsPartialError
	if staffErr == nil {
		partial = c.addSubstitutionGroups(ctx, staffID, groups)
	}

	ids := slices.Collect(maps.Keys(groups))
	if partial != nil {
		return c.partialGroups(ids, partial)
	}
	entry.storeGroups(ids)
	return ids, nil
}

// addTeacherGroups adds the groups assigned to the caller's teacher profile.
func (c *CallerContext) addTeacherGroups(ctx context.Context, teacherID int64, groups map[int64]struct{}) error {
	ids, err := c.deps.Structure.TeacherGroupIDs(ctx, teacherID)
	if err != nil {
		return &domain.CallerError{Op: opGetMyGroups, Err: err}
	}
	for _, id := range ids {
		groups[id] = struct{}{}
	}
	return nil
}

// addSubstitutionGroups adds the groups the caller substitutes for today. A
// failed read is reported as a partial failure, keeping the teacher groups.
func (c *CallerContext) addSubstitutionGroups(ctx context.Context, staffID int64, groups map[int64]struct{}) *domain.CallerGroupsPartialError {
	substituted, err := c.deps.Structure.SubstitutedGroups(ctx, staffID)
	if err != nil {
		return &domain.CallerGroupsPartialError{Op: "get my groups (substitutions)", LastErr: err, FailureCount: 1}
	}
	for id := range substituted {
		groups[id] = struct{}{}
	}
	return nil
}

// partialGroups logs a partially loaded group set and returns what loaded.
func (c *CallerContext) partialGroups(ids []int64, partial *domain.CallerGroupsPartialError) ([]int64, error) {
	c.logger().Warn("partial failure in GetMyGroups",
		slog.Int("success_count", partial.SuccessCount),
		slog.Int("failure_count", partial.FailureCount),
		slog.Any("failed_ids", partial.FailedIDs),
		slog.String("operation", partial.Op),
	)
	if len(ids) == 0 {
		return nil, partial
	}
	return ids, partial
}

// linkage reports whether the caller has a staff or teacher link. Both
// lookups failing with a clean "not linked" outcome means no link; any
// other failure is returned, the teacher's first.
func linkage(staffErr, teacherErr error) (bool, error) {
	if staffErr == nil || teacherErr == nil {
		return true, nil
	}
	staffExpected := isExpectedLinkageError(staffErr)
	teacherExpected := isExpectedLinkageError(teacherErr)
	if staffExpected && teacherExpected {
		return false, nil
	}
	if !teacherExpected {
		return false, teacherErr
	}
	return false, staffErr
}

// SubstitutedGroupIDs returns the groups the caller reaches through an
// active substitution whose regular staff slot is unassigned. A caller who
// is no staff member has none.
func (c *CallerContext) SubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	entry := c.entry(ctx)
	if subs, ok := entry.cachedSubstitutions(); ok {
		return subs, nil
	}
	result := make(map[int64]bool)
	staffID, err := c.StaffID(ctx)
	if err != nil {
		if isExpectedLinkageError(err) {
			entry.storeSubstitutions(result)
			return result, nil
		}
		return nil, &domain.CallerError{Op: opGetSubstitutedIDs, Err: err}
	}
	substituted, err := c.deps.Structure.SubstitutedGroups(ctx, staffID)
	if err != nil {
		return nil, &domain.CallerError{Op: opGetSubstitutedIDs, Err: err}
	}
	for groupID, unassigned := range substituted {
		if unassigned {
			result[groupID] = true
		}
	}
	entry.storeSubstitutions(result)
	return result, nil
}

// MySchoolClasses returns the school classes assigned to the caller's staff
// member (#1772) in class order. A caller who is no staff member has none.
func (c *CallerContext) MySchoolClasses(ctx context.Context) ([]string, error) {
	if c.principal(ctx).AccountID <= 0 {
		return nil, &domain.CallerError{Op: opGetMySchoolClasses, Err: domain.ErrCallerNotAuthenticated}
	}
	entry := c.entry(ctx)
	if classes, ok := entry.cachedSchoolClasses(); ok {
		return classes, nil
	}
	staffID, err := c.StaffID(ctx)
	if err != nil {
		if isNotStaff(err) {
			entry.storeSchoolClasses([]string{})
			return []string{}, nil
		}
		return nil, &domain.CallerError{Op: opGetMySchoolClasses, Err: err}
	}
	classes, err := c.deps.Structure.SchoolClasses(ctx, staffID)
	if err != nil {
		return nil, &domain.CallerError{Op: opGetMySchoolClasses, Err: err}
	}
	entry.storeSchoolClasses(classes)
	return classes, nil
}

// MyActivityGroupIDs returns the activity groups the caller supervises as
// planned. A caller who is no staff member has none.
func (c *CallerContext) MyActivityGroupIDs(ctx context.Context) ([]int64, error) {
	staffID, err := c.StaffID(ctx)
	if err != nil {
		if isNotStaff(err) {
			return []int64{}, nil
		}
		return nil, err
	}
	ids, err := c.deps.Activities.SupervisedActivityGroupIDs(ctx, staffID)
	if err != nil {
		return nil, &domain.CallerError{Op: "get my activity groups", Err: err}
	}
	if len(ids) == 0 {
		return []int64{}, nil
	}
	return ids, nil
}

// MyActiveSessionIDs returns the open room sessions of the caller's planned
// activities together with the sessions they supervise, without duplicates.
func (c *CallerContext) MyActiveSessionIDs(ctx context.Context) ([]int64, error) {
	staffID, err := c.StaffID(ctx)
	if err != nil {
		if isExpectedLinkageError(err) {
			return []int64{}, nil
		}
		return nil, err
	}
	activities, err := c.deps.Activities.SupervisedActivityGroupIDs(ctx, staffID)
	if err != nil {
		return nil, &domain.CallerError{Op: "get my active groups - activity groups", Err: err}
	}
	sessions, err := c.deps.Sessions.OpenSessionIDsForActivities(ctx, activities)
	if err != nil {
		return nil, &domain.CallerError{Op: "get my active groups - activity active", Err: err}
	}
	supervised, err := c.MySupervisedSessionIDs(ctx)
	if err != nil {
		return nil, &domain.CallerError{Op: "get my active groups - supervised", Err: err}
	}
	return mergeIDs(sessions, supervised), nil
}

// MySupervisedSessionIDs returns the open room sessions the caller
// supervises today. A caller who is no staff member supervises none.
func (c *CallerContext) MySupervisedSessionIDs(ctx context.Context) ([]int64, error) {
	staffID, err := c.StaffID(ctx)
	if err != nil {
		if isNotStaff(err) {
			return []int64{}, nil
		}
		return nil, err
	}
	ids, err := c.deps.Sessions.SupervisedSessionIDs(ctx, staffID)
	if err != nil {
		return nil, &domain.CallerError{Op: opGetSupervisedGroups, Err: err}
	}
	if len(ids) == 0 {
		return []int64{}, nil
	}
	return ids, nil
}

// mergeIDs returns the union of both lists, each ID once.
func mergeIDs(primary, additional []int64) []int64 {
	seen := make(map[int64]struct{}, len(primary)+len(additional))
	result := make([]int64, 0, len(primary)+len(additional))
	for _, id := range slices.Concat(primary, additional) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// GroupStudentIDs returns the students with a visit in a room session the
// caller reaches.
func (c *CallerContext) GroupStudentIDs(ctx context.Context, sessionID int64) ([]int64, error) {
	visits, err := c.reachableSessionVisits(ctx, opGetGroupStudents, sessionID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(visits))
	for _, visit := range visits {
		seen[visit.StudentID] = struct{}{}
	}
	return slices.Collect(maps.Keys(seen)), nil
}

// GroupVisits returns the open visits of a room session the caller reaches.
func (c *CallerContext) GroupVisits(ctx context.Context, sessionID int64) ([]domain.CallerVisit, error) {
	visits, err := c.reachableSessionVisits(ctx, opGetGroupVisits, sessionID)
	if err != nil {
		return nil, err
	}
	var open []domain.CallerVisit
	for _, visit := range visits {
		if visit.ExitTime == nil {
			open = append(open, visit)
		}
	}
	return open, nil
}

// reachableSessionVisits reads the visits of a room session the caller
// reaches, naming op in every error.
func (c *CallerContext) reachableSessionVisits(ctx context.Context, op string, sessionID int64) ([]domain.CallerVisit, error) {
	if err := c.checkSessionAccess(ctx, sessionID); err != nil {
		return nil, &domain.CallerError{Op: op, Err: err}
	}
	visits, err := c.deps.Sessions.SessionVisits(ctx, sessionID)
	if err != nil {
		return nil, &domain.CallerError{Op: op, Err: err}
	}
	return visits, nil
}

// checkSessionAccess requires the session to exist and to be one of the
// caller's active or supervised sessions.
func (c *CallerContext) checkSessionAccess(ctx context.Context, sessionID int64) error {
	exists, err := c.deps.Sessions.SessionExists(ctx, sessionID)
	if err != nil {
		return err
	}
	if !exists {
		// A missing session was always a lookup failure of the session read,
		// never ErrCallerGroupNotFound; /api/me keeps answering it with 500.
		return errSessionNotFound
	}
	active, err := c.MyActiveSessionIDs(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(active, sessionID) {
		return nil
	}
	supervised, err := c.MySupervisedSessionIDs(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(supervised, sessionID) {
		return nil
	}
	return domain.ErrCallerNotAuthorized
}
