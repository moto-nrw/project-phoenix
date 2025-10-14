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
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Broadcaster interface (re-exported from realtime for convenience)
type Broadcaster = realtime.Broadcaster

// Service implements the Active Service interface
type service struct {
	groupRepo             active.GroupRepository
	visitRepo             active.VisitRepository
	supervisorRepo        active.GroupSupervisorRepository
	combinedGroupRepo     active.CombinedGroupRepository
	groupMappingRepo      active.GroupMappingRepository
	scheduledCheckoutRepo active.ScheduledCheckoutRepository

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
func NewService(
	groupRepo active.GroupRepository,
	visitRepo active.VisitRepository,
	supervisorRepo active.GroupSupervisorRepository,
	combinedGroupRepo active.CombinedGroupRepository,
	groupMappingRepo active.GroupMappingRepository,
	scheduledCheckoutRepo active.ScheduledCheckoutRepository,
	studentRepo userModels.StudentRepository,
	roomRepo facilityModels.RoomRepository,
	activityGroupRepo activitiesModels.GroupRepository,
	activityCatRepo activitiesModels.CategoryRepository,
	educationGroupRepo educationModels.GroupRepository,
	personRepo userModels.PersonRepository,
	attendanceRepo active.AttendanceRepository,
	educationService education.Service,
	usersService users.PersonService,
	teacherRepo userModels.TeacherRepository,
	staffRepo userModels.StaffRepository,
	db *bun.DB,
	broadcaster Broadcaster, // SSE event broadcaster (optional - can be nil)
) Service {
	return &service{
		groupRepo:             groupRepo,
		visitRepo:             visitRepo,
		supervisorRepo:        supervisorRepo,
		combinedGroupRepo:     combinedGroupRepo,
		groupMappingRepo:      groupMappingRepo,
		scheduledCheckoutRepo: scheduledCheckoutRepo,
		studentRepo:           studentRepo,
		roomRepo:              roomRepo,
		activityGroupRepo:     activityGroupRepo,
		activityCatRepo:       activityCatRepo,
		educationGroupRepo:    educationGroupRepo,
		personRepo:            personRepo,
		attendanceRepo:        attendanceRepo,
		educationService:      educationService,
		usersService:          usersService,
		teacherRepo:           teacherRepo,
		staffRepo:             staffRepo,
		db:                    db,
		txHandler:             base.NewTxHandler(db),
		broadcaster:           broadcaster,
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
	return group, nil
}

func (s *service) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	if err := group.Validate(); err != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: ErrInvalidData}
	}

	// Check for room conflicts if room is assigned
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, group.RoomID, 0)
		if err != nil {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrDatabaseOperation}
		}
		if hasConflict {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrRoomConflict}
		}
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

	// Check for room conflicts if room is assigned (exclude current group)
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, group.RoomID, group.ID)
		if err != nil {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrDatabaseOperation}
		}
		if hasConflict {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrRoomConflict}
		}
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

	// Broadcast SSE event (fire-and-forget)
	if s.broadcaster != nil {
		activeGroupID := fmt.Sprintf("%d", visit.ActiveGroupID)
		studentID := fmt.Sprintf("%d", visit.StudentID)

		// Query student for display data
		var studentName string
		if student, err := s.studentRepo.FindByID(ctx, visit.StudentID); err == nil && student != nil {
			if person, err := s.personRepo.FindByID(ctx, student.PersonID); err == nil && person != nil {
				studentName = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
			}
		}

		event := realtime.NewEvent(
			realtime.EventStudentCheckIn,
			activeGroupID,
			realtime.EventData{
				StudentID:   &studentID,
				StudentName: &studentName,
			},
		)

		if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
			// Fire-and-forget: log but don't fail the operation
			if logging.Logger != nil {
				logging.Logger.WithFields(map[string]interface{}{
					"error":           err.Error(),
					"event_type":      "student_checkin",
					"active_group_id": activeGroupID,
					"student_id":      studentID,
				}).Error("SSE broadcast failed")
			}
		}
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

	// Broadcast SSE event (fire-and-forget)
	if s.broadcaster != nil {
		// Reload visit to get complete data for event payload
		visit, err := s.visitRepo.FindByID(ctx, id)
		if err == nil && visit != nil {
			activeGroupID := fmt.Sprintf("%d", visit.ActiveGroupID)
			studentID := fmt.Sprintf("%d", visit.StudentID)

			// Query student for display data
			var studentName string
			if student, err := s.studentRepo.FindByID(ctx, visit.StudentID); err == nil && student != nil {
				if person, err := s.personRepo.FindByID(ctx, student.PersonID); err == nil && person != nil {
					studentName = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
				}
			}

			event := realtime.NewEvent(
				realtime.EventStudentCheckOut,
				activeGroupID,
				realtime.EventData{
					StudentID:   &studentID,
					StudentName: &studentName,
				},
			)

			if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
				// Fire-and-forget: log but don't fail the operation
				if logging.Logger != nil {
					logging.Logger.WithFields(map[string]interface{}{
						"error":           err.Error(),
						"event_type":      "student_checkout",
						"active_group_id": activeGroupID,
						"student_id":      studentID,
					}).Error("SSE broadcast failed")
				}
			}
		}
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

