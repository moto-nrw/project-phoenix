package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Broadcaster interface (re-exported from realtime for convenience)
type Broadcaster = realtime.Broadcaster

const (
	// sseErrorMessage is the standard error message for SSE broadcast failures
	sseErrorMessage = "SSE broadcast failed"
	// supervisorAssignmentWarning is the format string for supervisor assignment failures
	supervisorAssignmentWarning = "Warning: Failed to assign supervisor %d to session %d: %v\n"
	// visitTransferMessage is the format string for visit transfer logging
	visitTransferMessage = "Transferred %d active visits to new session %d\n"
)

// RoomConflictStrategy defines how to handle room conflicts when determining room ID
type RoomConflictStrategy int

const (
	// RoomConflictFail returns error if room has conflicts
	RoomConflictFail RoomConflictStrategy = iota
	// RoomConflictIgnore skips conflict checking entirely
	RoomConflictIgnore
	// RoomConflictWarn logs warning but continues
	RoomConflictWarn
)

// ServiceDependencies contains all dependencies required by the active service
type ServiceDependencies struct {
	// Active domain repositories
	GroupRepo         active.GroupRepository
	VisitRepo         active.VisitRepository
	SupervisorRepo    active.GroupSupervisorRepository
	CombinedGroupRepo active.CombinedGroupRepository
	GroupMappingRepo  active.GroupMappingRepository
	AttendanceRepo    active.AttendanceRepository

	// User domain repositories
	StudentRepo userModels.StudentRepository
	PersonRepo  userModels.PersonRepository
	TeacherRepo userModels.TeacherRepository
	StaffRepo   userModels.StaffRepository

	// Supporting domain repositories
	RoomRepo           facilityModels.RoomRepository
	ActivityGroupRepo  activitiesModels.GroupRepository
	ActivityCatRepo    activitiesModels.CategoryRepository
	EducationGroupRepo educationModels.GroupRepository

	// External services
	EducationService education.Service
	UsersService     users.PersonService

	// Infrastructure
	DB          *bun.DB
	Broadcaster Broadcaster // SSE event broadcaster (optional - can be nil for testing)
}

// Service implements the Active Service interface
type service struct {
	groupRepo         active.GroupRepository
	visitRepo         active.VisitRepository
	supervisorRepo    active.GroupSupervisorRepository
	combinedGroupRepo active.CombinedGroupRepository
	groupMappingRepo  active.GroupMappingRepository

	// Additional repositories for dashboard analytics
	studentRepo        userModels.StudentRepository
	roomRepo           facilityModels.RoomRepository
	activityGroupRepo  activitiesModels.GroupRepository
	activityCatRepo    activitiesModels.CategoryRepository
	educationGroupRepo educationModels.GroupRepository
	personRepo         userModels.PersonRepository

	// New dependencies for attendance tracking
	attendanceRepo   active.AttendanceRepository
	educationService education.Service
	usersService     users.PersonService
	teacherRepo      userModels.TeacherRepository
	staffRepo        userModels.StaffRepository

	db        *bun.DB
	txHandler *base.TxHandler

	// SSE real-time event broadcasting (optional - can be nil for testing)
	broadcaster Broadcaster
}

