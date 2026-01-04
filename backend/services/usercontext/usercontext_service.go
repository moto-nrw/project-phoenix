package usercontext

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	db                 *bun.DB
	txHandler          *base.TxHandler
}

// NewUserContextServiceWithRepos creates a new user context service using a repositories struct
func NewUserContextServiceWithRepos(repos UserContextRepositories, db *bun.DB) UserContextService {
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
		db:                 db,
		txHandler:          base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *userContextService) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction
	var accountRepo = s.accountRepo
	var personRepo = s.personRepo
	var staffRepo = s.staffRepo
	var teacherRepo = s.teacherRepo
	var studentRepo = s.studentRepo
	var educationGroupRepo = s.educationGroupRepo
	var activityGroupRepo = s.activityGroupRepo
	var activeGroupRepo = s.activeGroupRepo
	var visitsRepo = s.visitsRepo
	var supervisorRepo = s.supervisorRepo
	var profileRepo = s.profileRepo

	// Apply transaction to repositories that implement TransactionalRepository
	if txRepo, ok := s.accountRepo.(base.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(auth.AccountRepository)
	}
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(users.PersonRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(users.StaffRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(users.TeacherRepository)
	}
	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(users.StudentRepository)
	}
	if txRepo, ok := s.educationGroupRepo.(base.TransactionalRepository); ok {
		educationGroupRepo = txRepo.WithTx(tx).(education.GroupRepository)
	}
	if txRepo, ok := s.activityGroupRepo.(base.TransactionalRepository); ok {
		activityGroupRepo = txRepo.WithTx(tx).(activities.GroupRepository)
	}
	if txRepo, ok := s.activeGroupRepo.(base.TransactionalRepository); ok {
		activeGroupRepo = txRepo.WithTx(tx).(active.GroupRepository)
	}
	if txRepo, ok := s.visitsRepo.(base.TransactionalRepository); ok {
		visitsRepo = txRepo.WithTx(tx).(active.VisitRepository)
	}
	if txRepo, ok := s.supervisorRepo.(base.TransactionalRepository); ok {
		supervisorRepo = txRepo.WithTx(tx).(active.GroupSupervisorRepository)
	}
	if txRepo, ok := s.profileRepo.(base.TransactionalRepository); ok {
		profileRepo = txRepo.WithTx(tx).(users.ProfileRepository)
	}

	// Return a new service with the transaction
	return &userContextService{
		accountRepo:        accountRepo,
		personRepo:         personRepo,
		staffRepo:          staffRepo,
		teacherRepo:        teacherRepo,
		studentRepo:        studentRepo,
		educationGroupRepo: educationGroupRepo,
		activityGroupRepo:  activityGroupRepo,
		activeGroupRepo:    activeGroupRepo,
		visitsRepo:         visitsRepo,
		supervisorRepo:     supervisorRepo,
		profileRepo:        profileRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
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

	account, err := s.accountRepo.FindByID(ctx, int64(userID))
	if err != nil {
		return nil, &UserContextError{Op: "get current user", Err: err}
	}
	if account == nil {
		return nil, &UserContextError{Op: "get current user", Err: ErrUserNotFound}
	}

	return account, nil
}

// GetCurrentPerson retrieves the person linked to the currently authenticated user
func (s *userContextService) GetCurrentPerson(ctx context.Context) (*users.Person, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	person, err := s.personRepo.FindByAccountID(ctx, int64(userID))
	if err != nil {
		return nil, &UserContextError{Op: "get current person", Err: err}
	}
	if person == nil {
		return nil, &UserContextError{Op: "get current person", Err: ErrUserNotLinkedToPerson}
	}

	return person, nil
}

// GetCurrentStaff retrieves the staff member linked to the currently authenticated user
func (s *userContextService) GetCurrentStaff(ctx context.Context) (*users.Staff, error) {
	person, err := s.GetCurrentPerson(ctx)
	if err != nil {
		return nil, err
	}

	staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		// Check if it's a "no rows" error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &UserContextError{Op: "get current staff", Err: ErrUserNotLinkedToStaff}
		}
		return nil, &UserContextError{Op: "get current staff", Err: err}
	}
	if staff == nil {
		return nil, &UserContextError{Op: "get current staff", Err: ErrUserNotLinkedToStaff}
	}

	return staff, nil
}

// GetCurrentTeacher retrieves the teacher linked to the currently authenticated user
func (s *userContextService) GetCurrentTeacher(ctx context.Context) (*users.Teacher, error) {
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		return nil, err
	}

	teacher, err := s.teacherRepo.FindByStaffID(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get current teacher", Err: err}
	}
	if teacher == nil {
		return nil, &UserContextError{Op: "get current teacher", Err: ErrUserNotLinkedToTeacher}
	}

	return teacher, nil
}

