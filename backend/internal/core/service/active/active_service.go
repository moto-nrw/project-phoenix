package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	activitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/activities"
	educationPort "github.com/moto-nrw/project-phoenix/internal/core/port/education"
	facilityPort "github.com/moto-nrw/project-phoenix/internal/core/port/facilities"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/moto-nrw/project-phoenix/internal/core/service/education"
	"github.com/moto-nrw/project-phoenix/internal/core/service/users"
	"github.com/uptrace/bun"
)

// Broadcaster interface - uses the port interface from Hexagonal Architecture.
// Services depend on the port interface, not the concrete adapter implementation.
type Broadcaster = port.Broadcaster

const (
	// sseErrorMessage is the standard error message for SSE broadcast failures
	sseErrorMessage = "SSE broadcast failed"
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
	GroupReadRepo      activePort.GroupReadRepository
	GroupWriteRepo     activePort.GroupWriteRepository
	GroupRelationsRepo activePort.GroupRelationsRepository
	VisitRepo          activePort.VisitRepository
	SupervisorRepo     activePort.GroupSupervisorRepository
	CombinedGroupRepo  activePort.CombinedGroupRepository
	GroupMappingRepo   activePort.GroupMappingRepository
	AttendanceRepo     activePort.AttendanceRepository

	// User domain repositories
	StudentRepo userPort.StudentRepository
	PersonRepo  userPort.PersonRepository
	TeacherRepo userPort.TeacherRepository
	StaffRepo   userPort.StaffRepository

	// Supporting domain repositories
	RoomRepo           facilityPort.RoomRepository
	ActivityGroupRepo  activitiesPort.GroupRepository
	ActivityCatRepo    activitiesPort.CategoryRepository
	EducationGroupRepo educationPort.GroupRepository

	// External services
	EducationService education.Service
	UsersService     users.PersonService

	// Infrastructure
	DB          *bun.DB
	Broadcaster Broadcaster // SSE event broadcaster (optional - can be nil for testing)
}

// Service implements the Active Service interface
type service struct {
	groupReadRepo      activePort.GroupReadRepository
	groupWriteRepo     activePort.GroupWriteRepository
	groupRelationsRepo activePort.GroupRelationsRepository
	visitRepo          activePort.VisitRepository
	supervisorRepo     activePort.GroupSupervisorRepository
	combinedGroupRepo  activePort.CombinedGroupRepository
	groupMappingRepo   activePort.GroupMappingRepository

	// Additional repositories for dashboard analytics
	studentRepo        userPort.StudentRepository
	roomRepo           facilityPort.RoomRepository
	activityGroupRepo  activitiesPort.GroupRepository
	activityCatRepo    activitiesPort.CategoryRepository
	educationGroupRepo educationPort.GroupRepository
	personRepo         userPort.PersonRepository

	// New dependencies for attendance tracking
	attendanceRepo   activePort.AttendanceRepository
	educationService education.Service
	usersService     users.PersonService
	teacherRepo      userPort.TeacherRepository
	staffRepo        userPort.StaffRepository

	db        *bun.DB
	txHandler *base.TxHandler

	// SSE real-time event broadcasting (optional - can be nil for testing)
	broadcaster Broadcaster
}

// NewService creates a new active service instance
func NewService(deps ServiceDependencies) Service {
	return &service{
		groupReadRepo:      deps.GroupReadRepo,
		groupWriteRepo:     deps.GroupWriteRepo,
		groupRelationsRepo: deps.GroupRelationsRepo,
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
func (s *service) WithTx(tx bun.Tx) any {
	repos := wrapRepositoriesWithTx(s, tx)

	return &service{
		groupReadRepo:      repos.groupReadRepo,
		groupWriteRepo:     repos.groupWriteRepo,
		groupRelationsRepo: repos.groupRelationsRepo,
		visitRepo:          repos.visitRepo,
		supervisorRepo:     repos.supervisorRepo,
		combinedGroupRepo:  repos.combinedGroupRepo,
		groupMappingRepo:   repos.groupMappingRepo,
		studentRepo:        repos.studentRepo,
		roomRepo:           repos.roomRepo,
		activityGroupRepo:  repos.activityGroupRepo,
		activityCatRepo:    repos.activityCatRepo,
		educationGroupRepo: repos.educationGroupRepo,
		personRepo:         repos.personRepo,
		attendanceRepo:     repos.attendanceRepo,
		educationService:   s.educationService,
		usersService:       s.usersService,
		teacherRepo:        repos.teacherRepo,
		staffRepo:          repos.staffRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
		broadcaster:        s.broadcaster,
	}
}

// CheckActivityConflict checks for conflicts before starting an activity session
func (s *service) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error) {
	// Check if device is already running another session
	existingDeviceSession, err := s.groupReadRepo.FindActiveByDeviceID(ctx, deviceID)
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
	existingActivitySessions, err := s.groupReadRepo.FindActiveByGroupID(ctx, activityID)
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
	group, err := s.groupReadRepo.FindByID(ctx, activeGroupID)
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
		if err := txService.groupWriteRepo.EndSession(ctx, activeGroupID); err != nil {
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
	session, err := s.groupReadRepo.FindActiveByDeviceIDWithNames(ctx, deviceID)
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
	groups, err := s.groupReadRepo.FindUnclaimed(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}

	return groups, nil
}

// ClaimActiveGroup allows a staff member to claim supervision of an active group
// This is primarily used for deviceless rooms like Schulhof
func (s *service) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error) {
	// Verify group exists and is still active
	group, err := s.groupReadRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: ErrActiveGroupNotFound}
	}

	if group.EndTime != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: ErrCannotClaimEndedGroup}
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
	_, err := s.groupReadRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "GetVisitsWithDisplayData", Err: ErrActiveGroupNotFound}
	}

	// Delegate to repository for data access
	results, err := s.visitRepo.FindActiveByGroupIDWithDisplayData(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "GetVisitsWithDisplayData", Err: err}
	}

	// Convert model type to service type
	serviceResults := make([]VisitWithDisplayData, len(results))
	for i, r := range results {
		serviceResults[i] = VisitWithDisplayData{
			VisitID:       r.VisitID,
			StudentID:     r.StudentID,
			ActiveGroupID: r.ActiveGroupID,
			EntryTime:     r.EntryTime,
			ExitTime:      r.ExitTime,
			FirstName:     r.FirstName,
			LastName:      r.LastName,
			SchoolClass:   r.SchoolClass,
			OGSGroupName:  r.OGSGroupName,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
		}
	}

	return serviceResults, nil
}
