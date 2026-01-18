package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
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
	DeviceRepo         iotModels.DeviceRepository

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
	deviceRepo         iotModels.DeviceRepository

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
		deviceRepo:         deps.DeviceRepo,
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
	var deviceRepo = s.deviceRepo
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
	if txRepo, ok := s.deviceRepo.(base.TransactionalRepository); ok {
		deviceRepo = txRepo.WithTx(tx).(iotModels.DeviceRepository)
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
		deviceRepo:         deviceRepo,
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

// Visit operations
func (s *service) GetVisit(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetVisit", Err: ErrVisitNotFound}
	}
	return visit, nil
}

func (s *service) CreateVisit(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}

	// Validate student exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateStudentExists(ctx, visit.StudentID); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	// Validate active group exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateActiveGroupExists(ctx, visit.ActiveGroupID); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	deviceID, staffID := s.extractContextIDs(ctx)

	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		// Ensure no existing active visit for this student
		if err := txService.ensureStudentHasNoActiveVisit(txCtx, visit.StudentID); err != nil {
			return err
		}

		// Handle attendance (create new or update on re-entry)
		if err := txService.ensureOrUpdateAttendance(txCtx, visit, staffID, deviceID); err != nil {
			return err
		}

		// Auto-clear sickness when student checks in
		txService.autoClearStudentSickness(txCtx, visit.StudentID)

		// Create the visit record
		if txService.visitRepo.Create(txCtx, visit) != nil {
			return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
		}

		return nil
	})

	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}

	// Broadcast SSE event (fire-and-forget, outside transaction)
	s.broadcastVisitCreated(ctx, visit)

	return nil
}

// isNotFoundError checks if an error is due to "not found" (sql.ErrNoRows) vs. other database errors
func isNotFoundError(err error) bool {
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}

// validateStudentExists checks if a student exists, returning appropriate errors
func (s *service) validateStudentExists(ctx context.Context, studentID int64) error {
	if _, err := s.studentRepo.FindByID(ctx, studentID); err != nil {
		if isNotFoundError(err) {
			return ErrStudentNotFound
		}
		return err
	}
	return nil
}

// validateActiveGroupExists checks if an active group exists, returning appropriate errors
func (s *service) validateActiveGroupExists(ctx context.Context, groupID int64) error {
	if _, err := s.groupRepo.FindByID(ctx, groupID); err != nil {
		if isNotFoundError(err) {
			return ErrActiveGroupNotFound
		}
		return err
	}
	return nil
}

// validateStaffExists checks if a staff member exists, returning appropriate errors
func (s *service) validateStaffExists(ctx context.Context, staffID int64) error {
	if _, err := s.staffRepo.FindByID(ctx, staffID); err != nil {
		if isNotFoundError(err) {
			return ErrStaffNotFound
		}
		return err
	}
	return nil
}

// validateCombinedGroupExists checks if a combined group exists, returning appropriate errors
func (s *service) validateCombinedGroupExists(ctx context.Context, groupID int64) error {
	if _, err := s.combinedGroupRepo.FindByID(ctx, groupID); err != nil {
		if isNotFoundError(err) {
			return ErrCombinedGroupNotFound
		}
		return err
	}
	return nil
}

// extractContextIDs extracts device and staff IDs from context
func (s *service) extractContextIDs(ctx context.Context) (deviceID, staffID int64) {
	if deviceCtx := device.DeviceFromCtx(ctx); deviceCtx != nil {
		deviceID = deviceCtx.ID
	}
	if staffCtx := device.StaffFromCtx(ctx); staffCtx != nil {
		staffID = staffCtx.ID
	}
	return deviceID, staffID
}

func (s *service) UpdateVisit(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrInvalidData}
	}

	if s.visitRepo.Update(ctx, visit) != nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteVisit(ctx context.Context, id int64) error {
	_, err := s.visitRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteVisit", Err: ErrVisitNotFound}
	}

	if s.visitRepo.Delete(ctx, id) != nil {
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
	autoSyncAttendance := shouldAutoSyncAttendance(ctx)
	deviceID, staffID := s.extractContextIDsIfAutoSync(ctx, autoSyncAttendance)

	var endedVisit *active.Visit
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		visit, err := txService.endVisitRecord(txCtx, id)
		if err != nil {
			return err
		}
		endedVisit = visit

		if visit.ExitTime == nil || !autoSyncAttendance {
			return nil
		}

		return txService.syncAttendanceOnVisitEnd(txCtx, visit, deviceID, staffID)
	})

	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	s.broadcastVisitCheckout(ctx, endedVisit)
	return nil
}

