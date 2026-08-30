package usercontext

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Operation name constants to avoid string duplication
const (
	opGetCurrentStaff  = "get current staff"
	opGetGroupStudents = "get group students"
)

// UserContextRepositories groups all repository dependencies for UserContextService
// This struct reduces the number of parameters passed to the constructor
type UserContextRepositories struct {
	AccountRepo        auth.AccountRepository
	PersonRepo         users.PersonRepository
	StaffRepo          users.StaffRepository
	TeacherRepo        users.TeacherRepository
	StudentRepo        users.StudentRepository
	EducationGroupRepo education.GroupRepository
	ActivityGroupRepo  activities.GroupRepository
	ActiveGroupRepo    active.GroupRepository
	VisitsRepo         active.VisitRepository
	SupervisorRepo     active.GroupSupervisorRepository
	ProfileRepo        users.ProfileRepository
	SubstitutionRepo   education.GroupSubstitutionRepository
	ClassTeacherRepo   education.ClassTeacherRepository

	// SSE subscription dependencies (optional — nil-safe). Injected so the
	// SSE handler can delegate staff-resolution + topic-building to this
	// service instead of orchestrating it in the api layer.
	ActiveService activeService.Service
	SSESettings   SSESettingsResolver
}

// userContextService implements the UserContextService interface
type userContextService struct {
	accountRepo        auth.AccountRepository
	personRepo         users.PersonRepository
	staffRepo          users.StaffRepository
	teacherRepo        users.TeacherRepository
	studentRepo        users.StudentRepository
	educationGroupRepo education.GroupRepository
	activityGroupRepo  activities.GroupRepository
	activeGroupRepo    active.GroupRepository
	visitsRepo         active.VisitRepository
	supervisorRepo     active.GroupSupervisorRepository
	profileRepo        users.ProfileRepository
	substitutionRepo   education.GroupSubstitutionRepository
	classTeacherRepo   education.ClassTeacherRepository
	sseActiveSvc       activeService.Service
	sseSettings        SSESettingsResolver
	logger             *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *userContextService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// NewUserContextServiceWithRepos creates a new user context service using a repositories struct
func NewUserContextServiceWithRepos(repos UserContextRepositories, logger *slog.Logger) UserContextService {
	return &userContextService{
		accountRepo:        repos.AccountRepo,
		personRepo:         repos.PersonRepo,
		staffRepo:          repos.StaffRepo,
		teacherRepo:        repos.TeacherRepo,
		studentRepo:        repos.StudentRepo,
		educationGroupRepo: repos.EducationGroupRepo,
		activityGroupRepo:  repos.ActivityGroupRepo,
		activeGroupRepo:    repos.ActiveGroupRepo,
		visitsRepo:         repos.VisitsRepo,
		supervisorRepo:     repos.SupervisorRepo,
		profileRepo:        repos.ProfileRepo,
		substitutionRepo:   repos.SubstitutionRepo,
		classTeacherRepo:   repos.ClassTeacherRepo,
		sseActiveSvc:       repos.ActiveService,
		sseSettings:        repos.SSESettings,
		logger:             logger,
	}
}

// getUserIDFromContext extracts the user ID from the JWT context
func (s *userContextService) getUserIDFromContext(ctx context.Context) (int, error) {
	// Try to get claims from context
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0, &UserContextError{Op: "get user ID from context", Err: ErrUserNotAuthenticated}
	}
	return claims.ID, nil
}

// GetCurrentUser retrieves the currently authenticated user account
func (s *userContextService) GetCurrentUser(ctx context.Context) (*auth.Account, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	cache, key, memo := identityMemo(ctx)
	if memo {
		if account, ok := cache.accountFor(key); ok && account != nil {
			return account, nil
		}
	}

	account, err := s.accountRepo.FindByID(ctx, int64(userID))
	if err != nil {
		return nil, &UserContextError{Op: "get current user", Err: err}
	}
	if account == nil {
		return nil, &UserContextError{Op: "get current user", Err: ErrUserNotFound}
	}

	if memo {
		cache.storeAccount(key, account)
	}
	return account, nil
}