func (s *service) FindSupervisorsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.GroupSupervisor, error) {
	supervisors, err := s.supervisorRepo.FindByActiveGroupIDs(ctx, activeGroupIDs)
	if err != nil {
		return nil, &ActiveError{Op: "FindSupervisorsByActiveGroupIDs", Err: ErrDatabaseOperation}
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

	// Get students currently present (checked in but not checked out)
	// Use UTC-based date calculation to match repository methods
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)

	// Get active visits first for presence calculation
	activeVisits, err := s.visitRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Create set of students with active visits
	studentsWithActiveVisits := make(map[int64]bool)
	for _, visit := range activeVisits {
		if visit.IsActive() {
			studentsWithActiveVisits[visit.StudentID] = true
		}
	}

	// Count students who are currently present (attendance records OR active visits)
	allStudents, err := s.studentRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Use a single map to track all present students (union of attendance + visits)
	studentsPresent := make(map[int64]bool)
	studentsWithAttendance := make(map[int64]bool)

	// First collect students with attendance records for today
	for _, student := range allStudents {
		// Check if this student has active attendance (checked in but not out) for today
		attendance, err := s.attendanceRepo.FindByStudentAndDate(ctx, student.ID, today)
		if err == nil {
			// Check if any attendance record shows student is still present (no check-out time)
			for _, record := range attendance {
				if record.CheckOutTime == nil {
					studentsWithAttendance[student.ID] = true
					studentsPresent[student.ID] = true
					break // Count each student only once
				}
			}
		}
	}

	// Add students with active visits (union, not sum)
	for studentID := range studentsWithActiveVisits {
		studentsPresent[studentID] = true
	}

	analytics.StudentsPresent = len(studentsPresent)

	// Create maps to track students and their locations
	studentLocationMap := make(map[int64]string) // studentID -> location
	recentCheckouts := make(map[int64]time.Time) // studentID -> checkout time

	// Track active visits for room calculations
	for _, visit := range activeVisits {
		if visit.IsActive() {
			studentLocationMap[visit.StudentID] = "active"
		} else if visit.ExitTime != nil {
			// Track recent checkouts for transit calculation
			recentCheckouts[visit.StudentID] = *visit.ExitTime
		}
	}

	// Get total enrolled students (use same allStudents variable)
	// allStudents already declared above for attendance check
	// Calculate students who are present but have no active visits (in transit)
	// Present students = those with attendance OR active visits
	// In transit = present students WITHOUT active visits
	studentsInTransitCount := 0

	// Count students with attendance who don't have active visits
	for studentID := range studentsWithAttendance {
		if !studentsWithActiveVisits[studentID] {
			studentsInTransitCount++
		}
	}
	analytics.StudentsInTransit = studentsInTransitCount

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
	roomStudentsMap := make(map[int64]map[int64]struct{})    // roomID -> set of unique student IDs
	uniqueStudentsInRoomsOverall := make(map[int64]struct{}) // Track unique students across all rooms

	for _, group := range activeGroups {
		// Count all active groups regardless of start date - if they're active, they should be counted
		if group.IsActive() {
			activeGroupsCount++
			occupiedRooms[group.RoomID] = true

			// Initialize room student set if not exists
			if roomStudentsMap[group.RoomID] == nil {
				roomStudentsMap[group.RoomID] = make(map[int64]struct{})
			}

			// Count unique students for this group
			groupVisits, err := s.visitRepo.FindByActiveGroupID(ctx, group.ID)
			if err == nil {
				for _, visit := range groupVisits {
					if visit.IsActive() {
						// Add student to room's unique student set (prevents double-counting within room)
						roomStudentsMap[group.RoomID][visit.StudentID] = struct{}{}
						// Add student to overall unique students set (prevents double-counting overall)
						uniqueStudentsInRoomsOverall[visit.StudentID] = struct{}{}
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

	// Calculate capacity utilization using unique student count
	studentsInRooms := len(uniqueStudentsInRoomsOverall)
	if roomCapacityTotal > 0 {
		analytics.CapacityUtilization = float64(studentsInRooms) / float64(roomCapacityTotal)
	}

	// Get supervisors today
	supervisors, err := s.supervisorRepo.List(ctx, nil)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	supervisorMap := make(map[int64]bool)
	// today variable already declared earlier in function
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
	studentsInGroupRooms := 0
	studentsInHomeRoom := 0

	// Process room visits to categorize students using unique student counts
	for roomID, studentSet := range roomStudentsMap {
		uniqueStudentCount := len(studentSet)
		if room, ok := roomByID[roomID]; ok {
			// Check for playground/school yard by category
			switch room.Category {
			case "Schulhof", "Playground", "school_yard":
				studentsOnPlayground += uniqueStudentCount
			}

			// Check if this room belongs to an educational group
			if educationGroupRooms[roomID] {
				studentsInGroupRooms += uniqueStudentCount
				// For now, consider all students in group rooms as in their home room
				studentsInHomeRoom = studentsInGroupRooms
			}
		}
	}

	// Calculate students in rooms: count unique students with active visits EXCLUDING playground/outdoor areas
	uniqueStudentsInRooms := make(map[int64]struct{})
	for _, visit := range activeVisits {
		if visit.IsActive() {
			// Find the room for this visit to check if it's indoor
			for _, group := range activeGroups {
				if group.ID == visit.ActiveGroupID && group.IsActive() {
					if room, ok := roomByID[group.RoomID]; ok {
						// Exclude playground/outdoor areas from "In Räumen" count
						switch room.Category {
						case "Schulhof", "Playground", "school_yard":
							// Don't count playground visits as "in rooms"
						default:
							// Add student ID to the set for indoor rooms (prevents double-counting)
							uniqueStudentsInRooms[visit.StudentID] = struct{}{}
						}
					}
					break
				}
			}
		}
	}
	studentsInRoomsTotal := len(uniqueStudentsInRooms)

	analytics.StudentsOnPlayground = studentsOnPlayground
	analytics.StudentsInRooms = studentsInRoomsTotal // Students in indoor rooms (excluding playground)
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

			// Count unique students for this group
			visitCount := 0
			if studentSet, ok := roomStudentsMap[group.RoomID]; ok {
				visitCount = len(studentSet)
			}

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
					if studentSet, ok := roomStudentsMap[group.RoomID]; ok {
						participantCount = len(studentSet)
					}
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

		// Get unique student count for this room
		studentCount := 0
		if studentSet, ok := roomStudentsMap[group.RoomID]; ok {
			studentCount = len(studentSet)
		}

		groupInfo := ActiveGroupInfo{
			Name:         groupName,
			Type:         groupType,
			StudentCount: studentCount,
			Location:     location,
			Status:       "active",
		}

		activeGroupsSummary = append(activeGroupsSummary, groupInfo)
	}
	analytics.ActiveGroupsSummary = activeGroupsSummary

	return analytics, nil
}

// Activity Session Management with Conflict Detection

// StartActivitySession starts a new activity session on a device with conflict detection
func (s *service) StartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	// First check for conflicts
	conflictInfo, err := s.CheckActivityConflict(ctx, activityID, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "StartActivitySession", Err: err}
	}

	if conflictInfo.HasConflict {
		return nil, &ActiveError{Op: "StartActivitySession", Err: ErrSessionConflict}
	}

	// Use transaction to ensure atomicity
	var newGroup *active.Group
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Check if device is already running another session
		existingSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
		if err != nil {
			return err
		}
		if existingSession != nil {
			return ErrDeviceAlreadyActive
		}

		// Determine room ID: priority is manual > planned > default
		finalRoomID := int64(1) // Default fallback

		if roomID != nil && *roomID > 0 {
			// Manual room selection provided
			finalRoomID = *roomID

			// Check if the manually selected room has conflicts
			hasRoomConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, finalRoomID, 0)
			if err != nil {
				return err
			}
			if hasRoomConflict {
				return ErrRoomConflict
			}
		} else {
			// Try to get room from activity configuration
			activityGroup, err := s.activityGroupRepo.FindByID(ctx, activityID)
			if err == nil && activityGroup != nil && activityGroup.PlannedRoomID != nil && *activityGroup.PlannedRoomID > 0 {
				finalRoomID = *activityGroup.PlannedRoomID
			}
			// If no planned room, keep default (1)
		}

		// Create new active group session
		now := time.Now()
		newGroup = &active.Group{
			StartTime:      now,
			LastActivity:   now, // Initialize activity tracking
			TimeoutMinutes: 30,  // Default 30 minutes timeout
			GroupID:        activityID,
			DeviceID:       &deviceID,
			RoomID:         finalRoomID,
		}

		if err := s.groupRepo.Create(ctx, newGroup); err != nil {
			return err
		}

		// Assign the authenticated staff member as supervisor
		supervisor := &active.GroupSupervisor{
			StaffID:   staffID,
			GroupID:   newGroup.ID,
			Role:      "Supervisor",
			StartDate: now,
		}
		if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
			// Log warning but don't fail the session start
			fmt.Printf("Warning: Failed to assign supervisor %d to session %d: %v\n",
				staffID, newGroup.ID, err)
		}

		// Transfer any active visits from recent ended sessions on the same device
		transferredCount, err := s.visitRepo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, deviceID)
		if err != nil {
			return err
		}

		// Log the transfer for debugging
		if transferredCount > 0 {
			// Using fmt.Printf for now since we don't have a logger instance here
			// In production, you might want to use a proper logger
			fmt.Printf("Transferred %d active visits to new session %d\n", transferredCount, newGroup.ID)
		}

		return nil
	})

	if err != nil {
		return nil, &ActiveError{Op: "StartActivitySession", Err: err}
	}

	// Broadcast SSE event (fire-and-forget, outside transaction)
	if s.broadcaster != nil && newGroup != nil {
		activeGroupID := fmt.Sprintf("%d", newGroup.ID)
		roomIDStr := fmt.Sprintf("%d", newGroup.RoomID)
		supervisorIDs := []string{fmt.Sprintf("%d", staffID)}

		// Query activity name
		var activityName string
		if activity, err := s.activityGroupRepo.FindByID(ctx, newGroup.GroupID); err == nil && activity != nil {
			activityName = activity.Name
		}

		// Query room name
		var roomName string
		if room, err := s.roomRepo.FindByID(ctx, newGroup.RoomID); err == nil && room != nil {
			roomName = room.Name
		}

		event := realtime.NewEvent(
			realtime.EventActivityStart,
			activeGroupID,
			realtime.EventData{
				ActivityName:  &activityName,
				RoomID:        &roomIDStr,
				RoomName:      &roomName,
				SupervisorIDs: &supervisorIDs,
			},
		)

		if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
			if logging.Logger != nil {
				logging.Logger.WithFields(map[string]interface{}{
					"error":           err.Error(),
					"event_type":      "activity_start",
					"active_group_id": activeGroupID,
					"activity_name":   activityName,
				}).Error("SSE broadcast failed")
			}
		}
	}

	return newGroup, nil
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
	// Validate supervisor IDs
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	// Check for conflicts
	conflictInfo, err := s.CheckActivityConflict(ctx, activityID, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "StartActivitySessionWithSupervisors", Err: err}
	}

	if conflictInfo.HasConflict {
		return nil, &ActiveError{Op: "StartActivitySessionWithSupervisors", Err: ErrSessionConflict}
	}

	// Use transaction to ensure atomicity
	var newGroup *active.Group
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Check if device is already running another session
		existingSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
		if err != nil {
			return err
		}
		if existingSession != nil {
			return ErrDeviceAlreadyActive
		}

		// Determine room ID: priority is manual > planned > default
		finalRoomID := int64(1) // Default fallback

		if roomID != nil && *roomID > 0 {
			// Manual room selection provided
			finalRoomID = *roomID

			// Check if the manually selected room has conflicts
			hasRoomConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, finalRoomID, 0)
			if err != nil {
				return err
			}
			if hasRoomConflict {
				return ErrRoomConflict
			}
		} else {
			// Try to get room from activity configuration
			activityGroup, err := s.activityGroupRepo.FindByID(ctx, activityID)
			if err == nil && activityGroup != nil && activityGroup.PlannedRoomID != nil && *activityGroup.PlannedRoomID > 0 {
				finalRoomID = *activityGroup.PlannedRoomID
			}
			// If no planned room, keep default (1)
		}

		// Create new active group session
		now := time.Now()
		newGroup = &active.Group{
			StartTime:      now,
			LastActivity:   now, // Initialize activity tracking
			TimeoutMinutes: 30,  // Default 30 minutes timeout
			GroupID:        activityID,
			DeviceID:       &deviceID,
			RoomID:         finalRoomID,
		}

		if err := s.groupRepo.Create(ctx, newGroup); err != nil {
			return err
		}

		// Create supervisors - deduplicate IDs first
		fmt.Printf("DEBUG: Received supervisor IDs: %v\n", supervisorIDs)
		for i, id := range supervisorIDs {
			fmt.Printf("DEBUG: supervisorIDs[%d] = %d\n", i, id)
		}
		uniqueSupervisors := make(map[int64]bool)
		for _, id := range supervisorIDs {
			uniqueSupervisors[id] = true
		}

		// Assign each supervisor
		fmt.Printf("DEBUG: Unique supervisors map: %v\n", uniqueSupervisors)
		for staffID := range uniqueSupervisors {
			fmt.Printf("DEBUG: Creating supervisor for staff ID: %d\n", staffID)
			supervisor := &active.GroupSupervisor{
				StaffID:   staffID,
				GroupID:   newGroup.ID,
				Role:      "supervisor",
				StartDate: now,
			}
			if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
				// Log warning but don't fail the session start
				fmt.Printf("Warning: Failed to assign supervisor %d to session %d: %v\n",
					staffID, newGroup.ID, err)
			}
		}

		// Transfer any active visits from recent ended sessions on the same device
		transferredCount, err := s.visitRepo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, deviceID)
		if err != nil {
			return err
		}

		// Log the transfer for debugging
		if transferredCount > 0 {
			fmt.Printf("Transferred %d active visits to new session %d\n", transferredCount, newGroup.ID)
		}

		return nil
	})

	if err != nil {
		return nil, &ActiveError{Op: "StartActivitySessionWithSupervisors", Err: err}
	}

	// Broadcast SSE event (fire-and-forget, outside transaction)
	if s.broadcaster != nil && newGroup != nil {
		activeGroupID := fmt.Sprintf("%d", newGroup.ID)
		roomIDStr := fmt.Sprintf("%d", newGroup.RoomID)

		// Convert supervisor IDs to strings
		supervisorIDStrs := make([]string, len(supervisorIDs))
		for i, id := range supervisorIDs {
			supervisorIDStrs[i] = fmt.Sprintf("%d", id)
		}

		// Query activity name
		var activityName string
		if activity, err := s.activityGroupRepo.FindByID(ctx, newGroup.GroupID); err == nil && activity != nil {
			activityName = activity.Name
		}

		// Query room name
		var roomName string
		if room, err := s.roomRepo.FindByID(ctx, newGroup.RoomID); err == nil && room != nil {
			roomName = room.Name
		}

		event := realtime.NewEvent(
			realtime.EventActivityStart,
			activeGroupID,
			realtime.EventData{
				ActivityName:  &activityName,
				RoomID:        &roomIDStr,
				RoomName:      &roomName,
				SupervisorIDs: &supervisorIDStrs,
			},
		)

		if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
			if logging.Logger != nil {
				logging.Logger.WithFields(map[string]interface{}{
					"error":           err.Error(),
					"event_type":      "activity_start",
					"active_group_id": activeGroupID,
					"activity_name":   activityName,
				}).Error("SSE broadcast failed")
			}
		}
	}

	return newGroup, nil
}

