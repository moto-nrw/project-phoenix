package active

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Service implements the Active Service interface
type service struct {
	groupRepo         active.GroupRepository
	visitRepo         active.VisitRepository
	supervisorRepo    active.GroupSupervisorRepository
	combinedGroupRepo active.CombinedGroupRepository
	groupMappingRepo  active.GroupMappingRepository
	db                *bun.DB
	txHandler         *base.TxHandler
}

// NewService creates a new active service instance
func NewService(
	groupRepo active.GroupRepository,
	visitRepo active.VisitRepository,
	supervisorRepo active.GroupSupervisorRepository,
	combinedGroupRepo active.CombinedGroupRepository,
	groupMappingRepo active.GroupMappingRepository,
	db *bun.DB,
) Service {
	return &service{
		groupRepo:         groupRepo,
		visitRepo:         visitRepo,
		supervisorRepo:    supervisorRepo,
		combinedGroupRepo: combinedGroupRepo,
		groupMappingRepo:  groupMappingRepo,
		db:                db,
		txHandler:         base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var groupRepo = s.groupRepo
	var visitRepo = s.visitRepo
	var supervisorRepo = s.supervisorRepo
	var combinedGroupRepo = s.combinedGroupRepo
	var groupMappingRepo = s.groupMappingRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.groupRepo.(base.TransactionalRepository); ok {
		groupRepo = txRepo.WithTx(tx).(active.GroupRepository)
	}
	if txRepo, ok := s.visitRepo.(base.TransactionalRepository); ok {
		visitRepo = txRepo.WithTx(tx).(active.VisitRepository)
	}
	if txRepo, ok := s.supervisorRepo.(base.TransactionalRepository); ok {
		supervisorRepo = txRepo.WithTx(tx).(active.GroupSupervisorRepository)
	}
	if txRepo, ok := s.combinedGroupRepo.(base.TransactionalRepository); ok {
		combinedGroupRepo = txRepo.WithTx(tx).(active.CombinedGroupRepository)
	}
	if txRepo, ok := s.groupMappingRepo.(base.TransactionalRepository); ok {
		groupMappingRepo = txRepo.WithTx(tx).(active.GroupMappingRepository)
	}

	// Return a new service with the transaction
	return &service{
		groupRepo:         groupRepo,
		visitRepo:         visitRepo,
		supervisorRepo:    supervisorRepo,
		combinedGroupRepo: combinedGroupRepo,
		groupMappingRepo:  groupMappingRepo,
		db:                s.db,
		txHandler:         s.txHandler.WithTx(tx),
	}
}

// Active Group operations
func (s *service) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
	}
	return group, nil
}

func (s *service) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	if err := group.Validate(); err != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: ErrInvalidData}
	}

	if err := s.groupRepo.Create(ctx, group); err != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateActiveGroup(ctx context.Context, group *active.Group) error {
	if err := group.Validate(); err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrInvalidData}
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteActiveGroup(ctx context.Context, id int64) error {
	// Check if there are any active visits for this group
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: ErrDatabaseOperation}
	}

	// Check if any of the visits are still active
	for _, visit := range visits {
		if visit.IsActive() {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrCannotDeleteActiveGroup}
		}
	}

	// Delete the active group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: ErrActiveGroupNotFound}
	}

	if err := s.groupRepo.Delete(ctx, group); err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	groups, err := s.groupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListActiveGroups", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	groups, err := s.groupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByRoomID", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	groups, err := s.groupRepo.FindActiveByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByGroupID", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindActiveGroupsByTimeRange", Err: ErrInvalidTimeRange}
	}

	groups, err := s.groupRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByTimeRange", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) EndActiveGroupSession(ctx context.Context, id int64) error {
	if err := s.groupRepo.EndSession(ctx, id); err != nil {
		return &ActiveError{Op: "EndActiveGroupSession", Err: ErrDatabaseOperation}
	}
	return nil
}

func (s *service) GetActiveGroupWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrActiveGroupNotFound}
	}

	// Get visits for this group
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrDatabaseOperation}
	}

	group.Visits = visits
	return group, nil
}

func (s *service) GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrActiveGroupNotFound}
	}

	// Get supervisors for this group
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrDatabaseOperation}
	}

	group.Supervisors = supervisors
	return group, nil
}

// Visit operations
func (s *service) GetVisit(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetVisit", Err: ErrVisitNotFound}
	}
	return visit, nil
}