// GetCurrentPerson retrieves the person linked to the currently authenticated user
func (s *userContextService) GetCurrentPerson(ctx context.Context) (*users.Person, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	cache, key, memo := identityMemo(ctx)
	if memo {
		if person, ok := cache.personFor(key); ok {
			if person == nil {
				return nil, &UserContextError{Op: "get current person", Err: ErrUserNotLinkedToPerson}
			}
			return person, nil
		}
	}

	person, err := s.personRepo.FindByAccountID(ctx, int64(userID))
	if err != nil {
		return nil, &UserContextError{Op: "get current person", Err: err}
	}
	if memo {
		// person may be nil here — a clean "not linked" outcome worth memoizing.
		cache.storePerson(key, person)
	}
	if person == nil {
		return nil, &UserContextError{Op: "get current person", Err: ErrUserNotLinkedToPerson}
	}

	return person, nil
}

// GetCurrentStaff retrieves the staff member linked to the currently authenticated user
func (s *userContextService) GetCurrentStaff(ctx context.Context) (*users.Staff, error) {
	cache, key, memo := identityMemo(ctx)
	if memo {
		if staff, ok := cache.staffFor(key); ok {
			if staff == nil {
				return nil, &UserContextError{Op: opGetCurrentStaff, Err: ErrUserNotLinkedToStaff}
			}
			return staff, nil
		}
	}

	person, err := s.GetCurrentPerson(ctx)
	if err != nil {
		return nil, err
	}

	staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		// Check if it's a "no rows" error
		if errors.Is(err, sql.ErrNoRows) {
			if memo {
				cache.storeStaff(key, nil)
			}
			return nil, &UserContextError{Op: opGetCurrentStaff, Err: ErrUserNotLinkedToStaff}
		}
		return nil, &UserContextError{Op: opGetCurrentStaff, Err: err}
	}
	if staff == nil {
		if memo {
			cache.storeStaff(key, nil)
		}
		return nil, &UserContextError{Op: opGetCurrentStaff, Err: ErrUserNotLinkedToStaff}
	}

	if memo {
		cache.storeStaff(key, staff)
	}
	return staff, nil
}

// HasCurrentStaff exposes only the existence fact required by authorization
// policies, keeping persistence models out of the policy boundary.
func (s *userContextService) HasCurrentStaff(ctx context.Context) (bool, error) {
	staff, err := s.GetCurrentStaff(ctx)
	return err == nil && staff != nil, err
}

// GetCurrentTeacher retrieves the teacher linked to the currently authenticated user
func (s *userContextService) GetCurrentTeacher(ctx context.Context) (*users.Teacher, error) {
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		return nil, err
	}

	return s.resolveTeacherForStaff(ctx, staff)
}

// resolveTeacherForStaff resolves the teacher linked to an already-resolved
// staff member. Shared by GetCurrentTeacher and GetMyGroups so both memoize
// the same teacher stage. The error wrapping is load-bearing:
// hasValidStaffOrTeacher relies on ErrUserNotLinkedToTeacher for the clean
// "staff without teacher role" outcome, which is memoized as a nil teacher.
func (s *userContextService) resolveTeacherForStaff(ctx context.Context, staff *users.Staff) (*users.Teacher, error) {
	cache, key, memo := identityMemo(ctx)
	if memo {
		if teacher, ok := cache.teacherFor(key); ok {
			if teacher == nil {
				return nil, &UserContextError{Op: "get current teacher", Err: ErrUserNotLinkedToTeacher}
			}
			return teacher, nil
		}
	}

	teacher, err := s.teacherRepo.FindByStaffID(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get current teacher", Err: err}
	}
	if memo {
		// teacher may be nil — staff without a teacher role is a common,
		// clean outcome (see GetMyGroups) and is memoized as such.
		cache.storeTeacher(key, teacher)
	}
	if teacher == nil {
		return nil, &UserContextError{Op: "get current teacher", Err: ErrUserNotLinkedToTeacher}
	}

	return teacher, nil
}