// NewService creates a new active service instance
func NewService(deps ServiceDependencies) Service {
	return &service{
		groupRepo:          deps.GroupRepo,
		visitRepo:          deps.VisitRepo,
		supervisorRepo:     deps.SupervisorRepo,
		combinedGroupRepo:  deps.CombinedGroupRepo,
		groupMappingRepo:   deps.GroupMappingRepo,
		studentRepo:        deps.StudentRepo,
		roomRepo:           deps.RoomRepo,
		activityGroupRepo:  deps.ActivityGroupRepo,
		activityCatRepo:    deps.ActivityCatRepo,
		educationGroupRepo: deps.EducationGroupRepo,
		personRepo:         deps.PersonRepo,
		attendanceRepo:     deps.AttendanceRepo,
		educationService:   deps.EducationService,
		usersService:       deps.UsersService,
		teacherRepo:        deps.TeacherRepo,
		staffRepo:          deps.StaffRepo,
		db:                 deps.DB,
		txHandler:          base.NewTxHandler(deps.DB),
		broadcaster:        deps.Broadcaster,
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
	var attendanceRepo = s.attendanceRepo
	var teacherRepo = s.teacherRepo
	var staffRepo = s.staffRepo

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
	if txRepo, ok := s.attendanceRepo.(base.TransactionalRepository); ok {
		attendanceRepo = txRepo.WithTx(tx).(active.AttendanceRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(userModels.TeacherRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(userModels.StaffRepository)
	}

	// Return a new service with the transaction
	return &service{
		groupRepo:          groupRepo,
		visitRepo:          visitRepo,
		supervisorRepo:     supervisorRepo,
		combinedGroupRepo:  combinedGroupRepo,
		groupMappingRepo:   groupMappingRepo,
		studentRepo:        studentRepo,
		roomRepo:           roomRepo,
		activityGroupRepo:  activityGroupRepo,
		activityCatRepo:    activityCatRepo,
		educationGroupRepo: educationGroupRepo,
		personRepo:         personRepo,
		attendanceRepo:     attendanceRepo,
		educationService:   s.educationService,
		usersService:       s.usersService,
		teacherRepo:        teacherRepo,
		staffRepo:          staffRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
		broadcaster:        s.broadcaster, // Propagate broadcaster to transactional clone
	}
}

// Active Group operations
func (s *service) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
	}

	// Ensure we always have room metadata so downstream callers
	// (location resolver, SSE payloads) can render friendly labels.
	if group != nil && group.Room == nil && group.RoomID > 0 {
		if room, roomErr := s.roomRepo.FindByID(ctx, group.RoomID); roomErr == nil {
			group.Room = room
		}
	}

	return group, nil
}

func (s *service) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error) {
	if len(groupIDs) == 0 {
		return map[int64]*active.Group{}, nil
	}

	groups, err := s.groupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupsByIDs", Err: ErrDatabaseOperation}
	}

	if groups == nil {
		groups = make(map[int64]*active.Group)
	}

	return groups, nil
}

func (s *service) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: ErrInvalidData}
	}

	// Check for room conflicts if room is assigned
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, group.RoomID, 0)
		if err != nil {
			return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrRoomConflict}
		}
	}

	if err := s.groupRepo.Create(ctx, group); err != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("create failed: %w", err)}
	}

	return nil
}

func (s *service) UpdateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrInvalidData}
	}

	// Check for room conflicts if room is assigned (exclude current group)
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, group.RoomID, group.ID)
		if err != nil {
			return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrRoomConflict}
		}
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("update failed: %w", err)}
	}

	return nil
}

func (s *service) DeleteActiveGroup(ctx context.Context, id int64) error {
	// Check if there are any active visits for this group
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find visits: %w", err)}
	}

	// Check if any of the visits are still active
	for _, visit := range visits {
		if visit.IsActive() {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrCannotDeleteActiveGroup}
		}
	}

	// Delete the active group
	_, err = s.groupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find group: %w", err)}
	}

	if err := s.groupRepo.Delete(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("delete failed: %w", err)}
	}

	return nil
}