// ForceStartActivitySession starts an activity session with override capability
func (s *service) ForceStartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	// Use transaction to handle conflicts and cleanup
	var newGroup *active.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// End any existing session for this device
		existingDeviceSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
		if err != nil {
			return err
		}
		if existingDeviceSession != nil {
			if err := s.groupRepo.EndSession(ctx, existingDeviceSession.ID); err != nil {
				return err
			}
		}

		// Determine room ID: priority is manual > planned > default
		finalRoomID := int64(1) // Default fallback

		if roomID != nil && *roomID > 0 {
			// Manual room selection provided
			finalRoomID = *roomID
			// Note: For force start, we don't check room conflicts as we're overriding
		} else {
			// Try to get room from activity configuration
			activityGroup, err := s.activityGroupRepo.FindByID(ctx, activityID)
			if err == nil && activityGroup != nil && activityGroup.PlannedRoomID != nil && *activityGroup.PlannedRoomID > 0 {
				finalRoomID = *activityGroup.PlannedRoomID
			}
			// If no planned room, keep default (1)
		}

		// Create new active group session
		now := time.Now()
		newGroup = &active.Group{
			StartTime:      now,
			LastActivity:   now, // Initialize activity tracking
			TimeoutMinutes: 30,  // Default 30 minutes timeout
			GroupID:        activityID,
			DeviceID:       &deviceID,
			RoomID:         finalRoomID,
		}

		if err := s.groupRepo.Create(ctx, newGroup); err != nil {
			return err
		}

		// Assign the authenticated staff member as supervisor
		supervisor := &active.GroupSupervisor{
			StaffID:   staffID,
			GroupID:   newGroup.ID,
			Role:      "Supervisor",
			StartDate: now,
		}
		if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
			// Log warning but don't fail the session start
			fmt.Printf("Warning: Failed to assign supervisor %d to session %d: %v\n",
				staffID, newGroup.ID, err)
		}

		// Transfer any active visits from recent ended sessions on the same device
		transferredCount, err := s.visitRepo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, deviceID)
		if err != nil {
			return err
		}

		// Log the transfer for debugging
		if transferredCount > 0 {
			fmt.Printf("Transferred %d active visits to new session %d (force start)\n", transferredCount, newGroup.ID)
		}

		return nil
	})

	if err != nil {
		return nil, &ActiveError{Op: "ForceStartActivitySession", Err: err}
	}

	return newGroup, nil
}