// GetMyGroups retrieves educational groups associated with the current user
func (s *userContextService) GetMyGroups(ctx context.Context) ([]*education.Group, error) {
	if !isAuthenticated(ctx) {
		return nil, &UserContextError{Op: "get my groups", Err: ErrUserNotAuthenticated}
	}

	cache, key, memo := identityMemo(ctx)
	if memo {
		if groups, ok := cache.groupsFor(key); ok {
			return groups, nil
		}
	}

	staff, staffErr := s.GetCurrentStaff(ctx)

	// Derive teacher from the already-resolved staff to avoid duplicate identity queries.
	// GetCurrentTeacher would call GetCurrentStaff again (which calls GetCurrentPerson),
	// duplicating 2 DB round-trips. resolveTeacherForStaff reuses the staff result
	// directly and shares the memoized teacher stage with GetCurrentTeacher.
	var teacher *users.Teacher
	var teacherErr error
	if staffErr == nil && staff != nil {
		teacher, teacherErr = s.resolveTeacherForStaff(ctx, staff)
	} else {
		teacherErr = staffErr
	}

	// Check for valid linkage and unexpected errors
	valid, unexpectedErr := s.hasValidStaffOrTeacher(staffErr, teacherErr)
	if unexpectedErr != nil {
		return nil, &UserContextError{Op: "get my groups", Err: unexpectedErr}
	}
	if !valid {
		empty := []*education.Group{}
		if memo {
			cache.storeGroups(key, empty)
		}
		return empty, nil
	}

	groupMap := make(map[int64]*education.Group)
	var partialErr *PartialError

	// Add teacher's groups
	if teacher != nil && teacherErr == nil {
		if err := s.addTeacherGroups(ctx, teacher.ID, groupMap); err != nil {
			return nil, err
		}
	}

	// Add substitution groups
	if staff != nil && staffErr == nil {
		partialErr = s.addSubstitutionGroups(ctx, staff.ID, groupMap)
	}

	groups, err := s.handlePartialError(mapToSlice(groupMap), partialErr)
	// Memoize only complete results — a partially loaded group set must not
	// be pinned for the rest of the request.
	if err == nil && memo {
		cache.storeGroups(key, groups)
	}
	return groups, err
}

// isAuthenticated returns true when the context carries JWT claims with a
// non-zero user ID (i.e. the request passed through auth middleware).
func isAuthenticated(ctx context.Context) bool {
	return jwt.ClaimsFromCtx(ctx).ID > 0
}

// hasValidStaffOrTeacher checks if the user has valid staff or teacher linkage
// Returns: valid bool, unexpectedErr error
func (s *userContextService) hasValidStaffOrTeacher(staffErr, teacherErr error) (bool, error) {
	// If either lookup succeeded, user has valid linkage
	if staffErr == nil || teacherErr == nil {
		return true, nil
	}

	// Both lookups failed - check if any error is unexpected
	staffExpected := isExpectedLinkageError(staffErr)
	teacherExpected := isExpectedLinkageError(teacherErr)

	// If both errors are expected "not linked" errors, user is simply not linked
	if staffExpected && teacherExpected {
		return false, nil
	}

	// At least one unexpected error - return it for visibility
	if !teacherExpected {
		return false, teacherErr
	}
	return false, staffErr
}

// isExpectedLinkageError checks if an error is an expected "user not linked" error
func isExpectedLinkageError(err error) bool {
	return errors.Is(err, ErrUserNotLinkedToTeacher) ||
		errors.Is(err, ErrUserNotLinkedToStaff) ||
		errors.Is(err, ErrUserNotLinkedToPerson)
}