func (s *service) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	groups, err := s.groupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	groups, err := s.groupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
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
	// Delegate to EndActivitySession which properly ends visits and broadcasts SSE
	if err := s.EndActivitySession(ctx, id); err != nil {
		// Wrap the error with our operation name for clarity
		if activeErr, ok := err.(*ActiveError); ok {
			return &ActiveError{Op: "EndActiveGroupSession", Err: activeErr.Err}
		}
		return &ActiveError{Op: "EndActiveGroupSession", Err: err}
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

	// Get supervisors for this group (only active ones)
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, id, true)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrDatabaseOperation}
	}

	group.Supervisors = supervisors
	return group, nil
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
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrInvalidData}
	}

	if s.combinedGroupRepo.Create(ctx, group) != nil {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	if group == nil || group.ID == 0 || group.Validate() != nil {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrInvalidData}
	}

	if s.combinedGroupRepo.Update(ctx, group) != nil {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteCombinedGroup(ctx context.Context, id int64) error {
	_, err := s.combinedGroupRepo.FindByID(ctx, id)
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
			if err := s.groupMappingRepo.Delete(ctx, mapping.ID); err != nil {
				return err
			}
		}

		// Delete the combined group
		return s.combinedGroupRepo.Delete(ctx, id)
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
	// Verify group exists first
	_, err := s.combinedGroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "EndCombinedGroup", Err: ErrCombinedGroupNotFound}
		}
		return &ActiveError{Op: "EndCombinedGroup", Err: fmt.Errorf("failed to verify combined group: %w", err)}
	}

	if err := s.combinedGroupRepo.EndCombination(ctx, id); err != nil {
		return &ActiveError{Op: "EndCombinedGroup", Err: fmt.Errorf("end combination failed: %w", err)}
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
	if s.groupMappingRepo.AddGroupToCombination(ctx, combinedGroupID, activeGroupID) != nil {
		return &ActiveError{Op: "AddGroupToCombination", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	if s.groupMappingRepo.RemoveGroupFromCombination(ctx, combinedGroupID, activeGroupID) != nil {
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

// GetRoomUtilization returns the current occupancy ratio for a room.
//
// Deprecated: This method is not used by any frontend UI components and provides
// limited value in its current form. The dashboard uses GetDashboardAnalytics instead.
// Consider using facilities.Service.GetRoomUtilization or removing this endpoint entirely.
//
// Current behavior:
// - Returns real-time occupancy ratio: (active students) / (room capacity)
// - Example: 15 students in a 20-capacity room = 0.75 (75%)
// - Does NOT calculate historical time-based utilization
//
// API endpoint: GET /api/active/analytics/room/{roomId}/utilization
// Exposed but unused by frontend. May be removed in a future version.
func (s *service) GetRoomUtilization(ctx context.Context, roomID int64) (float64, error) {
	capacity, err := s.getRoomCapacityOrZero(ctx, roomID)
	if err != nil {
		return 0.0, err
	}
	if capacity == 0 {
		return 0.0, nil
	}

	activeGroups, err := s.groupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return 0.0, &ActiveError{Op: "GetRoomUtilization", Err: err}
	}

	currentOccupancy := s.countActiveOccupancyInRoom(ctx, activeGroups)
	return float64(currentOccupancy) / float64(capacity), nil
}

// getRoomCapacityOrZero retrieves room capacity, returning 0 if room not found or has no capacity
func (s *service) getRoomCapacityOrZero(ctx context.Context, roomID int64) (int, error) {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return 0, &ActiveError{Op: "GetRoomUtilization", Err: err}
	}

	if room.Capacity == nil || *room.Capacity <= 0 {
		return 0, nil
	}

	return *room.Capacity, nil
}

// countActiveOccupancyInRoom counts the number of active visits across all active groups
func (s *service) countActiveOccupancyInRoom(ctx context.Context, activeGroups []*active.Group) int {
	currentOccupancy := 0
	for _, group := range activeGroups {
		if !group.IsActive() {
			continue
		}

		visits, err := s.visitRepo.FindByActiveGroupID(ctx, group.ID)
		if err != nil {
			continue
		}

		for _, visit := range visits {
			if visit.IsActive() {
				currentOccupancy++
			}
		}
	}
	return currentOccupancy
}

// GetStudentAttendanceRate returns a binary presence indicator for a student.
//
// Deprecated: This method is not used by any frontend UI components and provides
// misleading semantics. Despite the name "AttendanceRate", it only returns binary
// presence (1.0 if present, 0.0 if not), not a historical attendance rate.
// The dashboard uses GetDashboardAnalytics or GetStudentAttendanceStatus instead.
//
// Current behavior:
// - Returns 1.0 if student currently has an active visit (present)
// - Returns 0.0 if student has no active visit (not present)
// - Does NOT calculate historical attendance rates or activity participation
//
// API endpoint: GET /api/active/analytics/student/{studentId}/attendance
// Exposed but unused by frontend. May be removed in a future version.
// For actual attendance tracking, use GetStudentAttendanceStatus instead.
func (s *service) GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error) {
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

	// Use local date for analytics (school operates in local timezone)
	// This must match the query in GetStudentCurrentStatus which also uses local date
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Phase 1: Fetch all base data
	baseData, err := s.fetchDashboardBaseData(ctx, today)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 2: Calculate presence metrics
	analytics.StudentsPresent = len(baseData.studentsPresent)
	analytics.StudentsInTransit = calculateStudentsInTransit(baseData.studentsWithAttendance, baseData.studentsWithActiveVisits)
	analytics.TotalRooms = len(baseData.allRooms)
	analytics.ActivityCategories = baseData.activityCategories
	analytics.SupervisorsToday = baseData.supervisorsToday

	// Phase 3: Build room lookup maps
	roomData := s.buildRoomLookupMaps(baseData.allRooms)

	// Phase 4: Load students with groups for home room calculation
	studentIDs := extractUniqueStudentIDs(baseData.activeVisits)
	studentsWithGroups, err := s.loadStudentsWithGroups(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 5: Build group-related maps
	groupData, err := s.buildEducationGroupMaps(ctx, baseData.activeGroups, baseData.allEducationGroups, studentsWithGroups)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 6: Process active groups and calculate group metrics
	activeGroupsCount, ogsGroupsCount, uniqueStudentsInRoomsOverall := s.processActiveGroups(
		baseData.activeGroups, baseData.visitsByGroupID, groupData, roomData,
	)
	analytics.ActiveActivities = activeGroupsCount
	analytics.ActiveOGSGroups = ogsGroupsCount
	analytics.FreeRooms = analytics.TotalRooms - len(roomData.occupiedRooms)

	// Phase 7: Calculate capacity utilization
	if roomData.roomCapacityTotal > 0 {
		analytics.CapacityUtilization = float64(len(uniqueStudentsInRoomsOverall)) / float64(roomData.roomCapacityTotal)
	}

	// Phase 8: Calculate location-based metrics
	locationData := s.calculateLocationMetrics(roomData, groupData, baseData.activeVisits, baseData.activeGroups)
	analytics.StudentsOnPlayground = locationData.studentsOnPlayground
	analytics.StudentsInRooms = locationData.studentsInIndoorRooms
	analytics.StudentsInGroupRooms = locationData.studentsInGroupRooms
	analytics.StudentsInHomeRoom = locationData.studentsInHomeRoom

	// Phase 9: Build summary lists
	analytics.RecentActivity = s.buildRecentActivity(ctx, baseData.activeGroups, roomData)
	analytics.CurrentActivities = s.buildCurrentActivities(ctx, baseData.activeGroups, roomData)
	analytics.ActiveGroupsSummary = s.buildActiveGroupsSummary(ctx, baseData.activeGroups, roomData)

	return analytics, nil
}

// Activity Session Management with Conflict Detection

// StartActivitySession starts a new activity session on a device with conflict detection
func (s *service) StartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	var newGroup *active.Group
	err := s.executeSessionStart(ctx, activityID, deviceID, roomID, "StartActivitySession", func(ctx context.Context, finalRoomID int64) (*active.Group, error) {
		group, err := s.createSessionWithSupervisor(ctx, activityID, deviceID, staffID, finalRoomID)
		newGroup = group
		return group, err
	})

	if err != nil {
		return nil, err
	}

	s.broadcastActivityStartEvent(ctx, newGroup, []int64{staffID})
	return newGroup, nil
}

// determineSessionRoomID determines the room for a session with conflict checking
func (s *service) determineSessionRoomID(ctx context.Context, activityID int64, roomID *int64) (int64, error) {
	return s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictFail)
}