// ForceStartActivitySessionWithSupervisors starts an activity session with multiple supervisors and override capability
func (s *service) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	// Debug logging
	fmt.Printf("ForceStartActivitySessionWithSupervisors called with supervisorIDs: %v (len=%d)\n", supervisorIDs, len(supervisorIDs))

	// Validate supervisor IDs
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	// Use transaction to handle conflicts and cleanup
	var newGroup *active.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// End any existing session for this device
		existingDeviceSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
		if err != nil {
			return err
		}

		if existingDeviceSession != nil {
			// End the existing session
			if err := s.EndActivitySession(ctx, existingDeviceSession.ID); err != nil {
				return err
			}
		}

		// Determine room ID: priority is manual > planned > default
		finalRoomID := int64(1) // Default fallback

		if roomID != nil && *roomID > 0 {
			// Manual room selection provided
			finalRoomID = *roomID

			// Check if the manually selected room has conflicts
			hasRoomConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, finalRoomID, 0)
			if err != nil {
				return err
			}
			if hasRoomConflict {
				// In force mode, we still override room conflicts
				fmt.Printf("Warning: Overriding room conflict for room %d\n", finalRoomID)
			}
		} else {
			// Try to get room from activity configuration
			activityGroup, err := s.activityGroupRepo.FindByID(ctx, activityID)
			if err == nil && activityGroup != nil && activityGroup.PlannedRoomID != nil && *activityGroup.PlannedRoomID > 0 {
				finalRoomID = *activityGroup.PlannedRoomID
			}
			// If no planned room, keep default (1)
		}

		// Create new active group session
		now := time.Now()
		newGroup = &active.Group{
			StartTime:      now,
			LastActivity:   now, // Initialize activity tracking
			TimeoutMinutes: 30,  // Default 30 minutes timeout
			GroupID:        activityID,
			DeviceID:       &deviceID,
			RoomID:         finalRoomID,
		}

		if err := s.groupRepo.Create(ctx, newGroup); err != nil {
			return err
		}

		// Create supervisors - deduplicate IDs first
		fmt.Printf("DEBUG: Received supervisor IDs: %v\n", supervisorIDs)
		for i, id := range supervisorIDs {
			fmt.Printf("DEBUG: supervisorIDs[%d] = %d\n", i, id)
		}
		uniqueSupervisors := make(map[int64]bool)
		for _, id := range supervisorIDs {
			uniqueSupervisors[id] = true
		}

		// Assign each supervisor
		fmt.Printf("DEBUG: Unique supervisors map: %v\n", uniqueSupervisors)
		for staffID := range uniqueSupervisors {
			fmt.Printf("DEBUG: Creating supervisor for staff ID: %d\n", staffID)
			supervisor := &active.GroupSupervisor{
				StaffID:   staffID,
				GroupID:   newGroup.ID,
				Role:      "supervisor",
				StartDate: now,
			}
			if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
				// Log warning but don't fail the session start
				fmt.Printf("Warning: Failed to assign supervisor %d to session %d: %v\n",
					staffID, newGroup.ID, err)
			}
		}

		// Transfer any active visits from recent ended sessions on the same device
		transferredCount, err := s.visitRepo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, deviceID)
		if err != nil {
			return err
		}

		// Log the transfer for debugging
		if transferredCount > 0 {
			fmt.Printf("Transferred %d active visits to new session %d\n", transferredCount, newGroup.ID)
		}

		return nil
	})

	if err != nil {
		return nil, &ActiveError{Op: "ForceStartActivitySessionWithSupervisors", Err: err}
	}

	return newGroup, nil
}

