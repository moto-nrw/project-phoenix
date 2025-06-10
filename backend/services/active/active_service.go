package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Service implements the Active Service interface
type service struct {
	groupRepo         active.GroupRepository
	visitRepo         active.VisitRepository
	supervisorRepo    active.GroupSupervisorRepository
	combinedGroupRepo active.CombinedGroupRepository
	groupMappingRepo  active.GroupMappingRepository
	
	// Additional repositories for dashboard analytics
	studentRepo        userModels.StudentRepository
	roomRepo          facilityModels.RoomRepository
	activityGroupRepo activitiesModels.GroupRepository
	activityCatRepo   activitiesModels.CategoryRepository
	educationGroupRepo educationModels.GroupRepository
	personRepo        userModels.PersonRepository
	
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
	studentRepo userModels.StudentRepository,
	roomRepo facilityModels.RoomRepository,
	activityGroupRepo activitiesModels.GroupRepository,
	activityCatRepo activitiesModels.CategoryRepository,
	educationGroupRepo educationModels.GroupRepository,
	personRepo userModels.PersonRepository,
	db *bun.DB,
) Service {
	return &service{
		groupRepo:         groupRepo,
		visitRepo:         visitRepo,
		supervisorRepo:    supervisorRepo,
		combinedGroupRepo: combinedGroupRepo,
		groupMappingRepo:  groupMappingRepo,
		studentRepo:        studentRepo,
		roomRepo:          roomRepo,
		activityGroupRepo: activityGroupRepo,
		activityCatRepo:   activityCatRepo,
		educationGroupRepo: educationGroupRepo,
		personRepo:        personRepo,
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
	var studentRepo = s.studentRepo
	var roomRepo = s.roomRepo
	var activityGroupRepo = s.activityGroupRepo
	var activityCatRepo = s.activityCatRepo
	var educationGroupRepo = s.educationGroupRepo
	var personRepo = s.personRepo

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
	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(userModels.StudentRepository)
	}
	if txRepo, ok := s.roomRepo.(base.TransactionalRepository); ok {
		roomRepo = txRepo.WithTx(tx).(facilityModels.RoomRepository)
	}
	if txRepo, ok := s.activityGroupRepo.(base.TransactionalRepository); ok {
		activityGroupRepo = txRepo.WithTx(tx).(activitiesModels.GroupRepository)
	}
	if txRepo, ok := s.activityCatRepo.(base.TransactionalRepository); ok {
		activityCatRepo = txRepo.WithTx(tx).(activitiesModels.CategoryRepository)
	}
	if txRepo, ok := s.educationGroupRepo.(base.TransactionalRepository); ok {
		educationGroupRepo = txRepo.WithTx(tx).(educationModels.GroupRepository)
	}
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(userModels.PersonRepository)
	}

	// Return a new service with the transaction
	return &service{
		groupRepo:         groupRepo,
		visitRepo:         visitRepo,
		supervisorRepo:    supervisorRepo,
		combinedGroupRepo: combinedGroupRepo,
		groupMappingRepo:  groupMappingRepo,
		studentRepo:       studentRepo,
		roomRepo:          roomRepo,
		activityGroupRepo: activityGroupRepo,
		activityCatRepo:   activityCatRepo,
		educationGroupRepo: educationGroupRepo,
		personRepo:        personRepo,
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
	// TODO: Current Implementation vs Original Intent
	//
	// CURRENT IMPLEMENTATION (Dashboard Branch):
	// - Returns current occupancy ratio: active students / room capacity
	// - Example: 15 students in a room with capacity 20 = 0.75 (75%)
	// - This is a real-time snapshot matching dashboard's capacity calculation
	//
	// ORIGINAL INTENT (from deleted comments):
	// - Calculate total hours the room has been used vs available hours
	// - Example: Room used 6 hours out of 8 available hours = 0.75 (75%)
	// - This would be a time-based historical utilization
	//
	// WHAT NEEDS TO BE DONE FOR FULL IMPLEMENTATION:
	// 1. Add time range parameters (start, end time.Time)
	// 2. Query historical active_groups data within time range
	// 3. Calculate total hours room was occupied
	// 4. Calculate total available hours in time range
	// 5. Return ratio of used hours to available hours
	//
	// NOTE: This method is NOT used by the dashboard (uses GetDashboardAnalytics instead)
	// but API routes exist at /api/active/analytics/room/[roomId]/utilization
	// The statistics page would need this for historical room usage analysis

	// Get room to check capacity
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return 0.0, &ActiveError{Op: "GetRoomUtilization", Err: err}
	}
	
	// If room has no capacity, utilization is 0
	if room.Capacity <= 0 {
		return 0.0, nil
	}
	
	// Count active visits in this room (same pattern as dashboard)
	activeGroups, err := s.groupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return 0.0, &ActiveError{Op: "GetRoomUtilization", Err: err}
	}
	
	currentOccupancy := 0
	for _, group := range activeGroups {
		if group.IsActive() {
			visits, err := s.visitRepo.FindByActiveGroupID(ctx, group.ID)
			if err == nil {
				for _, visit := range visits {
					if visit.IsActive() {
						currentOccupancy++
					}
				}
			}
		}
	}
	
	// Return utilization as a ratio between 0.0 and 1.0
	return float64(currentOccupancy) / float64(room.Capacity), nil
}