// createSessionWithSupervisor creates a new session, assigns supervisor, and transfers visits
func (s *service) createSessionWithSupervisor(ctx context.Context, activityID, deviceID, staffID, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignSupervisorNonCritical(ctx, newGroup.ID, staffID, newGroup.StartTime)

	if transferredCount > 0 {
		fmt.Printf(visitTransferMessage, transferredCount, newGroup.ID)
	}

	return newGroup, nil
}

// assignSupervisorNonCritical assigns a supervisor but doesn't fail if assignment fails
func (s *service) assignSupervisorNonCritical(ctx context.Context, groupID, staffID int64, startDate time.Time) {
	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   groupID,
		Role:      "Supervisor",
		StartDate: startDate,
	}
	if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
		fmt.Printf(supervisorAssignmentWarning, staffID, groupID, err)
	}
}

// validateSupervisorIDs validates that all supervisor IDs exist as staff members
func (s *service) validateSupervisorIDs(ctx context.Context, supervisorIDs []int64) error {
	if len(supervisorIDs) == 0 {
		return &ActiveError{Op: "ValidateSupervisors", Err: fmt.Errorf("at least one supervisor is required")}
	}

	// Deduplicate supervisor IDs
	uniqueIDs := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueIDs[id] = true
	}

	// Validate each unique supervisor ID exists
	for id := range uniqueIDs {
		_, err := s.staffRepo.FindByID(ctx, id)
		if err != nil {
			return &ActiveError{Op: "ValidateSupervisors", Err: ErrStaffNotFound}
		}
	}

	return nil
}