// UpdateActiveGroupSupervisors replaces all supervisors for an active group
func (s *service) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*active.Group, error) {
	// Validate the active group exists and is active
	activeGroup, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: ErrActiveGroupNotFound}
	}

	if !activeGroup.IsActive() {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: fmt.Errorf("cannot update supervisors for an ended session")}
	}

	// Validate supervisor IDs
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	// Deduplicate supervisor IDs
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}

	// Use transaction for atomic update
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get current supervisors
		currentSupervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, activeGroupID)
		if err != nil {
			return err
		}

		// End all current supervisors (soft delete by setting end_date)
		now := time.Now()
		for _, supervisor := range currentSupervisors {
			if supervisor.EndDate == nil {
				supervisor.EndDate = &now
				if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
					return err
				}
			}
		}

		// Create new supervisor records
		for supervisorID := range uniqueSupervisors {
			// Check if this supervisor already exists (even if ended)
			// to avoid unique constraint violation
			existingFound := false
			for _, existing := range currentSupervisors {
				if existing.StaffID == supervisorID && existing.Role == "supervisor" {
					// Reactivate if it was ended
					if existing.EndDate != nil {
						existing.EndDate = nil
						existing.StartDate = now
						if err := s.supervisorRepo.Update(ctx, existing); err != nil {
							return err
						}
						existingFound = true
						break
					}
				}
			}

			// Only create if not found
			if !existingFound {
				supervisor := &active.GroupSupervisor{
					StaffID:   supervisorID,
					GroupID:   activeGroupID,
					Role:      "supervisor",
					StartDate: now,
				}

				if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	// Return the updated group with new supervisors
	updatedGroup, err := s.groupRepo.FindWithSupervisors(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	return updatedGroup, nil
}

// CheckActivityConflict checks for conflicts before starting an activity session
func (s *service) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error) {
	// Only check if device is already running another session
	existingDeviceSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}

	conflictInfo := &ActivityConflictInfo{
		HasConflict: existingDeviceSession != nil,
		CanOverride: true, // Administrative override is always possible
	}

	if existingDeviceSession != nil {
		conflictInfo.ConflictingGroup = existingDeviceSession
		conflictInfo.ConflictMessage = fmt.Sprintf("Device %d is already running another session", deviceID)
		deviceIDStr := fmt.Sprintf("%d", deviceID)
		conflictInfo.ConflictingDevice = &deviceIDStr
	}

	return conflictInfo, nil
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

	// Use transaction to ensure atomic cleanup
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// End all active visits first
		visits, err := s.visitRepo.FindByActiveGroupID(ctx, activeGroupID)
		if err != nil {
			return err
		}

		for _, visit := range visits {
			if visit.IsActive() {
				if err := s.visitRepo.EndVisit(ctx, visit.ID); err != nil {
					return err
				}
			}
		}

		// End the session
		if err := s.groupRepo.EndSession(ctx, activeGroupID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: err}
	}

	// Broadcast SSE event (fire-and-forget, outside transaction)
	if s.broadcaster != nil {
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)

		// Reload group to get final state
		if finalGroup, err := s.groupRepo.FindByID(ctx, activeGroupID); err == nil && finalGroup != nil {
			roomIDStr := fmt.Sprintf("%d", finalGroup.RoomID)

			// Query activity name
			var activityName string
			if activity, err := s.activityGroupRepo.FindByID(ctx, finalGroup.GroupID); err == nil && activity != nil {
				activityName = activity.Name
			}

			// Query room name
			var roomName string
			if room, err := s.roomRepo.FindByID(ctx, finalGroup.RoomID); err == nil && room != nil {
				roomName = room.Name
			}

			event := realtime.NewEvent(
				realtime.EventActivityEnd,
				activeGroupIDStr,
				realtime.EventData{
					ActivityName: &activityName,
					RoomID:       &roomIDStr,
					RoomName:     &roomName,
				},
			)

			if err := s.broadcaster.BroadcastToGroup(activeGroupIDStr, event); err != nil {
				if logging.Logger != nil {
					logging.Logger.WithFields(map[string]interface{}{
						"error":           err.Error(),
						"event_type":      "activity_end",
						"active_group_id": activeGroupIDStr,
						"activity_name":   activityName,
					}).Error("SSE broadcast failed")
				}
			}
		}
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

// ProcessSessionTimeout handles device-triggered session timeout
func (s *service) ProcessSessionTimeout(ctx context.Context, deviceID int64) (*TimeoutResult, error) {
	// Validate device has active session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeout", Err: ErrNoActiveSession}
	}

	// Perform atomic cleanup: end session and checkout all students
	var result *TimeoutResult
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// End all active visits first
		visits, err := s.visitRepo.FindByActiveGroupID(ctx, session.ID)
		if err != nil {
			return err
		}

		studentsCheckedOut := 0
		for _, visit := range visits {
			if visit.IsActive() {
				if err := s.visitRepo.EndVisit(ctx, visit.ID); err != nil {
					return err
				}
				studentsCheckedOut++
			}
		}

		// End the session
		if err := s.groupRepo.EndSession(ctx, session.ID); err != nil {
			return err
		}

		result = &TimeoutResult{
			SessionID:          session.ID,
			ActivityID:         session.GroupID,
			StudentsCheckedOut: studentsCheckedOut,
			TimeoutAt:          time.Now(),
		}
		return nil
	})

	return result, err
}

