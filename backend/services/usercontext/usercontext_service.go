package usercontext

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// userContextService implements the UserContextService interface
type userContextService struct {
	accountRepo       auth.AccountRepository
	personRepo        users.PersonRepository
	staffRepo         users.StaffRepository
	teacherRepo       users.TeacherRepository
	studentRepo       users.StudentRepository
	educationGroupRepo education.GroupRepository
	activityGroupRepo activities.GroupRepository
	activeGroupRepo   active.GroupRepository
	visitsRepo        active.VisitRepository
	supervisorRepo    active.GroupSupervisorRepository
	db                *bun.DB
	txHandler         *base.TxHandler
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
	db *bun.DB,
) UserContextService {
	return &userContextService{
		accountRepo:       accountRepo,
		personRepo:        personRepo,
		staffRepo:         staffRepo,
		teacherRepo:       teacherRepo,
		studentRepo:       studentRepo,
		educationGroupRepo: educationGroupRepo,
		activityGroupRepo: activityGroupRepo,
		activeGroupRepo:   activeGroupRepo,
		visitsRepo:        visitsRepo,
		supervisorRepo:    supervisorRepo,
		db:                db,
		txHandler:         base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *userContextService) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction
	var accountRepo auth.AccountRepository = s.accountRepo
	var personRepo users.PersonRepository = s.personRepo
	var staffRepo users.StaffRepository = s.staffRepo
	var teacherRepo users.TeacherRepository = s.teacherRepo
	var studentRepo users.StudentRepository = s.studentRepo
	var educationGroupRepo education.GroupRepository = s.educationGroupRepo
	var activityGroupRepo activities.GroupRepository = s.activityGroupRepo
	var activeGroupRepo active.GroupRepository = s.activeGroupRepo
	var visitsRepo active.VisitRepository = s.visitsRepo
	var supervisorRepo active.GroupSupervisorRepository = s.supervisorRepo

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

	// Return a new service with the transaction
	return &userContextService{
		accountRepo:       accountRepo,
		personRepo:        personRepo,
		staffRepo:         staffRepo,
		teacherRepo:       teacherRepo,
		studentRepo:       studentRepo,
		educationGroupRepo: educationGroupRepo,
		activityGroupRepo: activityGroupRepo,
		activeGroupRepo:   activeGroupRepo,
		visitsRepo:        visitsRepo,
		supervisorRepo:    supervisorRepo,
		db:                s.db,
		txHandler:         s.txHandler.WithTx(tx),
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
	// Try to get the current teacher
	teacher, err := s.GetCurrentTeacher(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToTeacher) && !errors.Is(err, ErrUserNotLinkedToStaff) {
			return nil, err
		}
		
		// User is not a teacher, return empty list (could expand to other user types later)
		return []*education.Group{}, nil
	}

	// Get groups where the teacher is assigned
	groups, err := s.educationGroupRepo.FindByTeacher(ctx, teacher.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get my groups", Err: err}
	}

	return groups, nil
}

// GetMyActivityGroups retrieves activity groups associated with the current user
func (s *userContextService) GetMyActivityGroups(ctx context.Context) ([]*activities.Group, error) {
	// Try to get the current staff
	_, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) {
			return nil, err
		}
		
		// User is not staff, return empty list (could expand to other user types later)
		return []*activities.Group{}, nil
	}

	// Get activity groups where the staff is a supervisor
	// Note: This implementation would need to be adapted based on how activity groups and supervisors are related
	// Placeholder implementation returning empty list
	groups := []*activities.Group{}
	// TODO: Implement proper lookup through supervisor repository

	return groups, nil
}

// GetMyActiveGroups retrieves active groups associated with the current user
func (s *userContextService) GetMyActiveGroups(ctx context.Context) ([]*active.Group, error) {
	// Try to get the current staff
	_, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) {
			return nil, err
		}
		
		// User is not staff, return empty list
		return []*active.Group{}, nil
	}

	// Get active groups linked to educational groups or activity groups the staff is associated with
	// This is a placeholder implementation - the actual logic would depend on how
	// active groups are linked to staff members
	filterOptions := &base.QueryOptions{}
	groups, err := s.activeGroupRepo.List(ctx, filterOptions)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups", Err: err}
	}

	// Filter groups based on staff access - this would be better implemented
	// directly in the repository with a proper query
	var accessibleGroups []*active.Group
	for _, group := range groups {
		// Check if staff has access to this group
		// This is a placeholder - actual implementation would check against group ownership,
		// supervisor status, etc.
		hasAccess := true // Simplified for now
		if hasAccess {
			accessibleGroups = append(accessibleGroups, group)
		}
	}

	return accessibleGroups, nil
}

// GetMySupervisedGroups retrieves active groups supervised by the current user
func (s *userContextService) GetMySupervisedGroups(ctx context.Context) ([]*active.Group, error) {
	// Try to get the current staff
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) {
			return nil, err
		}
		
		// User is not staff, return empty list
		return []*active.Group{}, nil
	}

	// Get active groups where the staff is actively supervising
	supervisorEntries, err := s.supervisorRepo.FindActiveByStaffID(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get my supervised groups", Err: err}
	}

	// Extract group IDs from supervisor entries
	groupIDs := make([]int64, 0, len(supervisorEntries))
	for _, entry := range supervisorEntries {
		if entry.EndDate == nil { // Only include active supervision entries
			groupIDs = append(groupIDs, entry.GroupID)
		}
	}

	// If no groups are supervised, return empty list
	if len(groupIDs) == 0 {
		return []*active.Group{}, nil
	}

	// Get active groups by IDs
	var groups []*active.Group
	for _, id := range groupIDs {
		group, err := s.activeGroupRepo.FindByID(ctx, id)
		if err != nil {
			return nil, &UserContextError{Op: "get my supervised groups", Err: err}
		}
		if group != nil {
			groups = append(groups, group)
		}
	}

	return groups, nil
}

// GetGroupStudents retrieves students in a specific group where the current user has access
func (s *userContextService) GetGroupStudents(ctx context.Context, groupID int64) ([]*users.Student, error) {
	// Verify group exists
	group, err := s.activeGroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: "get group students", Err: err}
	}
	if group == nil {
		return nil, &UserContextError{Op: "get group students", Err: ErrGroupNotFound}
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
		return nil, &UserContextError{Op: "get group students", Err: ErrUserNotAuthorized}
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
	// Verify group exists
	group, err := s.activeGroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: "get group visits", Err: err}
	}
	if group == nil {
		return nil, &UserContextError{Op: "get group visits", Err: ErrGroupNotFound}
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
		return nil, &UserContextError{Op: "get group visits", Err: ErrUserNotAuthorized}
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