// extractContextIDsIfAutoSync extracts device and staff IDs from context when auto-sync is enabled
func (s *service) extractContextIDsIfAutoSync(ctx context.Context, autoSyncAttendance bool) (deviceID, staffID int64) {
	if !autoSyncAttendance {
		return 0, 0
	}
	return s.extractContextIDs(ctx)
}

// endVisitRecord ends the visit record and returns the updated visit
func (s *service) endVisitRecord(ctx context.Context, id int64) (*active.Visit, error) {
	visit, err := s.visitRepo.FindByID(ctx, id)
	if err != nil || visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}

	if s.visitRepo.EndVisit(ctx, id) != nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	visit, err = s.visitRepo.FindByID(ctx, id)
	if err != nil || visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}

	return visit, nil
}

// syncAttendanceOnVisitEnd synchronizes attendance record when a visit ends
func (s *service) syncAttendanceOnVisitEnd(ctx context.Context, visit *active.Visit, deviceID, staffID int64) error {
	// Only auto-check the student out if no other active visits remain
	activeVisits, err := s.visitRepo.FindActiveByStudentID(ctx, visit.StudentID)
	if err != nil {
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}
	if len(activeVisits) > 0 {
		return nil
	}

	attendance, err := s.getStudentAttendanceOrIgnoreMissing(ctx, visit.StudentID)
	if err != nil {
		return err
	}
	if attendance == nil || attendance.CheckOutTime != nil {
		return nil
	}

	return s.updateAttendanceCheckout(ctx, attendance, visit, deviceID, staffID)
}

// getStudentAttendanceOrIgnoreMissing retrieves attendance or returns nil if not found
func (s *service) getStudentAttendanceOrIgnoreMissing(ctx context.Context, studentID int64) (*active.Attendance, error) {
	attendance, err := s.attendanceRepo.GetStudentCurrentStatus(ctx, studentID)
	if err == nil {
		return attendance, nil
	}

	// Ignore missing attendance – nothing to sync
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) && errors.Is(dbErr.Err, sql.ErrNoRows) {
		return nil, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nil, &ActiveError{Op: "EndVisit", Err: err}
}

// updateAttendanceCheckout updates attendance with checkout information
func (s *service) updateAttendanceCheckout(ctx context.Context, attendance *active.Attendance, visit *active.Visit, deviceID, staffID int64) error {
	resolvedStaffID := staffID
	if resolvedStaffID == 0 && deviceID > 0 {
		if supervisorID, err := s.getDeviceSupervisorID(ctx, deviceID); err == nil {
			resolvedStaffID = supervisorID
		}
	}

	checkoutTime := *visit.ExitTime
	attendance.CheckOutTime = &checkoutTime
	if resolvedStaffID > 0 {
		attendance.CheckedOutBy = &resolvedStaffID
	}

	if err := s.attendanceRepo.Update(ctx, attendance); err != nil {
		return &ActiveError{Op: "EndVisit", Err: err}
	}
	return nil
}

// broadcastVisitCheckout broadcasts SSE event for visit checkout
func (s *service) broadcastVisitCheckout(ctx context.Context, endedVisit *active.Visit) {
	if s.broadcaster == nil || endedVisit == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", endedVisit.ActiveGroupID)
	studentID := fmt.Sprintf("%d", endedVisit.StudentID)
	studentName, studentRec := s.getStudentDisplayData(ctx, endedVisit.StudentID)

	event := realtime.NewEvent(
		realtime.EventStudentCheckOut,
		activeGroupID,
		realtime.EventData{
			StudentID:   &studentID,
			StudentName: &studentName,
		},
	)

	s.broadcastWithLogging(activeGroupID, studentID, event, "student_checkout")
	s.broadcastToEducationalGroup(studentRec, event)
}

// broadcastToEducationalGroup mirrors active-group broadcasts to the student's OGS group topic
func (s *service) broadcastToEducationalGroup(student *userModels.Student, event realtime.Event) {
	if s.broadcaster == nil || student == nil || student.GroupID == nil {
		return
	}
	groupID := fmt.Sprintf("edu:%d", *student.GroupID)
	if err := s.broadcaster.BroadcastToGroup(groupID, event); err != nil {
		if logging.Logger != nil {
			studentID := ""
			if event.Data.StudentID != nil {
				studentID = *event.Data.StudentID
			}
			logging.Logger.WithFields(map[string]interface{}{
				"error":                 err.Error(),
				"event_type":            string(event.Type),
				"education_group_topic": groupID,
				"student_id":            studentID,
			}).Error(sseErrorMessage + " for educational topic")
		}
	}
}