// UpdateSessionActivity updates the last activity timestamp for a session
func (s *service) UpdateSessionActivity(ctx context.Context, activeGroupID int64) error {
	// Get the current session to validate it exists and is active
	session, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "UpdateSessionActivity", Err: err}
	}

	if session == nil {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupNotFound}
	}

	if !session.IsActive() {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupAlreadyEnded}
	}

	// Update last activity timestamp
	return s.groupRepo.UpdateLastActivity(ctx, activeGroupID, time.Now())
}

// ValidateSessionTimeout validates if a timeout request is valid
func (s *service) ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error {
	// Validate device has active session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return &ActiveError{Op: "ValidateSessionTimeout", Err: err}
	}

	// Validate timeout parameters
	if timeoutMinutes <= 0 || timeoutMinutes > 480 { // Max 8 hours
		return &ActiveError{Op: "ValidateSessionTimeout", Err: fmt.Errorf("invalid timeout minutes: %d", timeoutMinutes)}
	}

	// Check if session is actually timed out based on inactivity
	timeoutDuration := time.Duration(timeoutMinutes) * time.Minute
	inactivityDuration := time.Since(session.LastActivity)

	if inactivityDuration < timeoutDuration {
		return &ActiveError{Op: "ValidateSessionTimeout", Err: fmt.Errorf("session not yet timed out: %v remaining", timeoutDuration-inactivityDuration)}
	}

	return nil
}

// GetSessionTimeoutInfo provides comprehensive timeout information for a device session
func (s *service) GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*SessionTimeoutInfo, error) {
	// Get current session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "GetSessionTimeoutInfo", Err: err}
	}

	// Count active students in the session
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, session.ID)
	if err != nil {
		return nil, &ActiveError{Op: "GetSessionTimeoutInfo", Err: err}
	}

	activeStudentCount := 0
	for _, visit := range visits {
		if visit.IsActive() {
			activeStudentCount++
		}
	}

	info := &SessionTimeoutInfo{
		SessionID:          session.ID,
		ActivityID:         session.GroupID,
		StartTime:          session.StartTime,
		LastActivity:       session.LastActivity,
		TimeoutMinutes:     session.TimeoutMinutes,
		InactivityDuration: session.GetInactivityDuration(),
		TimeUntilTimeout:   session.GetTimeUntilTimeout(),
		IsTimedOut:         session.IsTimedOut(),
		ActiveStudentCount: activeStudentCount,
	}

	return info, nil
}

// CleanupAbandonedSessions cleans up sessions that have been abandoned for longer than the specified duration
func (s *service) CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error) {
	// Find sessions that have been active for longer than the threshold
	cutoffTime := time.Now().Add(-olderThan)

	// This would require a new repository method to find sessions by last activity
	// For now, let's implement a conservative approach
	sessions, err := s.groupRepo.FindActiveSessionsOlderThan(ctx, cutoffTime)
	if err != nil {
		return 0, &ActiveError{Op: "CleanupAbandonedSessions", Err: err}
	}

	cleanedCount := 0
	for _, session := range sessions {
		// Only cleanup sessions that are clearly abandoned (more than 2x timeout threshold)
		if session.GetInactivityDuration() >= 2*session.GetTimeoutDuration() {
			// Use ProcessSessionTimeout to ensure proper cleanup
			_, err := s.ProcessSessionTimeout(ctx, *session.DeviceID)
			if err != nil {
				// Log error but continue with other sessions
				continue
			}
			cleanedCount++
		}
	}

	return cleanedCount, nil
}

// Attendance tracking operations

// GetStudentAttendanceStatus gets today's latest attendance record and determines status
func (s *service) GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error) {
	// Get today's latest attendance record
	attendance, err := s.attendanceRepo.GetStudentCurrentStatus(ctx, studentID)
	if err != nil {
		// If no record found, return not_checked_in status
		return &AttendanceStatus{
			StudentID: studentID,
			Status:    "not_checked_in",
			Date:      time.Now().Truncate(24 * time.Hour),
		}, nil
	}

	// Determine status based on CheckOutTime
	status := "checked_in"
	if attendance.CheckOutTime != nil {
		status = "checked_out"
	}

	result := &AttendanceStatus{
		StudentID:    studentID,
		Status:       status,
		Date:         attendance.Date,
		CheckInTime:  &attendance.CheckInTime,
		CheckOutTime: attendance.CheckOutTime,
	}

	// Load staff names for checked_in_by
	if attendance.CheckedInBy > 0 {
		staff, err := s.staffRepo.FindByID(ctx, attendance.CheckedInBy)
		if err == nil && staff != nil {
			person, err := s.usersService.Get(ctx, staff.PersonID)
			if err == nil && person != nil {
				result.CheckedInBy = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
			}
		}
	}

	// Load staff names for checked_out_by
	if attendance.CheckedOutBy != nil && *attendance.CheckedOutBy > 0 {
		staff, err := s.staffRepo.FindByID(ctx, *attendance.CheckedOutBy)
		if err == nil && staff != nil {
			person, err := s.usersService.Get(ctx, staff.PersonID)
			if err == nil && person != nil {
				result.CheckedOutBy = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
			}
		}
	}

	return result, nil
}

// ToggleStudentAttendance toggles the attendance state (check-in or check-out)
func (s *service) ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64) (*AttendanceResult, error) {
	// Check if this is an IoT device request
	isIoTDevice := device.IsIoTDeviceRequest(ctx)

	if !isIoTDevice {
		// Web/manual flow - check teacher access
		hasAccess, err := s.CheckTeacherStudentAccess(ctx, staffID, studentID)
		if err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
		}
		if !hasAccess {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("teacher does not have access to this student")}
		}
	} else {
		// IoT device flow - get supervisor from device's active group
		supervisorStaffID, err := s.getDeviceSupervisorID(ctx, deviceID)
		if err != nil {
			return nil, &ActiveError{
				Op:  "ToggleStudentAttendance",
				Err: fmt.Errorf("device must have an active group with supervisors: %w", err),
			}
		}
		staffID = supervisorStaffID
	}

	// Get current status
	currentStatus, err := s.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	if currentStatus.Status == "not_checked_in" || currentStatus.Status == "checked_out" {
		// Create new attendance record with check_in_time
		attendance := &active.Attendance{
			StudentID:   studentID,
			Date:        today,
			CheckInTime: now,
			CheckedInBy: staffID,
			DeviceID:    deviceID,
		}

		if err := s.attendanceRepo.Create(ctx, attendance); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
		}

		return &AttendanceResult{
			Action:       "checked_in",
			AttendanceID: attendance.ID,
			StudentID:    studentID,
			Timestamp:    now,
		}, nil
	} else {
		// Student is currently checked in, so check them out
		// Find today's latest record to update
		attendance, err := s.attendanceRepo.GetStudentCurrentStatus(ctx, studentID)
		if err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
		}

		// Update record with check_out_time and checked_out_by
		attendance.CheckOutTime = &now
		attendance.CheckedOutBy = &staffID

		if err := s.attendanceRepo.Update(ctx, attendance); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
		}

		return &AttendanceResult{
			Action:       "checked_out",
			AttendanceID: attendance.ID,
			StudentID:    studentID,
			Timestamp:    now,
		}, nil
	}
}