// StartActivitySessionWithSupervisors starts an activity session with multiple supervisors
func (s *service) StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	var newGroup *active.Group
	err := s.executeSessionStart(ctx, activityID, deviceID, roomID, "StartActivitySessionWithSupervisors", func(ctx context.Context, finalRoomID int64) (*active.Group, error) {
		group, err := s.createSessionWithMultipleSupervisors(ctx, activityID, deviceID, supervisorIDs, finalRoomID)
		newGroup = group
		return group, err
	})

	if err != nil {
		return nil, err
	}

	s.broadcastActivityStartEvent(ctx, newGroup, supervisorIDs)
	return newGroup, nil
}

// executeSessionStart handles common session start logic: conflict checking, device validation, and room determination
func (s *service) executeSessionStart(ctx context.Context, activityID, deviceID int64, roomID *int64, operation string, createSession func(context.Context, int64) (*active.Group, error)) error {
	conflictInfo, err := s.CheckActivityConflict(ctx, activityID, deviceID)
	if err != nil {
		return &ActiveError{Op: operation, Err: err}
	}
	if conflictInfo.HasConflict {
		return &ActiveError{Op: operation, Err: ErrSessionConflict}
	}

	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		existingSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
		if err != nil {
			return err
		}
		if existingSession != nil {
			return ErrDeviceAlreadyActive
		}

		finalRoomID, err := s.determineSessionRoomID(ctx, activityID, roomID)
		if err != nil {
			return err
		}

		_, err = createSession(ctx, finalRoomID)
		return err
	})
}

// createSessionWithMultipleSupervisors creates a new session with multiple supervisors and transfers visits
func (s *service) createSessionWithMultipleSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignMultipleSupervisorsNonCritical(ctx, newGroup.ID, supervisorIDs, newGroup.StartTime)

	if transferredCount > 0 {
		fmt.Printf(visitTransferMessage, transferredCount, newGroup.ID)
	}

	return newGroup, nil
}

// assignMultipleSupervisorsNonCritical assigns multiple supervisors but doesn't fail if assignment fails
func (s *service) assignMultipleSupervisorsNonCritical(ctx context.Context, groupID int64, supervisorIDs []int64, startDate time.Time) {
	// Deduplicate supervisor IDs
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}

	// Assign each unique supervisor
	for staffID := range uniqueSupervisors {
		supervisor := &active.GroupSupervisor{
			StaffID:   staffID,
			GroupID:   groupID,
			Role:      "supervisor",
			StartDate: startDate,
		}
		if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
			fmt.Printf(supervisorAssignmentWarning, staffID, groupID, err)
		}
	}
}

// ForceStartActivitySession starts an activity session with override capability
func (s *service) ForceStartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	var newGroup *active.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err := s.endExistingDeviceSessionIfPresent(ctx, deviceID); err != nil {
			return err
		}

		finalRoomID := s.determineRoomIDWithoutConflictCheck(ctx, activityID, roomID)

		var err error
		newGroup, err = s.createSessionWithSupervisorForceStart(ctx, activityID, deviceID, staffID, finalRoomID)
		return err
	})

	if err != nil {
		return nil, &ActiveError{Op: "ForceStartActivitySession", Err: err}
	}

	return newGroup, nil
}

// endExistingDeviceSessionIfPresent ends any existing session for the device using simple cleanup
func (s *service) endExistingDeviceSessionIfPresent(ctx context.Context, deviceID int64) error {
	return s.endExistingDeviceSession(ctx, deviceID, false)
}

// determineRoomIDWithoutConflictCheck determines room ID without checking conflicts (for force start)
func (s *service) determineRoomIDWithoutConflictCheck(ctx context.Context, activityID int64, roomID *int64) int64 {
	finalRoomID, _ := s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictIgnore)
	return finalRoomID
}

// createSessionWithSupervisorForceStart creates a session for force start with special logging
func (s *service) createSessionWithSupervisorForceStart(ctx context.Context, activityID, deviceID, staffID, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignSupervisorNonCritical(ctx, newGroup.ID, staffID, newGroup.StartTime)

	if transferredCount > 0 {
		fmt.Printf("Transferred %d active visits to new session %d (force start)\n", transferredCount, newGroup.ID)
	}

	return newGroup, nil
}

// createSessionBase creates a new active group session and transfers visits from recent sessions
func (s *service) createSessionBase(ctx context.Context, activityID, deviceID, roomID int64) (*active.Group, int, error) {
	now := time.Now()
	newGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        activityID,
		DeviceID:       &deviceID,
		RoomID:         roomID,
	}

	if err := s.groupRepo.Create(ctx, newGroup); err != nil {
		return nil, 0, err
	}

	transferredCount, err := s.visitRepo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, deviceID)
	if err != nil {
		return nil, 0, err
	}

	return newGroup, transferredCount, nil
}