// addTeacherGroups adds groups where the teacher is assigned
func (s *userContextService) addTeacherGroups(ctx context.Context, teacherID int64, groupMap map[int64]*education.Group) error {
	teacherGroups, err := s.educationGroupRepo.FindByTeacher(ctx, teacherID)
	if err != nil {
		return &UserContextError{Op: "get my groups", Err: err}
	}
	for _, group := range teacherGroups {
		groupMap[group.ID] = group
	}
	return nil
}

// addSubstitutionGroups adds groups where the staff is an active substitute
func (s *userContextService) addSubstitutionGroups(ctx context.Context, staffID int64, groupMap map[int64]*education.Group) *PartialError {
	today := timezone.TodayDate()

	substitutions, err := s.substitutionRepo.FindActiveBySubstituteWithRelations(ctx, staffID, today)
	if err != nil {
		return &PartialError{Op: "get my groups (substitutions)", LastErr: err, FailureCount: 1}
	}

	var partialErr *PartialError
	for _, sub := range substitutions {
		group, err := s.resolveSubstitutionGroup(ctx, sub)
		if err != nil {
			partialErr = s.recordSubstitutionFailure(partialErr, sub.GroupID, err)
			continue
		}
		if group != nil {
			groupMap[group.ID] = group
			if partialErr != nil {
				partialErr.SuccessCount++
			}
		}
	}
	return partialErr
}

// GetSubstitutedGroupIDs returns the set of education group IDs the current
// staff member can access via an active substitution where the regular staff
// slot is unassigned (RegularStaffID == nil). Returns an empty set when the
// user is not staff or has no active substitutions; only unexpected errors
// are propagated.
func (s *userContextService) GetSubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	cache, key, memo := identityMemo(ctx)
	if memo {
		if subs, ok := cache.substitutedFor(key); ok {
			return subs, nil
		}
	}

	result := make(map[int64]bool)

	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if isExpectedLinkageError(err) {
			if memo {
				cache.storeSubstituted(key, result)
			}
			return result, nil
		}
		return nil, &UserContextError{Op: "get substituted group IDs", Err: err}
	}

	today := timezone.TodayDate()
	activeSubs, err := s.substitutionRepo.FindActiveBySubstitute(ctx, staff.ID, today)
	if err != nil {
		return nil, &UserContextError{Op: "get substituted group IDs", Err: err}
	}

	for _, sub := range activeSubs {
		if sub.RegularStaffID == nil {
			result[sub.GroupID] = true
		}
	}
	if memo {
		cache.storeSubstituted(key, result)
	}
	return result, nil
}

// resolveSubstitutionGroup gets the group from substitution, with fallback lookup
func (s *userContextService) resolveSubstitutionGroup(ctx context.Context, sub *education.GroupSubstitution) (*education.Group, error) {
	if sub.Group != nil {
		return sub.Group, nil
	}
	return s.educationGroupRepo.FindByID(ctx, sub.GroupID)
}

// recordSubstitutionFailure records a failure to load a substitution group
func (s *userContextService) recordSubstitutionFailure(partialErr *PartialError, groupID int64, err error) *PartialError {
	s.getLogger().Warn("failed to load group for substitution",
		slog.Int64("group_id", groupID),
		slog.String("error", err.Error()),
	)

	if partialErr == nil {
		partialErr = &PartialError{
			Op:        "get my groups (load substitution groups)",
			FailedIDs: make([]int64, 0),
		}
	}
	partialErr.FailedIDs = append(partialErr.FailedIDs, groupID)
	partialErr.FailureCount++
	partialErr.LastErr = err
	return partialErr
}