func (s *service) GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error) {
	// TODO: Current Implementation vs Original Intent
	//
	// CURRENT IMPLEMENTATION (Dashboard Branch):
	// - Returns binary presence: 1.0 if student is currently present, 0.0 if not
	// - This is a simple "is the student here right now?" check
	// - Matches dashboard's real-time presence tracking
	//
	// ORIGINAL INTENT (from deleted comments):
	// - Calculate ratio of attended activities vs scheduled activities
	// - Example: Student attended 4 out of 5 scheduled activities = 0.8 (80%)
	// - This would be a historical attendance rate over a time period
	//
	// WHAT NEEDS TO BE DONE FOR FULL IMPLEMENTATION:
	// 1. Add time range parameters (start, end time.Time)
	// 2. Query student's scheduled activities within time range
	//    - This requires linking to activities.student_enrollments
	//    - And checking activity schedules
	// 3. Query student's actual attendance (visits) for those activities
	// 4. Calculate ratio: attended activities / scheduled activities
	// 5. Handle edge cases (no scheduled activities, partial attendance, etc.)
	//
	// NOTE: This method is NOT used by the dashboard (uses GetDashboardAnalytics instead)
	// but API routes exist at /api/active/analytics/student/[studentId]/attendance
	// Individual student pages or reports would need this for attendance tracking
	
	// Simple implementation matching dashboard's binary presence logic
	// Returns 1.0 if student has active visit, 0.0 if not
	
	visit, err := s.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		// If error, assume student not present
		return 0.0, nil
	}
	
	if visit != nil && visit.IsActive() {
		return 1.0, nil // Student is present
	}
	
	return 0.0, nil // Student is not present
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

	// Create maps to track students and their locations
	studentLocationMap := make(map[int64]string) // studentID -> location
	roomVisitsMap := make(map[int64]int)        // roomID -> visit count
	recentCheckouts := make(map[int64]time.Time) // studentID -> checkout time
	
	studentsPresent := 0
	for _, visit := range activeVisits {
		if visit.IsActive() {
			studentsPresent++
			studentLocationMap[visit.StudentID] = "active"
		} else if visit.ExitTime != nil {
			// Track recent checkouts for transit calculation
			recentCheckouts[visit.StudentID] = *visit.ExitTime
		}
	}
	analytics.StudentsPresent = studentsPresent

	// Get total enrolled students
	allStudents, err := s.studentRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}
	analytics.StudentsEnrolled = len(allStudents)

	// Get all rooms
	allRooms, err := s.roomRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}
	analytics.TotalRooms = len(allRooms)

	// Create room lookup maps
	roomByID := make(map[int64]*facilityModels.Room)
	roomCapacityTotal := 0
	for _, room := range allRooms {
		roomByID[room.ID] = room
		if room.Capacity > 0 {
			roomCapacityTotal += room.Capacity
		}
	}

	// Get active groups with visits preloaded
	activeGroups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Process active groups to calculate various metrics
	activeGroupsCount := 0
	ogsGroupsCount := 0
	occupiedRooms := make(map[int64]bool)
	studentsInRooms := 0
	
	for _, group := range activeGroups {
		if group.IsActive() {
			activeGroupsCount++
			occupiedRooms[group.RoomID] = true
			
			// Count visits for this group
			groupVisits, err := s.visitRepo.FindByActiveGroupID(ctx, group.ID)
			if err == nil {
				for _, visit := range groupVisits {
					if visit.IsActive() {
						roomVisitsMap[group.RoomID]++
						studentsInRooms++
					}
				}
			}
			
			// Since all educational groups are OGS groups, we count all active education group sessions
			eduGroup, err := s.educationGroupRepo.FindByID(ctx, group.GroupID)
			if err == nil && eduGroup != nil {
				// This is an OGS group (educational group)
				ogsGroupsCount++
			}
		}
	}
	analytics.ActiveOGSGroups = ogsGroupsCount
	analytics.ActiveActivities = activeGroupsCount

	// Calculate free rooms
	analytics.FreeRooms = analytics.TotalRooms - len(occupiedRooms)

	// Calculate capacity utilization
	if roomCapacityTotal > 0 {
		analytics.CapacityUtilization = float64(studentsInRooms) / float64(roomCapacityTotal)
	}

	// Get supervisors today
	supervisors, err := s.supervisorRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	supervisorMap := make(map[int64]bool)
	today := time.Now().Truncate(24 * time.Hour)
	for _, supervisor := range supervisors {
		if supervisor.IsActive() || (supervisor.StartDate.After(today) && supervisor.StartDate.Before(time.Now())) {
			supervisorMap[supervisor.StaffID] = true
		}
	}
	analytics.SupervisorsToday = len(supervisorMap)

	// Get activity categories
	activityCategories, err := s.activityCatRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}
	analytics.ActivityCategories = len(activityCategories)

	// Get all educational groups to identify group rooms
	allEducationGroups, err := s.educationGroupRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}
	
	// Create a set of room IDs that belong to educational groups
	educationGroupRooms := make(map[int64]bool)
	for _, eduGroup := range allEducationGroups {
		if eduGroup.RoomID != nil && *eduGroup.RoomID > 0 {
			educationGroupRooms[*eduGroup.RoomID] = true
		}
	}
	
	// Calculate students by location
	studentsOnPlayground := 0
	studentsInTransit := 0
	studentsInGroupRooms := 0
	studentsInHomeRoom := 0
	studentsInWC := 0
	studentsInSchoolYard := 0
	
	// Process room visits to categorize students
	for roomID, visitCount := range roomVisitsMap {
		if room, ok := roomByID[roomID]; ok {
			// Check for playground/school yard by category
			switch room.Category {
			case "Schulhof", "Playground", "school_yard":
				studentsOnPlayground += visitCount
				studentsInSchoolYard += visitCount
			case "WC", "Toilette", "Restroom", "wc":
				// Track students in WC
				studentsInWC += visitCount
			}
			
			// Check if this room belongs to an educational group
			if educationGroupRooms[roomID] {
				studentsInGroupRooms += visitCount
				// For now, consider all students in group rooms as in their home room
				studentsInHomeRoom = studentsInGroupRooms
			}
		}
	}
	
	// Calculate students in transit: students with in_house=true but not in any room/WC/schoolyard
	// First, get all students who are in_house (in OGS)
	studentsInOGS := 0
	ogsStudentIDs := make(map[int64]bool)
	for _, student := range allStudents {
		if student.InHouse {
			studentsInOGS++
			ogsStudentIDs[student.ID] = true
		}
	}
	
	// Now check which OGS students are NOT in any location
	studentsInTransit = 0
	for studentID := range ogsStudentIDs {
		// Check if this OGS student has an active visit (is in a room)
		if _, hasActiveVisit := studentLocationMap[studentID]; !hasActiveVisit {
			// This OGS student is not in any room/location
			studentsInTransit++
		}
	}
	
	analytics.StudentsOnPlayground = studentsOnPlayground
	analytics.StudentsInTransit = studentsInTransit
	analytics.StudentsInGroupRooms = studentsInGroupRooms
	analytics.StudentsInHomeRoom = studentsInHomeRoom

	// Build recent activity (privacy-compliant - no individual student data)
	recentActivity := []RecentActivity{}
	
	// Sort active groups by start time (most recent first)
	for i, group := range activeGroups {
		if i >= 3 { // Limit to 3 recent activities
			break
		}

		if time.Since(group.StartTime) < 30*time.Minute && group.IsActive() {
			// Get actual group name - first try activity group, then education group
			groupName := fmt.Sprintf("Gruppe %d", group.GroupID)
			
			// Try to find in activity groups first
			if actGroup, err := s.activityGroupRepo.FindByID(ctx, group.GroupID); err == nil && actGroup != nil {
				groupName = actGroup.Name
			} else if eduGroup, err := s.educationGroupRepo.FindByID(ctx, group.GroupID); err == nil && eduGroup != nil {
				// Fall back to education group
				groupName = eduGroup.Name
			}
			
			// Get actual room name
			roomName := fmt.Sprintf("Raum %d", group.RoomID)
			if room, ok := roomByID[group.RoomID]; ok {
				roomName = room.Name
			}
			
			// Count active visits for this group
			visitCount := roomVisitsMap[group.RoomID]
			
			activity := RecentActivity{
				Type:      "group_start",
				GroupName: groupName,
				RoomName:  roomName,
				Count:     visitCount,
				Timestamp: group.StartTime,
			}
			recentActivity = append(recentActivity, activity)
		}
	}
	analytics.RecentActivity = recentActivity

	// Build current activities
	currentActivities := []CurrentActivity{}
	
	// Get active activity groups
	activityGroups, err := s.activityGroupRepo.List(ctx, nil)
	if err == nil {
		for i, actGroup := range activityGroups {
			if i >= 2 { // Limit to 2 current activities
				break
			}
			
			// Check if this activity has an active session
			hasActiveSession := false
			participantCount := 0
			
			for _, group := range activeGroups {
				if group.IsActive() && group.GroupID == actGroup.ID {
					hasActiveSession = true
					participantCount = roomVisitsMap[group.RoomID]
					break
				}
			}
			
			if hasActiveSession {
				categoryName := "Sonstiges"
				if actGroup.Category != nil {
					categoryName = actGroup.Category.Name
				}
				
				status := "active"
				if participantCount >= actGroup.MaxParticipants {
					status = "full"
				} else if participantCount > int(float64(actGroup.MaxParticipants)*0.8) {
					status = "ending_soon"
				}
				
				activity := CurrentActivity{
					Name:         actGroup.Name,
					Category:     categoryName,
					Participants: participantCount,
					MaxCapacity:  actGroup.MaxParticipants,
					Status:       status,
				}
				currentActivities = append(currentActivities, activity)
			}
		}
	}
	analytics.CurrentActivities = currentActivities

	// Build active groups summary
	activeGroupsSummary := []ActiveGroupInfo{}
	for i, group := range activeGroups {
		if i >= 2 || !group.IsActive() { // Limit to 2 groups
			break
		}

		// Get group details
		groupName := fmt.Sprintf("Gruppe %d", group.GroupID)
		groupType := "activity"
		
		if eduGroup, err := s.educationGroupRepo.FindByID(ctx, group.GroupID); err == nil && eduGroup != nil {
			groupName = eduGroup.Name
			// All educational groups are OGS groups
			groupType = "ogs_group"
		}
		
		// Get room name
		location := fmt.Sprintf("Raum %d", group.RoomID)
		if room, ok := roomByID[group.RoomID]; ok {
			location = room.Name
		}
		
		groupInfo := ActiveGroupInfo{
			Name:         groupName,
			Type:         groupType,
			StudentCount: roomVisitsMap[group.RoomID],
			Location:     location,
			Status:       "active",
		}

		activeGroupsSummary = append(activeGroupsSummary, groupInfo)
	}
	analytics.ActiveGroupsSummary = activeGroupsSummary

	return analytics, nil
}