// ForceStartActivitySessionWithSupervisors starts an activity session with multiple supervisors and override capability
func (s *service) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	var newGroup *active.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err := s.endExistingDeviceSessionWithCleanup(ctx, deviceID); err != nil {
			return err
		}

		finalRoomID, err := s.determineRoomIDForForceStart(ctx, activityID, roomID)
		if err != nil {
			return err
		}

		newGroup, err = s.createSessionWithMultipleSupervisors(ctx, activityID, deviceID, supervisorIDs, finalRoomID)
		return err
	})

	if err != nil {
		return nil, &ActiveError{Op: "ForceStartActivitySessionWithSupervisors", Err: err}
	}

	return newGroup, nil
}

// endExistingDeviceSessionWithCleanup ends existing device session using full cleanup (EndActivitySession)
func (s *service) endExistingDeviceSessionWithCleanup(ctx context.Context, deviceID int64) error {
	return s.endExistingDeviceSession(ctx, deviceID, true)
}

// endExistingDeviceSession ends any existing session for the device
func (s *service) endExistingDeviceSession(ctx context.Context, deviceID int64, fullCleanup bool) error {
	existingSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	if existingSession == nil {
		return nil
	}

	if fullCleanup {
		return s.EndActivitySession(ctx, existingSession.ID)
	}

	return s.groupRepo.EndSession(ctx, existingSession.ID)
}

// determineRoomIDForForceStart determines room ID for force start with conflict warning but no failure
func (s *service) determineRoomIDForForceStart(ctx context.Context, activityID int64, roomID *int64) (int64, error) {
	return s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictWarn)
}

// determineRoomIDWithStrategy determines room ID with configurable conflict handling strategy
func (s *service) determineRoomIDWithStrategy(ctx context.Context, activityID int64, roomID *int64, strategy RoomConflictStrategy) (int64, error) {
	// Manual room selection has highest priority
	if roomID != nil && *roomID > 0 {
		return s.validateManualRoomSelection(ctx, *roomID, strategy)
	}

	// Try to get planned room from activity configuration
	if plannedRoomID := s.getPlannedRoomID(ctx, activityID); plannedRoomID > 0 {
		return plannedRoomID, nil
	}

	// Default fallback room
	return 1, nil
}

// validateManualRoomSelection validates manually selected room based on conflict strategy
func (s *service) validateManualRoomSelection(ctx context.Context, roomID int64, strategy RoomConflictStrategy) (int64, error) {
	if strategy == RoomConflictIgnore {
		return roomID, nil
	}

	hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, roomID, 0)
	if err != nil {
		return 0, err
	}

	if hasConflict {
		if strategy == RoomConflictFail {
			return 0, ErrRoomConflict
		}
		fmt.Printf("Warning: Overriding room conflict for room %d\n", roomID)
	}

	return roomID, nil
}

// getPlannedRoomID retrieves the planned room ID from activity configuration
func (s *service) getPlannedRoomID(ctx context.Context, activityID int64) int64 {
	activityGroup, err := s.activityGroupRepo.FindByID(ctx, activityID)
	if err == nil && activityGroup != nil && activityGroup.PlannedRoomID != nil && *activityGroup.PlannedRoomID > 0 {
		return *activityGroup.PlannedRoomID
	}
	return 0
}

// UpdateActiveGroupSupervisors replaces all supervisors for an active group
func (s *service) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*active.Group, error) {
	if err := s.validateActiveGroupForSupervisorUpdate(ctx, activeGroupID); err != nil {
		return nil, err
	}

	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	uniqueSupervisors := deduplicateSupervisorIDs(supervisorIDs)

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return s.replaceSupervisorsInTransaction(ctx, activeGroupID, uniqueSupervisors)
	})

	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	updatedGroup, err := s.groupRepo.FindWithSupervisors(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	return updatedGroup, nil
}

// validateActiveGroupForSupervisorUpdate validates that the group exists and is active
func (s *service) validateActiveGroupForSupervisorUpdate(ctx context.Context, activeGroupID int64) error {
	activeGroup, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: ErrActiveGroupNotFound}
	}

	if !activeGroup.IsActive() {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: fmt.Errorf("cannot update supervisors for an ended session")}
	}

	return nil
}