// handlePartialError handles partial error reporting for GetMyGroups
func (s *userContextService) handlePartialError(groups []*education.Group, partialErr *PartialError) ([]*education.Group, error) {
	if partialErr == nil || partialErr.FailureCount == 0 {
		return groups, nil
	}

	s.getLogger().Warn("partial failure in GetMyGroups",
		slog.Int("success_count", partialErr.SuccessCount),
		slog.Int("failure_count", partialErr.FailureCount),
		slog.Any("failed_ids", partialErr.FailedIDs),
		slog.String("operation", partialErr.Op),
	)

	if len(groups) > 0 {
		return groups, partialErr
	}
	return nil, partialErr
}

// mapToSlice converts a group map to a slice
func mapToSlice(groupMap map[int64]*education.Group) []*education.Group {
	groups := make([]*education.Group, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, group)
	}
	return groups
}

// GetMyActivityGroups retrieves activity groups associated with the current user
func (s *userContextService) GetMyActivityGroups(ctx context.Context) ([]*activities.Group, error) {
	// Try to get the current staff
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) && !errors.Is(err, ErrUserNotLinkedToPerson) {
			return nil, err
		}

		// User is not staff or not linked to person, return empty list (could expand to other user types later)
		return []*activities.Group{}, nil
	}

	// Get activity groups where the staff is a supervisor
	// First, get all planned supervisions for this staff member
	plannedSupervisions, err := s.activityGroupRepo.FindByStaffSupervisor(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get my activity groups", Err: err}
	}

	// If no groups found, return empty list
	if len(plannedSupervisions) == 0 {
		return []*activities.Group{}, nil
	}

	return plannedSupervisions, nil
}

// GetMyActiveGroups retrieves active groups associated with the current user
func (s *userContextService) GetMyActiveGroups(ctx context.Context) ([]*active.Group, error) {
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if isExpectedLinkageError(err) {
			return []*active.Group{}, nil
		}
		return nil, err
	}

	// Get active groups from activity groups
	activeGroups, err := s.getActiveGroupsFromActivities(ctx, staff.ID)
	if err != nil {
		return nil, err
	}

	// Add supervised groups
	supervisedGroups, err := s.GetMySupervisedGroups(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups - supervised", Err: err}
	}

	return mergeActiveGroups(activeGroups, supervisedGroups), nil
}

// getActiveGroupsFromActivities gets active groups for staff's activity supervisions
func (s *userContextService) getActiveGroupsFromActivities(ctx context.Context, staffID int64) ([]*active.Group, error) {
	activityGroups, err := s.activityGroupRepo.FindByStaffSupervisor(ctx, staffID)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups - activity groups", Err: err}
	}

	groupIDs := make([]int64, len(activityGroups))
	for i, group := range activityGroups {
		groupIDs[i] = group.ID
	}
	result, err := s.activeGroupRepo.FindActiveByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups - activity active", Err: err}
	}
	return result, nil
}

// mergeActiveGroups combines two slices of active groups, removing duplicates
func mergeActiveGroups(primary, additional []*active.Group) []*active.Group {
	groupMap := make(map[int64]*active.Group, len(primary)+len(additional))
	for _, group := range primary {
		groupMap[group.ID] = group
	}
	for _, group := range additional {
		if _, exists := groupMap[group.ID]; !exists {
			groupMap[group.ID] = group
		}
	}
	result := make([]*active.Group, 0, len(groupMap))
	for _, group := range groupMap {
		result = append(result, group)
	}
	return result
}

