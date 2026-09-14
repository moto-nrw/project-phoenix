package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// stubPresenceOperations is the func-field double for the presence
// operations port. A nil field answers with zero values, so a test wires only
// the operations its handler exercises.
type stubPresenceOperations struct {
	startSession               func(context.Context, studentpresence.LiveGroup) (studentpresence.LiveGroup, error)
	reviseSession              func(context.Context, studentpresence.LiveGroup) (studentpresence.LiveGroup, error)
	removeSession              func(context.Context, int64) error
	endSession                 func(context.Context, int64) error
	touchSession               func(context.Context, int64) error
	admitVisit                 func(context.Context, studentpresence.Visit) (studentpresence.Visit, error)
	amendVisit                 func(context.Context, studentpresence.Visit) error
	removeVisit                func(context.Context, int64) error
	endVisit                   func(context.Context, int64) error
	presenceMode               func(context.Context) (string, error)
	assignSupervision          func(context.Context, studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error)
	amendSupervision           func(context.Context, studentpresence.GroupSupervision) error
	removeSupervision          func(context.Context, int64) error
	endSupervision             func(context.Context, int64) error
	claimSupervision           func(context.Context, int64, int64, string) (studentpresence.ClaimedSupervision, error)
	unclaimedSessions          func(context.Context) ([]studentpresence.UnclaimedSession, error)
	createCombination          func(context.Context, studentpresence.CombinedGroup, []int64) (studentpresence.CombinedGroup, error)
	amendCombination           func(context.Context, studentpresence.CombinedGroup) (studentpresence.CombinedGroup, error)
	removeCombination          func(context.Context, int64) error
	closeCombination           func(context.Context, int64) error
	studentAttendanceStatus    func(context.Context, int64) (*studentpresence.AttendanceStatus, error)
	studentsAttendanceStatuses func(context.Context, []int64) (map[int64]*studentpresence.AttendanceStatus, error)
	checkOutStudent            func(context.Context, int64, int64) (studentpresence.CheckoutOutcome, error)
	assignTransitStudents      func(context.Context, []int64, int64, studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error)
	moveStudentsToSession      func(context.Context, []int64, int64, studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error)
	moveStudentsToTransit      func(context.Context, []int64, studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error)
	sessionVisitsWithDisplay   func(context.Context, int64) ([]studentpresence.VisitDisplay, error)
	sessionRooms               func(context.Context, []int64) ([]studentpresence.SessionRoomSummary, error)
	dashboardAnalytics         func(context.Context) (studentpresence.DashboardAnalytics, error)
	crossTenantStudents        func(context.Context, int64) ([]studentpresence.CrossTenantStudent, error)
	trackingIndicators         func(context.Context, []int64, []string) (map[int64][]bool, error)
}

func (s *stubPresenceOperations) StartSession(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	if s.startSession != nil {
		return s.startSession(ctx, group)
	}
	return group, nil
}

func (s *stubPresenceOperations) ReviseSession(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	if s.reviseSession != nil {
		return s.reviseSession(ctx, group)
	}
	return group, nil
}