// deduplicateSupervisorIDs removes duplicate supervisor IDs
func deduplicateSupervisorIDs(supervisorIDs []int64) map[int64]bool {
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}
	return uniqueSupervisors
}

// replaceSupervisorsInTransaction replaces all supervisors for a group within a transaction
func (s *service) replaceSupervisorsInTransaction(ctx context.Context, activeGroupID int64, uniqueSupervisors map[int64]bool) error {
	currentSupervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return err
	}

	if err := s.endAllCurrentSupervisors(ctx, currentSupervisors); err != nil {
		return err
	}

	return s.upsertSupervisors(ctx, activeGroupID, uniqueSupervisors, currentSupervisors)
}

// endAllCurrentSupervisors ends all current supervisors by setting end_date
func (s *service) endAllCurrentSupervisors(ctx context.Context, supervisors []*active.GroupSupervisor) error {
	now := time.Now()
	for _, supervisor := range supervisors {
		supervisor.EndDate = &now
		if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
			return err
		}
	}
	return nil
}

// upsertSupervisors creates new supervisors or reactivates existing ones
func (s *service) upsertSupervisors(ctx context.Context, activeGroupID int64, uniqueSupervisors map[int64]bool, currentSupervisors []*active.GroupSupervisor) error {
	now := time.Now()

	for supervisorID := range uniqueSupervisors {
		existingSuper := s.findExistingSupervisor(currentSupervisors, supervisorID)

		if existingSuper != nil {
			if err := s.reactivateSupervisor(ctx, existingSuper, now); err != nil {
				return err
			}
		} else {
			if err := s.createNewSupervisor(ctx, activeGroupID, supervisorID, now); err != nil {
				return err
			}
		}
	}

	return nil
}

// findExistingSupervisor finds a supervisor in the list by staff ID and role
func (s *service) findExistingSupervisor(supervisors []*active.GroupSupervisor, staffID int64) *active.GroupSupervisor {
	for _, existing := range supervisors {
		if existing.StaffID == staffID && existing.Role == "supervisor" {
			return existing
		}
	}
	return nil
}

// reactivateSupervisor reactivates an ended supervisor
func (s *service) reactivateSupervisor(ctx context.Context, supervisor *active.GroupSupervisor, now time.Time) error {
	if supervisor.EndDate == nil {
		return nil
	}

	supervisor.EndDate = nil
	supervisor.StartDate = now
	return s.supervisorRepo.Update(ctx, supervisor)
}

// createNewSupervisor creates a new supervisor record
func (s *service) createNewSupervisor(ctx context.Context, activeGroupID, supervisorID int64, now time.Time) error {
	supervisor := &active.GroupSupervisor{
		StaffID:   supervisorID,
		GroupID:   activeGroupID,
		Role:      "supervisor",
		StartDate: now,
	}
	return s.supervisorRepo.Create(ctx, supervisor)
}

// CheckActivityConflict checks for conflicts before starting an activity session
func (s *service) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error) {
	// Check if device is already running another session
	existingDeviceSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}

	if existingDeviceSession != nil {
		deviceIDStr := fmt.Sprintf("%d", deviceID)
		return &ActivityConflictInfo{
			HasConflict:       true,
			ConflictingGroup:  existingDeviceSession,
			ConflictMessage:   fmt.Sprintf("Device %d is already running another session", deviceID),
			ConflictingDevice: &deviceIDStr,
			CanOverride:       true, // Administrative override is always possible
		}, nil
	}

	// Check if activity is already active on a different device
	existingActivitySessions, err := s.groupRepo.FindActiveByGroupID(ctx, activityID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}

	if len(existingActivitySessions) > 0 {
		// Activity is already active on another device
		existingSession := existingActivitySessions[0]
		var conflictDeviceStr *string
		if existingSession.DeviceID != nil {
			deviceIDStr := fmt.Sprintf("%d", *existingSession.DeviceID)
			conflictDeviceStr = &deviceIDStr
		}
		return &ActivityConflictInfo{
			HasConflict:       true,
			ConflictingGroup:  existingSession,
			ConflictMessage:   fmt.Sprintf("Activity is already active on device %s", getDeviceIDString(existingSession.DeviceID)),
			ConflictingDevice: conflictDeviceStr,
			CanOverride:       true, // Administrative override is always possible
		}, nil
	}

	// No conflicts
	return &ActivityConflictInfo{
		HasConflict: false,
		CanOverride: true,
	}, nil
}

// getDeviceIDString returns a string representation of device ID or "unknown" if nil
func getDeviceIDString(deviceID *int64) string {
	if deviceID == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d", *deviceID)
}