// GetMySupervisedGroups retrieves active groups supervised by the current user
func (s *userContextService) GetMySupervisedGroups(ctx context.Context) ([]*active.Group, error) {
	// Get current staff member
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) && !errors.Is(err, ErrUserNotLinkedToPerson) {
			return nil, err
		}
		// User is not staff, return empty list
		return []*active.Group{}, nil
	}

	// Find active supervisions for this staff member
	supervisions, err := s.supervisorRepo.FindActiveByStaffID(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get supervised groups", Err: err}
	}

	if len(supervisions) == 0 {
		return []*active.Group{}, nil
	}

	// Collect group IDs for batch loading (more efficient than individual FindByID calls)
	now := time.Now()
	groupIDs := make([]int64, 0, len(supervisions))
	for _, supervision := range supervisions {
		// Check if supervision itself is still active (not ended)
		if !activeService.IsSupervisorActive(supervision, now) {
			// Log for observability; helps diagnose silent filters of ended supervisions
			s.getLogger().DebugContext(ctx, "skipping ended supervision",
				slog.Int64("supervision_id", supervision.ID),
				slog.Int64("group_id", supervision.GroupID),
				slog.Int64("staff_id", staff.ID))
			continue // Skip ended supervisions
		}
		groupIDs = append(groupIDs, supervision.GroupID)
	}

	if len(groupIDs) == 0 {
		return []*active.Group{}, nil
	}

	// Batch load all groups with their rooms (FindByIDs loads Room relation)
	groupsMap, err := s.activeGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, &UserContextError{Op: "get supervised groups", Err: err}
	}

	// Convert map to slice, filtering only active groups
	var supervisedGroups []*active.Group
	for _, groupID := range groupIDs {
		if group, ok := groupsMap[groupID]; ok && group.IsActive() {
			supervisedGroups = append(supervisedGroups, group)
		}
	}

	return supervisedGroups, nil
}

// checkGroupAccess is a helper function to check if the current user has access to a specific group
func (s *userContextService) checkGroupAccess(ctx context.Context, groupID int64) (*active.Group, error) {
	// Verify group exists
	group, err := s.activeGroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}

	// Check if current user has access to this group
	// This could be more sophisticated checking specific permissions
	// For now, we'll assume the user has access if they can see the group in their supervised/active groups
	userGroups, err := s.GetMyActiveGroups(ctx)
	if err != nil {
		return nil, err
	}

	hasAccess := false
	for _, g := range userGroups {
		if g.ID == groupID {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		// Try supervised groups if not found in active groups
		supervisedGroups, err := s.GetMySupervisedGroups(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range supervisedGroups {
			if g.ID == groupID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		return nil, ErrUserNotAuthorized
	}

	return group, nil
}

// GetGroupStudents retrieves students in a specific group where the current user has access
func (s *userContextService) GetGroupStudents(ctx context.Context, groupID int64) ([]*users.Student, error) {
	// Check access to the group
	_, err := s.checkGroupAccess(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: opGetGroupStudents, Err: err}
	}

	// Get all visits for this group
	visits, err := s.visitsRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: opGetGroupStudents, Err: err}
	}

	// Create a map to deduplicate student IDs
	studentIDs := make(map[int64]bool)
	for _, visit := range visits {
		studentIDs[visit.StudentID] = true
	}

	// Convert map keys to slice
	ids := slices.Collect(maps.Keys(studentIDs))

	// If no students found, return empty slice
	if len(ids) == 0 {
		return []*users.Student{}, nil
	}

	// Get students by IDs in a single batch query
	studentsMap, err := s.studentRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &UserContextError{Op: opGetGroupStudents, Err: err}
	}
	students := make([]*users.Student, 0, len(studentsMap))
	for _, student := range studentsMap {
		students = append(students, student)
	}

	return students, nil
}

// GetGroupVisits retrieves active visits for a specific group where the current user has access
func (s *userContextService) GetGroupVisits(ctx context.Context, groupID int64) ([]*active.Visit, error) {
	// Check access to the group
	_, err := s.checkGroupAccess(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: "get group visits", Err: err}
	}

	// Get active visits for this group
	visits, err := s.visitsRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: "get group visits", Err: err}
	}

	// Filter to only include active visits (no end time)
	var activeVisits []*active.Visit
	for _, visit := range visits {
		if visit.ExitTime == nil {
			activeVisits = append(activeVisits, visit)
		}
	}

	return activeVisits, nil
}