// GetMyGroups retrieves educational groups associated with the current user
func (s *userContextService) GetMyGroups(ctx context.Context) ([]*education.Group, error) {
	staff, staffErr := s.GetCurrentStaff(ctx)
	teacher, teacherErr := s.GetCurrentTeacher(ctx)

	// Check for valid linkage and unexpected errors
	valid, unexpectedErr := s.hasValidStaffOrTeacher(staffErr, teacherErr)
	if unexpectedErr != nil {
		return nil, &UserContextError{Op: "get my groups", Err: unexpectedErr}
	}
	if !valid {
		return []*education.Group{}, nil
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

	groups := mapToSlice(groupMap)
	return s.handlePartialError(groups, partialErr)
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
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

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

// resolveSubstitutionGroup gets the group from substitution, with fallback lookup
func (s *userContextService) resolveSubstitutionGroup(ctx context.Context, sub *education.GroupSubstitution) (*education.Group, error) {
	if sub.Group != nil {
		return sub.Group, nil
	}
	return s.educationGroupRepo.FindByID(ctx, sub.GroupID)
}

// recordSubstitutionFailure records a failure to load a substitution group
func (s *userContextService) recordSubstitutionFailure(partialErr *PartialError, groupID int64, err error) *PartialError {
	logrus.WithFields(logrus.Fields{
		"group_id": groupID,
		"error":    err,
	}).Warn("Failed to load group for substitution")

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

	logrus.WithFields(logrus.Fields{
		"success_count": partialErr.SuccessCount,
		"failure_count": partialErr.FailureCount,
		"failed_ids":    partialErr.FailedIDs,
		"operation":     partialErr.Op,
	}).Warn("Partial failure in GetMyGroups")

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

	var result []*active.Group
	for _, group := range activityGroups {
		activeGroups, err := s.activeGroupRepo.FindActiveByGroupID(ctx, group.ID)
		if err != nil {
			return nil, &UserContextError{Op: "get my active groups - activity active", Err: err}
		}
		result = append(result, activeGroups...)
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
	groupIDs := make([]int64, 0, len(supervisions))
	for _, supervision := range supervisions {
		// Check if supervision itself is still active (not ended)
		if !supervision.IsActive() {
			// Log for observability; helps diagnose silent filters of ended supervisions
			log.Printf("Skipping ended supervision: supervision_id=%d group_id=%d staff_id=%d", supervision.ID, supervision.GroupID, staff.ID)
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
		return nil, &UserContextError{Op: "get group students", Err: err}
	}

	// Get all visits for this group
	visits, err := s.visitsRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: "get group students", Err: err}
	}

	// Create a map to deduplicate student IDs
	studentIDs := make(map[int64]bool)
	for _, visit := range visits {
		studentIDs[visit.StudentID] = true
	}

	// Convert map keys to slice
	ids := make([]int64, 0, len(studentIDs))
	for id := range studentIDs {
		ids = append(ids, id)
	}

	// If no students found, return empty slice
	if len(ids) == 0 {
		return []*users.Student{}, nil
	}

	// Get students by IDs
	var students []*users.Student
	for _, id := range ids {
		student, err := s.studentRepo.FindByID(ctx, id)
		if err != nil {
			return nil, &UserContextError{Op: "get group students", Err: err}
		}
		if student != nil {
			students = append(students, student)
		}
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
	return map[string]interface{}{
		"email":      account.Email,
		"username":   account.Username,
		"last_login": account.LastLogin,
	}
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

	addProfileFieldIfNotEmpty(response, "avatar", profile.Avatar)
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

	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if err := s.updatePersonDataInTx(txCtx, account, person, personErr, updates); err != nil {
			return err
		}

		if err := s.updateAccountUsernameInTx(txCtx, account, updates); err != nil {
			return err
		}

		return s.updateProfileBioInTx(txCtx, account.ID, updates)
	})

	if err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

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

	var oldAvatarPath string

	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		var updateErr error
		oldAvatarPath, updateErr = s.updateAvatarInTx(txCtx, account.ID, avatarURL)
		return updateErr
	})

	if err != nil {
		return nil, &UserContextError{Op: "update avatar", Err: err}
	}

	cleanupOldAvatar(oldAvatarPath)

	return s.GetCurrentProfile(ctx)
}

// updateAvatarInTx updates or creates profile with new avatar, returns old avatar path
func (s *userContextService) updateAvatarInTx(ctx context.Context, accountID int64, avatarURL string) (string, error) {
	profile, _ := s.profileRepo.FindByAccountID(ctx, accountID)
	if profile == nil {
		return "", s.createProfileWithAvatar(ctx, accountID, avatarURL)
	}

	oldPath := getOldAvatarPath(profile.Avatar)
	profile.Avatar = avatarURL

	if err := s.profileRepo.Update(ctx, profile); err != nil {
		return "", err
	}

	return oldPath, nil
}

// createProfileWithAvatar creates a new profile with avatar
func (s *userContextService) createProfileWithAvatar(ctx context.Context, accountID int64, avatarURL string) error {
	profile := &users.Profile{
		AccountID: accountID,
		Avatar:    avatarURL,
		Settings:  "{}",
	}
	return s.profileRepo.Create(ctx, profile)
}

// getOldAvatarPath returns the file path of old avatar if it needs cleanup
func getOldAvatarPath(currentAvatar string) string {
	if currentAvatar != "" && strings.HasPrefix(currentAvatar, "/uploads/avatars/") {
		// Strip leading slash to avoid filepath.Join treating it as absolute
		// filepath.Join("public", "/uploads/...") would drop "public" prefix
		relativePath := strings.TrimPrefix(currentAvatar, "/")
		return filepath.Join("public", relativePath)
	}
	return ""
}

// cleanupOldAvatar deletes old avatar file if path is provided
func cleanupOldAvatar(oldAvatarPath string) {
	if oldAvatarPath == "" {
		return
	}

	if err := os.Remove(oldAvatarPath); err != nil {
		log.Printf("Failed to delete old avatar file: %v", err)
	}
}
