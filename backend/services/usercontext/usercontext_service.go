package usercontext

import (
	"context"
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

// NewUserContextService creates a new user context service
func NewUserContextService(
	accountRepo auth.AccountRepository,
	personRepo users.PersonRepository,
	staffRepo users.StaffRepository,
	teacherRepo users.TeacherRepository,
	studentRepo users.StudentRepository,
	educationGroupRepo education.GroupRepository,
	activityGroupRepo activities.GroupRepository,
	activeGroupRepo active.GroupRepository,
	visitsRepo active.VisitRepository,
	supervisorRepo active.GroupSupervisorRepository,
	profileRepo users.ProfileRepository,
	substitutionRepo education.GroupSubstitutionRepository,
	db *bun.DB,
) UserContextService {
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
		substitutionRepo:   substitutionRepo,
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
		if err.Error() == "sql: no rows in result set" || strings.Contains(err.Error(), "no rows in result set") {
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
	// Try to get the current staff first (for substitutions)
	staff, staffErr := s.GetCurrentStaff(ctx)

	// Try to get the current teacher
	teacher, teacherErr := s.GetCurrentTeacher(ctx)

	// If user is neither staff nor teacher, return empty list
	if staffErr != nil && teacherErr != nil {
		if !errors.Is(teacherErr, ErrUserNotLinkedToTeacher) && !errors.Is(teacherErr, ErrUserNotLinkedToStaff) && !errors.Is(teacherErr, ErrUserNotLinkedToPerson) {
			return nil, teacherErr
		}
		return []*education.Group{}, nil
	}

	// Create a map to store unique groups (to avoid duplicates)
	groupMap := make(map[int64]*education.Group)

	// Track partial failures across all operations
	var partialErr *PartialError
	failedGroupIDs := make([]int64, 0)

	// Get groups where the teacher is assigned (if user is a teacher)
	if teacher != nil && teacherErr == nil {
		teacherGroups, err := s.educationGroupRepo.FindByTeacher(ctx, teacher.ID)
		if err != nil {
			return nil, &UserContextError{Op: "get my groups", Err: err}
		}

		// Add teacher's groups to the map
		for _, group := range teacherGroups {
			groupMap[group.ID] = group
		}
	}

	// Get groups where the staff member is an active substitute (if user is staff)
	if staff != nil && staffErr == nil {
		// Get today's date for checking active substitutions
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Find active substitutions for this staff member
		substitutions, err := s.substitutionRepo.FindActive(ctx, today)
		if err != nil {
			return nil, &UserContextError{Op: "get my groups (substitutions)", Err: err}
		}

		// Filter substitutions where current staff is the substitute
		for _, sub := range substitutions {
			if sub.SubstituteStaffID == staff.ID {
				// Get the group for this substitution
				group, err := s.educationGroupRepo.FindByID(ctx, sub.GroupID)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"group_id":        sub.GroupID,
						"substitution_id": sub.ID,
						"error":           err,
					}).Warn("Failed to load group for substitution")

					// Track the failure
					failedGroupIDs = append(failedGroupIDs, sub.GroupID)
					if partialErr == nil {
						partialErr = &PartialError{
							Op:        "get my groups (load substitution groups)",
							FailedIDs: failedGroupIDs,
						}
					}
					partialErr.FailureCount++
					partialErr.LastErr = err
					continue // Skip if we can't load the group
				}
				if group != nil {
					groupMap[group.ID] = group
					if partialErr != nil {
						partialErr.SuccessCount++
					}
				}
			}
		}
	}

	// Convert map to slice
	groups := make([]*education.Group, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, group)
	}

	// Return partial error if some groups failed to load
	if partialErr != nil && partialErr.FailureCount > 0 {
		partialErr.FailedIDs = failedGroupIDs
		// Log summary of partial failures
		logrus.WithFields(logrus.Fields{
			"success_count": partialErr.SuccessCount,
			"failure_count": partialErr.FailureCount,
			"failed_ids":    partialErr.FailedIDs,
			"operation":     partialErr.Op,
		}).Warn("Partial failure in GetMyGroups")

		// If we have at least some groups, return them with the partial error
		if len(groups) > 0 {
			return groups, partialErr
		}
		// If all failed, return the partial error as main error
		return nil, partialErr
	}

	return groups, nil
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
	// Try to get the current staff
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) && !errors.Is(err, ErrUserNotLinkedToPerson) {
			return nil, err
		}

		// User is not staff or not linked to person, return empty list
		return []*active.Group{}, nil
	}

	// Note: Educational groups don't directly create active sessions
	// Active groups are only created from activity groups via the group_id column
	// So we skip checking educational groups here

	// Get activity groups where the staff is a supervisor
	activityGroups, err := s.activityGroupRepo.FindByStaffSupervisor(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups - activity groups", Err: err}
	}

	var activityGroupIDs []int64
	for _, group := range activityGroups {
		activityGroupIDs = append(activityGroupIDs, group.ID)
	}

	// Get active groups related to the staff's activity groups
	var activeGroups []*active.Group

	// Get active groups from activity group IDs
	if len(activityGroupIDs) > 0 {
		for _, groupID := range activityGroupIDs {
			activityActiveGroups, err := s.activeGroupRepo.FindActiveByGroupID(ctx, groupID)
			if err != nil {
				return nil, &UserContextError{Op: "get my active groups - activity active", Err: err}
			}
			activeGroups = append(activeGroups, activityActiveGroups...)
		}
	}

	// Also include any active groups this staff member is currently supervising
	supervisedGroups, err := s.GetMySupervisedGroups(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups - supervised", Err: err}
	}

	// Add supervised groups, avoiding duplicates
	groupMap := make(map[int64]*active.Group)
	for _, group := range activeGroups {
		groupMap[group.ID] = group
	}

	for _, group := range supervisedGroups {
		if _, exists := groupMap[group.ID]; !exists {
			groupMap[group.ID] = group
		}
	}

	// Convert map back to slice
	result := make([]*active.Group, 0, len(groupMap))
	for _, group := range groupMap {
		result = append(result, group)
	}

	return result, nil
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

	// Get the active groups for these supervisions
	var supervisedGroups []*active.Group
	for _, supervision := range supervisions {
		// Check if supervision itself is still active (not ended)
		if !supervision.IsActive() {
			// Log for observability; helps diagnose silent filters of ended supervisions
			log.Printf("Skipping ended supervision: supervision_id=%d group_id=%d staff_id=%d", supervision.ID, supervision.GroupID, staff.ID)
			continue // Skip ended supervisions
		}

		group, err := s.activeGroupRepo.FindByID(ctx, supervision.GroupID)
		if err != nil {
			return nil, &UserContextError{Op: "get supervised groups", Err: err}
		}
		// Only include groups that are still active (not ended)
		if group != nil && group.IsActive() {
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
	// Get current account
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "get current profile", Err: err}
	}

	// Try to get current person (might not exist)
	person, err := s.GetCurrentPerson(ctx)

	// Build response starting with account data
	response := map[string]interface{}{
		"email":      account.Email,
		"username":   account.Username,
		"last_login": account.LastLogin,
	}

	// If person exists, add person data
	if err == nil && person != nil {
		response["id"] = person.ID
		response["first_name"] = person.FirstName
		response["last_name"] = person.LastName
		response["created_at"] = person.CreatedAt
		response["updated_at"] = person.UpdatedAt

		// Add RFID card if present
		if person.TagID != nil {
			response["rfid_card"] = *person.TagID
		}
	} else {
		// No person record - use account data for timestamps
		response["id"] = account.ID
		response["created_at"] = account.CreatedAt
		response["updated_at"] = account.UpdatedAt
		// Set empty values for person fields
		response["first_name"] = ""
		response["last_name"] = ""
	}

	// Try to get profile (might not exist)
	if account.ID > 0 {
		profile, _ := s.profileRepo.FindByAccountID(ctx, account.ID)

		// Add profile data if exists
		if profile != nil {
			if profile.Avatar != "" {
				response["avatar"] = profile.Avatar
			}
			if profile.Bio != "" {
				response["bio"] = profile.Bio
			}
			if profile.Settings != "" {
				response["settings"] = profile.Settings
			}
		}
	}

	return response, nil
}