func (s *service) CreateVisit(ctx context.Context, visit *active.Visit) error {
	if err := visit.Validate(); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}

	// Check if student already has an active visit
	visits, err := s.visitRepo.FindActiveByStudentID(ctx, visit.StudentID)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}

	if len(visits) > 0 {
		return &ActiveError{Op: "CreateVisit", Err: ErrStudentAlreadyActive}
	}

	if err := s.visitRepo.Create(ctx, visit); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateVisit(ctx context.Context, visit *active.Visit) error {
	if err := visit.Validate(); err != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrInvalidData}
	}

	if err := s.visitRepo.Update(ctx, visit); err != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteVisit(ctx context.Context, id int64) error {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrVisitNotFound}
	}

	if err := s.visitRepo.Delete(ctx, visit); err != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListVisits(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	visits, err := s.visitRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListVisits", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	visits, err := s.visitRepo.FindActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByStudentID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) FindVisitsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindVisitsByTimeRange", Err: ErrInvalidTimeRange}
	}

	visits, err := s.visitRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindVisitsByTimeRange", Err: ErrDatabaseOperation}
	}
	return visits, nil
}

func (s *service) EndVisit(ctx context.Context, id int64) error {
	if err := s.visitRepo.EndVisit(ctx, id); err != nil {
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}
	return nil
}

func (s *service) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error) {
	visits, err := s.visitRepo.FindActiveByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrDatabaseOperation}
	}

	if len(visits) == 0 {
		return nil, &ActiveError{Op: "GetStudentCurrentVisit", Err: ErrVisitNotFound}
	}

	// Return the first active visit (there should only be one)
	return visits[0], nil
}

// Group Supervisor operations
func (s *service) GetGroupSupervisor(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetGroupSupervisor", Err: ErrGroupSupervisorNotFound}
	}
	return supervisor, nil
}

func (s *service) CreateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	if err := supervisor.Validate(); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrInvalidData}
	}

	// Check if staff is already supervising this group
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, supervisor.GroupID)
	if err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	for _, s := range supervisors {
		if s.StaffID == supervisor.StaffID && s.IsActive() {
			return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrStaffAlreadySupervising}
		}
	}

	if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	if err := supervisor.Validate(); err != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrInvalidData}
	}

	if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	supervisor, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrGroupSupervisorNotFound}
	}

	if err := s.supervisorRepo.Delete(ctx, supervisor); err != nil {
		return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListGroupSupervisors(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListGroupSupervisors", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByStaffID", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) EndSupervision(ctx context.Context, id int64) error {
	if err := s.supervisorRepo.EndSupervision(ctx, id); err != nil {
		return &ActiveError{Op: "EndSupervision", Err: ErrDatabaseOperation}
	}
	return nil
}

func (s *service) GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindActiveByStaffID(ctx, staffID)
	if err != nil {
		return nil, &ActiveError{Op: "GetStaffActiveSupervisions", Err: ErrDatabaseOperation}
	}

	// Filter only active supervisions
	var activeSupervisions []*active.GroupSupervisor
	for _, supervisor := range supervisors {
		if supervisor.IsActive() {
			activeSupervisions = append(activeSupervisions, supervisor)
		}
	}

	return activeSupervisions, nil
}

// Combined Group operations
func (s *service) GetCombinedGroup(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	group, err := s.combinedGroupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetCombinedGroup", Err: ErrCombinedGroupNotFound}
	}
	return group, nil
}

func (s *service) CreateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	if err := group.Validate(); err != nil {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrInvalidData}
	}

	if err := s.combinedGroupRepo.Create(ctx, group); err != nil {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	if err := group.Validate(); err != nil {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrInvalidData}
	}

	if err := s.combinedGroupRepo.Update(ctx, group); err != nil {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteCombinedGroup(ctx context.Context, id int64) error {
	group, err := s.combinedGroupRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrCombinedGroupNotFound}
	}

	// Execute in transaction to ensure all mappings are deleted as well
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Delete all group mappings
		mappings, err := s.groupMappingRepo.FindByActiveCombinedGroupID(ctx, id)
		if err != nil {
			return err
		}

		for _, mapping := range mappings {
			if err := s.groupMappingRepo.Delete(ctx, mapping); err != nil {
				return err
			}
		}

		// Delete the combined group
		return s.combinedGroupRepo.Delete(ctx, group)
	})

	if err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListCombinedGroups(ctx context.Context, options *base.QueryOptions) ([]*active.CombinedGroup, error) {
	groups, err := s.combinedGroupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListCombinedGroups", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindActiveCombinedGroups(ctx context.Context) ([]*active.CombinedGroup, error) {
	groups, err := s.combinedGroupRepo.FindActive(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveCombinedGroups", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindCombinedGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.CombinedGroup, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindCombinedGroupsByTimeRange", Err: ErrInvalidTimeRange}
	}

	groups, err := s.combinedGroupRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindCombinedGroupsByTimeRange", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) EndCombinedGroup(ctx context.Context, id int64) error {
	if err := s.combinedGroupRepo.EndCombination(ctx, id); err != nil {
		return &ActiveError{Op: "EndCombinedGroup", Err: ErrDatabaseOperation}
	}
	return nil
}