func (s *stubPresenceOperations) RemoveSession(ctx context.Context, id int64) error {
	if s.removeSession != nil {
		return s.removeSession(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) EndSession(ctx context.Context, id int64) error {
	if s.endSession != nil {
		return s.endSession(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) TouchSession(ctx context.Context, id int64) error {
	if s.touchSession != nil {
		return s.touchSession(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) AdmitVisit(ctx context.Context, visit studentpresence.Visit) (studentpresence.Visit, error) {
	if s.admitVisit != nil {
		return s.admitVisit(ctx, visit)
	}
	return visit, nil
}

func (s *stubPresenceOperations) AmendVisit(ctx context.Context, visit studentpresence.Visit) error {
	if s.amendVisit != nil {
		return s.amendVisit(ctx, visit)
	}
	return nil
}

func (s *stubPresenceOperations) RemoveVisit(ctx context.Context, id int64) error {
	if s.removeVisit != nil {
		return s.removeVisit(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) EndVisit(ctx context.Context, id int64) error {
	if s.endVisit != nil {
		return s.endVisit(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) PresenceMode(ctx context.Context) (string, error) {
	if s.presenceMode != nil {
		return s.presenceMode(ctx)
	}
	return studentpresence.PresenceModeDetailed, nil
}

func (s *stubPresenceOperations) AssignSupervision(ctx context.Context, row studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error) {
	if s.assignSupervision != nil {
		return s.assignSupervision(ctx, row)
	}
	return row, nil
}

func (s *stubPresenceOperations) AmendSupervision(ctx context.Context, row studentpresence.GroupSupervision) error {
	if s.amendSupervision != nil {
		return s.amendSupervision(ctx, row)
	}
	return nil
}

func (s *stubPresenceOperations) RemoveSupervisionRecord(ctx context.Context, id int64) error {
	if s.removeSupervision != nil {
		return s.removeSupervision(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) EndSupervision(ctx context.Context, id int64) error {
	if s.endSupervision != nil {
		return s.endSupervision(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) ClaimSupervision(ctx context.Context, groupID, staffID int64, role string) (studentpresence.ClaimedSupervision, error) {
	if s.claimSupervision != nil {
		return s.claimSupervision(ctx, groupID, staffID, role)
	}
	return studentpresence.ClaimedSupervision{}, nil
}

func (s *stubPresenceOperations) UnclaimedSessions(ctx context.Context) ([]studentpresence.UnclaimedSession, error) {
	if s.unclaimedSessions != nil {
		return s.unclaimedSessions(ctx)
	}
	return nil, nil
}

func (s *stubPresenceOperations) CreateCombination(ctx context.Context, group studentpresence.CombinedGroup, groupIDs []int64) (studentpresence.CombinedGroup, error) {
	if s.createCombination != nil {
		return s.createCombination(ctx, group, groupIDs)
	}
	return group, nil
}

func (s *stubPresenceOperations) AmendCombination(ctx context.Context, group studentpresence.CombinedGroup) (studentpresence.CombinedGroup, error) {
	if s.amendCombination != nil {
		return s.amendCombination(ctx, group)
	}
	return group, nil
}

func (s *stubPresenceOperations) RemoveCombination(ctx context.Context, id int64) error {
	if s.removeCombination != nil {
		return s.removeCombination(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) CloseCombination(ctx context.Context, id int64) error {
	if s.closeCombination != nil {
		return s.closeCombination(ctx, id)
	}
	return nil
}

func (s *stubPresenceOperations) StudentAttendanceStatus(ctx context.Context, studentID int64) (*studentpresence.AttendanceStatus, error) {
	if s.studentAttendanceStatus != nil {
		return s.studentAttendanceStatus(ctx, studentID)
	}
	return nil, nil
}

func (s *stubPresenceOperations) StudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.AttendanceStatus, error) {
	if s.studentsAttendanceStatuses != nil {
		return s.studentsAttendanceStatuses(ctx, studentIDs)
	}
	return nil, nil
}

func (s *stubPresenceOperations) CheckOutStudent(ctx context.Context, studentID, staffID int64) (studentpresence.CheckoutOutcome, error) {
	if s.checkOutStudent != nil {
		return s.checkOutStudent(ctx, studentID, staffID)
	}
	return studentpresence.CheckoutOutcome{}, nil
}

func (s *stubPresenceOperations) AssignTransitStudents(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
	if s.assignTransitStudents != nil {
		return s.assignTransitStudents(ctx, studentIDs, activeGroupID, auth)
	}
	return studentpresence.TransitAssignResult{}, nil
}

func (s *stubPresenceOperations) MoveStudentsToSession(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
	if s.moveStudentsToSession != nil {
		return s.moveStudentsToSession(ctx, studentIDs, activeGroupID, auth)
	}
	return studentpresence.StudentMoveResult{}, nil
}

func (s *stubPresenceOperations) MoveStudentsToTransit(ctx context.Context, studentIDs []int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
	if s.moveStudentsToTransit != nil {
		return s.moveStudentsToTransit(ctx, studentIDs, auth)
	}
	return studentpresence.StudentMoveResult{}, nil
}

func (s *stubPresenceOperations) SessionVisitsWithDisplay(ctx context.Context, groupID int64) ([]studentpresence.VisitDisplay, error) {
	if s.sessionVisitsWithDisplay != nil {
		return s.sessionVisitsWithDisplay(ctx, groupID)
	}
	return nil, nil
}

func (s *stubPresenceOperations) SessionRooms(ctx context.Context, ids []int64) ([]studentpresence.SessionRoomSummary, error) {
	if s.sessionRooms != nil {
		return s.sessionRooms(ctx, ids)
	}
	return nil, nil
}

func (s *stubPresenceOperations) DashboardAnalytics(ctx context.Context) (studentpresence.DashboardAnalytics, error) {
	if s.dashboardAnalytics != nil {
		return s.dashboardAnalytics(ctx)
	}
	return studentpresence.DashboardAnalytics{}, nil
}

func (s *stubPresenceOperations) CrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]studentpresence.CrossTenantStudent, error) {
	if s.crossTenantStudents != nil {
		return s.crossTenantStudents(ctx, hostingTenantID)
	}
	return nil, nil
}

func (s *stubPresenceOperations) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	if s.trackingIndicators != nil {
		return s.trackingIndicators(ctx, studentIDs, labels)
	}
	return map[int64][]bool{}, nil
}

// operationError wraps a presence sentinel the way the retained service does,
// so error-classification tests exercise the unwrap path.
func operationError(op string, err error) error {
	return &presenceError{Op: op, Err: err}
}