// GetCurrentProfile retrieves the full profile for the current user including person, account, and profile data
func (s *userContextService) GetCurrentProfile(ctx context.Context) (map[string]interface{}, error) {
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "get current profile", Err: err}
	}

	person, _ := s.GetCurrentPerson(ctx)

	response := buildBaseResponse(account)
	addPersonOrAccountData(response, account, person)
	addProfileDataToResponse(ctx, s, response, account.ID)

	return response, nil
}

// buildBaseResponse builds the base response with account data
func buildBaseResponse(account *auth.Account) map[string]interface{} {
	response := map[string]interface{}{
		"email":      account.Email,
		"username":   account.Username,
		"last_login": account.LastLogin,
	}

	addProfileFieldIfNotEmpty(response, "avatar", account.Avatar)

	return response
}

// addPersonOrAccountData adds person data if available, otherwise account fallback
func addPersonOrAccountData(response map[string]interface{}, account *auth.Account, person *users.Person) {
	if person != nil {
		addPersonData(response, person)
	} else {
		addAccountFallbackData(response, account)
	}
}

// addPersonData adds person data to response
func addPersonData(response map[string]interface{}, person *users.Person) {
	response["id"] = person.ID
	response["first_name"] = person.FirstName
	response["last_name"] = person.LastName
	response["created_at"] = person.CreatedAt
	response["updated_at"] = person.UpdatedAt

	if person.TagID != nil {
		response["rfid_card"] = *person.TagID
	}
}

// addAccountFallbackData adds account data as fallback when person doesn't exist
func addAccountFallbackData(response map[string]interface{}, account *auth.Account) {
	response["id"] = account.ID
	response["created_at"] = account.CreatedAt
	response["updated_at"] = account.UpdatedAt
	response["first_name"] = ""
	response["last_name"] = ""
}

// addProfileDataToResponse adds profile data if it exists
func addProfileDataToResponse(ctx context.Context, s *userContextService, response map[string]interface{}, accountID int64) {
	if accountID <= 0 {
		return
	}

	profile, err := s.profileRepo.FindByAccountID(ctx, accountID)
	if err != nil || profile == nil {
		return
	}

	addProfileFieldIfNotEmpty(response, "bio", profile.Bio)
	addProfileFieldIfNotEmpty(response, "settings", profile.Settings)
}

// addProfileFieldIfNotEmpty adds a profile field to response if not empty
func addProfileFieldIfNotEmpty(response map[string]interface{}, key, value string) {
	if value != "" {
		response[key] = value
	}
}

// UpdateCurrentProfile updates the current user's profile with the provided data
func (s *userContextService) UpdateCurrentProfile(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error) {
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	person, personErr := s.GetCurrentPerson(ctx)

	if err := s.updatePersonDataInTx(ctx, account, person, personErr, updates); err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	if err := s.updateAccountUsernameInTx(ctx, account, updates); err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	if err := s.updateProfileBioInTx(ctx, account.ID, updates); err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	// The writes above may have created a person for an account whose person
	// stage is memoized as "not linked" (or changed memoized person/account
	// fields). Drop the entry so the trailing re-read returns committed state.
	invalidateIdentity(ctx)

	return s.GetCurrentProfile(ctx)
}

// updatePersonDataInTx handles person creation or update within transaction
func (s *userContextService) updatePersonDataInTx(ctx context.Context, account *auth.Account, person *users.Person, personErr error, updates map[string]interface{}) error {
	firstName, hasFirstName := updates["first_name"].(string)
	lastName, hasLastName := updates["last_name"].(string)

	if !hasFirstName && !hasLastName {
		return nil
	}

	if personErr != nil || person == nil {
		return s.createPersonFromUpdates(ctx, account.ID, firstName, lastName)
	}

	return s.updateExistingPersonFields(ctx, person, firstName, hasFirstName, lastName, hasLastName)
}