// UpdateCurrentProfile updates the current user's profile with the provided data
func (s *userContextService) UpdateCurrentProfile(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error) {
	// Get current account
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	// Try to get current person (might not exist)
	person, personErr := s.GetCurrentPerson(ctx)

	// Start transaction
	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		// Handle person data updates
		needsPersonUpdate := false

		// Check if we need to create or update person
		firstName, hasFirstName := updates["first_name"].(string)
		lastName, hasLastName := updates["last_name"].(string)

		if hasFirstName || hasLastName {
			if personErr != nil || person == nil {
				// Create new person record
				person = &users.Person{
					AccountID: &account.ID,
					FirstName: firstName,
					LastName:  lastName,
				}

				// Validate person data
				if person.FirstName == "" || person.LastName == "" {
					return errors.New("first name and last name are required to create profile")
				}

				if err := s.personRepo.Create(txCtx, person); err != nil {
					return err
				}
			} else {
				// Update existing person
				if hasFirstName && firstName != "" {
					person.FirstName = firstName
					needsPersonUpdate = true
				}
				if hasLastName && lastName != "" {
					person.LastName = lastName
					needsPersonUpdate = true
				}

				if needsPersonUpdate {
					if err := s.personRepo.Update(txCtx, person); err != nil {
						return err
					}
				}
			}
		}

		// Update account username if provided
		if username, ok := updates["username"].(string); ok {
			if username == "" {
				account.Username = nil
			} else {
				account.Username = &username
			}
			if err := s.accountRepo.Update(txCtx, account); err != nil {
				return err
			}
		}

		// Update or create profile for bio/avatar
		if bio, hasBio := updates["bio"].(string); hasBio {
			// Get or create profile
			profile, _ := s.profileRepo.FindByAccountID(txCtx, account.ID)
			if profile == nil {
				// Create new profile
				profile = &users.Profile{
					AccountID: account.ID,
					Bio:       bio,
					Settings:  "{}", // Initialize with empty JSON object
				}
				if err := s.profileRepo.Create(txCtx, profile); err != nil {
					return err
				}
			} else {
				// Update existing profile
				profile.Bio = bio
				if err := s.profileRepo.Update(txCtx, profile); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	// Return updated profile
	return s.GetCurrentProfile(ctx)
}

// UpdateAvatar updates the current user's avatar
func (s *userContextService) UpdateAvatar(ctx context.Context, avatarURL string) (map[string]interface{}, error) {
	// Get current account
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "update avatar", Err: err}
	}

	// Track old avatar path for cleanup after successful transaction
	var oldAvatarPath string

	// Start transaction
	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		// Get or create profile
		profile, _ := s.profileRepo.FindByAccountID(txCtx, account.ID)
		if profile == nil {
			// Create new profile with avatar
			profile = &users.Profile{
				AccountID: account.ID,
				Avatar:    avatarURL,
				Settings:  "{}", // Initialize with empty JSON object
			}
			if err := s.profileRepo.Create(txCtx, profile); err != nil {
				return err
			}
		} else {
			// Store old avatar path for deletion after successful commit
			if profile.Avatar != "" && strings.HasPrefix(profile.Avatar, "/uploads/avatars/") {
				oldAvatarPath = filepath.Join("public", profile.Avatar)
			}

			// Update existing profile
			profile.Avatar = avatarURL
			if err := s.profileRepo.Update(txCtx, profile); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, &UserContextError{Op: "update avatar", Err: err}
	}

	// Delete old avatar file only after successful transaction commit
	if oldAvatarPath != "" {
		if err := os.Remove(oldAvatarPath); err != nil {
			// Log error but don't fail the operation since DB update succeeded
			log.Printf("Failed to delete old avatar file: %v", err)
		}
	}

	// Return updated profile
	return s.GetCurrentProfile(ctx)
}