func (s *service) GetCombinedGroupWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	combinedGroup, err := s.combinedGroupRepo.FindWithGroups(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetCombinedGroupWithGroups", Err: ErrCombinedGroupNotFound}
	}
	return combinedGroup, nil
}

// Group Mapping operations
func (s *service) AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	// Check if the mapping already exists
	mappings, err := s.groupMappingRepo.FindByActiveCombinedGroupID(ctx, combinedGroupID)
	if err != nil {
		return &ActiveError{Op: "AddGroupToCombination", Err: ErrDatabaseOperation}
	}

	for _, mapping := range mappings {
		if mapping.ActiveGroupID == activeGroupID {
			return &ActiveError{Op: "AddGroupToCombination", Err: ErrGroupAlreadyInCombination}
		}
	}

	// Create the mapping
	if err := s.groupMappingRepo.AddGroupToCombination(ctx, combinedGroupID, activeGroupID); err != nil {
		return &ActiveError{Op: "AddGroupToCombination", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	if err := s.groupMappingRepo.RemoveGroupFromCombination(ctx, combinedGroupID, activeGroupID); err != nil {
		return &ActiveError{Op: "RemoveGroupFromCombination", Err: ErrDatabaseOperation}
	}
	return nil
}

func (s *service) GetGroupMappingsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error) {
	mappings, err := s.groupMappingRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "GetGroupMappingsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return mappings, nil
}

func (s *service) GetGroupMappingsByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error) {
	mappings, err := s.groupMappingRepo.FindByActiveCombinedGroupID(ctx, combinedGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "GetGroupMappingsByCombinedGroupID", Err: ErrDatabaseOperation}
	}
	return mappings, nil
}

// Analytics and statistics
func (s *service) GetActiveGroupsCount(ctx context.Context) (int, error) {
	// Implementation would count active groups without end time
	// This is a simplified implementation
	groups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		return 0, &ActiveError{Op: "GetActiveGroupsCount", Err: ErrDatabaseOperation}
	}

	count := 0
	for _, group := range groups {
		if group.IsActive() {
			count++
		}
	}

	return count, nil
}

func (s *service) GetTotalVisitsCount(ctx context.Context) (int, error) {
	visits, err := s.visitRepo.List(ctx, nil)
	if err != nil {
		return 0, &ActiveError{Op: "GetTotalVisitsCount", Err: ErrDatabaseOperation}
	}
	return len(visits), nil
}

func (s *service) GetActiveVisitsCount(ctx context.Context) (int, error) {
	visits, err := s.visitRepo.List(ctx, nil)
	if err != nil {
		return 0, &ActiveError{Op: "GetActiveVisitsCount", Err: ErrDatabaseOperation}
	}

	count := 0
	for _, visit := range visits {
		if visit.IsActive() {
			count++
		}
	}

	return count, nil
}

func (s *service) GetRoomUtilization(ctx context.Context, roomID int64) (float64, error) {
	// Room utilization is a complex calculation that would require looking at the history
	// of active groups in a room over a time period compared to the room's available hours
	// This is a simplified placeholder implementation

	// For a real implementation, you would:
	// 1. Determine the total available hours for the room
	// 2. Calculate the total hours the room has been used (active groups)
	// 3. Return the ratio of used hours to available hours

	return 0.0, &ActiveError{Op: "GetRoomUtilization", Err: errors.New("not implemented")}
}

func (s *service) GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error) {
	// Student attendance rate would require looking at the student's schedule vs. their actual attendance
	// This is a simplified placeholder implementation

	// For a real implementation, you would:
	// 1. Determine the total scheduled activities for the student
	// 2. Calculate how many of those activities the student attended
	// 3. Return the ratio of attended to scheduled activities

	return 0.0, &ActiveError{Op: "GetStudentAttendanceRate", Err: errors.New("not implemented")}
}