// broadcastStudentCheckoutEvents sends checkout SSE events for each visit.
// This helper reduces cognitive complexity in session timeout processing.
func (s *service) broadcastStudentCheckoutEvents(sessionIDStr string, visitsToNotify []visitSSEData) {
	for _, visitData := range visitsToNotify {
		studentIDStr := fmt.Sprintf("%d", visitData.StudentID)
		studentName := visitData.Name

		checkoutEvent := realtime.NewEvent(
			realtime.EventStudentCheckOut,
			sessionIDStr,
			realtime.EventData{
				StudentID:   &studentIDStr,
				StudentName: &studentName,
			},
		)

		s.broadcastWithLogging(sessionIDStr, studentIDStr, checkoutEvent, "student_checkout")
		s.broadcastToEducationalGroup(visitData.Student, checkoutEvent)
	}
}

// broadcastActivityEndEvent sends the activity_end SSE event for a completed session.
// This helper reduces cognitive complexity in session timeout processing.
func (s *service) broadcastActivityEndEvent(ctx context.Context, sessionID int64, sessionIDStr string) {
	finalGroup, err := s.groupRepo.FindByID(ctx, sessionID)
	if err != nil || finalGroup == nil {
		return
	}

	roomIDStr := fmt.Sprintf("%d", finalGroup.RoomID)
	activityName := s.getActivityName(ctx, finalGroup.GroupID)
	roomName := s.getRoomName(ctx, finalGroup.RoomID)

	event := realtime.NewEvent(
		realtime.EventActivityEnd,
		sessionIDStr,
		realtime.EventData{
			ActivityName: &activityName,
			RoomID:       &roomIDStr,
			RoomName:     &roomName,
		},
	)

	s.broadcastWithLogging(sessionIDStr, "", event, "activity_end")
}

// broadcastWithLogging broadcasts an event and logs any errors.
func (s *service) broadcastWithLogging(activeGroupID, studentID string, event realtime.Event, eventType string) {
	if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
		if logging.Logger != nil {
			fields := map[string]interface{}{
				"error":           err.Error(),
				"event_type":      eventType,
				"active_group_id": activeGroupID,
			}
			if studentID != "" {
				fields["student_id"] = studentID
			}
			logging.Logger.WithFields(fields).Error(sseErrorMessage)
		}
	}
}

// getActivityName retrieves the activity name by group ID, returning empty string on error.
func (s *service) getActivityName(ctx context.Context, groupID int64) string {
	activity, err := s.activityGroupRepo.FindByID(ctx, groupID)
	if err != nil || activity == nil {
		return ""
	}
	return activity.Name
}

// getRoomName retrieves the room name by room ID, returning empty string on error.
func (s *service) getRoomName(ctx context.Context, roomID int64) string {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil || room == nil {
		return ""
	}
	return room.Name
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

func (s *service) GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	if len(studentIDs) == 0 {
		return map[int64]*active.Visit{}, nil
	}

	visits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsCurrentVisits", Err: ErrDatabaseOperation}
	}

	if visits == nil {
		visits = make(map[int64]*active.Visit)
	}

	return visits, nil
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
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrInvalidData}
	}

	// Validate active group exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateActiveGroupExists(ctx, supervisor.GroupID); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: err}
	}

	// Validate staff exists before INSERT (prevents FK constraint errors in logs)
	if err := s.validateStaffExists(ctx, supervisor.StaffID); err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: err}
	}

	// Check if staff is already supervising this group (only check active supervisors)
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, supervisor.GroupID, true)
	if err != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	for _, s := range supervisors {
		if s.StaffID == supervisor.StaffID {
			return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrStaffAlreadySupervising}
		}
	}

	if s.supervisorRepo.Create(ctx, supervisor) != nil {
		return &ActiveError{Op: "CreateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	if supervisor == nil || supervisor.Validate() != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrInvalidData}
	}

	if s.supervisorRepo.Update(ctx, supervisor) != nil {
		return &ActiveError{Op: "UpdateGroupSupervisor", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	_, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteGroupSupervisor", Err: ErrGroupSupervisorNotFound}
	}

	if s.supervisorRepo.Delete(ctx, id) != nil {
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
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) FindSupervisorsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindByActiveGroupIDs(ctx, activeGroupIDs, true)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupIDs", Err: ErrDatabaseOperation}
	}
	return supervisors, nil
}

func (s *service) EndSupervision(ctx context.Context, id int64) error {
	// Verify supervision exists first
	_, err := s.supervisorRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "EndSupervision", Err: ErrGroupSupervisorNotFound}
		}
		return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("failed to verify supervision: %w", err)}
	}

	if err := s.supervisorRepo.EndSupervision(ctx, id); err != nil {
		return &ActiveError{Op: "EndSupervision", Err: fmt.Errorf("end supervision failed: %w", err)}
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

// visitSSEData holds data needed for SSE broadcasts after a visit is ended
type visitSSEData struct {
	VisitID   int64
	StudentID int64
	Name      string
	Student   *userModels.Student
}