// getDeviceSupervisorID retrieves the supervisor staff ID for a device's active group
func (s *service) getDeviceSupervisorID(ctx context.Context, deviceID int64) (int64, error) {
	// Find active group for device
	activeGroup, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		// Handle case where no active group exists for this device
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("no active group assigned to device %d", deviceID)
		}
		return 0, fmt.Errorf("error finding active group for device %d: %w", deviceID, err)
	}

	if activeGroup == nil {
		return 0, fmt.Errorf("no active group assigned to device %d", deviceID)
	}

	// Get supervisors for the active group
	supervisors, err := s.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get supervisors for group %d: %w", activeGroup.ID, err)
	}

	if len(supervisors) == 0 {
		return 0, fmt.Errorf("no supervisors assigned to active group %d", activeGroup.ID)
	}

	// Use first active supervisor
	for _, supervisor := range supervisors {
		if supervisor.IsActive() {
			return supervisor.StaffID, nil
		}
	}

	return 0, fmt.Errorf("no active supervisors found in group %d", activeGroup.ID)
}

// CheckTeacherStudentAccess checks if a teacher has access to mark attendance for a student
func (s *service) CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error) {
	// Get teacher from staff ID
	teacher, err := s.teacherRepo.FindByStaffID(ctx, teacherID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}
	if teacher == nil {
		return false, nil
	}

	// Get teacher's groups via educationService
	teacherGroups, err := s.educationService.GetTeacherGroups(ctx, teacher.ID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}

	// Get student info
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}
	if student == nil || student.GroupID == nil {
		return false, nil
	}

	// Check if student.GroupID is in teacher's groups
	for _, group := range teacherGroups {
		if group.ID == *student.GroupID {
			return true, nil
		}
	}

	return false, nil
}

// EndDailySessions ends all active sessions at the end of the day
func (s *service) EndDailySessions(ctx context.Context) (*DailySessionCleanupResult, error) {
	result := &DailySessionCleanupResult{
		ExecutedAt: time.Now(),
		Success:    true,
		Errors:     make([]string, 0),
	}

	// Get all active groups
	activeGroups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		result.Success = false
		return result, &ActiveError{Op: "EndDailySessions", Err: ErrDatabaseOperation}
	}

	// Process each active group
	for _, group := range activeGroups {
		if !group.IsActive() {
			continue // Skip already ended groups
		}

		// End all visits for this group first
		visits, err := s.visitRepo.FindByActiveGroupID(ctx, group.ID)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get visits for group %d: %v", group.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
			continue
		}

		// End active visits
		for _, visit := range visits {
			if visit.IsActive() {
				visit.EndVisit()
				if err := s.visitRepo.Update(ctx, visit); err != nil {
					errMsg := fmt.Sprintf("Failed to end visit %d: %v", visit.ID, err)
					result.Errors = append(result.Errors, errMsg)
					result.Success = false
				} else {
					result.VisitsEnded++
				}
			}
		}

		// End the group session
		group.EndSession()
		if err := s.groupRepo.Update(ctx, group); err != nil {
			errMsg := fmt.Sprintf("Failed to end group session %d: %v", group.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else {
			result.SessionsEnded++
		}

		// End all supervisor records for this group
		supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, group.ID)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get supervisors for group %d: %v", group.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else {
			for _, supervisor := range supervisors {
				if supervisor.IsActive() {
					now := time.Now()
					supervisor.EndDate = &now
					if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
						errMsg := fmt.Sprintf("Failed to end supervisor %d: %v", supervisor.ID, err)
						result.Errors = append(result.Errors, errMsg)
						result.Success = false
					} else {
						result.SupervisorsEnded++
					}
				}
			}
		}
	}

	return result, nil
}

// CreateScheduledCheckout creates a new scheduled checkout for a student
func (s *service) CreateScheduledCheckout(ctx context.Context, checkout *active.ScheduledCheckout) error {
	// Validate that the scheduled time is in the future
	if checkout.ScheduledFor.Before(time.Now()) {
		return &ActiveError{Op: "CreateScheduledCheckout", Err: fmt.Errorf("scheduled time must be in the future")}
	}

	// Check if student has an existing pending checkout
	existing, err := s.scheduledCheckoutRepo.GetPendingByStudentID(ctx, checkout.StudentID)
	if err != nil {
		return &ActiveError{Op: "CreateScheduledCheckout", Err: err}
	}
	if existing != nil {
		return &ActiveError{Op: "CreateScheduledCheckout", Err: fmt.Errorf("student already has a pending scheduled checkout")}
	}

	// Set default status
	checkout.Status = active.ScheduledCheckoutStatusPending

	// Create the scheduled checkout
	if err := s.scheduledCheckoutRepo.Create(ctx, checkout); err != nil {
		return &ActiveError{Op: "CreateScheduledCheckout", Err: err}
	}

	return nil
}

// GetScheduledCheckout retrieves a scheduled checkout by ID
func (s *service) GetScheduledCheckout(ctx context.Context, id int64) (*active.ScheduledCheckout, error) {
	checkout, err := s.scheduledCheckoutRepo.GetByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetScheduledCheckout", Err: err}
	}
	return checkout, nil
}

// GetPendingScheduledCheckout retrieves the pending scheduled checkout for a student
func (s *service) GetPendingScheduledCheckout(ctx context.Context, studentID int64) (*active.ScheduledCheckout, error) {
	checkout, err := s.scheduledCheckoutRepo.GetPendingByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "GetPendingScheduledCheckout", Err: err}
	}
	return checkout, nil
}