func (s *service) GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error) {
	analytics := &DashboardAnalytics{
		LastUpdated: time.Now(),
	}

	// Get active visits count (students present)
	activeVisits, err := s.visitRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	studentsPresent := 0
	for _, visit := range activeVisits {
		if visit.IsActive() {
			studentsPresent++
		}
	}
	analytics.StudentsPresent = studentsPresent

	// Get active groups
	activeGroups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	activeGroupsCount := 0
	ogsGroupsCount := 0
	for _, group := range activeGroups {
		if group.IsActive() {
			activeGroupsCount++
			// Check if it's an OGS group based on some criteria
			// This is simplified - in real implementation, you'd check group type
			if group.GroupID <= 10 { // Assuming first 10 groups are OGS groups
				ogsGroupsCount++
			}
		}
	}
	analytics.ActiveOGSGroups = ogsGroupsCount

	// Get supervisors today
	supervisors, err := s.supervisorRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	supervisorsToday := 0
	supervisorMap := make(map[int64]bool)
	for _, supervisor := range supervisors {
		if supervisor.IsActive() {
			supervisorMap[supervisor.StaffID] = true
		}
	}
	supervisorsToday = len(supervisorMap)
	analytics.SupervisorsToday = supervisorsToday

	// Placeholder values for other metrics
	// In a real implementation, these would be calculated from actual data
	analytics.StudentsEnrolled = 150    // Would come from student repository
	analytics.StudentsOnPlayground = 32 // Would be calculated based on room types
	analytics.StudentsInTransit = 8     // Would be calculated from recent check-ins/outs
	analytics.ActiveActivities = activeGroupsCount
	analytics.FreeRooms = 8              // Would be calculated from total rooms - occupied rooms
	analytics.TotalRooms = 24            // Would come from room repository
	analytics.CapacityUtilization = 0.73 // Would be calculated from room capacities
	analytics.ActivityCategories = 6     // Would come from activity repository
	analytics.StudentsInGroupRooms = 35  // Would be calculated from visits in group rooms
	analytics.StudentsInHomeRoom = 19    // Would be calculated from students in their assigned rooms

	// Recent activity (privacy-compliant - no individual student data)
	recentActivity := []RecentActivity{}

	// Get recent group starts/ends
	for i, group := range activeGroups {
		if i >= 3 { // Limit to 3 recent activities
			break
		}

		if time.Since(group.StartTime) < 30*time.Minute {
			activity := RecentActivity{
				Type:      "group_start",
				GroupName: fmt.Sprintf("Group %d", group.GroupID), // Placeholder - would get actual group name
				RoomName:  fmt.Sprintf("Room %d", group.RoomID),   // Placeholder - would get actual room name
				Count:     len(group.Visits),
				Timestamp: group.StartTime,
			}
			recentActivity = append(recentActivity, activity)
		}
	}
	analytics.RecentActivity = recentActivity

	// Current activities
	currentActivities := []CurrentActivity{}
	for i, group := range activeGroups {
		if i >= 2 || !group.IsActive() { // Limit to 2 current activities
			break
		}

		activity := CurrentActivity{
			Name:         fmt.Sprintf("Activity %d", group.GroupID), // Placeholder
			Category:     "Sport",                                   // Placeholder - would get from activity data
			Participants: len(group.Visits),
			MaxCapacity:  15, // Placeholder - would get from activity data
			Status:       "active",
		}

		if activity.Participants >= activity.MaxCapacity {
			activity.Status = "full"
		}

		currentActivities = append(currentActivities, activity)
	}
	analytics.CurrentActivities = currentActivities

	// Active groups summary
	activeGroupsSummary := []ActiveGroupInfo{}
	for i, group := range activeGroups {
		if i >= 2 || !group.IsActive() { // Limit to 2 groups
			break
		}

		groupInfo := ActiveGroupInfo{
			Name:         fmt.Sprintf("Group %d", group.GroupID), // Placeholder
			Type:         "ogs_group",
			StudentCount: len(group.Visits),
			Location:     fmt.Sprintf("Room %d", group.RoomID), // Placeholder
			Status:       "active",
		}

		activeGroupsSummary = append(activeGroupsSummary, groupInfo)
	}
	analytics.ActiveGroupsSummary = activeGroupsSummary

	return analytics, nil
}