// EndActivitySession ends an active activity session
func (s *service) EndActivitySession(ctx context.Context, activeGroupID int64) error {
	// Verify the session exists and is active
	group, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupNotFound}
	}

	if !group.IsActive() {
		return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupAlreadyEnded}
	}

	// Collect active visits BEFORE transaction for SSE broadcasts
	visitsToNotify, err := s.collectActiveVisitsForSSE(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: ErrDatabaseOperation}
	}

	// Use transaction to ensure atomic cleanup
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		// End all active visits
		for _, visitData := range visitsToNotify {
			if err := txService.visitRepo.EndVisit(ctx, visitData.VisitID); err != nil {
				return err
			}
		}

		// End the session
		if err := txService.groupRepo.EndSession(ctx, activeGroupID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: err}
	}

	// Broadcast SSE events (fire-and-forget, outside transaction)
	if s.broadcaster != nil {
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)
		s.broadcastStudentCheckoutEvents(activeGroupIDStr, visitsToNotify)
		s.broadcastActivityEndEvent(ctx, activeGroupID, activeGroupIDStr)
	}

	return nil
}

// GetDeviceCurrentSession gets the current active session for a device
func (s *service) GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*active.Group, error) {
	session, err := s.groupRepo.FindActiveByDeviceIDWithNames(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "GetDeviceCurrentSession", Err: err}
	}

	if session == nil {
		return nil, &ActiveError{Op: "GetDeviceCurrentSession", Err: ErrNoActiveSession}
	}

	return session, nil
}

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// GetUnclaimedActiveGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim supervision via frontend
func (s *service) GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error) {
	groups, err := s.groupRepo.FindUnclaimed(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}

	return groups, nil
}

// ClaimActiveGroup allows a staff member to claim supervision of an active group
// This is primarily used for deviceless rooms like Schulhof
func (s *service) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error) {
	// Verify group exists and is still active
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: errors.New("active group not found")}
	}

	if group.EndTime != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: errors.New("cannot claim ended group")}
	}

	// Check if staff is already supervising this group (only check active supervisors)
	existingSupervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, groupID, true)
	if err == nil {
		for _, sup := range existingSupervisors {
			if sup.StaffID == staffID {
				return nil, &ActiveError{Op: "ClaimActiveGroup", Err: ErrStaffAlreadySupervising}
			}
		}
	}

	// Create supervisor assignment
	if role == "" {
		role = "supervisor"
	}

	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   groupID,
		Role:      role,
		StartDate: time.Now(),
		// EndDate is nil (active supervision)
	}

	// Use existing CreateGroupSupervisor method for validation and creation
	if err := s.CreateGroupSupervisor(ctx, supervisor); err != nil {
		return nil, err
	}

	return supervisor, nil
}

// ======== Visit Display Operations ========

// GetVisitsWithDisplayData returns visits for an active group with student display information
func (s *service) GetVisitsWithDisplayData(ctx context.Context, activeGroupID int64) ([]VisitWithDisplayData, error) {
	// Verify the active group exists
	_, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "GetVisitsWithDisplayData", Err: ErrActiveGroupNotFound}
	}

	// Query visits with student display data
	var results []VisitWithDisplayData
	err = s.db.NewSelect().
		ColumnExpr("v.id AS visit_id").
		ColumnExpr("v.student_id").
		ColumnExpr("v.active_group_id").
		ColumnExpr("v.entry_time").
		ColumnExpr("v.exit_time").
		ColumnExpr("v.created_at").
		ColumnExpr("v.updated_at").
		ColumnExpr("p.first_name").
		ColumnExpr("p.last_name").
		ColumnExpr("COALESCE(s.school_class, '') AS school_class").
		ColumnExpr("COALESCE(g.name, '') AS ogs_group_name").
		TableExpr("active.visits AS v").
		Join("INNER JOIN users.students AS s ON s.id = v.student_id").
		Join("INNER JOIN users.persons AS p ON p.id = s.person_id").
		Join("LEFT JOIN education.groups AS g ON g.id = s.group_id").
		Where("v.active_group_id = ?", activeGroupID).
		Where("v.exit_time IS NULL").
		OrderExpr("v.entry_time DESC").
		Scan(ctx, &results)

	if err != nil {
		return nil, &ActiveError{Op: "GetVisitsWithDisplayData", Err: err}
	}

	return results, nil
}