// CancelScheduledCheckout cancels a scheduled checkout
func (s *service) CancelScheduledCheckout(ctx context.Context, id int64, cancelledBy int64) error {
	checkout, err := s.scheduledCheckoutRepo.GetByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "CancelScheduledCheckout", Err: err}
	}

	if checkout.Status != active.ScheduledCheckoutStatusPending {
		return &ActiveError{Op: "CancelScheduledCheckout", Err: fmt.Errorf("can only cancel pending checkouts")}
	}

	now := time.Now()
	checkout.Status = active.ScheduledCheckoutStatusCancelled
	checkout.CancelledAt = &now
	checkout.CancelledBy = &cancelledBy

	if err := s.scheduledCheckoutRepo.Update(ctx, checkout); err != nil {
		return &ActiveError{Op: "CancelScheduledCheckout", Err: err}
	}

	return nil
}

// ProcessDueScheduledCheckouts processes all scheduled checkouts that are due
func (s *service) ProcessDueScheduledCheckouts(ctx context.Context) (*ScheduledCheckoutResult, error) {
	result := &ScheduledCheckoutResult{
		ProcessedAt: time.Now(),
		Success:     true,
		Errors:      make([]string, 0),
	}

	// Get all due checkouts
	dueCheckouts, err := s.scheduledCheckoutRepo.GetDueCheckouts(ctx, time.Now())
	if err != nil {
		result.Success = false
		return result, &ActiveError{Op: "ProcessDueScheduledCheckouts", Err: err}
	}

	// Log how many checkouts are due
	if len(dueCheckouts) > 0 {
		fmt.Printf("Processing %d due scheduled checkouts\n", len(dueCheckouts))
	}

	// Process each checkout
	for _, checkout := range dueCheckouts {
		// End any active visit for the student
		fmt.Printf("Processing checkout ID %d for student %d\n", checkout.ID, checkout.StudentID)
		visit, err := s.visitRepo.GetCurrentByStudentID(ctx, checkout.StudentID)
		if err != nil {
			// Check if it's just "no rows" error - that's expected if student has no active visit
			// Need to check both direct error and wrapped DatabaseError
			isNoRows := errors.Is(err, sql.ErrNoRows)
			if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
				isNoRows = true
			}

			if isNoRows {
				fmt.Printf("No active visit found for student %d (expected if using old system)\n", checkout.StudentID)
				visit = nil // Set to nil and continue processing
			} else {
				// Real error
				errMsg := fmt.Sprintf("Failed to get current visit for student %d: %v", checkout.StudentID, err)
				result.Errors = append(result.Errors, errMsg)
				result.Success = false
				continue
			}
		}

		if visit != nil && visit.IsActive() {
			fmt.Printf("Found active visit %d for student %d, ending it\n", visit.ID, checkout.StudentID)
			visit.EndVisit()
			if err := s.visitRepo.Update(ctx, visit); err != nil {
				errMsg := fmt.Sprintf("Failed to end visit %d for student %d: %v", visit.ID, checkout.StudentID, err)
				result.Errors = append(result.Errors, errMsg)
				result.Success = false
				continue
			}
			result.VisitsEnded++

			// Broadcast SSE event (fire-and-forget, must NEVER block the loop)
			if s.broadcaster != nil {
				activeGroupIDStr := fmt.Sprintf("%d", visit.ActiveGroupID)
				studentIDStr := fmt.Sprintf("%d", visit.StudentID)
				source := "automated"

				// Query student for display data
				var studentName string
				if student, err := s.studentRepo.FindByID(ctx, visit.StudentID); err == nil && student != nil {
					if person, err := s.personRepo.FindByID(ctx, student.PersonID); err == nil && person != nil {
						studentName = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
					}
				}

				event := realtime.NewEvent(
					realtime.EventStudentCheckOut,
					activeGroupIDStr,
					realtime.EventData{
						StudentID:   &studentIDStr,
						StudentName: &studentName,
						Source:      &source,
					},
				)

				// Fire-and-forget: NEVER block on broadcast failure
				_ = s.broadcaster.BroadcastToGroup(activeGroupIDStr, event)
			}
		} else {
			fmt.Printf("No active visit found for student %d\n", checkout.StudentID)
		}

		// Update attendance record
		attendance, err := s.attendanceRepo.GetTodayByStudentID(ctx, checkout.StudentID)
		if err != nil {
			// If there's no attendance record, log but don't fail the entire checkout
			// This can happen if student was marked present using old system
			isNoRows := errors.Is(err, sql.ErrNoRows)
			if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
				isNoRows = true
			}

			if isNoRows {
				fmt.Printf("No attendance record found for student %d, skipping attendance update\n", checkout.StudentID)
			} else {
				errMsg := fmt.Sprintf("Failed to get attendance for student %d: %v", checkout.StudentID, err)
				result.Errors = append(result.Errors, errMsg)
				// Don't mark as failed, still mark checkout as executed
			}
		} else if attendance != nil && attendance.CheckOutTime == nil {
			checkoutTime := checkout.ScheduledFor
			attendance.CheckOutTime = &checkoutTime
			attendance.CheckedOutBy = &checkout.ScheduledBy

			if err := s.attendanceRepo.Update(ctx, attendance); err != nil {
				errMsg := fmt.Sprintf("Failed to update attendance for student %d: %v", checkout.StudentID, err)
				result.Errors = append(result.Errors, errMsg)
				// Don't mark as failed, still mark checkout as executed
			} else {
				result.AttendanceUpdated++
			}
		}

		// Mark scheduled checkout as executed
		fmt.Printf("Marking scheduled checkout %d as executed\n", checkout.ID)
		now := time.Now()
		checkout.Status = active.ScheduledCheckoutStatusExecuted
		checkout.ExecutedAt = &now

		if err := s.scheduledCheckoutRepo.Update(ctx, checkout); err != nil {
			errMsg := fmt.Sprintf("Failed to update scheduled checkout %d: %v", checkout.ID, err)
			fmt.Printf("ERROR: %s\n", errMsg)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
			continue
		}
		result.CheckoutsExecuted++
		fmt.Printf("Successfully processed scheduled checkout %d for student %d\n", checkout.ID, checkout.StudentID)
	}

	return result, nil
}

// GetStudentScheduledCheckouts retrieves all scheduled checkouts for a student
func (s *service) GetStudentScheduledCheckouts(ctx context.Context, studentID int64) ([]*active.ScheduledCheckout, error) {
	checkouts, err := s.scheduledCheckoutRepo.ListByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentScheduledCheckouts", Err: err}
	}
	return checkouts, nil
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

	// Check if staff is already supervising this group
	existingSupervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, groupID)
	if err == nil {
		for _, sup := range existingSupervisors {
			if sup.StaffID == staffID && sup.IsActive() {
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