// createPersonFromUpdates creates a new person record from update data
func (s *userContextService) createPersonFromUpdates(ctx context.Context, accountID int64, firstName, lastName string) error {
	if firstName == "" || lastName == "" {
		return errors.New("first name and last name are required to create profile")
	}

	person := &users.Person{
		AccountID: &accountID,
		FirstName: firstName,
		LastName:  lastName,
	}
	person.SetTenantID(tenant.FromContext(ctx))

	return s.personRepo.Create(ctx, person)
}

// updateExistingPersonFields updates existing person fields
func (s *userContextService) updateExistingPersonFields(ctx context.Context, person *users.Person, firstName string, hasFirstName bool, lastName string, hasLastName bool) error {
	needsUpdate := false

	if hasFirstName && firstName != "" {
		person.FirstName = firstName
		needsUpdate = true
	}

	if hasLastName && lastName != "" {
		person.LastName = lastName
		needsUpdate = true
	}

	if needsUpdate {
		return s.personRepo.Update(ctx, person)
	}

	return nil
}

// updateAccountUsernameInTx updates account username within transaction
func (s *userContextService) updateAccountUsernameInTx(ctx context.Context, account *auth.Account, updates map[string]interface{}) error {
	username, ok := updates["username"].(string)
	if !ok {
		return nil
	}

	if username == "" {
		account.Username = nil
	} else {
		account.Username = &username
	}

	return s.accountRepo.Update(ctx, account)
}

// updateProfileBioInTx updates or creates profile for bio update
func (s *userContextService) updateProfileBioInTx(ctx context.Context, accountID int64, updates map[string]interface{}) error {
	bio, hasBio := updates["bio"].(string)
	if !hasBio {
		return nil
	}

	profile, _ := s.profileRepo.FindByAccountID(ctx, accountID)
	if profile == nil {
		return s.createProfileWithBio(ctx, accountID, bio)
	}

	return s.updateExistingProfileBio(ctx, profile, bio)
}

// createProfileWithBio creates a new profile with bio
func (s *userContextService) createProfileWithBio(ctx context.Context, accountID int64, bio string) error {
	profile := &users.Profile{
		AccountID: accountID,
		Bio:       bio,
		Settings:  "{}",
	}
	return s.profileRepo.Create(ctx, profile)
}

// updateExistingProfileBio updates existing profile's bio
func (s *userContextService) updateExistingProfileBio(ctx context.Context, profile *users.Profile, bio string) error {
	profile.Bio = bio
	return s.profileRepo.Update(ctx, profile)
}

// UpdateAvatar updates the current user's avatar
func (s *userContextService) UpdateAvatar(ctx context.Context, avatarURL string) (map[string]interface{}, error) {
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "update avatar", Err: err}
	}

	oldAvatarPath := getOldAvatarPath(account.Avatar)
	if err := s.accountRepo.UpdateAvatar(ctx, account.ID, avatarURL); err != nil {
		return nil, &UserContextError{Op: "update avatar", Err: err}
	}

	s.cleanupOldAvatar(ctx, oldAvatarPath)

	// accountRepo.UpdateAvatar does not mutate the in-memory account struct —
	// a memoized account would ship the OLD avatar URL in the response below.
	invalidateIdentity(ctx)

	return s.GetCurrentProfile(ctx)
}

// getOldAvatarPath returns the file path of old avatar if it needs cleanup
func getOldAvatarPath(currentAvatar string) string {
	if currentAvatar != "" && strings.HasPrefix(currentAvatar, "/uploads/avatars/") {
		return filepath.Join("public", strings.TrimPrefix(currentAvatar, "/"))
	}
	return ""
}

// cleanupOldAvatar deletes old avatar file if path is provided
func (s *userContextService) cleanupOldAvatar(ctx context.Context, oldAvatarPath string) {
	if oldAvatarPath == "" {
		return
	}

	if err := os.Remove(oldAvatarPath); err != nil {
		s.getLogger().WarnContext(ctx, "failed to delete old avatar file",
			slog.String("path", oldAvatarPath),
			slog.String("error", err.Error()))
	}
}
